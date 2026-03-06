package downstream

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-proxy/logging"

	"github.com/gin-gonic/gin"
	"github.com/tmaxmax/go-sse"
)

func TestMalformedToolCalls_Capture(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name   string
		events []string
	}{
		{
			name: "incomplete_tool_call_section",
			events: []string{
				`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":"<|tool_calls_section_begin|>"}}]}`,
				`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":"<|tool_call_begin|>bash"}}]}`,
				`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":" some broken text"}}]}`,
			},
		},
		{
			name: "malformed_argument_json",
			events: []string{
				`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":"<|tool_calls_section_begin|><|tool_call_begin|>bash<|tool_call_argument_begin|>{broken json}"}}]}`,
			},
		},
		{
			name: "tokens_without_proper_structure",
			events: []string{
				`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":"Random text <|tool_call_begin|> incomplete"}}]}`,
			},
		},
		{
			name: "missing_end_tokens",
			events: []string{
				`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":"<|tool_calls_section_begin|><|tool_call_begin|>bash<|tool_call_argument_begin|>{\"cmd\":\"ls\"}"}}]}`,
				`{"id":"chatcmpl-1","choices":[{"delta":{},"finish_reason":"stop"}]}`,
			},
		},
		{
			name: "reasoning_content_field",
			events: []string{
				`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning_content":"<|tool_calls_section_begin|><|tool_call_begin|>bash<|tool_call_argument_begin|>{\"x\":1}<|tool_call_end|>"}}]}`,
			},
		},
		{
			name: "empty_chunks",
			events: []string{
				`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":"text"}}]}`,
				`{}`,
				`{"id":"chatcmpl-1","choices":[]}`,
				`{"id":"chatcmpl-1","choices":[{"delta":{}}]}`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var upstreamSSE strings.Builder
			for _, ev := range tc.events {
				upstreamSSE.WriteString("data: " + ev + "\n\n")
			}
			upstreamBody := strings.NewReader(upstreamSSE.String())

			recorder := logging.RequestRecorder{
				StartedAt: time.Now(),
				Method:    "POST",
				Path:      "/v1/chat/completions",
			}

			cc := &logging.CaptureContext{
				StartTime: time.Now(),
				Recorder:  &recorder,
			}

			cc.Recorder.UpstreamResponse = &logging.SSEResponseCapture{
				StatusCode: 200,
				Headers:    map[string]string{"content-type": "text/event-stream"},
				Chunks:     []logging.SSEChunk{},
			}

			downstreamCapture := logging.NewCaptureWriter(cc.StartTime)
			upstreamCapture := logging.NewCaptureWriter(cc.StartTime)

			w := httptest.NewRecorder()
			responseWriter := NewResponseRecorder(w, downstreamCapture)
			transformer := NewToolCallTransformer(responseWriter)

			eventCount := 0
			for ev, err := range sse.Read(upstreamBody, nil) {
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Fatalf("SSE read error: %v", err)
				}

				eventCount++
				t.Logf("Event %d: %s", eventCount, ev.Data)

				upstreamCapture.RecordChunk(ev.Type, []byte(ev.Data))
				transformer.Transform(&ev)
			}

			transformer.Close()

			cc.Recorder.DownstreamResponse = &logging.SSEResponseCapture{
				Chunks: downstreamCapture.Chunks(),
			}
			if cc.Recorder.UpstreamResponse != nil {
				cc.Recorder.UpstreamResponse.Chunks = upstreamCapture.Chunks()
			}

			t.Log("\n=== UPSTREAM CHUNKS ===")
			for i, chunk := range cc.Recorder.UpstreamResponse.Chunks {
				data := string(chunk.Data)
				t.Logf("  Chunk %d: %s", i, data)

				if strings.Contains(data, "<|tool_call") {
					t.Logf("    ✓ Contains raw tool_call tokens")
				}
			}

			t.Log("\n=== DOWNSTREAM CHUNKS ===")
			for i, chunk := range cc.Recorder.DownstreamResponse.Chunks {
				data := string(chunk.Data)
				t.Logf("  Chunk %d: %s", i, data)

				var parsed map[string]interface{}
				json.Unmarshal(chunk.Data, &parsed)
				if choices, ok := parsed["choices"].([]interface{}); ok && len(choices) > 0 {
					if choice, ok := choices[0].(map[string]interface{}); ok {
						if delta, ok := choice["delta"].(map[string]interface{}); ok {
							if toolCalls, ok := delta["tool_calls"].([]interface{}); ok {
								t.Logf("    ✓ Transformed tool_calls: %d", len(toolCalls))
							}
						}
					}
				}
			}

			upstreamCount := len(cc.Recorder.UpstreamResponse.Chunks)
			inputCount := len(tc.events)

			t.Logf("\nEvents: input=%d, upstream_captured=%d", inputCount, upstreamCount)

			if upstreamCount == 0 && inputCount > 0 {
				t.Error("BUG: No upstream chunks captured!")
			}

			for i, chunk := range cc.Recorder.UpstreamResponse.Chunks {
				data := string(chunk.Data)
				if strings.Contains(tc.events[i], "<|tool_call") {
					if !strings.Contains(data, "<|tool_call") {
						t.Errorf("BUG: Chunk %d lost tool_call tokens! Input had them, captured doesn't.", i)
					}
				}
			}
		})
	}
}
