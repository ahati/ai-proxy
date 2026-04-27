package convert

import (
	"encoding/json"
	"testing"

	"ai-proxy/types"
)

func TestConvertAnthropicContentBlocksToOpenAI_Thinking(t *testing.T) {
	t.Run("single thinking block", func(t *testing.T) {
		blocks := []interface{}{
			map[string]interface{}{
				"type":     "thinking",
				"thinking": "Let me analyze this.",
			},
		}

		content, toolCalls, toolCallID, reasoning := convertAnthropicContentBlocksToOpenAI(blocks)

		if content.(string) != "" {
			t.Errorf("expected empty content, got %q", content)
		}
		if len(toolCalls) != 0 {
			t.Errorf("expected 0 tool calls, got %d", len(toolCalls))
		}
		if toolCallID != "" {
			t.Errorf("expected empty toolCallID, got %q", toolCallID)
		}
		if reasoning == nil {
			t.Fatal("expected non-nil reasoning")
		}
		if *reasoning != "Let me analyze this." {
			t.Errorf("expected reasoning %q, got %q", "Let me analyze this.", *reasoning)
		}
	})

	t.Run("multiple thinking blocks concatenated", func(t *testing.T) {
		blocks := []interface{}{
			map[string]interface{}{
				"type":     "thinking",
				"thinking": "First thought.",
			},
			map[string]interface{}{
				"type":     "thinking",
				"thinking": "Second thought.",
			},
		}

		_, _, _, reasoning := convertAnthropicContentBlocksToOpenAI(blocks)

		if reasoning == nil {
			t.Fatal("expected non-nil reasoning")
		}
		if *reasoning != "First thought.\nSecond thought." {
			t.Errorf("expected concatenated reasoning, got %q", *reasoning)
		}
	})

	t.Run("thinking with text content", func(t *testing.T) {
		blocks := []interface{}{
			map[string]interface{}{
				"type":     "thinking",
				"thinking": "Analyzing...",
			},
			map[string]interface{}{
				"type": "text",
				"text": "Here is the answer.",
			},
		}

		content, toolCalls, toolCallID, reasoning := convertAnthropicContentBlocksToOpenAI(blocks)

		if content.(string) != "Here is the answer." {
			t.Errorf("expected text content, got %q", content)
		}
		if len(toolCalls) != 0 {
			t.Errorf("expected 0 tool calls, got %d", len(toolCalls))
		}
		if toolCallID != "" {
			t.Errorf("expected empty toolCallID, got %q", toolCallID)
		}
		if reasoning == nil {
			t.Fatal("expected non-nil reasoning")
		}
		if *reasoning != "Analyzing..." {
			t.Errorf("expected reasoning %q, got %q", "Analyzing...", *reasoning)
		}
	})

	t.Run("thinking with tool_use", func(t *testing.T) {
		blocks := []interface{}{
			map[string]interface{}{
				"type":     "thinking",
				"thinking": "I should call a tool.",
			},
			map[string]interface{}{
				"type":  "tool_use",
				"id":    "tool_1",
				"name":  "search",
				"input": map[string]interface{}{"q": "weather"},
			},
		}

		content, toolCalls, toolCallID, reasoning := convertAnthropicContentBlocksToOpenAI(blocks)

		if content.(string) != "" {
			t.Errorf("expected empty content, got %q", content)
		}
		if len(toolCalls) != 1 {
			t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
		}
		if toolCalls[0].ID != "tool_1" {
			t.Errorf("expected tool call ID tool_1, got %s", toolCalls[0].ID)
		}
		if toolCallID != "" {
			t.Errorf("expected empty toolCallID, got %q", toolCallID)
		}
		if reasoning == nil {
			t.Fatal("expected non-nil reasoning")
		}
		if *reasoning != "I should call a tool." {
			t.Errorf("expected reasoning %q, got %q", "I should call a tool.", *reasoning)
		}
	})

	t.Run("no thinking blocks returns nil reasoning", func(t *testing.T) {
		blocks := []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "Just text.",
			},
		}

		content, toolCalls, toolCallID, reasoning := convertAnthropicContentBlocksToOpenAI(blocks)

		if content.(string) != "Just text." {
			t.Errorf("expected text content, got %q", content)
		}
		if reasoning != nil {
			t.Errorf("expected nil reasoning, got %v", *reasoning)
		}
		_ = toolCalls
		_ = toolCallID
	})

	t.Run("empty thinking text is ignored", func(t *testing.T) {
		blocks := []interface{}{
			map[string]interface{}{
				"type":     "thinking",
				"thinking": "",
			},
			map[string]interface{}{
				"type": "text",
				"text": "Just text.",
			},
		}

		_, _, _, reasoning := convertAnthropicContentBlocksToOpenAI(blocks)

		if reasoning != nil {
			t.Errorf("expected nil reasoning for empty thinking, got %v", *reasoning)
		}
	})

	t.Run("thinking alongside text and tool_use", func(t *testing.T) {
		blocks := []interface{}{
			map[string]interface{}{
				"type":     "thinking",
				"thinking": "Reasoning step 1.",
			},
			map[string]interface{}{
				"type": "text",
				"text": "Let me check something.",
			},
			map[string]interface{}{
				"type":  "tool_use",
				"id":    "tool_a",
				"name":  "get_data",
				"input": map[string]interface{}{},
			},
			map[string]interface{}{
				"type":     "thinking",
				"thinking": "Reasoning step 2.",
			},
			map[string]interface{}{
				"type": "text",
				"text": "Final answer.",
			},
		}

		content, toolCalls, _, reasoning := convertAnthropicContentBlocksToOpenAI(blocks)

		if content.(string) != "Let me check something.\nFinal answer." {
			t.Errorf("expected combined text, got %q", content)
		}
		if len(toolCalls) != 1 {
			t.Errorf("expected 1 tool call, got %d", len(toolCalls))
		}
		if reasoning == nil {
			t.Fatal("expected non-nil reasoning")
		}
		expectedReasoning := "Reasoning step 1.\nReasoning step 2."
		if *reasoning != expectedReasoning {
			t.Errorf("expected reasoning %q, got %q", expectedReasoning, *reasoning)
		}
	})
}

func TestTransformAnthropicToChat_ThinkingBlocks(t *testing.T) {
	t.Run("thinking blocks convert to reasoning_content", func(t *testing.T) {
		body := `{
			"model": "claude-3-opus",
			"max_tokens": 1024,
			"messages": [
				{"role": "user", "content": "Hello"},
				{"role": "assistant", "content": [
					{"type": "thinking", "thinking": "Let me think..."},
					{"type": "text", "text": "Hi there!"}
				]}
			]
		}`

		result, err := TransformAnthropicToChat([]byte(body))
		if err != nil {
			t.Fatalf("TransformAnthropicToChat failed: %v", err)
		}

		var req types.ChatCompletionRequest
		if err := json.Unmarshal(result, &req); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if len(req.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(req.Messages))
		}

		assistantMsg := req.Messages[1]
		if assistantMsg.Role != "assistant" {
			t.Errorf("expected assistant role, got %s", assistantMsg.Role)
		}
		if assistantMsg.ReasoningContent == nil {
			t.Fatal("expected ReasoningContent to be set")
		}
		if *assistantMsg.ReasoningContent != "Let me think..." {
			t.Errorf("expected reasoning %q, got %q", "Let me think...", *assistantMsg.ReasoningContent)
		}
		if str, ok := assistantMsg.Content.(string); !ok || str != "Hi there!" {
			t.Errorf("expected content 'Hi there!', got %v", assistantMsg.Content)
		}
	})

	t.Run("no thinking blocks yields nil ReasoningContent", func(t *testing.T) {
		body := `{
			"model": "claude-3-opus",
			"max_tokens": 1024,
			"messages": [
				{"role": "user", "content": "Hello"},
				{"role": "assistant", "content": "Hi there!"}
			]
		}`

		result, err := TransformAnthropicToChat([]byte(body))
		if err != nil {
			t.Fatalf("TransformAnthropicToChat failed: %v", err)
		}

		var req types.ChatCompletionRequest
		if err := json.Unmarshal(result, &req); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		assistantMsg := req.Messages[1]
		if assistantMsg.ReasoningContent != nil {
			t.Errorf("expected nil ReasoningContent, got %v", *assistantMsg.ReasoningContent)
		}
	})

	t.Run("thinking-only blocks produce reasoning with empty content", func(t *testing.T) {
		body := `{
			"model": "claude-3-opus",
			"max_tokens": 1024,
			"messages": [
				{"role": "user", "content": "Hello"},
				{"role": "assistant", "content": [
					{"type": "thinking", "thinking": "Pure reasoning."}
				]}
			]
		}`

		result, err := TransformAnthropicToChat([]byte(body))
		if err != nil {
			t.Fatalf("TransformAnthropicToChat failed: %v", err)
		}

		var req types.ChatCompletionRequest
		if err := json.Unmarshal(result, &req); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		assistantMsg := req.Messages[1]
		if assistantMsg.ReasoningContent == nil {
			t.Fatal("expected ReasoningContent to be set")
		}
		if *assistantMsg.ReasoningContent != "Pure reasoning." {
			t.Errorf("expected reasoning 'Pure reasoning.', got %q", *assistantMsg.ReasoningContent)
		}
		// Content should be empty string for reasoning-only messages
		if str, ok := assistantMsg.Content.(string); !ok || str != "" {
			t.Errorf("expected empty content, got %v", assistantMsg.Content)
		}
	})
}
