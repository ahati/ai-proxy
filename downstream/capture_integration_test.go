package downstream

import (
	"bytes"
	"testing"

	"ai-proxy/logging"

	"github.com/tmaxmax/go-sse"
)

func TestUpstreamVsDownstreamCapture(t *testing.T) {
	upstreamEvents := []string{
		`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":"Some text <|tool_calls_section_begin|>"}}]}`,
		`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":"<|tool_call_begin|>bash<|tool_call_argument_begin|>{\"command\":\"ls\"}<|tool_call_end|>"}}]}`,
		`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":"<|tool_calls_section_end|> done"}}]}`,
	}

	upstreamCapture := logging.NewCaptureWriter(logging.CaptureContext{}.StartTime)
	downstreamCapture := logging.NewCaptureWriter(logging.CaptureContext{}.StartTime)

	var downstreamOutput bytes.Buffer
	transformer := NewToolCallTransformer(&downstreamOutput)

	for _, eventData := range upstreamEvents {
		ev := sse.Event{Type: "message", Data: eventData}

		upstreamCapture.RecordChunk(ev.Type, []byte(ev.Data))
		transformer.Transform(&ev)
		downstreamCapture.RecordChunk(ev.Type, downstreamOutput.Bytes())
		downstreamOutput.Reset()
	}

	upstreamChunks := upstreamCapture.Chunks()
	downstreamChunks := downstreamCapture.Chunks()

	t.Logf("=== Upstream chunks (%d) ===", len(upstreamChunks))
	for i, chunk := range upstreamChunks {
		t.Logf("Chunk %d: %s", i, string(chunk.Data))
	}

	t.Logf("\n=== Downstream chunks (%d) ===", len(downstreamChunks))
	for i, chunk := range downstreamChunks {
		t.Logf("Chunk %d: %s", i, string(chunk.Data))
	}

	for i, chunk := range upstreamChunks {
		data := string(chunk.Data)
		if !containsToolCallTokensInString(data) {
			t.Errorf("Upstream chunk %d should contain tool call tokens: %s", i, data)
		}
	}
}

func containsToolCallTokensInString(s string) bool {
	tokens := []string{
		"<|tool_calls_section_begin|>",
		"<|tool_call_begin|>",
		"<|tool_call_argument_begin|>",
		"<|tool_call_end|>",
		"<|tool_calls_section_end|>",
	}
	for _, tok := range tokens {
		if len(s) >= len(tok) {
			for i := 0; i <= len(s)-len(tok); i++ {
				if s[i:i+len(tok)] == tok {
					return true
				}
			}
		}
	}
	return false
}
