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

		textContent, toolCalls, reasoning, toolResults := convertAnthropicContentBlocksToOpenAI(blocks)

		if textContent != "" {
			t.Errorf("expected empty content, got %q", textContent)
		}
		if len(toolCalls) != 0 {
			t.Errorf("expected 0 tool calls, got %d", len(toolCalls))
		}
		if reasoning == nil {
			t.Fatal("expected non-nil reasoning")
		}
		if *reasoning != "Let me analyze this." {
			t.Errorf("expected reasoning %q, got %q", "Let me analyze this.", *reasoning)
		}
		if len(toolResults) != 0 {
			t.Errorf("expected 0 tool results, got %d", len(toolResults))
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

		_, _, reasoning, _ := convertAnthropicContentBlocksToOpenAI(blocks)

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

		textContent, toolCalls, reasoning, toolResults := convertAnthropicContentBlocksToOpenAI(blocks)

		if textContent != "Here is the answer." {
			t.Errorf("expected text content, got %q", textContent)
		}
		if len(toolCalls) != 0 {
			t.Errorf("expected 0 tool calls, got %d", len(toolCalls))
		}
		if reasoning == nil {
			t.Fatal("expected non-nil reasoning")
		}
		if *reasoning != "Analyzing..." {
			t.Errorf("expected reasoning %q, got %q", "Analyzing...", *reasoning)
		}
		if len(toolResults) != 0 {
			t.Errorf("expected 0 tool results, got %d", len(toolResults))
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

		textContent, toolCalls, reasoning, toolResults := convertAnthropicContentBlocksToOpenAI(blocks)

		if textContent != "" {
			t.Errorf("expected empty content, got %q", textContent)
		}
		if len(toolCalls) != 1 {
			t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
		}
		if toolCalls[0].ID != "tool_1" {
			t.Errorf("expected tool call ID tool_1, got %s", toolCalls[0].ID)
		}
		if reasoning == nil {
			t.Fatal("expected non-nil reasoning")
		}
		if *reasoning != "I should call a tool." {
			t.Errorf("expected reasoning %q, got %q", "I should call a tool.", *reasoning)
		}
		if len(toolResults) != 0 {
			t.Errorf("expected 0 tool results, got %d", len(toolResults))
		}
	})

	t.Run("no thinking blocks returns nil reasoning", func(t *testing.T) {
		blocks := []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "Just text.",
			},
		}

		textContent, _, reasoning, _ := convertAnthropicContentBlocksToOpenAI(blocks)

		if textContent != "Just text." {
			t.Errorf("expected text content, got %q", textContent)
		}
		if reasoning != nil {
			t.Errorf("expected nil reasoning, got %v", *reasoning)
		}
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

		_, _, reasoning, _ := convertAnthropicContentBlocksToOpenAI(blocks)

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

		textContent, toolCalls, reasoning, toolResults := convertAnthropicContentBlocksToOpenAI(blocks)

		if textContent != "Let me check something.\nFinal answer." {
			t.Errorf("expected combined text, got %q", textContent)
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
		if len(toolResults) != 0 {
			t.Errorf("expected 0 tool results, got %d", len(toolResults))
		}
	})
}

func TestConvertAnthropicContentBlocksToOpenAI_ToolResult(t *testing.T) {
	t.Run("single tool_result block", func(t *testing.T) {
		blocks := []interface{}{
			map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": "call_123",
				"content":     "Result from tool",
			},
		}

		textContent, toolCalls, reasoning, toolResults := convertAnthropicContentBlocksToOpenAI(blocks)

		if textContent != "" {
			t.Errorf("expected empty text content, got %q", textContent)
		}
		if len(toolCalls) != 0 {
			t.Errorf("expected 0 tool calls, got %d", len(toolCalls))
		}
		if reasoning != nil {
			t.Errorf("expected nil reasoning, got %v", *reasoning)
		}
		if len(toolResults) != 1 {
			t.Fatalf("expected 1 tool result, got %d", len(toolResults))
		}
		if toolResults[0].ToolUseID != "call_123" {
			t.Errorf("expected tool_use_id 'call_123', got %s", toolResults[0].ToolUseID)
		}
		if toolResults[0].Content != "Result from tool" {
			t.Errorf("expected content 'Result from tool', got %s", toolResults[0].Content)
		}
	})

	t.Run("multiple tool_result blocks", func(t *testing.T) {
		blocks := []interface{}{
			map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": "call_001",
				"content":     "First tool result",
			},
			map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": "call_002",
				"content":     "Second tool result",
			},
		}

		_, _, _, toolResults := convertAnthropicContentBlocksToOpenAI(blocks)

		if len(toolResults) != 2 {
			t.Fatalf("expected 2 tool results, got %d", len(toolResults))
		}
		if toolResults[0].ToolUseID != "call_001" {
			t.Errorf("expected tool_use_id 'call_001', got %s", toolResults[0].ToolUseID)
		}
		if toolResults[1].ToolUseID != "call_002" {
			t.Errorf("expected tool_use_id 'call_002', got %s", toolResults[1].ToolUseID)
		}
	})

	t.Run("tool_result with array content extracts text", func(t *testing.T) {
		blocks := []interface{}{
			map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": "call_abc",
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Line 1",
					},
					map[string]interface{}{
						"type": "text",
						"text": "Line 2",
					},
				},
			},
		}

		_, _, _, toolResults := convertAnthropicContentBlocksToOpenAI(blocks)

		if len(toolResults) != 1 {
			t.Fatalf("expected 1 tool result, got %d", len(toolResults))
		}
		if toolResults[0].Content != "Line 1\nLine 2" {
			t.Errorf("expected content 'Line 1\\nLine 2', got %s", toolResults[0].Content)
		}
	})

	t.Run("skips tool_result without tool_use_id", func(t *testing.T) {
		blocks := []interface{}{
			map[string]interface{}{
				"type":    "tool_result",
				"content": "No ID",
			},
			map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": "call_valid",
				"content":     "Valid result",
			},
		}

		_, _, _, toolResults := convertAnthropicContentBlocksToOpenAI(blocks)

		if len(toolResults) != 1 {
			t.Fatalf("expected 1 tool result (skipped invalid), got %d", len(toolResults))
		}
		if toolResults[0].ToolUseID != "call_valid" {
			t.Errorf("expected tool_use_id 'call_valid', got %s", toolResults[0].ToolUseID)
		}
	})

	t.Run("mixed content with text and tool_result", func(t *testing.T) {
		blocks := []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "Here's some context",
			},
			map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": "call_x",
				"content":     "Tool output",
			},
		}

		textContent, _, _, toolResults := convertAnthropicContentBlocksToOpenAI(blocks)

		if textContent != "Here's some context" {
			t.Errorf("expected text content, got %q", textContent)
		}
		if len(toolResults) != 1 {
			t.Fatalf("expected 1 tool result, got %d", len(toolResults))
		}
		if toolResults[0].ToolUseID != "call_x" {
			t.Errorf("expected tool_use_id 'call_x', got %s", toolResults[0].ToolUseID)
		}
	})
}

func TestConvertAnthropicMessageToOpenAI(t *testing.T) {
	t.Run("string content returns single message", func(t *testing.T) {
		anthMsg := types.MessageInput{
			Role:    "user",
			Content: "Hello",
		}

		msgs := convertAnthropicMessageToOpenAI(anthMsg)

		if len(msgs) != 1 {
			t.Fatalf("expected 1 message, got %d", len(msgs))
		}
		if msgs[0].Role != "user" {
			t.Errorf("expected role 'user', got %s", msgs[0].Role)
		}
		if msgs[0].Content != "Hello" {
			t.Errorf("expected content 'Hello', got %v", msgs[0].Content)
		}
	})

	t.Run("pure tool_result expands to multiple tool messages", func(t *testing.T) {
		anthMsg := types.MessageInput{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": "call_01",
					"content":     "Result 1",
				},
				map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": "call_02",
					"content":     "Result 2",
				},
			},
		}

		msgs := convertAnthropicMessageToOpenAI(anthMsg)

		if len(msgs) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(msgs))
		}
		if msgs[0].Role != "tool" {
			t.Errorf("expected role 'tool', got %s", msgs[0].Role)
		}
		if msgs[0].ToolCallID != "call_01" {
			t.Errorf("expected tool_call_id 'call_01', got %s", msgs[0].ToolCallID)
		}
		if msgs[1].ToolCallID != "call_02" {
			t.Errorf("expected tool_call_id 'call_02', got %s", msgs[1].ToolCallID)
		}
	})

	t.Run("mixed content (text + tool_result) expands to multiple messages", func(t *testing.T) {
		anthMsg := types.MessageInput{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Here's some context",
				},
				map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": "call_x",
					"content":     "Tool output",
				},
			},
		}

		msgs := convertAnthropicMessageToOpenAI(anthMsg)

			// Tool messages must come first to immediately follow assistant's
			// tool_calls; then the user text message
			if len(msgs) != 2 {
				t.Fatalf("expected 2 messages, got %d", len(msgs))
			}
			if msgs[0].Role != "tool" {
				t.Errorf("expected role 'tool' for message 0, got %s", msgs[0].Role)
			}
			if msgs[0].ToolCallID != "call_x" {
				t.Errorf("expected tool_call_id 'call_x', got %s", msgs[0].ToolCallID)
			}
			if msgs[1].Role != "user" {
				t.Errorf("expected role 'user' for message 1, got %s", msgs[1].Role)
			}
			if msgs[1].Content != "Here's some context" {
				t.Errorf("expected content 'Here's some context', got %v", msgs[1].Content)
			}
		})

	t.Run("assistant with tool_use returns single message", func(t *testing.T) {
		anthMsg := types.MessageInput{
			Role: "assistant",
			Content: []interface{}{
				map[string]interface{}{
					"type":  "tool_use",
					"id":    "call_abc",
					"name":  "search",
					"input": map[string]interface{}{"q": "test"},
				},
			},
		}

		msgs := convertAnthropicMessageToOpenAI(anthMsg)

		if len(msgs) != 1 {
			t.Fatalf("expected 1 message, got %d", len(msgs))
		}
		if msgs[0].Role != "assistant" {
			t.Errorf("expected role 'assistant', got %s", msgs[0].Role)
		}
		if len(msgs[0].ToolCalls) != 1 {
			t.Errorf("expected 1 tool call, got %d", len(msgs[0].ToolCalls))
		}
		if msgs[0].ToolCalls[0].ID != "call_abc" {
			t.Errorf("expected tool call ID 'call_abc', got %s", msgs[0].ToolCalls[0].ID)
		}
	})

	t.Run("tool_result only with no other content returns only tool messages", func(t *testing.T) {
		anthMsg := types.MessageInput{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": "call_only",
					"content":     "Just a result",
				},
			},
		}

		msgs := convertAnthropicMessageToOpenAI(anthMsg)

		if len(msgs) != 1 {
			t.Fatalf("expected 1 message, got %d", len(msgs))
		}
		if msgs[0].Role != "tool" {
			t.Errorf("expected role 'tool', got %s", msgs[0].Role)
		}
	})
}

func TestTransformAnthropicToChat_ToolResultExpansion(t *testing.T) {
	t.Run("multiple tool_results expand to separate tool messages", func(t *testing.T) {
		body := `{
			"model": "claude-3-opus",
			"max_tokens": 1024,
			"messages": [
				{"role": "user", "content": "What is the weather?"},
				{"role": "assistant", "content": [
					{"type": "tool_use", "id": "call_01", "name": "get_weather", "input": {"city": "NYC"}},
					{"type": "tool_use", "id": "call_02", "name": "get_weather", "input": {"city": "LA"}}
				]},
				{"role": "user", "content": [
					{"type": "tool_result", "tool_use_id": "call_01", "content": "NYC: 72°F"},
					{"type": "tool_result", "tool_use_id": "call_02", "content": "LA: 68°F"}
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

		// Should have 4 messages: user, assistant (with 2 tool_calls), tool, tool
		if len(req.Messages) != 4 {
			t.Fatalf("expected 4 messages, got %d", len(req.Messages))
		}

		// First message: user
		if req.Messages[0].Role != "user" {
			t.Errorf("expected role 'user' for message 0, got %s", req.Messages[0].Role)
		}

		// Second message: assistant with tool_calls
		if req.Messages[1].Role != "assistant" {
			t.Errorf("expected role 'assistant' for message 1, got %s", req.Messages[1].Role)
		}
		if len(req.Messages[1].ToolCalls) != 2 {
			t.Errorf("expected 2 tool_calls, got %d", len(req.Messages[1].ToolCalls))
		}

		// Third message: tool response for call_01
		if req.Messages[2].Role != "tool" {
			t.Errorf("expected role 'tool' for message 2, got %s", req.Messages[2].Role)
		}
		if req.Messages[2].ToolCallID != "call_01" {
			t.Errorf("expected tool_call_id 'call_01' for message 2, got %s", req.Messages[2].ToolCallID)
		}
		if req.Messages[2].Content != "NYC: 72°F" {
			t.Errorf("expected content 'NYC: 72°F' for message 2, got %v", req.Messages[2].Content)
		}

		// Fourth message: tool response for call_02
		if req.Messages[3].Role != "tool" {
			t.Errorf("expected role 'tool' for message 3, got %s", req.Messages[3].Role)
		}
		if req.Messages[3].ToolCallID != "call_02" {
			t.Errorf("expected tool_call_id 'call_02' for message 3, got %s", req.Messages[3].ToolCallID)
		}
		if req.Messages[3].Content != "LA: 68°F" {
			t.Errorf("expected content 'LA: 68°F' for message 3, got %v", req.Messages[3].Content)
		}
	})

	t.Run("single tool_result produces single tool message", func(t *testing.T) {
		body := `{
			"model": "claude-3-opus",
			"max_tokens": 1024,
			"messages": [
				{"role": "user", "content": "Check the weather"},
				{"role": "assistant", "content": [
					{"type": "tool_use", "id": "call_single", "name": "get_weather", "input": {"city": "Paris"}}
				]},
				{"role": "user", "content": [
					{"type": "tool_result", "tool_use_id": "call_single", "content": "Paris: 65°F"}
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

		// Should have 3 messages: user, assistant, tool
		if len(req.Messages) != 3 {
			t.Fatalf("expected 3 messages, got %d", len(req.Messages))
		}

		if req.Messages[2].Role != "tool" {
			t.Errorf("expected role 'tool' for message 2, got %s", req.Messages[2].Role)
		}
		if req.Messages[2].ToolCallID != "call_single" {
			t.Errorf("expected tool_call_id 'call_single', got %s", req.Messages[2].ToolCallID)
		}
	})

	t.Run("tool_result with array content blocks", func(t *testing.T) {
		body := `{
			"model": "claude-3-opus",
			"max_tokens": 1024,
			"messages": [
				{"role": "user", "content": "Search"},
				{"role": "assistant", "content": [
					{"type": "tool_use", "id": "call_search", "name": "search", "input": {"q": "test"}}
				]},
				{"role": "user", "content": [
					{"type": "tool_result", "tool_use_id": "call_search", "content": [
						{"type": "text", "text": "Result 1"},
						{"type": "text", "text": "Result 2"}
					]}
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

		if len(req.Messages) != 3 {
			t.Fatalf("expected 3 messages, got %d", len(req.Messages))
		}

		// Content should be concatenated text
		if req.Messages[2].Content != "Result 1\nResult 2" {
			t.Errorf("expected content 'Result 1\\nResult 2', got %v", req.Messages[2].Content)
		}
	})

	t.Run("mixed content with text and tool_result expands correctly", func(t *testing.T) {
		body := `{
			"model": "claude-3-opus",
			"max_tokens": 1024,
			"messages": [
				{"role": "user", "content": "Check this"},
				{"role": "assistant", "content": [
					{"type": "tool_use", "id": "call_mixed", "name": "analyze", "input": {}}
				]},
				{"role": "user", "content": [
					{"type": "text", "text": "Additional context here"},
					{"type": "tool_result", "tool_use_id": "call_mixed", "content": "Analysis result"}
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

		// Should have 4 messages: user, assistant, tool, user (text)
		if len(req.Messages) != 4 {
			t.Fatalf("expected 4 messages, got %d", len(req.Messages))
		}
			// Third message: tool response (must immediately follow assistant's tool_calls)
			if req.Messages[2].Role != "tool" {
				t.Errorf("expected role 'tool' for message 2, got %s", req.Messages[2].Role)
			}
			if req.Messages[2].ToolCallID != "call_mixed" {
				t.Errorf("expected tool_call_id 'call_mixed', got %s", req.Messages[2].ToolCallID)
			}
			if req.Messages[2].Content != "Analysis result" {
				t.Errorf("expected content 'Analysis result', got %v", req.Messages[2].Content)
			}

			// Fourth message: user with text context
			if req.Messages[3].Role != "user" {
				t.Errorf("expected role 'user' for message 3, got %s", req.Messages[3].Role)
			}
			if req.Messages[3].Content != "Additional context here" {
				t.Errorf("expected content 'Additional context here', got %v", req.Messages[3].Content)
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
		if assistantMsg.Content != "Hi there!" {
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
		if assistantMsg.Content != "" {
			t.Errorf("expected empty content, got %v", assistantMsg.Content)
		}
	})
}
