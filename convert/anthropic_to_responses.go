package convert

import (
	"encoding/json"
	"fmt"

	"ai-proxy/types"
)

// ─────────────────────────────────────────────────────────────────────────────
// Anthropic → Responses — Converter
// ─────────────────────────────────────────────────────────────────────────────

// TransformAnthropicToResponses converts an Anthropic MessageRequest body to ResponsesRequest format.
// This is the primary entry point for converting Anthropic requests to Responses API format.
//
// Dropped fields (no Responses API equivalent):
//   - top_k: No equivalent in Responses API
//   - stop_sequences: No stop field in Responses API
//   - thinking blocks in messages: No equivalent in Responses API input
func TransformAnthropicToResponses(body []byte) ([]byte, error) {
	var req types.MessageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic request: %w", err)
	}

	out, err := AnthropicToResponsesRequest(&req)
	if err != nil {
		return nil, err
	}

	return json.Marshal(out)
}

// ─────────────────────────────────────────────────────────────────────────────
// Anthropic → Responses — Request
// ─────────────────────────────────────────────────────────────────────────────

// AnthropicToResponsesRequest converts an Anthropic MessageRequest into a Responses Request.
//
// Dropped: top_k, stop_sequences (Responses API has no stop field).
func AnthropicToResponsesRequest(req *types.MessageRequest) (*types.ResponsesRequest, error) {
	stream := req.Stream
	out := &types.ResponsesRequest{
		Model:        req.Model,
		Stream:       &stream,
		Instructions: extractSystemFromRequest(req.System),
	}

	if req.MaxTokens > 0 {
		out.MaxOutputTokens = req.MaxTokens
	}

	if req.Temperature > 0 {
		out.Temperature = req.Temperature
	}

	if req.TopP > 0 {
		out.TopP = req.TopP
	}

	if req.Metadata != nil && req.Metadata.UserID != "" {
		out.Metadata = map[string]interface{}{"user_id": req.Metadata.UserID}
	}

	// thinking → reasoning
	if req.Thinking != nil && req.Thinking.Type == "enabled" {
		out.Reasoning = &types.ReasoningConfig{
			Effort: BudgetToReasoningEffort(req.Thinking.BudgetTokens),
		}
	}

	// tools
	for _, t := range req.Tools {
		out.Tools = append(out.Tools, types.ResponsesTool{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		})
	}
	if len(out.Tools) > 0 {
		out.ToolChoice = AnthropicToolChoiceToResponses(marshalToolChoice(req.ToolChoice))
	}

	input, err := anthropicMessagesToResponsesInput(req.Messages)
	if err != nil {
		return nil, fmt.Errorf("converting messages: %w", err)
	}
	out.Input = input
	return out, nil
}

func anthropicMessagesToResponsesInput(msgs []types.MessageInput) (interface{}, error) {
	if len(msgs) == 0 {
		return "", nil
	}

	var items []types.InputItem
	for _, msg := range msgs {
		msgItems, err := anthropicMessageToResponsesItems(msg)
		if err != nil {
			return nil, err
		}
		items = append(items, msgItems...)
	}

	if len(items) == 1 && items[0].Type == "message" && items[0].Role == "user" {
		// Return as simple string if single user message
		if str, ok := items[0].Content.(string); ok {
			return str, nil
		}
	}
	return items, nil
}

func anthropicMessageToResponsesItems(msg types.MessageInput) ([]types.InputItem, error) {
	switch content := msg.Content.(type) {
	case string:
		if msg.Role == "assistant" {
			return []types.InputItem{{
				Type: "message",
				Role: "assistant",
				Content: []types.ContentPart{{
					Type: "output_text",
					Text: content,
				}},
			}}, nil
		}
		return []types.InputItem{{
			Type:    "message",
			Role:    msg.Role,
			Content: content,
		}}, nil
	case []interface{}:
		return anthropicContentBlocksToResponsesItems(msg.Role, content)
	case json.RawMessage:
		// Try string first
		var s string
		if err := json.Unmarshal(content, &s); err == nil {
			if msg.Role == "assistant" {
				return []types.InputItem{{
					Type: "message",
					Role: "assistant",
					Content: []types.ContentPart{{
						Type: "output_text",
						Text: s,
					}},
				}}, nil
			}
			return []types.InputItem{{
				Type:    "message",
				Role:    msg.Role,
				Content: s,
			}}, nil
		}
		// Try as array of blocks
		var blocks []interface{}
		if err := json.Unmarshal(content, &blocks); err == nil {
			return anthropicContentBlocksToResponsesItems(msg.Role, blocks)
		}
		return nil, fmt.Errorf("unknown content format")
	default:
		return nil, fmt.Errorf("unknown content type: %T", content)
	}
}

func anthropicContentBlocksToResponsesItems(role string, blocks []interface{}) ([]types.InputItem, error) {
	switch role {
	case "user":
		return anthropicUserBlocksToResponsesItems(blocks)
	case "assistant":
		return anthropicAssistantBlocksToResponsesItems(blocks)
	default:
		return nil, fmt.Errorf("unknown role: %s", role)
	}
}

func anthropicUserBlocksToResponsesItems(blocks []interface{}) ([]types.InputItem, error) {
	var items []types.InputItem

	// Group all user blocks into a single message item
	var content []types.ContentPart
	var functionOutputItems []types.InputItem

	for _, block := range blocks {
		b, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		blockType, _ := b["type"].(string)

		switch blockType {
		case "text":
			text, _ := b["text"].(string)
			content = append(content, types.ContentPart{
				Type: "input_text",
				Text: text,
			})
		case "image":
			// Convert Anthropic image block to Responses format
			source, ok := b["source"].(map[string]interface{})
			if !ok {
				continue
			}
			imageURL, err := anthropicImageSourceToURL(source)
			if err != nil {
				return nil, err
			}
			content = append(content, types.ContentPart{
				Type:     "input_image",
				ImageURL: imageURL,
			})
		case "tool_result":
			// Convert tool_result to a function_call_output item
			toolUseID, _ := b["tool_use_id"].(string)
			content := extractToolResultContent(b["content"])
			functionOutputItems = append(functionOutputItems, types.InputItem{
				Type:   "function_call_output",
				CallID: toolUseID,
				Output: content,
			})
		}
	}

	// Build items: message first (if has content), then function outputs
	if len(content) > 0 {
		items = append(items, types.InputItem{
			Type:    "message",
			Role:    "user",
			Content: content,
		})
	}
	items = append(items, functionOutputItems...)

	return items, nil
}

func anthropicAssistantBlocksToResponsesItems(blocks []interface{}) ([]types.InputItem, error) {
	var items []types.InputItem
	var messageContent []types.ContentPart

	for _, block := range blocks {
		b, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		blockType, _ := b["type"].(string)

		switch blockType {
		case "text":
			text, _ := b["text"].(string)
			messageContent = append(messageContent, types.ContentPart{
				Type: "output_text",
				Text: text,
			})
		case "thinking":
			// Drop thinking blocks - no equivalent in Responses API input
		case "tool_use":
			id, _ := b["id"].(string)
			name, _ := b["name"].(string)
			inputBytes, _ := json.Marshal(b["input"])

			items = append(items, types.InputItem{
				Type:      "function_call",
				CallID:    id,
				Name:      name,
				Arguments: string(inputBytes),
			})
		}
	}

	// Add message item first if there's text content
	if len(messageContent) > 0 {
		items = append([]types.InputItem{{
			Type:    "message",
			Role:    "assistant",
			Content: messageContent,
		}}, items...)
	}

	return items, nil
}

func anthropicImageSourceToURL(src map[string]interface{}) (string, error) {
	srcType, _ := src["type"].(string)
	switch srcType {
	case "base64":
		mediaType, _ := src["media_type"].(string)
		data, _ := src["data"].(string)
		return BuildDataURI(mediaType, data), nil
	case "url":
		url, _ := src["url"].(string)
		return url, nil
	default:
		return "", fmt.Errorf("unknown image source type: %q", srcType)
	}
}
