package convert

import (
	"encoding/json"
	"fmt"

	"ai-proxy/types"
)

// ─────────────────────────────────────────────────────────────────────────────
// Anthropic → OpenAI Chat — Converter
// ─────────────────────────────────────────────────────────────────────────────

// TransformAnthropicToChat converts an Anthropic MessageRequest body to
// OpenAI ChatCompletionRequest format. This is the primary entry point for
// converting Anthropic /v1/messages requests to /v1/chat/completions format.
//
// Mapped fields:
//   - model → model
//   - max_tokens → max_tokens
//   - system (string or content blocks) → system (top-level string)
//   - messages → messages (role + content conversion)
//   - tools (input_schema) → tools (parameters)
//   - temperature, top_p → temperature, top_p
//   - stream → stream (forced true)
//
// @param body Raw Anthropic MessageRequest JSON.
// @return OpenAI ChatCompletionRequest JSON.
// @return Error if parsing or conversion fails.
func TransformAnthropicToChat(body []byte) ([]byte, error) {
	var anthReq types.MessageRequest
	if err := json.Unmarshal(body, &anthReq); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic request: %w", err)
	}

	// Preserve the client's streaming preference.
	// When stream:false, the proxy uses a non-SSE response path.
	stream := anthReq.Stream

	openReq := types.ChatCompletionRequest{
		Model:       anthReq.Model,
		MaxTokens:   anthReq.MaxTokens,
		Stream:      stream,
		Temperature: anthReq.Temperature,
		TopP:        anthReq.TopP,
	}

	// Request usage statistics from upstream only when streaming
	if stream {
		openReq.StreamOptions = &types.StreamOptions{IncludeUsage: true}
	}

	// Convert system message (may be string or array of content blocks)
	openReq.System = ExtractTextFromContent(anthReq.System)
	// Convert messages array (handles content blocks with tool use/results)
	openReq.Messages = convertAnthropicMessagesToOpenAI(anthReq.Messages)
	// Convert tool definitions (Anthropic input_schema → OpenAI parameters)
	openReq.Tools = ConvertAnthropicToolsToOpenAI(anthReq.Tools)

	return json.Marshal(openReq)
}

// ─────────────────────────────────────────────────────────────────────────────
// Anthropic → OpenAI Chat — Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

// convertAnthropicMessagesToOpenAI transforms a slice of Anthropic messages to
// OpenAI format.
func convertAnthropicMessagesToOpenAI(anthMsgs []types.MessageInput) []types.Message {
	openMsgs := make([]types.Message, 0, len(anthMsgs))
	for _, anthMsg := range anthMsgs {
		openMsgs = append(openMsgs, convertAnthropicMessageToOpenAI(anthMsg))
	}
	return openMsgs
}

// convertAnthropicMessageToOpenAI transforms a single Anthropic message to
// OpenAI format.
func convertAnthropicMessageToOpenAI(anthMsg types.MessageInput) types.Message {
	openMsg := types.Message{Role: anthMsg.Role}

	switch content := anthMsg.Content.(type) {
	case string:
		openMsg.Content = content
	case []interface{}:
		openMsg.Content, openMsg.ToolCalls, openMsg.ToolCallID, openMsg.ReasoningContent = convertAnthropicContentBlocksToOpenAI(content)
		// Only pure tool_result turns can be represented as OpenAI tool messages.
		if openMsg.ToolCallID != "" && isPureAnthropicToolResultTurn(content) {
			openMsg.Role = "tool"
		}
	}

	return openMsg
}

// isPureAnthropicToolResultTurn checks if all blocks in a content array are
// tool_result blocks.
func isPureAnthropicToolResultTurn(blocks []interface{}) bool {
	if len(blocks) == 0 {
		return false
	}

	for _, item := range blocks {
		block, ok := item.(map[string]interface{})
		if !ok {
			return false
		}
		blockType, _ := block["type"].(string)
		if blockType != "tool_result" {
			return false
		}
	}

	return true
}

// convertAnthropicContentBlocksToOpenAI extracts text content, tool calls,
// tool result IDs, and reasoning content from Anthropic content blocks.
// Returns reasoning content as *string (nil if no thinking blocks, pointer
// to concatenated text otherwise).
func convertAnthropicContentBlocksToOpenAI(blocks []interface{}) (interface{}, []types.ToolCall, string, *string) {
	var textContent string
	var toolCalls []types.ToolCall
	var toolCallID string
	var thinkingText string

	for _, item := range blocks {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		switch m["type"] {
		case "text":
			if text, ok := m["text"].(string); ok {
				if textContent != "" {
					textContent += "\n"
				}
				textContent += text
			}
		case "thinking":
			if text, ok := m["thinking"].(string); ok && text != "" {
				if thinkingText != "" {
					thinkingText += "\n"
				}
				thinkingText += text
			}
		case "tool_use":
			if id, ok := m["id"].(string); ok {
				if name, ok := m["name"].(string); ok {
					input, _ := json.Marshal(m["input"])
					toolCalls = append(toolCalls, types.ToolCall{
						ID:   id,
						Type: "function",
						Function: types.Function{
							Name:      name,
							Arguments: string(input),
						},
					})
				}
			}
		case "tool_result":
			if id, ok := m["tool_use_id"].(string); ok {
				toolCallID = id
			}
			if content, ok := m["content"]; ok {
				switch c := content.(type) {
				case string:
					if textContent != "" {
						textContent += "\n"
					}
					textContent += c
				case []interface{}:
					for _, block := range c {
						if blockMap, ok := block.(map[string]interface{}); ok {
							if blockType, ok := blockMap["type"].(string); ok && blockType == "text" {
								if t, ok := blockMap["text"].(string); ok {
									if textContent != "" {
										textContent += "\n"
									}
									textContent += t
								}
							}
						}
					}
				}
			}
		}
	}

	var reasoning *string
	if thinkingText != "" {
		reasoning = &thinkingText
	}
	return textContent, toolCalls, toolCallID, reasoning
}

// ConvertAnthropicToolsToOpenAI transforms Anthropic tool definitions to OpenAI
// format (exported for reuse).
func ConvertAnthropicToolsToOpenAI(anthTools []types.ToolDef) []types.Tool {
	if len(anthTools) == 0 {
		return nil
	}

	openTools := make([]types.Tool, 0, len(anthTools))
	for _, anthTool := range anthTools {
		openTools = append(openTools, types.Tool{
			Type: "function",
			Function: types.ToolFunction{
				Name:        anthTool.Name,
				Description: anthTool.Description,
				Parameters:  anthTool.InputSchema,
			},
		})
	}
	return openTools
}
