package convert

import (
	"bytes"
	"encoding/json"
	"testing"

	"ai-proxy/transform"
	"ai-proxy/types"
)

// TestResponsesToAnthropicStreaming_BasicEvent tests basic event conversion.
//
// @brief Tests that response.created event is converted to message_start.
func TestResponsesToAnthropicStreaming_BasicEvent(t *testing.T) {
	var buf bytes.Buffer
	converter := NewResponsesToAnthropicStreamingConverter(&buf)

	// Test response.created event
	createdEvent := types.ResponsesStreamEvent{
		Type: "response.created",
		Response: &types.ResponsesResponse{
			ID:    "resp_123",
			Model: "claude-3-opus-20240229",
		},
	}

	eventData, err := json.Marshal(createdEvent)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	if err := converter.Process(createResponsesEvent(eventData)); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify output contains message_start
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("message_start")) {
		t.Errorf("Expected message_start in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("resp_123")) {
		t.Errorf("Expected response ID in output, got: %s", output)
	}
}

// TestResponsesToAnthropicStreaming_TextDelta tests text delta conversion.
//
// @brief Tests that output_text.delta is converted to content_block_delta.
func TestResponsesToAnthropicStreaming_TextDelta(t *testing.T) {
	var buf bytes.Buffer
	converter := NewResponsesToAnthropicStreamingConverter(&buf)

	// Setup: track block index (simulating output_item.added)
	converter.outputIndexToBlock[0] = 0

	// Test output_text.delta event
	deltaEvent := types.ResponsesStreamEvent{
		Type:         "response.output_text.delta",
		ContentIndex: 0,
		Delta:        "Hello",
	}

	eventData, err := json.Marshal(deltaEvent)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	if err := converter.Process(createResponsesEvent(eventData)); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify output contains content_block_delta with text_delta
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("content_block_delta")) {
		t.Errorf("Expected content_block_delta in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("text_delta")) {
		t.Errorf("Expected text_delta in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Hello")) {
		t.Errorf("Expected 'Hello' in output, got: %s", output)
	}
}

// TestResponsesToAnthropicStreaming_ToolUse tests tool use conversion.
//
// @brief Tests that function_call is converted to tool_use.
func TestResponsesToAnthropicStreaming_ToolUse(t *testing.T) {
	var buf bytes.Buffer
	converter := NewResponsesToAnthropicStreamingConverter(&buf)

	// Test output_item.added with function_call
	toolEvent := types.ResponsesStreamEvent{
		Type:         "response.output_item.added",
		ContentIndex: 0,
		OutputItem: &types.OutputItem{
			Type:   "function_call",
			CallID: "call_abc123",
			Name:   "get_weather",
		},
	}

	eventData, err := json.Marshal(toolEvent)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	if err := converter.Process(createResponsesEvent(eventData)); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify output contains content_block_start with tool_use
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("content_block_start")) {
		t.Errorf("Expected content_block_start in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("tool_use")) {
		t.Errorf("Expected tool_use in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("get_weather")) {
		t.Errorf("Expected function name in output, got: %s", output)
	}
}

// TestResponsesToAnthropicStreaming_Completed tests response completion.
//
// @brief Tests that response.completed emits message_delta and message_stop.
func TestResponsesToAnthropicStreaming_Completed(t *testing.T) {
	var buf bytes.Buffer
	converter := NewResponsesToAnthropicStreamingConverter(&buf)

	// Test response.completed event
	completedEvent := types.ResponsesStreamEvent{
		Type: "response.completed",
		Response: &types.ResponsesResponse{
			ID:    "resp_123",
			Model: "claude-3-opus-20240229",
			Usage: &types.ResponsesUsage{
				InputTokens:  100,
				OutputTokens: 50,
			},
		},
	}

	eventData, err := json.Marshal(completedEvent)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	if err := converter.Process(createResponsesEvent(eventData)); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify output contains message_delta and message_stop
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("message_delta")) {
		t.Errorf("Expected message_delta in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("message_stop")) {
		t.Errorf("Expected message_stop in output, got: %s", output)
	}
}

// TestResponsesToChatStreaming_BasicEvent tests basic OpenAI chunk conversion.
//
// @brief Tests that response.created stores metadata correctly.
func TestResponsesToChatStreaming_BasicEvent(t *testing.T) {
	var buf bytes.Buffer
	converter := NewResponsesToChatStreamingConverter(&buf)

	// Test response.created event
	createdEvent := types.ResponsesStreamEvent{
		Type: "response.created",
		Response: &types.ResponsesResponse{
			ID:    "resp_123",
			Model: "gpt-4",
		},
	}

	eventData, err := json.Marshal(createdEvent)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	if err := converter.Process(createResponsesEvent(eventData)); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify internal state
	if converter.responseID != "resp_123" {
		t.Errorf("Expected responseID=resp_123, got %s", converter.responseID)
	}
	if converter.model != "gpt-4" {
		t.Errorf("Expected model=gpt-4, got %s", converter.model)
	}
}

// TestResponsesToChatStreaming_TextDelta tests text delta to OpenAI chunk.
//
// @brief Tests that output_text.delta emits OpenAI chunk with content.
func TestResponsesToChatStreaming_TextDelta(t *testing.T) {
	var buf bytes.Buffer
	converter := NewResponsesToChatStreamingConverter(&buf)
	converter.responseID = "resp_123"
	converter.model = "gpt-4"

	// Test output_text.delta event
	deltaEvent := types.ResponsesStreamEvent{
		Type:         "response.output_text.delta",
		ContentIndex: 0,
		Delta:        "Hello",
	}

	eventData, err := json.Marshal(deltaEvent)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	if err := converter.Process(createResponsesEvent(eventData)); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify output is valid OpenAI chunk
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("chat.completion.chunk")) {
		t.Errorf("Expected chat.completion.chunk in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Hello")) {
		t.Errorf("Expected 'Hello' in output, got: %s", output)
	}
}

// TestResponsesToChatStreaming_ToolCall tests tool call to OpenAI format.
//
// @brief Tests that function_call_arguments.delta emits tool call chunks.
func TestResponsesToChatStreaming_ToolCall(t *testing.T) {
	var buf bytes.Buffer
	converter := NewResponsesToChatStreamingConverter(&buf)
	converter.responseID = "resp_123"
	converter.model = "gpt-4"
	converter.currentToolCallID = "call_abc123"
	converter.currentToolCallName = "get_weather"

	// Test function_call_arguments.delta event
	deltaEvent := types.ResponsesStreamEvent{
		Type:         "response.function_call_arguments.delta",
		ContentIndex: 0,
		Delta:        `{"loc`,
	}

	eventData, err := json.Marshal(deltaEvent)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	if err := converter.Process(createResponsesEvent(eventData)); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify output contains tool_calls
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("tool_calls")) {
		t.Errorf("Expected tool_calls in output, got: %s", output)
	}
}

// Helper function to create a Responses event
func createResponsesEvent(data []byte) transform.PipelineEvent {
	return transform.PipelineEvent{
		Type: transform.EventResponsesEvent,
		Data: data,
	}
}