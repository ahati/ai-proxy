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

type AnthropicHandler struct {
	cfg *config.Config
}

func NewAnthropicHandler(cfg *config.Config) gin.HandlerFunc {
	handler := &AnthropicHandler{
		cfg: cfg,
	}
	return handler.Handle
}

func (h *AnthropicHandler) Handle(c *gin.Context) {
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

func (h *AnthropicHandler) readBody(c *gin.Context) ([]byte, error) {
	return io.ReadAll(c.Request.Body)
}

func (h *AnthropicHandler) isStreamingRequest(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	json.Unmarshal(body, &req)
	return req.Stream
}

func (h *AnthropicHandler) proxyAndStream(c *gin.Context, body []byte) {
	apiKey := h.resolveAPIKey(c)
	client := upstream.NewClient(h.cfg.AnthropicUpstreamURL, apiKey)
	defer client.Close()

	req, err := client.BuildRequest(c.Request.Context(), body)
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "Failed to create upstream request")
		return
	}

	client.SetHeaders(req)

	for k, v := range c.Request.Header {
		if strings.HasPrefix(k, "X-") || k == "Anthropic-Version" || k == "Anthropic-Beta" {
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
		h.handleUpstreamError(c, resp)
		return
	}

	h.streamResponse(c, resp.Body)
}

func (h *AnthropicHandler) resolveAPIKey(c *gin.Context) string {
	return h.cfg.AnthropicAPIKey
}

func (h *AnthropicHandler) streamResponse(c *gin.Context, body io.Reader) {
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
		transformer := NewAnthropicToolCallTransformer(recorder)
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
		transformer := NewAnthropicToolCallTransformer(c.Writer)
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

func (h *AnthropicHandler) sendError(c *gin.Context, status int, msg string) {
	logging.ErrorMsg("Anthropic handler error: %s", msg)
	c.JSON(status, AnthropicError{
		Type: "error",
		Error: ErrorDetail{
			Type:    "invalid_request_error",
			Message: msg,
		},
	})
}

func (h *AnthropicHandler) handleUpstreamError(c *gin.Context, resp *http.Response) {
	body, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}
