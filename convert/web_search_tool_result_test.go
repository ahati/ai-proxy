package convert

import (
	"encoding/json"
	"testing"
)

func TestNormalizeWebSearchToolResultsInMessages(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string // expected type after conversion
	}{
		{
			name: "converts web_search_tool_result to tool_result",
			input: `{
				"model": "test",
				"messages": [{
					"role": "user",
					"content": [{
						"type": "web_search_tool_result",
						"tool_use_id": "toolu_123",
						"content": [
							{"type": "web_search_result", "title": "Test", "url": "https://example.com", "content": "Test content"}
						]
					}]
				}]
			}`,
			wantType: "tool_result",
		},
		{
			name: "leaves tool_result unchanged",
			input: `{
				"model": "test",
				"messages": [{
					"role": "user",
					"content": [{
						"type": "tool_result",
						"tool_use_id": "toolu_123",
						"content": "result"
					}]
				}]
			}`,
			wantType: "tool_result",
		},
		{
			name: "leaves text unchanged",
			input: `{
				"model": "test",
				"messages": [{
					"role": "user",
					"content": [{"type": "text", "text": "hello"}]
				}]
			}`,
			wantType: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeWebSearchToolResultsInMessages([]byte(tt.input))

			var req map[string]interface{}
			if err := json.Unmarshal(result, &req); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}

			messages := req["messages"].([]interface{})
			msg := messages[0].(map[string]interface{})
			content := msg["content"].([]interface{})
			block := content[0].(map[string]interface{})

			if block["type"] != tt.wantType {
				t.Errorf("got type %v, want %v", block["type"], tt.wantType)
			}
		})
	}
}

func TestConvertWebSearchContentToText(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		want     string
		contains string
	}{
		{
			name: "converts web_search_result array",
			input: []interface{}{
				map[string]interface{}{
					"type":    "web_search_result",
					"title":   "Test Title",
					"url":     "https://example.com",
					"content": "Test content",
				},
			},
			contains: "## Test Title",
		},
		{
			name:     "returns string as-is",
			input:    "simple string",
			want:     "simple string",
			contains: "",
		},
		{
			name: "handles error type",
			input: []interface{}{
				map[string]interface{}{
					"type":       "web_search_tool_result_error",
					"error_code": "timeout",
				},
			},
			contains: "Search error: timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertWebSearchContentToText(tt.input)

			str, ok := result.(string)
			if !ok {
				t.Fatalf("expected string result, got %T", result)
			}

			if tt.want != "" && str != tt.want {
				t.Errorf("got %q, want %q", str, tt.want)
			}
			if tt.contains != "" && !contains(str, tt.contains) {
				t.Errorf("result %q should contain %q", str, tt.contains)
			}
		})
	}
}
