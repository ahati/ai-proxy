package downstream

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tmaxmax/go-sse"
)

func TestOpenAIToAnthropic_SimpleText(t *testing.T) {
	var output bytes.Buffer
	transformer := NewOpenAIToAnthropicTransformer(&output)

	events := []string{
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"content":"Hello"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"content":"!"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
	}

	for _, data := range events {
		transformer.Transform(&sse.Event{Data: data})
	}
	transformer.Flush()

	result := output.String()

	if !strings.Contains(result, "event: message_start") {
		t.Error("Expected message_start event")
	}
	if !strings.Contains(result, "Hello") {
		t.Error("Expected 'Hello' in output")
	}
	if !strings.Contains(result, "event: message_stop") {
		t.Error("Expected message_stop event")
	}
	if !strings.Contains(result, "end_turn") {
		t.Error("Expected end_turn stop reason")
	}
}

func TestOpenAIToAnthropic_WithThinking(t *testing.T) {
	var output bytes.Buffer
	transformer := NewOpenAIToAnthropicTransformer(&output)

	events := []string{
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"reasoning_content":"Let me think about this..."}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"reasoning_content":" The answer is 42."}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":15,"total_tokens":25}}`,
	}

	for _, data := range events {
		transformer.Transform(&sse.Event{Data: data})
	}
	transformer.Flush()

	result := output.String()

	if !strings.Contains(result, "event: content_block_start") {
		t.Error("Expected content_block_start")
	}
	if !strings.Contains(result, "thinking") {
		t.Error("Expected thinking content block")
	}
	if !strings.Contains(result, "Let me think") {
		t.Error("Expected thinking content")
	}
}

func TestOpenAIToAnthropic_ToolCalls(t *testing.T) {
	var output bytes.Buffer
	transformer := NewOpenAIToAnthropicTransformer(&output)

	events := []string{
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"reasoning_content":"<|tool_calls_section_begin|><|tool_call_begin|>bash:1<|tool_call_argument_begin|>{\"command\":\"ls -la\"}<|tool_call_end|>"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
	}

	for _, data := range events {
		transformer.Transform(&sse.Event{Data: data})
	}
	transformer.Flush()

	result := output.String()

	if !strings.Contains(result, "tool_use") {
		t.Error("Expected tool_use block")
	}
	if !strings.Contains(result, "bash") {
		t.Error("Expected bash function name")
	}
	if !strings.Contains(result, "ls -la") {
		t.Error("Expected command in arguments")
	}
	if !strings.Contains(result, "tool_use") {
		t.Error("Expected tool_use stop reason")
	}
}

func TestOpenAIToAnthropic_MultipleTools(t *testing.T) {
	var output bytes.Buffer
	transformer := NewOpenAIToAnthropicTransformer(&output)

	events := []string{
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"reasoning_content":"<|tool_calls_section_begin|><|tool_call_begin|>bash:1<|tool_call_argument_begin|>{\"cmd\":\"ls\"}<|tool_call_end|><|tool_call_begin|>read:2<|tool_call_argument_begin|>{\"path\":\"file.txt\"}<|tool_call_end|><|tool_calls_section_end|>"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
	}

	for _, data := range events {
		transformer.Transform(&sse.Event{Data: data})
	}
	transformer.Flush()

	result := output.String()

	t.Logf("Output: %s", result)

	toolUseCount := strings.Count(result, "tool_use")
	if toolUseCount < 2 {
		t.Errorf("Expected at least 2 tool_use blocks, got %d", toolUseCount)
	}
	if !strings.Contains(result, "bash") {
		t.Error("Expected bash function")
	}
	if !strings.Contains(result, "read") {
		t.Error("Expected read function")
	}
}

func TestOpenAIToAnthropic_ComplexArguments(t *testing.T) {
	var output bytes.Buffer
	transformer := NewOpenAIToAnthropicTransformer(&output)

	events := []string{
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"reasoning_content":"<|tool_calls_section_begin|><|tool_call_begin|>bash:1<|tool_call_argument_begin|>{\"command\": \"echo \\\"hello\\\" && grep test file.txt\"}<|tool_call_end|><|tool_calls_section_end|>"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
	}

	for _, data := range events {
		transformer.Transform(&sse.Event{Data: data})
	}
	transformer.Flush()

	result := output.String()

	if !strings.Contains(result, "echo") {
		t.Error("Expected echo in command")
	}
	if !strings.Contains(result, "grep test") {
		t.Error("Expected grep in command")
	}
}

func TestOpenAIToAnthropic_MessageStart(t *testing.T) {
	var output bytes.Buffer
	transformer := NewOpenAIToAnthropicTransformer(&output)

	event := `{"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`
	transformer.Transform(&sse.Event{Data: event})
	transformer.Flush()

	result := output.String()

	if !strings.Contains(result, "event: message_start") {
		t.Error("Expected message_start event")
	}
	if !strings.Contains(result, "chatcmpl-abc123") {
		t.Error("Expected message ID")
	}
	if !strings.Contains(result, "kimi-k2.5") {
		t.Error("Expected model name")
	}
}

func TestOpenAIToAnthropic_MessageDelta(t *testing.T) {
	var output bytes.Buffer
	transformer := NewOpenAIToAnthropicTransformer(&output)

	events := []string{
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
	}

	for _, data := range events {
		transformer.Transform(&sse.Event{Data: data})
	}
	transformer.Flush()

	result := output.String()

	if !strings.Contains(result, "event: message_delta") {
		t.Error("Expected message_delta event")
	}
	if !strings.Contains(result, "end_turn") {
		t.Error("Expected stop_reason mapping")
	}
}

func TestOpenAIToAnthropic_StopReasonMapping(t *testing.T) {
	tests := []struct {
		name           string
		finishReason   string
		expectedReason string
	}{
		{"tool_calls", "tool_calls", "tool_use"},
		{"stop", "stop", "end_turn"},
		{"length", "length", "max_tokens"},
		{"content_filter", "content_filter", "content_filter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			transformer := NewOpenAIToAnthropicTransformer(&output)

			events := []string{
				`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
				`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{},"finish_reason":"` + tt.finishReason + `"}]}`,
				`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
			}

			for _, data := range events {
				transformer.Transform(&sse.Event{Data: data})
			}
			transformer.Flush()

			result := output.String()
			if !strings.Contains(result, tt.expectedReason) {
				t.Errorf("Expected stop reason %q, got output: %s", tt.expectedReason, result)
			}
		})
	}
}

func TestOpenAIToAnthropic_ThinkingWithTools(t *testing.T) {
	var output bytes.Buffer
	transformer := NewOpenAIToAnthropicTransformer(&output)

	events := []string{
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"reasoning_content":"Let me analyze this."}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"reasoning_content":"<|tool_calls_section_begin|><|tool_call_begin|>bash:1<|tool_call_argument_begin|>{\"cmd\":\"pwd\"}<|tool_call_end|><|tool_calls_section_end|>"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"reasoning_content":"Now I know the path."}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`,
	}

	for _, data := range events {
		transformer.Transform(&sse.Event{Data: data})
	}
	transformer.Flush()

	result := output.String()

	if !strings.Contains(result, "Let me analyze") {
		t.Error("Expected thinking content before tools")
	}
	if !strings.Contains(result, "bash") {
		t.Error("Expected bash tool")
	}
	if !strings.Contains(result, "pwd") {
		t.Error("Expected command in tool")
	}
	if !strings.Contains(result, "Now I know") {
		t.Error("Expected thinking content after tools")
	}
}

func TestOpenAIToAnthropic_ToolCallIDGeneration(t *testing.T) {
	var output bytes.Buffer
	transformer := NewOpenAIToAnthropicTransformer(&output)

	events := []string{
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"reasoning_content":"<|tool_calls_section_begin|><|tool_call_begin|>1<|tool_call_argument_begin|>{\"cmd\":\"ls\"}<|tool_call_end|><|tool_calls_section_end|>"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
	}

	for _, data := range events {
		transformer.Transform(&sse.Event{Data: data})
	}
	transformer.Flush()

	result := output.String()

	if !strings.Contains(result, "toolu_") {
		t.Error("Expected toolu_ prefix in tool ID")
	}
}

func TestOpenAIToAnthropic_ToolCallIDPreservation(t *testing.T) {
	var output bytes.Buffer
	transformer := NewOpenAIToAnthropicTransformer(&output)

	events := []string{
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"reasoning_content":"<|tool_calls_section_begin|><|tool_call_begin|>call_123<|tool_call_argument_begin|>{\"cmd\":\"ls\"}<|tool_call_end|><|tool_calls_section_end|>"}}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
	}

	for _, data := range events {
		transformer.Transform(&sse.Event{Data: data})
	}
	transformer.Flush()

	result := output.String()

	if !strings.Contains(result, "call_123") {
		t.Error("Expected call_123 ID to be preserved")
	}
}

func TestOpenAIToAnthropic_EmptyData(t *testing.T) {
	var output bytes.Buffer
	transformer := NewOpenAIToAnthropicTransformer(&output)

	transformer.Transform(&sse.Event{Data: ""})
	transformer.Flush()

	result := output.String()
	if result != "" {
		t.Error("Expected no output for empty data")
	}
}

func TestOpenAIToAnthropic_InvalidJSON(t *testing.T) {
	var output bytes.Buffer
	transformer := NewOpenAIToAnthropicTransformer(&output)

	transformer.Transform(&sse.Event{Data: "invalid json"})
	transformer.Flush()

	result := output.String()
	if !strings.Contains(result, "event: error") {
		t.Error("Expected error event for invalid JSON")
	}
}

func TestOpenAIToAnthropic_FunctionNameParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "bash", "bash"},
		{"with namespace", "functions.bash", "bash"},
		{"with colon only", "bash:123", "bash"},
		{"with namespace and colon", "bash:123", "bash"},
		{"complex namespace", "tools.functions.bash:456", "bash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			transformer := NewOpenAIToAnthropicTransformer(&output)

			event := `{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1234567890,"model":"kimi-k2.5","choices":[{"index":0,"delta":{"role":"assistant"}}]}`
			transformer.Transform(&sse.Event{Data: event})

			name := transformer.parseFunctionName(tt.input)
			if name != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, name)
			}
		})
	}
}
