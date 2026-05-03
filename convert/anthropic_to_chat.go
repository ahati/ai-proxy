package convert

import (
	"encoding/json"
	"fmt"
	"strings"

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

// ToolResultInfo holds extracted tool_result data for creating OpenAI tool messages.
type ToolResultInfo struct {
	ToolUseID string
	Content   string
}

// convertAnthropicMessagesToOpenAI transforms a slice of Anthropic messages to
// OpenAI format. A single Anthropic message with multiple tool_result blocks
// expands to multiple OpenAI tool messages.
func convertAnthropicMessagesToOpenAI(anthMsgs []types.MessageInput) []types.Message {
	openMsgs := make([]types.Message, 0, len(anthMsgs))
	for _, anthMsg := range anthMsgs {
		openMsgs = append(openMsgs, convertAnthropicMessageToOpenAI(anthMsg)...)
	}
	return openMsgs
}

// convertAnthropicMessageToOpenAI transforms a single Anthropic message to
// OpenAI format. Returns a slice because a single Anthropic user message with
// multiple tool_result blocks must become multiple OpenAI tool messages.
func convertAnthropicMessageToOpenAI(anthMsg types.MessageInput) []types.Message {
	switch content := anthMsg.Content.(type) {
	case string:
		return []types.Message{{Role: anthMsg.Role, Content: content}}
	case []interface{}:
		textContent, toolCalls, reasoning, toolResults := convertAnthropicContentBlocksToOpenAI(content)

		msgs := make([]types.Message, 0, 1+len(toolResults))

		// When a user message contains tool_results, emit tool messages FIRST.
		// OpenAI requires tool messages to immediately follow the assistant
		// message that contains tool_calls, so they must precede any text.
		if anthMsg.Role == "user" {
			for _, tr := range toolResults {
				msgs = append(msgs, types.Message{
					Role:       "tool",
					ToolCallID: tr.ToolUseID,
					Content:    tr.Content,
				})
			}
		}

		// Create the original role message if there's any non-tool_result content
		if textContent != "" || len(toolCalls) > 0 || reasoning != nil {
			msgs = append(msgs, types.Message{
				Role:             anthMsg.Role,
				Content:          textContent,
				ToolCalls:        toolCalls,
				ReasoningContent: reasoning,
			})
		}

		// For non-user roles (e.g. assistant), tool_results go after
		if anthMsg.Role != "user" {
			for _, tr := range toolResults {
				msgs = append(msgs, types.Message{
					Role:       "tool",
					ToolCallID: tr.ToolUseID,
					Content:    tr.Content,
				})
			}
		}

		return msgs
	default:
		return []types.Message{{Role: anthMsg.Role}}
	}
}

// convertAnthropicContentBlocksToOpenAI extracts text content, tool calls,
// reasoning content, and tool_result info from Anthropic content blocks.
// Returns:
//   - textContent: concatenated text from text blocks
//   - toolCalls: tool_use blocks converted to OpenAI format
//   - reasoning: thinking blocks concatenated (nil if none)
//   - toolResults: tool_result blocks as ToolResultInfo slice
func convertAnthropicContentBlocksToOpenAI(blocks []interface{}) (string, []types.ToolCall, *string, []ToolResultInfo) {
	var textContent string
	var toolCalls []types.ToolCall
	var thinkingText string
	var toolResults []ToolResultInfo

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
			toolUseID, _ := m["tool_use_id"].(string)
			if toolUseID == "" {
				continue
			}
			var resultContent string
			if content, ok := m["content"]; ok {
				switch c := content.(type) {
				case string:
					resultContent = c
				case []interface{}:
					var textParts []string
					for _, block := range c {
						if bm, ok := block.(map[string]interface{}); ok {
							if bm["type"] == "text" {
								if t, ok := bm["text"].(string); ok && t != "" {
									textParts = append(textParts, t)
								}
							}
						}
					}
					if len(textParts) > 0 {
						resultContent = strings.Join(textParts, "\n")
					}
				}
			}
			toolResults = append(toolResults, ToolResultInfo{
				ToolUseID: toolUseID,
				Content:   resultContent,
			})
		}
	}

	var reasoning *string
	if thinkingText != "" {
		reasoning = &thinkingText
	}
	return textContent, toolCalls, reasoning, toolResults
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
