package transform

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"ai-proxy/types"
)

func TestChatToSSEAdapter(t *testing.T) {
	buf := &bytes.Buffer{}
	receiver := NewSSEChatWriter(buf)
	adapter := NewChatToSSEAdapter(receiver)

	// Write SSE-formatted data
	sseData := "data: {\"id\":\"test\",\"object\":\"chat.completion.chunk\"}\n\n"
	n, err := adapter.Write([]byte(sseData))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(sseData) {
		t.Errorf("Write returned %d, expected %d", n, len(sseData))
	}

	// Verify output
	output := buf.String()
	if !strings.Contains(output, "\"id\":\"test\"") {
		t.Errorf("Output doesn't contain expected JSON: %s", output)
	}
}

func TestChatToSSEAdapter_Done(t *testing.T) {
	buf := &bytes.Buffer{}
	receiver := NewSSEChatWriter(buf)
	adapter := NewChatToSSEAdapter(receiver)

	// Write [DONE] marker
	sseData := "data: [DONE]\n\n"
	_, err := adapter.Write([]byte(sseData))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify output
	output := buf.String()
	if !strings.Contains(output, "[DONE]") {
		t.Errorf("Output doesn't contain [DONE]: %s", output)
	}
}

func TestSSEChatWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewSSEChatWriter(buf)

	// Write a chunk
	chunkJSON := `{"id":"test-123","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"}}]}`
	err := writer.Receive(chunkJSON)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	// Verify SSE format
	output := buf.String()
	if !strings.HasPrefix(output, "data: ") {
		t.Errorf("Output doesn't start with 'data: ': %s", output)
	}
	if !strings.HasSuffix(output, "\n\n") {
		t.Errorf("Output doesn't end with double newline: %s", output)
	}
	if !strings.Contains(output, "Hello") {
		t.Errorf("Output doesn't contain 'Hello': %s", output)
	}
}

func TestSSEChatWriter_ReceiveDone(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewSSEChatWriter(buf)

	err := writer.ReceiveDone()
	if err != nil {
		t.Fatalf("ReceiveDone failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[DONE]") {
		t.Errorf("Output doesn't contain [DONE]: %s", output)
	}
}

func TestSSEAnthropicWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewSSEAnthropicWriter(buf)

	// Write an Anthropic event
	eventJSON := `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`
	err := writer.Receive(eventJSON)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	// Verify SSE format
	output := buf.String()
	if !strings.Contains(output, "event: content_block_delta") {
		t.Errorf("Output doesn't contain event type: %s", output)
	}
	if !strings.Contains(output, "data: ") {
		t.Errorf("Output doesn't contain 'data: ': %s", output)
	}
	if !strings.Contains(output, "Hello") {
		t.Errorf("Output doesn't contain 'Hello': %s", output)
	}
}

func TestExtractEventType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`{"type":"message_start"}`, "message_start"},
		{`{"type":"content_block_delta","index":0}`, "content_block_delta"},
		{`{"id":"test"}`, ""},
		{`{"type": "message_stop"}`, "message_stop"},
	}

	for _, tt := range tests {
		result := extractEventType(tt.input)
		if result != tt.expected {
			t.Errorf("extractEventType(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestExtractSSEData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantJSON string
		wantDone bool
	}{
		{
			name:     "data event",
			input:    "data: {\"test\":\"value\"}\n",
			wantJSON: `{"test":"value"}`,
			wantDone: false,
		},
		{
			name:     "done event",
			input:    "data: [DONE]\n",
			wantJSON: "",
			wantDone: true,
		},
		{
			name:     "no data prefix",
			input:    "{\"test\":\"value\"}\n",
			wantJSON: "",
			wantDone: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			json, done := extractSSEData([]byte(tt.input))
			if json != tt.wantJSON {
				t.Errorf("extractSSEData() json = %q, want %q", json, tt.wantJSON)
			}
			if done != tt.wantDone {
				t.Errorf("extractSSEData() done = %v, want %v", done, tt.wantDone)
			}
		})
	}
}

// Integration test: OpenAI chunk → receiver → SSE output
func TestOpenAIChunkToSSEChain(t *testing.T) {
	buf := &bytes.Buffer{}
	receiver := NewSSEChatWriter(buf)

	// Create a chunk
	chunk := types.Chunk{
		ID:      "test-123",
		Object:  "chat.completion.chunk",
		Created: 1234567890,
		Model:   "test-model",
		Choices: []types.Choice{{
			Index: 0,
			Delta: types.Delta{
				Content: "Hello, world!",
			},
		}},
	}

	chunkJSON, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("Failed to marshal chunk: %v", err)
	}

	// Send to receiver
	err = receiver.Receive(string(chunkJSON))
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	// Verify output
	output := buf.String()
	if !strings.Contains(output, "Hello, world!") {
		t.Errorf("Output doesn't contain expected content: %s", output)
	}
	if !strings.Contains(output, "test-123") {
		t.Errorf("Output doesn't contain expected ID: %s", output)
	}
}
