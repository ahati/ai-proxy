package downstream

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

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

	resp, err := client.Do(req)
	if err != nil {
		h.sendError(c, http.StatusBadGateway, "Upstream request failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.handleUpstreamError(c, resp)
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
	auth := c.GetHeader("x-api-key")
	if auth == "" {
		auth = c.GetHeader("Authorization")
	}
	if auth == "" {
		return h.cfg.OpenAIUpstreamAPIKey
	}
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return auth
}

func (h *OpenAIToAnthropicHandler) streamResponse(c *gin.Context, body io.Reader) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	var loggingTransformer *LoggingTransformer
	if h.cfg.SSELogDir != "" {
		var err error
		loggingTransformer, err = NewLoggingTransformer(h.cfg.SSELogDir)
		if err != nil {
			logging.ErrorMsg("Failed to create logging transformer: %v", err)
		}
	}
	defer func() {
		if loggingTransformer != nil {
			loggingTransformer.Close()
		}
	}()

	transformer := NewOpenAIToAnthropicTransformer(c.Writer)
	defer transformer.Close()

	c.Stream(func(w io.Writer) bool {
		for ev, err := range sse.Read(body, nil) {
			if err != nil {
				logging.ErrorMsg("SSE read error: %v", err)
				return false
			}
			if loggingTransformer != nil {
				loggingTransformer.Transform(&ev)
			}
			transformer.Transform(&ev)
		}
		return false
	})

	transformer.Flush()
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

func (h *OpenAIToAnthropicHandler) handleUpstreamError(c *gin.Context, resp *http.Response) {
	body, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}
