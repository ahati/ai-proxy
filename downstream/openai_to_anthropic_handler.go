package downstream

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ai-proxy/config"
	"ai-proxy/logging"
	"ai-proxy/upstream"

	"github.com/gin-gonic/gin"
	"github.com/tmaxmax/go-sse"
)

type OpenAIToAnthropicHandler struct {
	cfg *config.Config
}

func NewOpenAIToAnthropicHandler(cfg *config.Config) gin.HandlerFunc {
	handler := &OpenAIToAnthropicHandler{
		cfg: cfg,
	}
	return handler.Handle
}

func (h *OpenAIToAnthropicHandler) Handle(c *gin.Context) {
	body, err := h.readBody(c)
	if err != nil {
		h.sendError(c, http.StatusBadRequest, "Failed to read request body")
		return
	}

	logging.RecordDownstreamRequest(c, body)

	if !h.isStreamingRequest(body) {
		h.sendError(c, http.StatusBadRequest, "Non-streaming requests not supported")
		return
	}

	h.proxyAndStream(c, body)
}

func (h *OpenAIToAnthropicHandler) readBody(c *gin.Context) ([]byte, error) {
	return io.ReadAll(c.Request.Body)
}

func (h *OpenAIToAnthropicHandler) isStreamingRequest(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	json.Unmarshal(body, &req)
	return req.Stream
}

func (h *OpenAIToAnthropicHandler) proxyAndStream(c *gin.Context, body []byte) {
	openaiBody, err := h.transformRequest(body)
	if err != nil {
		h.sendError(c, http.StatusBadRequest, "Failed to transform request")
		return
	}

	apiKey := h.resolveAPIKey(c)
	client := upstream.NewClient(h.cfg.OpenAIUpstreamURL, apiKey)
	defer client.Close()

	req, err := client.BuildRequest(c.Request.Context(), openaiBody)
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "Failed to create upstream request")
		return
	}

	client.SetHeaders(req)

	for k, v := range c.Request.Header {
		if strings.HasPrefix(k, "X-") || k == "Extra" {
			req.Header[k] = v
		}
	}

	for _, h := range []string{"Connection", "Keep-Alive", "Upgrade", "TE"} {
		if v := c.Request.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		h.sendError(c, http.StatusBadGateway, "Upstream request failed")
		return
	}

	if resp.StatusCode != http.StatusOK {
		h.handleUpstreamErrorAndCapture(c, resp)
		return
	}

	h.streamResponse(c, resp.Body)
}

func (h *OpenAIToAnthropicHandler) transformRequest(anthropicBody []byte) ([]byte, error) {
	var anthReq AnthropicToOpenAIRequest
	if err := json.Unmarshal(anthropicBody, &anthReq); err != nil {
		return nil, err
	}

	openReq := OpenAIRequest{
		Model:       anthReq.Model,
		MaxTokens:   anthReq.MaxTokens,
		Stream:      anthReq.Stream,
		Temperature: anthReq.Temperature,
		TopP:        anthReq.TopP,
	}

	if system := h.extractSystemMessage(anthReq.System); system != "" {
		openReq.System = system
	}

	for _, msg := range anthReq.Messages {
		openMsg := h.convertMessage(msg)
		openReq.Messages = append(openReq.Messages, openMsg)
	}

	for _, tool := range anthReq.Tools {
		openTool := h.convertTool(tool)
		openReq.Tools = append(openReq.Tools, openTool)
	}

	return json.Marshal(openReq)
}

func (h *OpenAIToAnthropicHandler) extractSystemMessage(system interface{}) string {
	if system == nil {
		return ""
	}
	if s, ok := system.(string); ok {
		return s
	}
	if arr, ok := system.([]interface{}); ok {
		var content strings.Builder
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				if text, ok := m["text"].(string); ok {
					content.WriteString(text)
				}
			}
		}
		return content.String()
	}
	return ""
}

func (h *OpenAIToAnthropicHandler) convertMessage(anthMsg AnthropicMessageInput) OpenAIMessage {
	openMsg := OpenAIMessage{
		Role: anthMsg.Role,
	}

	switch content := anthMsg.Content.(type) {
	case string:
		openMsg.Content = content
	case []interface{}:
		var textContent strings.Builder
		var toolCalls []OpenAIToolCall
		var toolCallID string

		for _, item := range content {
			if m, ok := item.(map[string]interface{}); ok {
				switch m["type"] {
				case "text":
					if text, ok := m["text"].(string); ok {
						if textContent.Len() > 0 {
							textContent.WriteString("\n")
						}
						textContent.WriteString(text)
					}
				case "tool_use":
					if id, ok := m["id"].(string); ok {
						if name, ok := m["name"].(string); ok {
							input, _ := json.Marshal(m["input"])
							toolCalls = append(toolCalls, OpenAIToolCall{
								ID:   id,
								Type: "function",
								Function: OpenAIToolCallFunction{
									Name:      name,
									Arguments: string(input),
								},
							})
						}
					}
				case "tool_result":
					if id, ok := m["tool_use_id"].(string); ok {
						toolCallID = id
					}
				}
			}
		}

		if textContent.Len() > 0 {
			openMsg.Content = textContent.String()
		}
		if len(toolCalls) > 0 {
			openMsg.ToolCalls = toolCalls
		}
		if toolCallID != "" {
			openMsg.ToolCallID = toolCallID
		}
	}

	return openMsg
}

func (h *OpenAIToAnthropicHandler) convertTool(anthTool AnthropicToolDefinition) OpenAITool {
	return OpenAITool{
		Type: "function",
		Function: OpenAIFunction{
			Name:        anthTool.Name,
			Description: anthTool.Description,
			Parameters:  anthTool.InputSchema,
		},
	}
}

func (h *OpenAIToAnthropicHandler) resolveAPIKey(c *gin.Context) string {
	return h.cfg.OpenAIUpstreamAPIKey
}

func (h *OpenAIToAnthropicHandler) streamResponse(c *gin.Context, body io.Reader) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	var cc *logging.CaptureContext
	if cc = logging.GetCaptureContext(c.Request.Context()); cc != nil {
		startTime := cc.StartTime

		downstreamCapture := logging.NewCaptureWriter(startTime)
		upstreamCapture := logging.NewCaptureWriter(startTime)

		recorder := NewResponseRecorder(c.Writer, downstreamCapture)
		transformer := NewOpenAIToAnthropicTransformer(recorder)
		defer transformer.Close()

		c.Stream(func(w io.Writer) bool {
			for ev, err := range sse.Read(body, nil) {
				if err != nil {
					logging.ErrorMsg("SSE read error: %v", err)
					return false
				}
				if ev.Data != "" {
					upstreamCapture.RecordChunk(ev.Type, []byte(ev.Data))
				}
				transformer.Transform(&ev)
			}
			return false
		})

		transformer.Flush()

		cc.Recorder.DownstreamResponse = &logging.SSEResponseCapture{
			Chunks: downstreamCapture.Chunks(),
		}
		if cc.Recorder.UpstreamResponse != nil {
			cc.Recorder.UpstreamResponse.Chunks = upstreamCapture.Chunks()
		}
		if !cc.IDExtracted {
			for _, chunk := range downstreamCapture.Chunks() {
				if id := logging.ExtractRequestIDFromSSEChunk(chunk.Data); id != "" {
					cc.SetRequestID(id)
					break
				}
			}
		}
	} else {
		transformer := NewOpenAIToAnthropicTransformer(c.Writer)
		defer transformer.Close()

		c.Stream(func(w io.Writer) bool {
			for ev, err := range sse.Read(body, nil) {
				if err != nil {
					logging.ErrorMsg("SSE read error: %v", err)
					return false
				}
				transformer.Transform(&ev)
			}
			return false
		})

		transformer.Flush()
	}
}

func (h *OpenAIToAnthropicHandler) sendError(c *gin.Context, status int, msg string) {
	logging.ErrorMsg("OpenAI to Anthropic handler error: %s", msg)
	c.JSON(status, AnthropicError{
		Type: "error",
		Error: ErrorDetail{
			Type:    "invalid_request_error",
			Message: msg,
		},
	})
}

func (h *OpenAIToAnthropicHandler) handleUpstreamErrorAndCapture(c *gin.Context, resp *http.Response) {
	body, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)

	var cc *logging.CaptureContext
	if cc = logging.GetCaptureContext(c.Request.Context()); cc != nil {
		cc.Recorder.DownstreamResponse = &logging.SSEResponseCapture{
			StatusCode: resp.StatusCode,
			Headers:    logging.SanitizeHeaders(resp.Header),
			Chunks: []logging.SSEChunk{
				{
					OffsetMS: logging.OffsetMS(cc.StartTime),
					Event:    "error",
					Data:     body,
				},
			},
		}

		if !cc.IDExtracted {
			if id := logging.ExtractRequestIDFromSSEChunk(body); id != "" {
				cc.SetRequestID(id)
			} else {
				fallbackID := fmt.Sprintf("err_%s_%d", h.generateErrorID(body), time.Now().UnixMilli())
				cc.SetRequestID(fallbackID)
			}
		}
	}
}

func (h *OpenAIToAnthropicHandler) generateErrorID(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:8])
}
