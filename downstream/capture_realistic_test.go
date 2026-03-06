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

func TestCaptureRealisticFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamEvents := []string{
		`{"id":"chatcmpl-test1","object":"chat.completion.chunk","created":123,"model":"test","choices":[{"index":0,"delta":{"reasoning":"Some thinking... <|tool_calls_section_begin|>"}}]}`,
		`{"id":"chatcmpl-test1","object":"chat.completion.chunk","created":123,"model":"test","choices":[{"index":0,"delta":{"reasoning":"<|tool_call_begin|>functions.bash:15<|tool_call_argument_begin|>{\"command\":\"ls -la\"}<|tool_call_end|>"}}]}`,
		`{"id":"chatcmpl-test1","object":"chat.completion.chunk","created":123,"model":"test","choices":[{"index":0,"delta":{"reasoning":"<|tool_calls_section_end|>"}}]}`,
		`{"id":"chatcmpl-test1","object":"chat.completion.chunk","created":123,"model":"test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
	}

	var upstreamSSE strings.Builder
	for _, ev := range upstreamEvents {
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
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	responseWriter := NewResponseRecorder(w, downstreamCapture)
	transformer := NewToolCallTransformer(responseWriter)

	t.Log("=== Simulating stream processing ===")
	for ev, err := range sse.Read(upstreamBody, nil) {
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("SSE read error: %v", err)
		}

		t.Logf("Upstream event: type=%q data_len=%d", ev.Type, len(ev.Data))

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

	t.Log("\n=== UPSTREAM CHUNKS (should have raw tool call tokens) ===")
	for i, chunk := range cc.Recorder.UpstreamResponse.Chunks {
		t.Logf("Chunk %d: %s", i, string(chunk.Data))

		var parsed map[string]interface{}
		if err := json.Unmarshal(chunk.Data, &parsed); err != nil {
			t.Errorf("Failed to parse upstream chunk %d: %v", i, err)
			continue
		}

		choices, _ := parsed["choices"].([]interface{})
		if len(choices) > 0 {
			choice := choices[0].(map[string]interface{})
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				if reasoning, ok := delta["reasoning"].(string); ok {
					hasTokens := strings.Contains(reasoning, "<|tool_call")
					t.Logf("  → reasoning has tool_call tokens: %v", hasTokens)
					if !hasTokens && i < 3 {
						t.Errorf("  → ERROR: Expected tool call tokens in upstream chunk %d!", i)
					}
				}
			}
		}
	}

	t.Log("\n=== DOWNSTREAM CHUNKS (should have transformed tool_calls) ===")
	for i, chunk := range cc.Recorder.DownstreamResponse.Chunks {
		t.Logf("Chunk %d: %s", i, string(chunk.Data))

		var parsed map[string]interface{}
		if err := json.Unmarshal(chunk.Data, &parsed); err != nil {
			continue
		}

		choices, _ := parsed["choices"].([]interface{})
		if len(choices) > 0 {
			choice := choices[0].(map[string]interface{})
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				if toolCalls, ok := delta["tool_calls"].([]interface{}); ok {
					t.Logf("  → tool_calls: %d calls", len(toolCalls))
				}
			}
		}
	}

	t.Log("\n=== VERIFICATION ===")
	upstreamHasRawTokens := false
	for _, chunk := range cc.Recorder.UpstreamResponse.Chunks {
		if strings.Contains(string(chunk.Data), "<|tool_call") {
			upstreamHasRawTokens = true
			break
		}
	}

	downstreamHasTransformed := false
	downstreamHasRawTokens := false
	for _, chunk := range cc.Recorder.DownstreamResponse.Chunks {
		data := string(chunk.Data)
		if strings.Contains(data, `"tool_calls"`) {
			downstreamHasTransformed = true
		}
		if strings.Contains(data, "<|tool_call") {
			downstreamHasRawTokens = true
		}
	}

	t.Logf("Upstream has raw tool_call tokens: %v", upstreamHasRawTokens)
	t.Logf("Downstream has transformed tool_calls: %v", downstreamHasTransformed)
	t.Logf("Downstream has raw tool_call tokens: %v", downstreamHasRawTokens)

	if !upstreamHasRawTokens {
		t.Error("BUG: Upstream chunks should contain raw <|tool_call tokens!")
	}
	if !downstreamHasTransformed {
		t.Error("Downstream chunks should contain transformed tool_calls!")
	}
	if downstreamHasRawTokens {
		t.Error("BUG: Downstream chunks should NOT contain raw <|tool_call tokens!")
	}
}
