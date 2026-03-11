package protocols

import (
	"encoding/json"
	"io"
	"net/http"

	"ai-proxy/config"
	"ai-proxy/transform/toolcall"
	"ai-proxy/types"

	"github.com/gin-gonic/gin"
	"github.com/tmaxmax/go-sse"
)

type ProtocolAdapter interface {
	TransformRequest(body []byte) ([]byte, error)
	ValidateRequest(body []byte) error
	CreateTransformer(w io.Writer, base types.StreamChunk) *ToolCallTransformer
	UpstreamURL(cfg *config.Config) string
	UpstreamAPIKey(cfg *config.Config) string
	ForwardHeaders(src, dst http.Header)
	SendError(c *gin.Context, status int, msg string)
	IsStreamingRequest(body []byte) bool
}

type ToolCallTransformer struct {
	parser  *toolcall.Parser
	output  toolcall.EventHandler
	writer  io.Writer
	base    types.StreamChunk
	flusher interface{ Flush() }
}

func NewToolCallTransformer(writer io.Writer, base types.StreamChunk, output toolcall.EventHandler) *ToolCallTransformer {
	tokens := toolcall.DefaultTokenSet()
	t := &ToolCallTransformer{
		output: output,
		writer: writer,
		base:   base,
	}
	t.parser = toolcall.NewParser(tokens, output)
	if f, ok := output.(interface{ Flush() }); ok {
		t.flusher = f
	}
	return t
}

func (t *ToolCallTransformer) Transform(event *sse.Event) {
	if event.Data == "" || event.Data == "[DONE]" {
		if event.Data == "[DONE]" {
			t.writer.Write([]byte("data: [DONE]\n\n"))
		}
		return
	}

	var chunk types.StreamChunk
	if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
		return
	}

	t.base = chunk

	if len(chunk.Choices) > 0 {
		delta := chunk.Choices[0].Delta
		text := delta.Content
		if text == "" {
			text = delta.Reasoning
		}
		if text == "" {
			text = delta.ReasoningContent
		}
		if text != "" {
			t.parser.Feed(text)
		}
		for _, tc := range delta.ToolCalls {
			t.output.OnToolCallStart(tc.ID, tc.Function.Name, tc.Index)
			if tc.Function.Arguments != "" {
				t.output.OnToolCallArgs(tc.Function.Arguments, tc.Index)
			}
			t.output.OnToolCallEnd(tc.Index)
		}
	}
}

func (t *ToolCallTransformer) Flush() {
	t.parser.Flush()
	if t.flusher != nil {
		t.flusher.Flush()
	}
}

func (t *ToolCallTransformer) Close() {
	t.Flush()
}
