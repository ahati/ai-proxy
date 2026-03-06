package logging

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewSSEChunk_RawMessageReference(t *testing.T) {
	originalData := []byte(`{"choices":[{"delta":{"reasoning":"<|tool_calls_section_begin|><|tool_call_begin|>bash<|tool_call_argument_begin|>{\"command\":\"ls\"}<|tool_call_end|>"}}]}`)

	chunk := NewSSEChunk(0, "message", originalData)

	originalDataCopy := make([]byte, len(originalData))
	copy(originalDataCopy, originalData)

	for i := range originalData {
		originalData[i] = 'X'
	}

	t.Logf("Original (modified): %s", string(originalData))
	t.Logf("Original (copy): %s", string(originalDataCopy))
	t.Logf("Chunk.Data: %s", string(chunk.Data))

	if string(chunk.Data) != string(originalDataCopy) {
		t.Errorf("Chunk.Data was corrupted after modifying original slice!")
		t.Errorf("Expected: %s", string(originalDataCopy))
		t.Errorf("Got: %s", string(chunk.Data))
	}
}

func TestNewSSEChunk_TransformationScenario(t *testing.T) {
	upstreamData := []byte(`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":"<|tool_calls_section_begin|><|tool_call_begin|>bash<|tool_call_argument_begin|>{\"command\":\"ls\"}<|tool_call_end|>"}}]}`)

	chunk := NewSSEChunk(0, "message", upstreamData)

	var parsed map[string]interface{}
	if err := json.Unmarshal(chunk.Data, &parsed); err != nil {
		t.Fatalf("Failed to parse chunk data: %v", err)
	}

	choices := parsed["choices"].([]interface{})
	delta := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
	reasoning := delta["reasoning"].(string)

	hasToolCallTokens := containsToolCallTokens(reasoning)
	t.Logf("Reasoning field: %s", reasoning)
	t.Logf("Has tool call tokens: %v", hasToolCallTokens)

	if !hasToolCallTokens {
		t.Error("Expected reasoning to contain tool call tokens, but it doesn't!")
	}
}

func TestCaptureWriter_StreamSimulation(t *testing.T) {
	start := time.Now()
	cw := NewCaptureWriter(start)

	upstreamChunks := [][]byte{
		[]byte(`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":"Some text <|tool_calls_section_begin|>"}}]}`),
		[]byte(`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":"<|tool_call_begin|>bash<|tool_call_argument_begin|>{\"command\":\"ls\"}<|tool_call_end|>"}}]}`),
		[]byte(`{"id":"chatcmpl-1","choices":[{"delta":{"reasoning":"<|tool_calls_section_end|> more text"}}]}`),
	}

	for i, data := range upstreamChunks {
		dataCopy := make([]byte, len(data))
		copy(dataCopy, data)

		cw.RecordChunk("message", data)

		for j := range data {
			data[j] = 'X'
		}

		t.Logf("Chunk %d: modified original=%s", i, string(data))
		t.Logf("Chunk %d: original copy=%s", i, string(dataCopy))
	}

	captured := cw.Chunks()
	t.Logf("\nCaptured %d chunks", len(captured))

	for i, chunk := range captured {
		t.Logf("Chunk %d Data: %s", i, string(chunk.Data))

		var parsed map[string]interface{}
		if err := json.Unmarshal(chunk.Data, &parsed); err != nil {
			t.Errorf("Chunk %d: Failed to parse - %v", i, err)
			continue
		}

		choices, ok := parsed["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			t.Errorf("Chunk %d: No choices", i)
			continue
		}

		delta, ok := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
		if !ok {
			t.Errorf("Chunk %d: No delta", i)
			continue
		}

		if reasoning, ok := delta["reasoning"].(string); ok {
			hasTokens := containsToolCallTokens(reasoning)
			t.Logf("Chunk %d reasoning: %q (has tokens: %v)", i, reasoning, hasTokens)

			if i < 2 && !hasTokens {
				t.Errorf("Chunk %d: Expected tool call tokens in reasoning but found none!", i)
			}
		}
	}
}

func TestRawMessageBehavior(t *testing.T) {
	t.Run("RawMessage Unmarshal already copies", func(t *testing.T) {
		original := []byte(`{"test":"value"}`)

		var raw json.RawMessage
		err := json.Unmarshal(original, &raw)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		original[0] = 'X'

		if raw[0] == 'X' {
			t.Error("RawMessage was corrupted - json.Unmarshal should copy but didn't")
		} else {
			t.Logf("json.Unmarshal correctly copies: raw=%s (independent of modified original)", string(raw))
		}
	})
}

func containsToolCallTokens(s string) bool {
	tokens := []string{
		"<|tool_calls_section_begin|>",
		"<|tool_call_begin|>",
		"<|tool_call_argument_begin|>",
		"<|tool_call_end|>",
		"<|tool_calls_section_end|>",
	}
	for _, tok := range tokens {
		if containsStr(s, tok) {
			return true
		}
	}
	return false
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
