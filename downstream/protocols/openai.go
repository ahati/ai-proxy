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

type OpenAIAdapter struct{}

func NewOpenAIAdapter() *OpenAIAdapter {
	return &OpenAIAdapter{}
}

func (a *OpenAIAdapter) TransformRequest(body []byte) ([]byte, error) {
	return body, nil
}

func (a *OpenAIAdapter) ValidateRequest(body []byte) error {
	if !a.IsStreamingRequest(body) {
		return ErrNonStreamingNotSupported
	}
	return nil
}

func (a *OpenAIAdapter) CreateTransformer(w io.Writer, base types.StreamChunk) *ToolCallTransformer {
	output := toolcall.NewOpenAIOutput(w, base)
	return NewToolCallTransformer(w, base, output)
}

func (a *OpenAIAdapter) UpstreamURL(cfg *config.Config) string {
	return cfg.OpenAIUpstreamURL
}

func (a *OpenAIAdapter) UpstreamAPIKey(cfg *config.Config) string {
	return cfg.OpenAIUpstreamAPIKey
}

func (a *OpenAIAdapter) ForwardHeaders(src, dst http.Header) {
	for k, v := range src {
		if strings.HasPrefix(k, "X-") || k == "Extra" {
			dst[k] = v
		}
	}
	for _, h := range []string{"Connection", "Keep-Alive", "Upgrade", "TE"} {
		if v := src.Get(h); v != "" {
			dst.Set(h, v)
		}
	}
}

func (a *OpenAIAdapter) SendError(c *gin.Context, status int, msg string) {
	logging.ErrorMsg("OpenAI handler error: %s", msg)
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": msg,
			"type":    "invalid_request_error",
			"code":    "",
		},
	})
}

func (a *OpenAIAdapter) IsStreamingRequest(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	json.Unmarshal(body, &req)
	return req.Stream
}

var ErrNonStreamingNotSupported = &ProtocolError{Message: "Non-streaming requests not supported"}

type ProtocolError struct {
	Message string
}

func (e *ProtocolError) Error() string {
	return e.Message
}
