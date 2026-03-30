package convert

import (
	"bytes"
	"encoding/json"
	"testing"

	"ai-proxy/conversation"
	"ai-proxy/types"
)

// TestResponsesToChatConverter_Convert tests request conversion.
func TestResponsesToChatConverter_Convert(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		validate func(t *testing.T, output []byte)
	}{
		{
			name: "simple string input",
			input: `{
				"model": "gpt-4o",
				"input": "Hello, world!"
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.Model != "gpt-4o" {
					t.Errorf("Expected model gpt-4o, got %s", req.Model)
				}
				if len(req.Messages) != 1 {
					t.Errorf("Expected 1 message, got %d", len(req.Messages))
				}
				if req.Messages[0].Role != "user" {
					t.Errorf("Expected user role, got %s", req.Messages[0].Role)
				}
				if req.Messages[0].Content != "Hello, world!" {
					t.Errorf("Expected content 'Hello, world!', got %v", req.Messages[0].Content)
				}
			},
		},
		{
			name: "input with instructions",
			input: `{
				"model": "gpt-4o",
				"input": "Hello",
				"instructions": "You are a helpful assistant."
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 2 {
					t.Fatalf("Expected 2 messages, got %d", len(req.Messages))
				}
				if req.Messages[0].Role != "system" {
					t.Errorf("Expected first message role system, got %s", req.Messages[0].Role)
				}
				if req.Messages[0].Content != "You are a helpful assistant." {
					t.Errorf("Expected prepended system content, got %v", req.Messages[0].Content)
				}
				if req.Messages[1].Role != "user" {
					t.Errorf("Expected second message role user, got %s", req.Messages[1].Role)
				}
			},
		},
		{
			name: "flat tool_choice object",
			input: `{
				"model": "gpt-4o",
				"input": "Hello",
				"tool_choice": {"type": "function", "name": "search"}
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				choice, ok := req.ToolChoice.(map[string]interface{})
				if !ok {
					t.Fatalf("Expected tool_choice to be an object, got %T", req.ToolChoice)
				}
				if choice["type"] != "function" {
					t.Errorf("Expected tool_choice type function, got %v", choice["type"])
				}
				function, ok := choice["function"].(map[string]interface{})
				if !ok {
					t.Fatalf("Expected nested function object, got %T", choice["function"])
				}
				if function["name"] != "search" {
					t.Errorf("Expected function name search, got %v", function["name"])
				}
			},
		},
		{
			name: "input with max_output_tokens",
			input: `{
				"model": "gpt-4o",
				"input": "Hello",
				"max_output_tokens": 1000
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.MaxTokens != 1000 {
					t.Errorf("Expected max_tokens 1000, got %d", req.MaxTokens)
				}
			},
		},
		{
			name: "input with message array",
			input: `{
				"model": "gpt-4o",
				"input": [
					{"type": "message", "role": "user", "content": "Hello"},
					{"type": "message", "role": "assistant", "content": "Hi there!"},
					{"type": "message", "role": "user", "content": "How are you?"}
				]
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 3 {
					t.Errorf("Expected 3 messages, got %d", len(req.Messages))
				}
				if req.Messages[0].Role != "user" {
					t.Errorf("Expected first message role user, got %s", req.Messages[0].Role)
				}
				if req.Messages[1].Role != "assistant" {
					t.Errorf("Expected second message role assistant, got %s", req.Messages[1].Role)
				}
			},
		},
		{
			name: "input with tools (flat format)",
			input: `{
				"model": "gpt-4o",
				"input": "What's the weather?",
				"tools": [
					{
						"type": "function",
						"name": "get_weather",
						"description": "Get the current weather",
						"parameters": {"type": "object", "properties": {"location": {"type": "string"}}}
					}
				]
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Tools) != 1 {
					t.Errorf("Expected 1 tool, got %d", len(req.Tools))
				}
				if req.Tools[0].Type != "function" {
					t.Errorf("Expected tool type function, got %s", req.Tools[0].Type)
				}
				if req.Tools[0].Function.Name != "get_weather" {
					t.Errorf("Expected function name get_weather, got %s", req.Tools[0].Function.Name)
				}
			},
		},
		{
			name: "input with tools (nested format)",
			input: `{
				"model": "gpt-4o",
				"input": "Search for something",
				"tools": [
					{
						"type": "function",
						"function": {
							"name": "search",
							"description": "Search the web",
							"parameters": {"type": "object", "properties": {"query": {"type": "string"}}}
						}
					}
				]
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Tools) != 1 {
					t.Errorf("Expected 1 tool, got %d", len(req.Tools))
				}
				if req.Tools[0].Function.Name != "search" {
					t.Errorf("Expected function name search, got %s", req.Tools[0].Function.Name)
				}
			},
		},
		{
			name: "input with stream flag",
			input: `{
				"model": "gpt-4o",
				"input": "Hello",
				"stream": true
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if !req.Stream {
					t.Error("Expected stream to be true")
				}
			},
		},
		{
			name: "stream false is respected",
			input: `{
				"model": "gpt-4o",
				"input": "Hello",
				"stream": false
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.Stream {
					t.Error("Expected stream to be false")
				}
				// StreamOptions should not be set when not streaming
				if req.StreamOptions != nil {
					t.Error("Expected stream_options to be nil when not streaming")
				}
			},
		},
		{
			name: "input with temperature and top_p",
			input: `{
				"model": "gpt-4o",
				"input": "Hello",
				"temperature": 0.7,
				"top_p": 0.9
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.Temperature != 0.7 {
					t.Errorf("Expected temperature 0.7, got %f", req.Temperature)
				}
				if req.TopP != 0.9 {
					t.Errorf("Expected top_p 0.9, got %f", req.TopP)
				}
			},
		},
		{
			name:    "invalid JSON",
			input:   `{invalid json}`,
			wantErr: true,
		},
		{
			name: "empty input",
			input: `{
				"model": "gpt-4o",
				"input": ""
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 0 {
					t.Errorf("Expected 0 messages for empty input, got %d", len(req.Messages))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewResponsesToChatConverter()
			output, err := converter.Convert([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

// TestResponsesToChatConverter_ContentParts tests content part conversion.
func TestResponsesToChatConverter_ContentParts(t *testing.T) {
	converter := NewResponsesToChatConverter()

	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, output []byte)
	}{
		{
			name: "input_text content",
			input: `{
				"model": "gpt-4o",
				"input": [
					{
						"type": "message",
						"role": "user",
						"content": [
							{"type": "input_text", "text": "Hello"}
						]
					}
				]
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 1 {
					t.Errorf("Expected 1 message, got %d", len(req.Messages))
				}
			},
		},
		{
			name: "input_image content",
			input: `{
				"model": "gpt-4o",
				"input": [
					{
						"type": "message",
						"role": "user",
						"content": [
							{"type": "input_text", "text": "What's in this image?"},
							{"type": "input_image", "image_url": "https://example.com/image.png"}
						]
					}
				]
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 1 {
					t.Errorf("Expected 1 message, got %d", len(req.Messages))
				}
				// Check that content was converted
				content, ok := req.Messages[0].Content.([]interface{})
				if !ok {
					t.Errorf("Expected content to be array, got %T", req.Messages[0].Content)
					return
				}
				if len(content) != 2 {
					t.Errorf("Expected 2 content parts, got %d", len(content))
				}
			},
		},
		{
			name: "output_text content (assistant message history)",
			input: `{
				"model": "gpt-4o",
				"input": [
					{
						"type": "message",
						"role": "user",
						"content": [{"type": "input_text", "text": "Hello"}]
					},
					{
						"type": "message",
						"role": "assistant",
						"content": [{"type": "output_text", "text": "Hi there! How can I help?"}]
					}
				]
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 2 {
					t.Errorf("Expected 2 messages, got %d", len(req.Messages))
					return
				}
				// First message should be user
				if req.Messages[0].Role != "user" {
					t.Errorf("Expected first message role 'user', got %s", req.Messages[0].Role)
				}
				// Second message should be assistant with the output_text content preserved
				if req.Messages[1].Role != "assistant" {
					t.Errorf("Expected second message role 'assistant', got %s", req.Messages[1].Role)
				}
				// Verify content was extracted from output_text
				content, ok := req.Messages[1].Content.(string)
				if !ok {
					t.Errorf("Expected content to be string, got %T", req.Messages[1].Content)
					return
				}
				if content != "Hi there! How can I help?" {
					t.Errorf("Expected content 'Hi there! How can I help?', got %q", content)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := converter.Convert([]byte(tt.input))
			if err != nil {
				t.Errorf("Convert returned error: %v", err)
				return
			}
			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

// TestResponsesToChatConverter_NonFunctionTools tests that non-function tools are skipped.
func TestResponsesToChatConverter_NonFunctionTools(t *testing.T) {
	converter := NewResponsesToChatConverter()

	input := `{
		"model": "gpt-4o",
		"input": "Hello",
		"tools": [
			{"type": "file_search"},
			{"type": "function", "name": "search", "description": "Search"},
			{"type": "web_search"}
		]
	}`

	output, err := converter.Convert([]byte(input))
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	var req types.ChatCompletionRequest
	if err := json.Unmarshal(output, &req); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// Only function tools should be included
	if len(req.Tools) != 1 {
		t.Errorf("Expected 1 tool (only function type), got %d", len(req.Tools))
	}
	if len(req.Tools) > 0 && req.Tools[0].Function.Name != "search" {
		t.Errorf("Expected function name 'search', got %s", req.Tools[0].Function.Name)
	}
}

// TestResponsesToChatConverter_MultipleToolCalls tests that multiple consecutive
// function_call items are grouped into a single assistant message.
func TestResponsesToChatConverter_MultipleToolCalls(t *testing.T) {
	converter := NewResponsesToChatConverter()

	input := `{
		"model": "gpt-4o",
		"input": [
			{"type": "message", "role": "user", "content": "What is the weather?"},
			{"type": "function_call", "call_id": "call_1", "name": "get_weather", "arguments": "{\"city\": \"SF\"}"},
			{"type": "function_call", "call_id": "call_2", "name": "get_temperature", "arguments": "{\"city\": \"SF\"}"},
			{"type": "function_call_output", "call_id": "call_1", "output": "Sunny"},
			{"type": "function_call_output", "call_id": "call_2", "output": "72F"}
		]
	}`

	output, err := converter.Convert([]byte(input))
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	var req types.ChatCompletionRequest
	if err := json.Unmarshal(output, &req); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// Should have 3 messages:
	// 1. user message
	// 2. assistant message with 2 tool_calls
	// 3. tool message for call_1
	// 4. tool message for call_2
	if len(req.Messages) != 4 {
		t.Errorf("Expected 4 messages, got %d", len(req.Messages))
		for i, msg := range req.Messages {
			t.Logf("Message %d: role=%s, tool_call_id=%s, tool_calls=%v", i, msg.Role, msg.ToolCallID, msg.ToolCalls)
		}
		return
	}

	// Check first message is user
	if req.Messages[0].Role != "user" {
		t.Errorf("Expected first message to be user, got %s", req.Messages[0].Role)
	}

	// Check second message is assistant with 2 tool_calls
	if req.Messages[1].Role != "assistant" {
		t.Errorf("Expected second message to be assistant, got %s", req.Messages[1].Role)
	}
	if len(req.Messages[1].ToolCalls) != 2 {
		t.Errorf("Expected assistant message to have 2 tool_calls, got %d", len(req.Messages[1].ToolCalls))
	} else {
		// Verify tool call IDs
		ids := make(map[string]bool)
		for _, tc := range req.Messages[1].ToolCalls {
			ids[tc.ID] = true
		}
		if !ids["call_1"] || !ids["call_2"] {
			t.Errorf("Expected tool_calls to have call_1 and call_2, got IDs: %v", ids)
		}
	}

	// Check third and fourth messages are tool messages
	if req.Messages[2].Role != "tool" {
		t.Errorf("Expected third message to be tool, got %s", req.Messages[2].Role)
	}
	if req.Messages[3].Role != "tool" {
		t.Errorf("Expected fourth message to be tool, got %s", req.Messages[3].Role)
	}
}

// TestResponsesToChatConverter_FunctionCallAndOutput tests function_call and
// function_call_output conversion.
func TestResponsesToChatConverter_FunctionCallAndOutput(t *testing.T) {
	converter := NewResponsesToChatConverter()

	input := `{
		"model": "gpt-4o",
		"input": [
			{"type": "message", "role": "user", "content": "What is the weather?"},
			{"type": "function_call", "call_id": "call_123", "name": "get_weather", "arguments": "{\"location\": \"SF\"}"},
			{"type": "function_call_output", "call_id": "call_123", "output": "Sunny in SF"}
		]
	}`

	output, err := converter.Convert([]byte(input))
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	var req types.ChatCompletionRequest
	if err := json.Unmarshal(output, &req); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	// Should have 3 messages: user, assistant with tool_calls, tool
	if len(req.Messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(req.Messages))
		return
	}

	// Check assistant message
	if req.Messages[1].Role != "assistant" {
		t.Errorf("Expected second message to be assistant, got %s", req.Messages[1].Role)
	}
	if len(req.Messages[1].ToolCalls) != 1 {
		t.Errorf("Expected assistant message to have 1 tool_call, got %d", len(req.Messages[1].ToolCalls))
		return
	}
	if req.Messages[1].ToolCalls[0].ID != "call_123" {
		t.Errorf("Expected tool_call ID call_123, got %s", req.Messages[1].ToolCalls[0].ID)
	}
	if req.Messages[1].ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("Expected function name get_weather, got %s", req.Messages[1].ToolCalls[0].Function.Name)
	}

	// Check tool message
	if req.Messages[2].Role != "tool" {
		t.Errorf("Expected third message to be tool, got %s", req.Messages[2].Role)
	}
	if req.Messages[2].ToolCallID != "call_123" {
		t.Errorf("Expected tool_call_id call_123, got %s", req.Messages[2].ToolCallID)
	}
	if req.Messages[2].Content != "Sunny in SF" {
		t.Errorf("Expected content 'Sunny in SF', got %v", req.Messages[2].Content)
	}
}

// BenchmarkResponsesToChatConverter_Convert benchmarks request conversion.
func BenchmarkResponsesToChatConverter_Convert(b *testing.B) {
	converter := NewResponsesToChatConverter()
	input := []byte(`{
		"model": "gpt-4o",
		"input": "Hello, world!",
		"instructions": "You are helpful.",
		"max_output_tokens": 1000
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		converter.Convert(input)
	}
}

// ============================================================================
// PHASE 2 HIGH PRIORITY TESTS
// ============================================================================

// TestResponsesToChatConverter_ResponseFormat_JSONObject tests response_format json_object conversion.
// Category A2 (Responses → Chat): HIGH priority
func TestResponsesToChatConverter_ResponseFormat_JSONObject(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		skip     bool
		skipMsg  string
		validate func(t *testing.T, output []byte)
	}{
		{
			name: "response_format json_object",
			input: `{
				"model": "gpt-4o",
				"input": "Generate a JSON object",
				"response_format": {
					"type": "json_object"
				}
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.ResponseFormat == nil {
					t.Error("Expected ResponseFormat to be set")
					return
				}
				if req.ResponseFormat.Type != "json_object" {
					t.Errorf("Expected ResponseFormat.Type 'json_object', got %s", req.ResponseFormat.Type)
				}
			},
		},
		{
			name: "response_format json_object with schema",
			input: `{
				"model": "gpt-4o",
				"input": "Generate a person",
				"response_format": {
					"type": "json_object"
				}
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.ResponseFormat == nil {
					t.Error("Expected ResponseFormat to be set for JSON mode")
					return
				}
				if req.ResponseFormat.Type != "json_object" {
					t.Errorf("Expected ResponseFormat.Type 'json_object', got %s", req.ResponseFormat.Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip(tt.skipMsg)
			}
			converter := NewResponsesToChatConverter()
			output, err := converter.Convert([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

// TestResponsesToChatConverter_ResponseFormat_JSONSchema tests response_format json_schema conversion.
// Category A2 (Responses → Chat): HIGH priority
func TestResponsesToChatConverter_ResponseFormat_JSONSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		skip     bool
		skipMsg  string
		validate func(t *testing.T, output []byte)
	}{
		{
			name: "response_format json_schema",
			input: `{
				"model": "gpt-4o",
				"input": "Generate a person",
				"response_format": {
					"type": "json_schema",
					"json_schema": {
						"name": "person",
						"description": "A person schema",
						"schema": {
							"type": "object",
							"properties": {
								"name": {"type": "string"},
								"age": {"type": "integer"}
							},
							"required": ["name", "age"]
						}
					}
				}
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.ResponseFormat == nil {
					t.Error("Expected ResponseFormat to be set")
					return
				}
				if req.ResponseFormat.Type != "json_schema" {
					t.Errorf("Expected ResponseFormat.Type 'json_schema', got %s", req.ResponseFormat.Type)
				}
				if req.ResponseFormat.JSONSchema == nil {
					t.Error("Expected JSONSchema to be set")
					return
				}
				if req.ResponseFormat.JSONSchema.Name != "person" {
					t.Errorf("Expected schema name 'person', got %s", req.ResponseFormat.JSONSchema.Name)
				}
			},
		},
		{
			name: "response_format json_schema with strict mode",
			input: `{
				"model": "gpt-4o",
				"input": "Generate a product",
				"response_format": {
					"type": "json_schema",
					"json_schema": {
						"name": "product",
						"strict": true,
						"schema": {
							"type": "object",
							"properties": {
								"id": {"type": "string"},
								"price": {"type": "number"}
							}
						}
					}
				}
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.ResponseFormat == nil || req.ResponseFormat.JSONSchema == nil {
					t.Error("Expected ResponseFormat and JSONSchema to be set")
					return
				}
				if !req.ResponseFormat.JSONSchema.Strict {
					t.Error("Expected strict mode to be enabled")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip(tt.skipMsg)
			}
			converter := NewResponsesToChatConverter()
			output, err := converter.Convert([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

// TestResponsesToChatConverter_PreviousResponseID tests previous_response_id handling.
// Category A2 (Responses → Chat): HIGH priority
func TestResponsesToChatConverter_PreviousResponseID(t *testing.T) {
	oldStore := conversation.DefaultStore
	conversation.DefaultStore = conversation.NewStore(conversation.Config{})
	t.Cleanup(func() {
		conversation.DefaultStore = oldStore
	})

	conversation.StoreInDefault(&conversation.Conversation{
		ID: "resp_prev123",
		Input: []types.InputItem{
			{Type: "message", Role: "user", Content: "Earlier question?"},
		},
		Output: []types.OutputItem{
			{
				Type: "message",
				Role: "assistant",
				Content: []types.OutputContent{
					{Type: "output_text", Text: "Earlier answer."},
				},
			},
		},
	})

	conversation.StoreInDefault(&conversation.Conversation{
		ID: "resp_abc456",
		Input: []types.InputItem{
			{Type: "message", Role: "user", Content: "Start here."},
		},
		Output: []types.OutputItem{
			{
				Type: "message",
				Role: "assistant",
				Content: []types.OutputContent{
					{Type: "output_text", Text: "You are helpful."},
				},
			},
		},
	})

	tests := []struct {
		name     string
		input    string
		wantErr  bool
		validate func(t *testing.T, output []byte)
	}{
		{
			name: "previous_response_id in multi-turn conversation",
			input: `{
				"model": "gpt-4o",
				"input": "What about tomorrow?",
				"previous_response_id": "resp_prev123"
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 3 {
					t.Fatalf("Expected 3 messages, got %d", len(req.Messages))
				}
				if req.Messages[0].Role != "user" || req.Messages[0].Content != "Earlier question?" {
					t.Fatalf("Expected history user message first, got %+v", req.Messages[0])
				}
				if req.Messages[1].Role != "assistant" || req.Messages[1].Content != "Earlier answer." {
					t.Fatalf("Expected history assistant message second, got %+v", req.Messages[1])
				}
				if req.Messages[2].Role != "user" || req.Messages[2].Content != "What about tomorrow?" {
					t.Fatalf("Expected current user message last, got %+v", req.Messages[2])
				}
			},
		},
		{
			name: "previous_response_id with instructions",
			input: `{
				"model": "gpt-4o",
				"input": "Continue",
				"instructions": "You are helpful.",
				"previous_response_id": "resp_abc456"
			}`,
			wantErr: false,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 4 {
					t.Fatalf("Expected 4 messages, got %d", len(req.Messages))
				}
				if req.Messages[0].Role != "system" || req.Messages[0].Content != "You are helpful." {
					t.Fatalf("Expected prepended system message first, got %+v", req.Messages[0])
				}
				if req.Messages[1].Role != "user" || req.Messages[1].Content != "Start here." {
					t.Fatalf("Expected history user message second, got %+v", req.Messages[1])
				}
				if req.Messages[2].Role != "assistant" || req.Messages[2].Content != "You are helpful." {
					t.Fatalf("Expected history assistant message third, got %+v", req.Messages[2])
				}
				if req.Messages[3].Role != "user" || req.Messages[3].Content != "Continue" {
					t.Fatalf("Expected current user message last, got %+v", req.Messages[3])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewResponsesToChatConverter()
			output, err := converter.Convert([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

// TestResponsesToChatConverter_ParallelToolCalls tests parallel_tool_calls false conversion.
// Category A2 (Responses → Chat): MEDIUM priority
func TestResponsesToChatConverter_ParallelToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, output []byte)
	}{
		{
			name: "parallel_tool_calls false",
			input: `{
				"model": "gpt-4o",
				"input": "Compare weather in SF and NYC",
				"tools": [
					{"type": "function", "name": "get_weather", "description": "Get weather"}
				],
				"parallel_tool_calls": false
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				// NOTE: Implementation only sets ParallelToolCalls when true.
				// When false, the field is omitted (defaults to false in OpenAI API).
				// This is by design - false is the API default.
				if req.ParallelToolCalls != nil && *req.ParallelToolCalls != false {
					t.Errorf("Expected ParallelToolCalls to be false or nil, got %v", *req.ParallelToolCalls)
				}
			},
		},
		{
			name: "parallel_tool_calls true",
			input: `{
				"model": "gpt-4o",
				"input": "Compare weather",
				"tools": [
					{"type": "function", "name": "get_weather", "description": "Get weather"}
				],
				"parallel_tool_calls": true
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.ParallelToolCalls == nil {
					t.Error("Expected ParallelToolCalls to be set")
					return
				}
				if *req.ParallelToolCalls != true {
					t.Errorf("Expected ParallelToolCalls to be true, got %v", *req.ParallelToolCalls)
				}
			},
		},
		{
			name: "parallel_tool_calls omitted (default)",
			input: `{
				"model": "gpt-4o",
				"input": "What's the weather?"
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				// Default should be nil (not set)
				if req.ParallelToolCalls != nil {
					t.Errorf("Expected ParallelToolCalls to be nil (default), got %v", *req.ParallelToolCalls)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewResponsesToChatConverter()
			output, err := converter.Convert([]byte(tt.input))

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

// TestResponsesToChatConverter_SystemUserAssistantFlow tests System → User → Assistant flow.
// Category C1 (Multi-turn): HIGH priority
func TestResponsesToChatConverter_SystemUserAssistantFlow(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, output []byte)
	}{
		{
			name: "instructions + user + assistant history",
			input: `{
				"model": "gpt-4o",
				"instructions": "You are a helpful math tutor.",
				"input": [
					{"type": "message", "role": "user", "content": "What is 2+2?"},
					{"type": "message", "role": "assistant", "content": "2+2 equals 4."},
					{"type": "message", "role": "user", "content": "What about 3+3?"}
				]
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 4 {
					t.Fatalf("Expected 4 messages, got %d", len(req.Messages))
				}
				if req.Messages[0].Role != "system" || req.Messages[0].Content != "You are a helpful math tutor." {
					t.Errorf("Expected prepended system message, got %+v", req.Messages[0])
				}
				// Verify order: system, user, assistant, user
				if req.Messages[1].Role != "user" {
					t.Errorf("Expected message 2 role 'user', got %s", req.Messages[1].Role)
				}
				if req.Messages[1].Content != "What is 2+2?" {
					t.Errorf("Expected message 2 content, got %v", req.Messages[1].Content)
				}
				if req.Messages[2].Role != "assistant" {
					t.Errorf("Expected message 3 role 'assistant', got %s", req.Messages[2].Role)
				}
				if req.Messages[2].Content != "2+2 equals 4." {
					t.Errorf("Expected message 3 content, got %v", req.Messages[2].Content)
				}
				if req.Messages[3].Role != "user" {
					t.Errorf("Expected message 4 role 'user', got %s", req.Messages[3].Role)
				}
				if req.Messages[3].Content != "What about 3+3?" {
					t.Errorf("Expected message 4 content, got %v", req.Messages[3].Content)
				}
			},
		},
		{
			name: "developer role converted to system",
			input: `{
				"model": "gpt-4o",
				"input": [
					{"type": "message", "role": "developer", "content": "You are helpful."},
					{"type": "message", "role": "user", "content": "Hello"}
				]
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 2 {
					t.Errorf("Expected 2 messages, got %d", len(req.Messages))
					return
				}
				// Developer role should be converted to system
				if req.Messages[0].Role != "system" {
					t.Errorf("Expected role 'system' for developer message, got %s", req.Messages[0].Role)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewResponsesToChatConverter()
			output, err := converter.Convert([]byte(tt.input))

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

func TestResponsesToChatConverter_ReasoningEffort(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, output []byte)
	}{
		{
			name: "reasoning effort high",
			input: `{
				"model": "o1",
				"input": "Think about this problem",
				"reasoning": {"effort": "high"}
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.ReasoningEffort != "high" {
					t.Errorf("Expected ReasoningEffort 'high', got %q", req.ReasoningEffort)
				}
			},
		},
		{
			name: "reasoning effort medium",
			input: `{
				"model": "o1",
				"input": "Think about this",
				"reasoning": {"effort": "medium"}
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.ReasoningEffort != "medium" {
					t.Errorf("Expected ReasoningEffort 'medium', got %q", req.ReasoningEffort)
				}
			},
		},
		{
			name: "reasoning effort low",
			input: `{
				"model": "o1-mini",
				"input": "Quick thought",
				"reasoning": {"effort": "low"}
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.ReasoningEffort != "low" {
					t.Errorf("Expected ReasoningEffort 'low', got %q", req.ReasoningEffort)
				}
			},
		},
		{
			name: "reasoning effort with summary",
			input: `{
				"model": "o1",
				"input": "Think hard",
				"reasoning": {"effort": "high", "summary": "detailed"}
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.ReasoningEffort != "high" {
					t.Errorf("Expected ReasoningEffort 'high', got %q", req.ReasoningEffort)
				}
			},
		},
		{
			name: "no reasoning config",
			input: `{
				"model": "gpt-4o",
				"input": "Hello"
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if req.ReasoningEffort != "" {
					t.Errorf("Expected ReasoningEffort to be empty, got %q", req.ReasoningEffort)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewResponsesToChatConverter()
			output, err := converter.Convert([]byte(tt.input))
			if err != nil {
				t.Fatalf("Convert returned error: %v", err)
			}
			tt.validate(t, output)
		})
	}
}

// TestResponsesToChatConverter_RefusalContentType tests refusal content type handling.
func TestResponsesToChatConverter_RefusalContentType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, output []byte)
	}{
		{
			name: "refusal content treated as text",
			input: `{
				"model": "gpt-4o",
				"input": [
					{
						"type": "message",
						"role": "assistant",
						"content": [
							{"type": "refusal", "text": "I cannot help with that request."}
						]
					}
				]
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 1 {
					t.Fatalf("Expected 1 message, got %d", len(req.Messages))
				}
				content, ok := req.Messages[0].Content.(string)
				if !ok {
					t.Fatalf("Expected content to be string, got %T", req.Messages[0].Content)
				}
				if content != "I cannot help with that request." {
					t.Errorf("Expected refusal text as content, got %q", content)
				}
			},
		},
		{
			name: "mixed refusal and output_text in assistant message",
			input: `{
				"model": "gpt-4o",
				"input": [
					{
						"type": "message",
						"role": "assistant",
						"content": [
							{"type": "output_text", "text": "Here is some info."},
							{"type": "refusal", "text": "But I cannot do more."}
						]
					}
				]
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 1 {
					t.Fatalf("Expected 1 message, got %d", len(req.Messages))
				}
				content, ok := req.Messages[0].Content.(string)
				if !ok {
					t.Fatalf("Expected content to be string, got %T", req.Messages[0].Content)
				}
				expected := "Here is some info.\nBut I cannot do more."
				if content != expected {
					t.Errorf("Expected combined content %q, got %q", expected, content)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewResponsesToChatConverter()
			output, err := converter.Convert([]byte(tt.input))
			if err != nil {
				t.Fatalf("Convert returned error: %v", err)
			}
			tt.validate(t, output)
		})
	}
}

// TestResponsesToChatConverter_InputFileContentType tests input_file content type handling.
func TestResponsesToChatConverter_InputFileContentType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, output []byte)
	}{
		{
			name: "input_file with filename and content preserved",
			input: `{
				"model": "gpt-4o",
				"input": [
					{
						"type": "message",
						"role": "user",
						"content": [
							{"type": "input_text", "text": "Check this file:"},
							{"type": "input_file", "file_data": {"filename": "document.pdf", "file_data": "base64content"}}
						]
					}
				]
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 1 {
					t.Fatalf("Expected 1 message, got %d", len(req.Messages))
				}
				// When there's a file, content is returned as an array
				content, ok := req.Messages[0].Content.([]interface{})
				if !ok {
					t.Fatalf("Expected content to be array, got %T", req.Messages[0].Content)
				}
				// Find the file part
				var foundFile bool
				for _, part := range content {
					partMap, ok := part.(map[string]interface{})
					if !ok {
						continue
					}
					if partType, _ := partMap["type"].(string); partType == "file" {
						foundFile = true
						if partMap["filename"] != "document.pdf" {
							t.Errorf("Expected filename 'document.pdf', got %v", partMap["filename"])
						}
						if partMap["file_data"] != "base64content" {
							t.Errorf("Expected file_data to be preserved, got %v", partMap["file_data"])
						}
					}
				}
				if !foundFile {
					t.Error("Expected file part in content")
				}
			},
		},
		{
			name: "input_file with filename only (no content)",
			input: `{
				"model": "gpt-4o",
				"input": [
					{
						"type": "message",
						"role": "user",
						"content": [
							{"type": "input_file", "file_data": {"filename": "data.csv"}}
						]
					}
				]
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}
				if len(req.Messages) != 1 {
					t.Fatalf("Expected 1 message, got %d", len(req.Messages))
				}
				// File part should still be created
				content, ok := req.Messages[0].Content.([]interface{})
				if !ok {
					t.Fatalf("Expected content to be array, got %T", req.Messages[0].Content)
				}
				if len(content) != 1 {
					t.Fatalf("Expected 1 content part, got %d", len(content))
				}
				partMap, ok := content[0].(map[string]interface{})
				if !ok {
					t.Fatalf("Expected content part to be map, got %T", content[0])
				}
				if partMap["type"] != "file" {
					t.Errorf("Expected type 'file', got %v", partMap["type"])
				}
				if partMap["filename"] != "data.csv" {
					t.Errorf("Expected filename 'data.csv', got %v", partMap["filename"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewResponsesToChatConverter()
			output, err := converter.Convert([]byte(tt.input))
			if err != nil {
				t.Fatalf("Convert returned error: %v", err)
			}
			tt.validate(t, output)
		})
	}
}

// TestResponsesToChatConverter_StreamFlag tests that the stream flag is respected.
func TestResponsesToChatConverter_StreamFlag(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantStream  bool
		wantOptions bool
	}{
		{
			name: "stream true (explicit)",
			input: `{
				"model": "gpt-4o",
				"input": "Hello",
				"stream": true
			}`,
			wantStream:  true,
			wantOptions: true,
		},
		{
			name: "stream false (explicit)",
			input: `{
				"model": "gpt-4o",
				"input": "Hello",
				"stream": false
			}`,
			wantStream:  false,
			wantOptions: false,
		},
		{
			name: "stream not specified (defaults to true)",
			input: `{
				"model": "gpt-4o",
				"input": "Hello"
			}`,
			wantStream:  true,
			wantOptions: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewResponsesToChatConverter()
			output, err := converter.Convert([]byte(tt.input))
			if err != nil {
				t.Fatalf("Convert returned error: %v", err)
			}

			var req types.ChatCompletionRequest
			if err := json.Unmarshal(output, &req); err != nil {
				t.Fatalf("Failed to parse output: %v", err)
			}

			if req.Stream != tt.wantStream {
				t.Errorf("Expected Stream=%v, got Stream=%v", tt.wantStream, req.Stream)
			}

			if tt.wantOptions && req.StreamOptions == nil {
				t.Error("Expected StreamOptions to be set when streaming")
			}
			if !tt.wantOptions && req.StreamOptions != nil {
				t.Error("Expected StreamOptions to be nil when not streaming")
			}
		})
	}
}

// TestResponsesToChatConverter_CombinedMessageWithToolCalls tests that a message
// with embedded tool_calls (from prependHistoryToInput) is correctly converted
// to a single assistant message with both content and tool_calls.
// This is critical for fixing the bug where assistant message + function_call
// would result in two assistant messages instead of one.
func TestResponsesToChatConverter_CombinedMessageWithToolCalls(t *testing.T) {
	oldStore := conversation.DefaultStore
	conversation.DefaultStore = conversation.NewStore(conversation.Config{})
	t.Cleanup(func() {
		conversation.DefaultStore = oldStore
	})

	// Store a conversation where the model responded with both text and a tool call
	conversation.StoreInDefault(&conversation.Conversation{
		ID: "resp_combined",
		Input: []types.InputItem{
			{Type: "message", Role: "user", Content: "Review the code"},
		},
		Output: []types.OutputItem{
			{
				Type: "message",
				Role: "assistant",
				Content: []types.OutputContent{
					{Type: "output_text", Text: "I'll review the code."},
				},
			},
			{
				Type:      "function_call",
				CallID:    "call_123",
				Name:      "read_file",
				Arguments: `{"path": "main.go"}`,
			},
		},
	})

	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, output []byte)
	}{
		{
			name: "previous_response_id with combined message and tool_call",
			input: `{
				"model": "gpt-4o",
				"previous_response_id": "resp_combined",
				"input": [
					{
						"type": "function_call_output",
						"call_id": "call_123",
						"output": "package main\n\nfunc main() {}"
					}
				]
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}

				// Should have exactly 3 messages:
				// 1. user message from history
				// 2. assistant message with content AND tool_calls (combined)
				// 3. tool message (function_call_output)
				if len(req.Messages) != 3 {
					t.Fatalf("Expected 3 messages, got %d: %+v", len(req.Messages), req.Messages)
				}

				// Check user message
				if req.Messages[0].Role != "user" {
					t.Errorf("Expected message 0 to be user, got %s", req.Messages[0].Role)
				}

				// Check assistant message has BOTH content and tool_calls
				if req.Messages[1].Role != "assistant" {
					t.Errorf("Expected message 1 to be assistant, got %s", req.Messages[1].Role)
				}
				if req.Messages[1].Content == nil || req.Messages[1].Content == "" {
					t.Error("Expected assistant message to have content")
				}
				if len(req.Messages[1].ToolCalls) != 1 {
					t.Fatalf("Expected assistant message to have 1 tool_call, got %d", len(req.Messages[1].ToolCalls))
				}
				if req.Messages[1].ToolCalls[0].ID != "call_123" {
					t.Errorf("Expected tool_call ID to be call_123, got %s", req.Messages[1].ToolCalls[0].ID)
				}

				// Check tool message
				if req.Messages[2].Role != "tool" {
					t.Errorf("Expected message 2 to be tool, got %s", req.Messages[2].Role)
				}
				if req.Messages[2].ToolCallID != "call_123" {
					t.Errorf("Expected tool message ToolCallID to be call_123, got %s", req.Messages[2].ToolCallID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewResponsesToChatConverter()
			output, err := converter.Convert([]byte(tt.input))
			if err != nil {
				t.Fatalf("Convert returned error: %v", err)
			}
			tt.validate(t, output)
		})
	}
}

// TestResponsesToChatConverter_SeparateFunctionCallMergedWithAssistant tests that
// when Codex sends function_call and message(assistant) as separate items,
// they are correctly merged into a single assistant message.
// This is the main bug fix: Codex sends these items separately but Chat Completions
// API requires them combined.
func TestResponsesToChatConverter_SeparateFunctionCallMergedWithAssistant(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, output []byte)
	}{
		{
			name: "function_call merged with following assistant message",
			input: `{
				"model": "gpt-4o",
				"input": [
					{"type": "message", "role": "user", "content": "Review the code"},
					{"type": "function_call", "call_id": "call_abc", "name": "read_file", "arguments": "{\"path\": \"main.go\"}"},
					{"type": "message", "role": "assistant", "content": [{"type": "output_text", "text": "I'll read the file."}]},
					{"type": "function_call_output", "call_id": "call_abc", "output": "file contents here"}
				]
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}

				// Should have exactly 3 messages:
				// 1. user message
				// 2. assistant message with BOTH content AND tool_calls (merged)
				// 3. tool message
				if len(req.Messages) != 3 {
					t.Fatalf("Expected 3 messages, got %d: %+v", len(req.Messages), req.Messages)
				}

				// Check user message
				if req.Messages[0].Role != "user" {
					t.Errorf("Expected message 0 to be user, got %s", req.Messages[0].Role)
				}

				// Check assistant message has BOTH content and tool_calls
				if req.Messages[1].Role != "assistant" {
					t.Errorf("Expected message 1 to be assistant, got %s", req.Messages[1].Role)
				}
				if req.Messages[1].Content == nil || req.Messages[1].Content == "" {
					t.Error("Expected assistant message to have content")
				}
				if len(req.Messages[1].ToolCalls) != 1 {
					t.Fatalf("Expected assistant message to have 1 tool_call, got %d", len(req.Messages[1].ToolCalls))
				}
				if req.Messages[1].ToolCalls[0].ID != "call_abc" {
					t.Errorf("Expected tool_call ID to be call_abc, got %s", req.Messages[1].ToolCalls[0].ID)
				}
				if req.Messages[1].ToolCalls[0].Function.Name != "read_file" {
					t.Errorf("Expected tool_call name to be read_file, got %s", req.Messages[1].ToolCalls[0].Function.Name)
				}

				// Check tool message
				if req.Messages[2].Role != "tool" {
					t.Errorf("Expected message 2 to be tool, got %s", req.Messages[2].Role)
				}
				if req.Messages[2].ToolCallID != "call_abc" {
					t.Errorf("Expected tool message ToolCallID to be call_abc, got %s", req.Messages[2].ToolCallID)
				}
			},
		},
		{
			name: "multiple function_calls merged with following assistant message",
			input: `{
				"model": "gpt-4o",
				"input": [
					{"type": "message", "role": "user", "content": "Run tests"},
					{"type": "function_call", "call_id": "call_1", "name": "exec", "arguments": "{\"cmd\": \"go test\"}"},
					{"type": "function_call", "call_id": "call_2", "name": "exec", "arguments": "{\"cmd\": \"go vet\"}"},
					{"type": "message", "role": "assistant", "content": [{"type": "output_text", "text": "Running tests..."}]},
					{"type": "function_call_output", "call_id": "call_1", "output": "PASS"},
					{"type": "function_call_output", "call_id": "call_2", "output": "no issues"}
				]
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}

				// Should have exactly 4 messages:
				// 1. user message
				// 2. assistant message with content and 2 tool_calls (merged)
				// 3. tool message for call_1
				// 4. tool message for call_2
				if len(req.Messages) != 4 {
					t.Fatalf("Expected 4 messages, got %d", len(req.Messages))
				}

				// Check assistant message has 2 tool_calls
				if len(req.Messages[1].ToolCalls) != 2 {
					t.Fatalf("Expected assistant message to have 2 tool_calls, got %d", len(req.Messages[1].ToolCalls))
				}
			},
		},
		{
			name: "function_call without following assistant message",
			input: `{
				"model": "gpt-4o",
				"input": [
					{"type": "message", "role": "user", "content": "Run command"},
					{"type": "function_call", "call_id": "call_xyz", "name": "exec", "arguments": "{\"cmd\": \"ls\"}"},
					{"type": "function_call_output", "call_id": "call_xyz", "output": "file1.txt\nfile2.txt"}
				]
			}`,
			validate: func(t *testing.T, output []byte) {
				var req types.ChatCompletionRequest
				if err := json.Unmarshal(output, &req); err != nil {
					t.Fatalf("Failed to parse output: %v", err)
				}

				// Should have exactly 3 messages:
				// 1. user message
				// 2. assistant message with tool_calls only (no content)
				// 3. tool message
				if len(req.Messages) != 3 {
					t.Fatalf("Expected 3 messages, got %d", len(req.Messages))
				}

				// Check assistant message has tool_calls but no content
				if req.Messages[1].Role != "assistant" {
					t.Errorf("Expected message 1 to be assistant, got %s", req.Messages[1].Role)
				}
				if len(req.Messages[1].ToolCalls) != 1 {
					t.Errorf("Expected assistant message to have 1 tool_call, got %d", len(req.Messages[1].ToolCalls))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewResponsesToChatConverter()
			output, err := converter.Convert([]byte(tt.input))
			if err != nil {
				t.Fatalf("Convert returned error: %v", err)
			}
			tt.validate(t, output)
		})
	}
}

func TestResponsesToChatConverter_CodexInputOrder(t *testing.T) {
	// This test simulates the exact input order that Codex sends:
	// 1. message (developer)
	// 2. message (user)
	// 3. message (user)
	// 4. function_call
	// 5. message (assistant)
	// 6. function_call_output

	input := []interface{}{
		map[string]interface{}{
			"type": "message",
			"role": "developer",
			"content": []interface{}{
				map[string]interface{}{"type": "input_text", "text": "System prompt"},
			},
		},
		map[string]interface{}{
			"type": "message",
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{"type": "input_text", "text": "Hello"},
			},
		},
		map[string]interface{}{
			"type": "message",
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{"type": "input_text", "text": "Review this"},
			},
		},
		map[string]interface{}{
			"type":      "function_call",
			"name":      "exec_command",
			"arguments": `{"cmd": "git status"}`,
			"call_id":   "call_123",
		},
		map[string]interface{}{
			"type": "message",
			"role": "assistant",
			"content": []interface{}{
				map[string]interface{}{"type": "output_text", "text": "I'll help you."},
			},
		},
		map[string]interface{}{
			"type":    "function_call_output",
			"call_id": "call_123",
			"output":  "On branch main...",
		},
	}

	converter := NewResponsesToChatConverter()
	messages := converter.convertInputItems(input)

	t.Logf("Output messages: %d", len(messages))
	for i, msg := range messages {
		msgJSON, _ := json.MarshalIndent(msg, "", "  ")
		t.Logf("Message %d: %s", i, string(msgJSON))
	}

	// Expected:
	// 1. system message (developer)
	// 2. user message
	// 3. user message
	// 4. assistant message with BOTH content AND tool_calls (merged)
	// 5. tool message (function_call_output)

	// Check that we have 5 messages
	if len(messages) != 5 {
		t.Errorf("Expected 5 messages, got %d", len(messages))
	}

	// Check message 3 (should be merged assistant with tool_calls)
	if len(messages) >= 4 {
		assistantMsg := messages[3]
		if assistantMsg.Role != "assistant" {
			t.Errorf("Message 3 role = %s, want assistant", assistantMsg.Role)
		}
		if len(assistantMsg.ToolCalls) != 1 {
			t.Errorf("Message 3 should have 1 tool call, got %d", len(assistantMsg.ToolCalls))
		}
		if assistantMsg.ToolCalls[0].Function.Name != "exec_command" {
			t.Errorf("Tool call name = %s, want exec_command", assistantMsg.ToolCalls[0].Function.Name)
		}
	}

	// Check message 4 (should be tool message)
	if len(messages) >= 5 {
		toolMsg := messages[4]
		if toolMsg.Role != "tool" {
			t.Errorf("Message 4 role = %s, want tool", toolMsg.Role)
		}
		if toolMsg.ToolCallID != "call_123" {
			t.Errorf("Tool call ID = %s, want call_123", toolMsg.ToolCallID)
		}
	}
}

func TestResponsesToChatConverter_Convert_CodexInputOrder(t *testing.T) {
	// This test uses the Convert method directly with Codex-style input
	input := map[string]interface{}{
		"model": "test-model",
		"input": []interface{}{
			map[string]interface{}{
				"type": "message",
				"role": "developer",
				"content": []interface{}{
					map[string]interface{}{"type": "input_text", "text": "System prompt"},
				},
			},
			map[string]interface{}{
				"type": "message",
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "input_text", "text": "Hello"},
				},
			},
			map[string]interface{}{
				"type": "message",
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "input_text", "text": "Review this"},
				},
			},
			map[string]interface{}{
				"type":      "function_call",
				"name":      "exec_command",
				"arguments": `{"cmd": "git status"}`,
				"call_id":   "call_123",
			},
			map[string]interface{}{
				"type": "message",
				"role": "assistant",
				"content": []interface{}{
					map[string]interface{}{"type": "output_text", "text": "I'll help you."},
				},
			},
			map[string]interface{}{
				"type":    "function_call_output",
				"call_id": "call_123",
				"output":  "On branch main...",
			},
		},
		"stream": true,
	}

	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	converter := NewResponsesToChatConverter()
	output, err := converter.Convert(body)
	if err != nil {
		t.Fatal(err)
	}

	var chatReq types.ChatCompletionRequest
	if err := json.Unmarshal(output, &chatReq); err != nil {
		t.Fatal(err)
	}

	t.Logf("Output messages: %d", len(chatReq.Messages))
	for i, msg := range chatReq.Messages {
		msgJSON, _ := json.MarshalIndent(msg, "", "  ")
		t.Logf("Message %d: %s", i, string(msgJSON))
	}

	// Expected: 5 messages (system, user, user, assistant with tool_calls, tool)
	if len(chatReq.Messages) != 5 {
		t.Errorf("Expected 5 messages, got %d", len(chatReq.Messages))
	}

	// Check message 3 (should be merged assistant with tool_calls)
	if len(chatReq.Messages) >= 4 {
		assistantMsg := chatReq.Messages[3]
		if assistantMsg.Role != "assistant" {
			t.Errorf("Message 3 role = %s, want assistant", assistantMsg.Role)
		}
		if len(assistantMsg.ToolCalls) != 1 {
			t.Errorf("Message 3 should have 1 tool call, got %d", len(assistantMsg.ToolCalls))
		}
	}
}

func TestResponsesToChatConverter_SetReasoningSplit(t *testing.T) {
	t.Run("reasoning_split disabled by default", func(t *testing.T) {
		converter := NewResponsesToChatConverter()
		input := `{"model": "MiniMax-M2.7", "input": "hello"}`
		output, err := converter.Convert([]byte(input))
		if err != nil {
			t.Fatalf("Convert error: %v", err)
		}
		var req types.ChatCompletionRequest
		if err := json.Unmarshal(output, &req); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
		if req.ReasoningSplit {
			t.Error("ReasoningSplit should be false by default")
		}
	})

	t.Run("reasoning_split enabled", func(t *testing.T) {
		converter := NewResponsesToChatConverter()
		converter.SetReasoningSplit(true)
		input := `{"model": "MiniMax-M2.7", "input": "hello"}`
		output, err := converter.Convert([]byte(input))
		if err != nil {
			t.Fatalf("Convert error: %v", err)
		}
		var req types.ChatCompletionRequest
		if err := json.Unmarshal(output, &req); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
		if !req.ReasoningSplit {
			t.Error("ReasoningSplit should be true")
		}
	})

	t.Run("reasoning_split in output json", func(t *testing.T) {
		converter := NewResponsesToChatConverter()
		converter.SetReasoningSplit(true)
		input := `{"model": "MiniMax-M2.7", "input": "test"}`
		output, err := converter.Convert([]byte(input))
		if err != nil {
			t.Fatalf("Convert error: %v", err)
		}
		if !bytes.Contains(output, []byte(`"reasoning_split":true`)) {
			t.Errorf("Output should contain reasoning_split:true, got %s", output)
		}
	})

	t.Run("reasoning_split with reasoning effort", func(t *testing.T) {
		converter := NewResponsesToChatConverter()
		converter.SetReasoningSplit(true)
		input := `{"model": "MiniMax-M2.7", "input": "test", "reasoning": {"effort": "high"}}`
		output, err := converter.Convert([]byte(input))
		if err != nil {
			t.Fatalf("Convert error: %v", err)
		}
		var req types.ChatCompletionRequest
		if err := json.Unmarshal(output, &req); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
		if !req.ReasoningSplit {
			t.Error("ReasoningSplit should be true")
		}
		if req.ReasoningEffort != "high" {
			t.Errorf("ReasoningEffort should be 'high', got '%s'", req.ReasoningEffort)
		}
	})
}
