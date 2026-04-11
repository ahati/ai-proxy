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
		{
			name: "converts web_search_tool_result with encrypted_content to tool_result",
			input: `{
				"model": "test",
				"messages": [{
					"role": "assistant",
					"content": [{
						"type": "web_search_tool_result",
						"tool_use_id": "toolu_456",
						"content": [
							{"type": "web_search_result", "title": "Test", "url": "https://example.com", "encrypted_content": "Some content"}
						]
					}]
				}]
			}`,
			wantType: "tool_result",
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
			name: "converts web_search_result with content field (legacy)",
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
			name: "converts web_search_result with encrypted_content",
			input: []interface{}{
				map[string]interface{}{
					"type":              "web_search_result",
					"title":             "Encrypted Title",
					"url":               "https://example.com/enc",
					"encrypted_content": "Decrypted page content here",
				},
			},
			contains: "## Encrypted Title",
		},
		{
			name: "prefers encrypted_content over content",
			input: []interface{}{
				map[string]interface{}{
					"type":              "web_search_result",
					"title":             "Both Fields",
					"url":               "https://example.com/both",
					"encrypted_content": "from encrypted",
					"content":           "from plain",
				},
			},
			contains: "from encrypted",
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

func TestConvertServerWebSearchToFunctionTool(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantToolType  string
		wantToolName  string
		wantHasSchema bool
		wantModified  bool
	}{
		{
			name: "converts web_search_20250305 to custom tool",
			input: `{
				"model": "test",
				"tools": [
					{"type": "web_search_20250305", "name": "web_search", "max_uses": 8}
				],
				"messages": [{"role": "user", "content": "hello"}]
			}`,
			wantToolType:  "custom",
			wantToolName:  "web_search",
			wantHasSchema: true,
			wantModified:  true,
		},
		{
			name: "converts web_search_20250209 to custom tool",
			input: `{
				"model": "test",
				"tools": [
					{"type": "web_search_20250209", "name": "web_search", "max_uses": 5}
				],
				"messages": [{"role": "user", "content": "hello"}]
			}`,
			wantToolType:  "custom",
			wantToolName:  "web_search",
			wantHasSchema: true,
			wantModified:  true,
		},
		{
			name: "leaves non-web-search tools unchanged",
			input: `{
				"model": "test",
				"tools": [
					{"type": "custom", "name": "Bash", "input_schema": {"type": "object"}}
				],
				"messages": [{"role": "user", "content": "hello"}]
			}`,
			wantToolType:  "custom",
			wantToolName:  "Bash",
			wantHasSchema: true,
			wantModified:  false,
		},
		{
			name: "converts server_tool_use in messages to tool_use",
			input: `{
				"model": "test",
				"tools": [],
				"messages": [
					{"role": "assistant", "content": [
						{"type": "server_tool_use", "id": "toolu_1", "name": "web_search", "input": {"query": "test"}}
					]}
				]
			}`,
			wantModified: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertServerWebSearchToFunctionTool([]byte(tt.input))

			var req map[string]interface{}
			if err := json.Unmarshal(result, &req); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}

			// Check tools
			if tools, ok := req["tools"].([]interface{}); ok && len(tools) > 0 {
				tool := tools[0].(map[string]interface{})
				if tt.wantToolType != "" && tool["type"] != tt.wantToolType {
					t.Errorf("tool type = %v, want %v", tool["type"], tt.wantToolType)
				}
				if tt.wantToolName != "" && tool["name"] != tt.wantToolName {
					t.Errorf("tool name = %v, want %v", tool["name"], tt.wantToolName)
				}
				if tt.wantHasSchema {
					if _, hasSchema := tool["input_schema"]; !hasSchema {
						t.Error("expected input_schema to be present")
					}
				}
				// max_uses should be removed from converted tools
				if tool["type"] == "custom" && tool["name"] == "web_search" {
					if _, hasMaxUses := tool["max_uses"]; hasMaxUses {
						t.Error("max_uses should be removed from converted tool")
					}
				}
			}

			// Check messages for server_tool_use conversion
			if messages, ok := req["messages"].([]interface{}); ok {
				for _, msg := range messages {
					msgMap, _ := msg.(map[string]interface{})
					if content, ok := msgMap["content"].([]interface{}); ok {
						for _, block := range content {
							blockMap, _ := block.(map[string]interface{})
							if blockMap["type"] == "server_tool_use" {
								t.Error("server_tool_use should have been converted to tool_use")
							}
						}
					}
				}
			}
		})
	}
}
