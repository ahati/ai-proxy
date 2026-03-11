package protocols

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"ai-proxy/config"
	"ai-proxy/logging"
	"ai-proxy/transform/toolcall"
	"ai-proxy/types"

	"github.com/gin-gonic/gin"
)

type AnthropicAdapter struct{}

func NewAnthropicAdapter() *AnthropicAdapter {
	return &AnthropicAdapter{}
}

func (a *AnthropicAdapter) TransformRequest(body []byte) ([]byte, error) {
	return body, nil
}

func (a *AnthropicAdapter) ValidateRequest(body []byte) error {
	if !a.IsStreamingRequest(body) {
		return ErrNonStreamingNotSupported
	}
	return nil
}

func (a *AnthropicAdapter) CreateTransformer(w io.Writer, base types.StreamChunk) *ToolCallTransformer {
	output := toolcall.NewAnthropicOutput(w, toolcall.ContextText, 0)
	return NewToolCallTransformer(w, base, output)
}

func (a *AnthropicAdapter) UpstreamURL(cfg *config.Config) string {
	return cfg.AnthropicUpstreamURL
}

func (a *AnthropicAdapter) UpstreamAPIKey(cfg *config.Config) string {
	return cfg.AnthropicAPIKey
}

func (a *AnthropicAdapter) ForwardHeaders(src, dst http.Header) {
	for k, v := range src {
		if strings.HasPrefix(k, "X-") || k == "Anthropic-Version" || k == "Anthropic-Beta" {
			dst[k] = v
		}
	}
	for _, h := range []string{"Connection", "Keep-Alive", "Upgrade", "TE"} {
		if v := src.Get(h); v != "" {
			dst.Set(h, v)
		}
	}
}

func (a *AnthropicAdapter) SendError(c *gin.Context, status int, msg string) {
	logging.ErrorMsg("Anthropic handler error: %s", msg)
	c.JSON(status, types.Error{
		Type: "error",
		Error: types.ErrorDetail{
			Type:    "invalid_request_error",
			Message: msg,
		},
	})
}

func (a *AnthropicAdapter) IsStreamingRequest(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	json.Unmarshal(body, &req)
	return req.Stream
}
