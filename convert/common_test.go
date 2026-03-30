package convert

import (
	"ai-proxy/types"
	"encoding/json"
	"reflect"
	"testing"
)

// TestConvertOpenAIToolsToAnthropic tests the conversion from OpenAI to Anthropic tools.
func TestConvertOpenAIToolsToAnthropic(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`)

	tests := []struct {
		name     string
		input    []types.Tool
		expected []types.ToolDef
	}{
		{
			name:     "empty slice",
			input:    []types.Tool{},
			expected: []types.ToolDef{},
		},
		{
			name:     "nil slice",
			input:    nil,
			expected: []types.ToolDef{},
		},
		{
			name: "single function tool",
			input: []types.Tool{
				{
					Type: "function",
					Function: types.ToolFunction{
						Name:        "search",
						Description: "Search the web",
						Parameters:  schema,
					},
				},
			},
			expected: []types.ToolDef{
				{
					Name:        "search",
					Description: "Search the web",
					InputSchema: schema,
				},
			},
		},
		{
			name: "skip non-function tools",
			input: []types.Tool{
				{
					Type: "custom",
					Function: types.ToolFunction{
						Name: "custom_tool",
					},
				},
				{
					Type: "function",
					Function: types.ToolFunction{
						Name:        "search",
						Description: "Search",
						Parameters:  schema,
					},
				},
			},
			expected: []types.ToolDef{
				{
					Name:        "search",
					Description: "Search",
					InputSchema: schema,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertOpenAIToolsToAnthropic(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d tools, got %d", len(tt.expected), len(result))
				return
			}

			for i, tool := range result {
				exp := tt.expected[i]
				if tool.Name != exp.Name {
					t.Errorf("tool %d: expected name %q, got %q", i, exp.Name, tool.Name)
				}
				if tool.Description != exp.Description {
					t.Errorf("tool %d: expected description %q, got %q", i, exp.Description, tool.Description)
				}
			}
		})
	}
}

// TestExtractTextFromContent tests text extraction from various content formats.
func TestExtractTextFromContent(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "nil content",
			input:    nil,
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "simple string",
			input:    "Hello, world!",
			expected: "Hello, world!",
		},
		{
			name:     "empty array",
			input:    []interface{}{},
			expected: "",
		},
		{
			name: "single text block",
			input: []interface{}{
				map[string]interface{}{"type": "text", "text": "Hello"},
			},
			expected: "Hello",
		},
		{
			name: "multiple text blocks",
			input: []interface{}{
				map[string]interface{}{"type": "text", "text": "Hello"},
				map[string]interface{}{"type": "text", "text": "World"},
			},
			expected: "Hello\nWorld",
		},
		{
			name: "input_text block",
			input: []interface{}{
				map[string]interface{}{"type": "input_text", "text": "Input text"},
			},
			expected: "Input text",
		},
		{
			name: "mixed content blocks",
			input: []interface{}{
				map[string]interface{}{"type": "text", "text": "Text"},
				map[string]interface{}{"type": "image", "source": "data"},
				map[string]interface{}{"type": "text", "text": "More text"},
			},
			expected: "Text\nMore text",
		},
		{
			name:     "unknown type",
			input:    12345,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTextFromContent(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestExtractSystemText tests system extraction from Anthropic system payloads.
func TestExtractSystemText(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "json string",
			input:    json.RawMessage(`"You are helpful."`),
			expected: "You are helpful.",
		},
		{
			name:     "raw block array",
			input:    json.RawMessage(`[{"type":"text","text":"You are helpful."},{"type":"text","text":"Be concise."}]`),
			expected: "You are helpful.Be concise.",
		},
		{
			name: "structured blocks preserve order",
			input: []interface{}{
				map[string]interface{}{"type": "text", "text": "One"},
				map[string]interface{}{"type": "text", "text": "Two"},
			},
			expected: "OneTwo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractSystemText(tt.input); got != tt.expected {
				t.Errorf("ExtractSystemText() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestToolChoiceHelpers tests tool choice normalization helpers.
func TestToolChoiceHelpers(t *testing.T) {
	anthropicCases := []struct {
		name     string
		input    interface{}
		expected *types.ToolChoice
	}{
		{
			name:     "string auto",
			input:    "auto",
			expected: &types.ToolChoice{Type: "auto"},
		},
		{
			name:     "string required",
			input:    "required",
			expected: &types.ToolChoice{Type: "any"},
		},
		{
			name:  "string none",
			input: "none",
		},
		{
			name:     "flat responses object",
			input:    map[string]interface{}{"type": "function", "name": "search"},
			expected: &types.ToolChoice{Type: "tool", Name: "search"},
		},
		{
			name:     "nested function object",
			input:    map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "search"}},
			expected: &types.ToolChoice{Type: "tool", Name: "search"},
		},
	}

	for _, tt := range anthropicCases {
		t.Run("anthropic_"+tt.name, func(t *testing.T) {
			got := ConvertToolChoiceOpenAIToAnthropic(tt.input)
			if tt.expected == nil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %+v, got nil", tt.expected)
			}
			if got.Type != tt.expected.Type || got.Name != tt.expected.Name {
				t.Fatalf("got %+v, want %+v", got, tt.expected)
			}
		})
	}

	responsesCases := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "string auto",
			input:    "auto",
			expected: "auto",
		},
		{
			name:     "flat responses object",
			input:    map[string]interface{}{"type": "function", "name": "search"},
			expected: map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "search"}},
		},
		{
			name:     "nested function object",
			input:    map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "search"}},
			expected: map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "search"}},
		},
	}

	for _, tt := range responsesCases {
		t.Run("responses_"+tt.name, func(t *testing.T) {
			got := ConvertResponsesToolChoiceToOpenAI(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Fatalf("got %#v, want %#v", got, tt.expected)
			}
		})
	}
}
