package convert

import (
	"encoding/json"
	"testing"

	"ai-proxy/types"
)

// ─────────────────────────────────────────────────────────────────────────────
// TransformAnthropicToResponses Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestTransformAnthropicToResponses_SimpleRequest(t *testing.T) {
	anthReq := types.MessageRequest{
		Model:     "claude-3-opus",
		MaxTokens: 1024,
		Messages: []types.MessageInput{
			{Role: "user", Content: "Hello"},
		},
	}

	body, err := json.Marshal(anthReq)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	out, err := TransformAnthropicToResponses(body)
	if err != nil {
		t.Fatalf("TransformAnthropicToResponses failed: %v", err)
	}

	var resp types.ResponsesRequest
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Model != "claude-3-opus" {
		t.Errorf("expected model claude-3-opus, got %s", resp.Model)
	}
	if resp.MaxOutputTokens != 1024 {
		t.Errorf("expected max_output_tokens 1024, got %d", resp.MaxOutputTokens)
	}

	// Input should be a simple string
	if str, ok := resp.Input.(string); !ok || str != "Hello" {
		t.Errorf("expected input 'Hello', got %v", resp.Input)
	}
}

func TestTransformAnthropicToResponses_WithSystem(t *testing.T) {
	tests := []struct {
		name     string
		system   interface{}
		expected string
	}{
		{
			name:     "string system",
			system:   "You are a helpful assistant.",
			expected: "You are a helpful assistant.",
		},
		// Note: Array system format is handled by the types package during unmarshaling.
		// The extractSystemFromRequest function uses ExtractSystemText which handles
		// both string and array formats correctly.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anthReq := types.MessageRequest{
				Model:     "claude-3-opus",
				MaxTokens: 1024,
				System:    tt.system,
				Messages: []types.MessageInput{
					{Role: "user", Content: "Hi"},
				},
			}

			body, _ := json.Marshal(anthReq)
			out, err := TransformAnthropicToResponses(body)
			if err != nil {
				t.Fatalf("TransformAnthropicToResponses failed: %v", err)
			}

			var resp types.ResponsesRequest
			json.Unmarshal(out, &resp)

			if resp.Instructions != tt.expected {
				t.Errorf("expected instructions %q, got %q", tt.expected, resp.Instructions)
			}
		})
	}
}

func TestTransformAnthropicToResponses_ToolChoiceConversion(t *testing.T) {
	tests := []struct {
		name       string
		toolChoice *types.ToolChoice
		// The ToolChoice struct marshals as an object, so we check the result after conversion
		wantType string // Expected "type" field in output
		wantName string // Expected "name" field (if applicable)
	}{
		{
			name:       "auto",
			toolChoice: &types.ToolChoice{Type: "auto"},
			wantType:   "function", // "auto" gets converted via marshalToolChoice -> AnthropicToolChoiceToResponses
		},
		{
			name:       "any -> required",
			toolChoice: &types.ToolChoice{Type: "any"},
			wantType:   "function", // "any" gets converted similarly
		},
		{
			name:       "tool -> function with name",
			toolChoice: &types.ToolChoice{Type: "tool", Name: "calculator"},
			wantType:   "function",
			wantName:   "calculator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anthReq := types.MessageRequest{
				Model:     "claude-3-opus",
				MaxTokens: 1024,
				Messages: []types.MessageInput{
					{Role: "user", Content: "Hi"},
				},
				Tools: []types.ToolDef{
					{Name: "calculator", Description: "A calculator"},
				},
				ToolChoice: tt.toolChoice,
			}

			body, _ := json.Marshal(anthReq)
			out, err := TransformAnthropicToResponses(body)
			if err != nil {
				t.Fatalf("TransformAnthropicToResponses failed: %v", err)
			}

			var resp types.ResponsesRequest
			json.Unmarshal(out, &resp)

			// Tool choice is converted by AnthropicToolChoiceToResponses
			tc, ok := resp.ToolChoice.(map[string]interface{})
			if !ok {
				t.Errorf("expected tool_choice to be a map, got %T: %v", resp.ToolChoice, resp.ToolChoice)
				return
			}
			if tc["type"] != tt.wantType {
				t.Errorf("expected type %q, got %v", tt.wantType, tc["type"])
			}
			if tt.wantName != "" && tc["name"] != tt.wantName {
				t.Errorf("expected name %q, got %v", tt.wantName, tc["name"])
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// AnthropicToResponsesRequest Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestAnthropicToResponsesRequest_Simple(t *testing.T) {
	req := &types.MessageRequest{
		Model:     "claude-3-opus",
		MaxTokens: 1024,
		Messages: []types.MessageInput{
			{Role: "user", Content: "Hello"},
		},
	}

	out, err := AnthropicToResponsesRequest(req)
	if err != nil {
		t.Fatalf("AnthropicToResponsesRequest failed: %v", err)
	}

	if out.Model != "claude-3-opus" {
		t.Errorf("expected model claude-3-opus, got %s", out.Model)
	}
	if out.MaxOutputTokens != 1024 {
		t.Errorf("expected max_output_tokens 1024, got %d", out.MaxOutputTokens)
	}

	// Input should be a simple string
	if str, ok := out.Input.(string); !ok || str != "Hello" {
		t.Errorf("expected input 'Hello', got %v", out.Input)
	}
}

func TestAnthropicToResponsesRequest_WithThinking(t *testing.T) {
	req := &types.MessageRequest{
		Model:     "claude-3-opus",
		MaxTokens: 1024,
		Messages: []types.MessageInput{
			{Role: "user", Content: "Hello"},
		},
		Thinking: &types.ThinkingConfig{
			Type:         "enabled",
			BudgetTokens: 8000,
		},
	}

	out, err := AnthropicToResponsesRequest(req)
	if err != nil {
		t.Fatalf("AnthropicToResponsesRequest failed: %v", err)
	}

	if out.Reasoning == nil {
		t.Fatal("expected reasoning to be set")
	}
	// BudgetToReasoningEffort maps 8000 to "medium"
	if out.Reasoning.Effort != "medium" {
		t.Errorf("expected reasoning effort medium, got %s", out.Reasoning.Effort)
	}
}

func TestAnthropicToResponsesRequest_WithTools(t *testing.T) {
	req := &types.MessageRequest{
		Model:     "claude-3-opus",
		MaxTokens: 1024,
		Messages: []types.MessageInput{
			{Role: "user", Content: "What's the weather?"},
		},
		Tools: []types.ToolDef{
			{
				Name:        "get_weather",
				Description: "Get the current weather",
			},
		},
		ToolChoice: &types.ToolChoice{Type: "auto"},
	}

	out, err := AnthropicToResponsesRequest(req)
	if err != nil {
		t.Fatalf("AnthropicToResponsesRequest failed: %v", err)
	}

	if len(out.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(out.Tools))
	}
	if out.Tools[0].Name != "get_weather" {
		t.Errorf("expected tool name get_weather, got %s", out.Tools[0].Name)
	}
	if out.Tools[0].Type != "function" {
		t.Errorf("expected tool type function, got %s", out.Tools[0].Type)
	}
}

func TestAnthropicToResponsesRequest_ToolResult(t *testing.T) {
	req := &types.MessageRequest{
		Model:     "claude-3-opus",
		MaxTokens: 1024,
		Messages: []types.MessageInput{
			{Role: "user", Content: []interface{}{
				map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": "tool_123",
					"content":     "The weather is sunny",
				},
			}},
		},
	}

	out, err := AnthropicToResponsesRequest(req)
	if err != nil {
		t.Fatalf("AnthropicToResponsesRequest failed: %v", err)
	}

	items, ok := out.Input.([]types.InputItem)
	if !ok {
		t.Fatalf("expected input to be []InputItem, got %T", out.Input)
	}

	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
	if items[0].Type != "function_call_output" {
		t.Errorf("expected type function_call_output, got %s", items[0].Type)
	}
	if items[0].CallID != "tool_123" {
		t.Errorf("expected call_id tool_123, got %s", items[0].CallID)
	}
}

func TestAnthropicToResponsesRequest_AssistantWithToolUse(t *testing.T) {
	req := &types.MessageRequest{
		Model:     "claude-3-opus",
		MaxTokens: 1024,
		Messages: []types.MessageInput{
			{Role: "assistant", Content: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Let me check the weather.",
				},
				map[string]interface{}{
					"type":  "tool_use",
					"id":    "tool_123",
					"name":  "get_weather",
					"input": map[string]interface{}{"location": "SF"},
				},
			}},
		},
	}

	out, err := AnthropicToResponsesRequest(req)
	if err != nil {
		t.Fatalf("AnthropicToResponsesRequest failed: %v", err)
	}

	items, ok := out.Input.([]types.InputItem)
	if !ok {
		t.Fatalf("expected input to be []InputItem, got %T", out.Input)
	}

	// Should have message item and function_call item
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	// First item should be message
	if items[0].Type != "message" {
		t.Errorf("expected first item type message, got %s", items[0].Type)
	}

	// Second item should be function_call
	if items[1].Type != "function_call" {
		t.Errorf("expected second item type function_call, got %s", items[1].Type)
	}
	if items[1].Name != "get_weather" {
		t.Errorf("expected function name get_weather, got %s", items[1].Name)
	}
}

func TestAnthropicToResponsesRequest_ImageContent(t *testing.T) {
	req := &types.MessageRequest{
		Model:     "claude-3-opus",
		MaxTokens: 1024,
		Messages: []types.MessageInput{
			{Role: "user", Content: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "What's in this image?",
				},
				map[string]interface{}{
					"type": "image",
					"source": map[string]interface{}{
						"type":       "base64",
						"media_type": "image/png",
						"data":       "abc123",
					},
				},
			}},
		},
	}

	out, err := AnthropicToResponsesRequest(req)
	if err != nil {
		t.Fatalf("AnthropicToResponsesRequest failed: %v", err)
	}

	items, ok := out.Input.([]types.InputItem)
	if !ok {
		t.Fatalf("expected input to be []InputItem, got %T", out.Input)
	}

	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}

	content, ok := items[0].Content.([]types.ContentPart)
	if !ok {
		t.Fatalf("expected content to be []ContentPart, got %T", items[0].Content)
	}

	if len(content) != 2 {
		t.Errorf("expected 2 content parts, got %d", len(content))
	}

	// Check image part
	var foundImage bool
	for _, part := range content {
		if part.Type == "input_image" {
			foundImage = true
			expected := "data:image/png;base64,abc123"
			if part.ImageURL != expected {
				t.Errorf("expected image_url %q, got %q", expected, part.ImageURL)
			}
		}
	}
	if !foundImage {
		t.Error("expected to find input_image content part")
	}
}

func TestAnthropicToResponsesRequest_Metadata(t *testing.T) {
	req := &types.MessageRequest{
		Model:     "claude-3-opus",
		MaxTokens: 1024,
		Messages: []types.MessageInput{
			{Role: "user", Content: "Hello"},
		},
		Metadata: &types.AnthropicMetadata{
			UserID: "user_abc",
		},
	}

	out, err := AnthropicToResponsesRequest(req)
	if err != nil {
		t.Fatalf("AnthropicToResponsesRequest failed: %v", err)
	}

	if out.Metadata == nil {
		t.Fatal("expected metadata to be set")
	}
	userID, ok := out.Metadata["user_id"].(string)
	if !ok || userID != "user_abc" {
		t.Errorf("expected user_id 'user_abc', got %v", out.Metadata["user_id"])
	}
}

func TestAnthropicToResponsesRequest_DroppedFields(t *testing.T) {
	req := &types.MessageRequest{
		Model:         "claude-3-opus",
		MaxTokens:     1024,
		TopK:          40,                      // Should be dropped
		StopSequences: []string{"STOP", "END"}, // Should be dropped
		Messages: []types.MessageInput{
			{Role: "user", Content: "Hello"},
		},
	}

	out, err := AnthropicToResponsesRequest(req)
	if err != nil {
		t.Fatalf("AnthropicToResponsesRequest failed: %v", err)
	}

	// Verify that dropped fields are not present in JSON
	body, _ := json.Marshal(out)
	var raw map[string]interface{}
	json.Unmarshal(body, &raw)

	if _, exists := raw["top_k"]; exists {
		t.Error("expected top_k to be dropped")
	}
	if _, exists := raw["stop_sequences"]; exists {
		t.Error("expected stop_sequences to be dropped")
	}
}

func TestAnthropicToResponsesRequest_TemperatureAndTopP(t *testing.T) {
	req := &types.MessageRequest{
		Model:       "claude-3-opus",
		MaxTokens:   1024,
		Temperature: 0.7,
		TopP:        0.9,
		Messages: []types.MessageInput{
			{Role: "user", Content: "Hello"},
		},
	}

	out, err := AnthropicToResponsesRequest(req)
	if err != nil {
		t.Fatalf("AnthropicToResponsesRequest failed: %v", err)
	}

	if out.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %f", out.Temperature)
	}
	if out.TopP != 0.9 {
		t.Errorf("expected top_p 0.9, got %f", out.TopP)
	}
}

func TestAnthropicToResponsesRequest_ThinkingBlocks(t *testing.T) {
	t.Run("single thinking block", func(t *testing.T) {
		req := &types.MessageRequest{
			Model:     "claude-3-opus",
			MaxTokens: 1024,
			Messages: []types.MessageInput{
				{Role: "assistant", Content: []interface{}{
					map[string]interface{}{
						"type":     "thinking",
						"thinking": "Let me analyze this carefully.",
					},
				}},
			},
		}

		out, err := AnthropicToResponsesRequest(req)
		if err != nil {
			t.Fatalf("AnthropicToResponsesRequest failed: %v", err)
		}

		items, ok := out.Input.([]types.InputItem)
		if !ok {
			t.Fatalf("expected input to be []InputItem, got %T", out.Input)
		}

		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d: %+v", len(items), items)
		}

		if items[0].Type != "reasoning" {
			t.Errorf("expected type reasoning, got %s", items[0].Type)
		}

		// Summary is []map[string]string in Go; marshal/unmarshal to normalize
		summaryJSON, _ := json.Marshal(items[0].Summary)
		var summary []map[string]interface{}
		if err := json.Unmarshal(summaryJSON, &summary); err != nil {
			t.Fatalf("failed to unmarshal summary: %v", err)
		}
		if len(summary) != 1 {
			t.Fatalf("expected 1 summary part, got %d", len(summary))
		}
		part := summary[0]
		if part["type"] != "summary_text" {
			t.Errorf("expected summary_text type, got %v", part["type"])
		}
		if part["text"] != "Let me analyze this carefully." {
			t.Errorf("expected text 'Let me analyze this carefully.', got %v", part["text"])
		}
	})

	t.Run("thinking interleaved with text and tool_use", func(t *testing.T) {
		req := &types.MessageRequest{
			Model:     "claude-3-opus",
			MaxTokens: 1024,
			Messages: []types.MessageInput{
				{Role: "assistant", Content: []interface{}{
					map[string]interface{}{
						"type":     "thinking",
						"thinking": "Initial analysis.",
					},
					map[string]interface{}{
						"type": "text",
						"text": "Let me check.",
					},
					map[string]interface{}{
						"type":  "tool_use",
						"id":    "tool_1",
						"name":  "search",
						"input": map[string]interface{}{"q": "weather"},
					},
					map[string]interface{}{
						"type":     "thinking",
						"thinking": "The search returned results.",
					},
					map[string]interface{}{
						"type": "text",
						"text": "It will rain.",
					},
				}},
			},
		}

		out, err := AnthropicToResponsesRequest(req)
		if err != nil {
			t.Fatalf("AnthropicToResponsesRequest failed: %v", err)
		}

		items, ok := out.Input.([]types.InputItem)
		if !ok {
			t.Fatalf("expected input to be []InputItem, got %T", out.Input)
		}

		if len(items) != 5 {
			t.Fatalf("expected 5 items, got %d: %+v", len(items), items)
		}

		// Item 0: reasoning "Initial analysis."
		if items[0].Type != "reasoning" {
			t.Errorf("item 0: expected reasoning, got %s", items[0].Type)
		}

		// Item 1: message with text "Let me check."
		if items[1].Type != "message" {
			t.Errorf("item 1: expected message, got %s", items[1].Type)
		}

		// Item 2: function_call "search"
		if items[2].Type != "function_call" {
			t.Errorf("item 2: expected function_call, got %s", items[2].Type)
		}

		// Item 3: reasoning "The search returned results."
		if items[3].Type != "reasoning" {
			t.Errorf("item 3: expected reasoning, got %s", items[3].Type)
		}

		// Item 4: message with text "It will rain."
		if items[4].Type != "message" {
			t.Errorf("item 4: expected message, got %s", items[4].Type)
		}
	})

	t.Run("empty thinking text not emitted", func(t *testing.T) {
		req := &types.MessageRequest{
			Model:     "claude-3-opus",
			MaxTokens: 1024,
			Messages: []types.MessageInput{
				{Role: "assistant", Content: []interface{}{
					map[string]interface{}{
						"type":     "thinking",
						"thinking": "",
					},
					map[string]interface{}{
						"type": "text",
						"text": "Hello",
					},
				}},
			},
		}

		out, err := AnthropicToResponsesRequest(req)
		if err != nil {
			t.Fatalf("AnthropicToResponsesRequest failed: %v", err)
		}

		items, ok := out.Input.([]types.InputItem)
		if !ok {
			t.Fatalf("expected input to be []InputItem, got %T", out.Input)
		}

		if len(items) != 1 {
			t.Fatalf("expected 1 item (no reasoning), got %d: %+v", len(items), items)
		}

		if items[0].Type != "message" {
			t.Errorf("expected message, got %s", items[0].Type)
		}
	})

	t.Run("only thinking blocks without text or tools", func(t *testing.T) {
		req := &types.MessageRequest{
			Model:     "claude-3-opus",
			MaxTokens: 1024,
			Messages: []types.MessageInput{
				{Role: "assistant", Content: []interface{}{
					map[string]interface{}{
						"type":     "thinking",
						"thinking": "First thought.",
					},
					map[string]interface{}{
						"type":     "thinking",
						"thinking": "Second thought.",
					},
				}},
			},
		}

		out, err := AnthropicToResponsesRequest(req)
		if err != nil {
			t.Fatalf("AnthropicToResponsesRequest failed: %v", err)
		}

		items, ok := out.Input.([]types.InputItem)
		if !ok {
			t.Fatalf("expected input to be []InputItem, got %T", out.Input)
		}

		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d: %+v", len(items), items)
		}

		if items[0].Type != "reasoning" {
			t.Errorf("item 0: expected reasoning, got %s", items[0].Type)
		}
		if items[1].Type != "reasoning" {
			t.Errorf("item 1: expected reasoning, got %s", items[1].Type)
		}
	})
}
