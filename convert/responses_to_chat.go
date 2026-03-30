// Package convert provides converters between different API formats.
// This file implements OpenAI Responses API to OpenAI Chat Completions conversion.
package convert

import (
	"encoding/json"
	"fmt"
	"strings"

	"ai-proxy/conversation"
	"ai-proxy/logging"
	"ai-proxy/types"
)

// ResponsesToChatConverter converts OpenAI ResponsesRequest to ChatCompletionRequest.
type ResponsesToChatConverter struct {
	reasoningSplit bool
	cacheHit       bool
	shouldStore    bool // Controls whether to store conversation (default: true)
}

// NewResponsesToChatConverter creates a new converter for Responses to Chat format.
func NewResponsesToChatConverter() *ResponsesToChatConverter {
	return &ResponsesToChatConverter{
		shouldStore: true, // default to storing
	}
}

// SetReasoningSplit enables reasoning_split in the output ChatCompletionRequest.
// When enabled, supported providers like MiniMax will return reasoning in a
// separate reasoning_details field instead of embedded in content.
func (c *ResponsesToChatConverter) SetReasoningSplit(enabled bool) {
	c.reasoningSplit = enabled
}

// CacheHit returns true if the converter found a cached conversation during conversion.
func (c *ResponsesToChatConverter) CacheHit() bool {
	return c.cacheHit
}

// SetStore controls whether to store the conversation.
// Set to false when the request specifies store:false.
// Default is true (conversations are stored).
func (c *ResponsesToChatConverter) SetStore(store bool) {
	c.shouldStore = store
}

// Convert transforms a ResponsesRequest body to ChatCompletionRequest format.
func (c *ResponsesToChatConverter) Convert(body []byte) ([]byte, error) {
	var req types.ResponsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("failed to parse ResponsesRequest: %w", err)
	}

	chatReq := c.convertRequest(&req)
	return json.Marshal(chatReq)
}

// convertRequest transforms a ResponsesRequest to ChatCompletionRequest.
// When previous_response_id is provided and store is true, it walks the conversation chain
// from the store and prepends all history to the current input.
// When store is false, it skips the DB chain walk and uses encrypted_reasoning directly.
func (c *ResponsesToChatConverter) convertRequest(req *types.ResponsesRequest) *types.ChatCompletionRequest {
	var reasoningItemID string

	// Fetch conversation history chain if previous_response_id is provided and store is true
	// In ZDR mode (store:false), we skip the DB chain walk and rely on encrypted_reasoning
	if req.PreviousResponseID != "" && c.shouldStore {
		// TODO: Wire userID from request headers for proper ownership validation
		chain, err := conversation.WalkChainFromDefaultWithOwnership(req.PreviousResponseID, "")
		if err != nil {
			// Ownership error - return nil to trigger warning below
			logging.InfoMsg("Warning: Conversation access denied: %s", err.Error())
		} else if len(chain) > 0 {
			// Mark cache hit
			c.cacheHit = true
			// Prepend all conversations in the chain (oldest first)
			for _, hist := range chain {
				req.Input = prependHistoryToInput(hist, req.Input)
				// Capture reasoning_item_id from the most recent conversation
				// (the last one in the chain, which is the most recent turn)
				if hist.ReasoningItemID != "" {
					reasoningItemID = hist.ReasoningItemID
				}
			}
		} else {
			logging.InfoMsg("Warning: Previous response ID not found in conversation store: %s", req.PreviousResponseID)
		}
	}

	maxTokens := req.MaxOutputTokens
	if maxTokens == 0 {
		maxTokens = 65536 // Default max tokens (64k) for OpenAI-compatible APIs
	}

	// Respect the stream flag from the request, default to true for streaming
	stream := true
	if req.Stream != nil {
		stream = *req.Stream
	}

	chatReq := &types.ChatCompletionRequest{
		Model:       req.Model,
		MaxTokens:   maxTokens,
		Stream:      stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}

	// Request usage statistics in the final streaming chunk.
	if stream {
		chatReq.StreamOptions = &types.StreamOptions{
			IncludeUsage: true,
		}
	}

	// Convert parallel_tool_calls if set
	if req.ParallelToolCalls {
		chatReq.ParallelToolCalls = &req.ParallelToolCalls
	}

	// Convert input to messages
	chatReq.Messages = c.convertInput(req.Input)

	// Convert instructions to a prepended system message.
	if req.Instructions != "" {
		chatReq.Messages = append([]types.Message{{
			Role:    "system",
			Content: req.Instructions,
		}}, chatReq.Messages...)
	}

	// Convert tools
	chatReq.Tools = c.convertTools(req.Tools)

	// Convert tool_choice from Responses object form to Chat object form.
	chatReq.ToolChoice = c.convertToolChoice(req.ToolChoice)

	// Convert response_format
	chatReq.ResponseFormat = req.ResponseFormat

	// Forward reasoning effort to Chat Completions
	// Note: reasoning.summary ("concise"/"detailed") is not forwarded because
	// Chat Completions API has no equivalent field for summary style preference.
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		chatReq.ReasoningEffort = req.Reasoning.Effort
	}

	// Enable reasoning_split if configured (for MiniMax and similar providers)
	if c.reasoningSplit {
		chatReq.ReasoningSplit = true
	}

	// Pass reasoning_item_id to upstream for reasoning continuity across turns
	if reasoningItemID != "" {
		chatReq.ReasoningItemID = reasoningItemID
	}

	// Pass encrypted_reasoning through in ZDR mode
	// When store:false and encrypted_reasoning is provided, forward it to upstream
	if req.EncryptedReasoning != "" {
		chatReq.EncryptedReasoning = req.EncryptedReasoning
	}

	// Convert metadata.user_id to user field
	if req.Metadata != nil {
		if userID, ok := req.Metadata["user_id"]; ok {
			if s, ok := userID.(string); ok && s != "" {
				chatReq.User = s
			}
		}
	}

	return chatReq
}

func (c *ResponsesToChatConverter) convertToolChoice(toolChoice interface{}) interface{} {
	if toolChoice == nil {
		return nil
	}

	raw, err := json.Marshal(toolChoice)
	if err != nil {
		return toolChoice
	}

	converted := ResponsesToolChoiceToChat(raw)
	if len(converted) == 0 {
		return toolChoice
	}

	var out interface{}
	if err := json.Unmarshal(converted, &out); err != nil {
		return toolChoice
	}

	return out
}

// convertInput transforms Responses API input to Chat Completions messages.
// Input can be:
// - string: a simple user message
// - []InputItem: an array of input items
func (c *ResponsesToChatConverter) convertInput(input interface{}) []types.Message {
	if input == nil {
		return []types.Message{}
	}

	switch v := input.(type) {
	case string:
		if v == "" {
			return []types.Message{}
		}
		return []types.Message{
			{Role: "user", Content: v},
		}

	case []interface{}:
		return c.convertInputItems(v)

	default:
		// Try to marshal and unmarshal as InputItem array
		data, err := json.Marshal(input)
		if err != nil {
			return []types.Message{}
		}
		var items []types.InputItem
		if err := json.Unmarshal(data, &items); err != nil {
			return []types.Message{}
		}
		return c.convertInputItemsFromTyped(items)
	}
}

// inputItemGroup represents a logical grouping of input items
// that should become a single Chat Completions message.
// This handles the case where Codex sends function_call and message separately.
type inputItemGroup struct {
	itemType   string           // "message", "merged_assistant", "assistant_tool_calls", or "function_call_output"
	message    *types.Message   // Parsed message content (for message items)
	toolCalls  []types.ToolCall // Tool calls to merge (for function_call items)
	toolOutput *types.Message   // Tool output message (for function_call_output items)
}

// groupProcessor holds state during input item processing.
type groupProcessor struct {
	groups           []inputItemGroup
	pendingToolCalls []types.ToolCall
	converter        *ResponsesToChatConverter
}

// convertInputItems converts an array of raw input items to messages.
// It uses a two-phase approach: grouping then conversion.
func (c *ResponsesToChatConverter) convertInputItems(items []interface{}) []types.Message {
	groups := c.groupInputItems(items)
	return c.convertGroupsToMessages(groups)
}

// groupInputItems iterates over input items and delegates to handlers.
// Phase 1: Group related items that should become a single message.
func (c *ResponsesToChatConverter) groupInputItems(items []interface{}) []inputItemGroup {
	p := &groupProcessor{
		groups:    make([]inputItemGroup, 0, len(items)),
		converter: c,
	}

	for _, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		itemType, _ := itemMap["type"].(string)
		switch itemType {
		case "function_call":
			p.handleFunctionCallItem(itemMap)
		case "message":
			p.handleMessageItem(itemMap)
		case "function_call_output":
			p.handleFunctionCallOutputItem(itemMap)
		}
	}

	p.flushPendingToolCalls()
	return p.groups
}

// handleFunctionCallItem accumulates function_call items into pendingToolCalls.
func (p *groupProcessor) handleFunctionCallItem(itemMap map[string]interface{}) {
	if tc := p.converter.parseFunctionCallItem(itemMap); tc != nil {
		p.pendingToolCalls = append(p.pendingToolCalls, *tc)
	}
}

// handleMessageItem handles message items, merging with pending tool calls if assistant.
func (p *groupProcessor) handleMessageItem(itemMap map[string]interface{}) {
	role, _ := itemMap["role"].(string)

	// Check if this message has embedded tool_calls (combined format from prependHistoryToInput)
	if toolCallsRaw, ok := itemMap["tool_calls"]; ok && role == "assistant" {
		// Flush any pending tool calls first (they're orphaned)
		if len(p.pendingToolCalls) > 0 {
			p.flushPendingToolCalls()
		}
		// This is already a combined message with tool_calls
		msg := &types.Message{
			Role:      "assistant",
			ToolCalls: p.converter.extractToolCallsFromArray(toolCallsRaw),
		}
		if content, ok := itemMap["content"]; ok {
			msg.Content = p.converter.convertContentToValue(content)
		}
		p.groups = append(p.groups, inputItemGroup{
			itemType: "message",
			message:  msg,
		})
		return
	}

	msg := p.converter.parseMessageItem(itemMap)

	// If this is an assistant message and we have pending tool calls, MERGE them
	if role == "assistant" && len(p.pendingToolCalls) > 0 {
		p.groups = append(p.groups, inputItemGroup{
			itemType:  "merged_assistant",
			message:   msg,
			toolCalls: p.pendingToolCalls,
		})
		p.pendingToolCalls = nil
		return
	}

	// Flush any pending tool calls first (no assistant message to merge with)
	if len(p.pendingToolCalls) > 0 {
		p.flushPendingToolCalls()
	}

	// Add message as its own group
	if msg != nil {
		p.groups = append(p.groups, inputItemGroup{
			itemType: "message",
			message:  msg,
		})
	}
}

// handleFunctionCallOutputItem handles function_call_output items.
func (p *groupProcessor) handleFunctionCallOutputItem(itemMap map[string]interface{}) {
	// Flush any pending tool calls first
	if len(p.pendingToolCalls) > 0 {
		p.flushPendingToolCalls()
	}

	// Add tool output as its own group
	if toolMsg := p.converter.parseFunctionCallOutputItem(itemMap); toolMsg != nil {
		p.groups = append(p.groups, inputItemGroup{
			itemType:   "function_call_output",
			toolOutput: toolMsg,
		})
	}
}

// flushPendingToolCalls adds accumulated tool calls as a standalone group.
func (p *groupProcessor) flushPendingToolCalls() {
	if len(p.pendingToolCalls) == 0 {
		return
	}
	p.groups = append(p.groups, inputItemGroup{
		itemType:  "assistant_tool_calls",
		toolCalls: p.pendingToolCalls,
	})
	p.pendingToolCalls = nil
}

// convertGroupsToMessages converts grouped items to Chat Completions messages.
// Phase 2: Convert each group to its final message representation.
func (c *ResponsesToChatConverter) convertGroupsToMessages(groups []inputItemGroup) []types.Message {
	messages := make([]types.Message, 0, len(groups))

	for _, group := range groups {
		switch group.itemType {
		case "merged_assistant":
			// Combined assistant message with both content and tool_calls
			msg := *group.message
			msg.ToolCalls = group.toolCalls
			messages = append(messages, msg)

		case "assistant_tool_calls":
			// Assistant message with only tool_calls (no content)
			messages = append(messages, types.Message{
				Role:      "assistant",
				ToolCalls: group.toolCalls,
			})

		case "message":
			messages = append(messages, *group.message)

		case "function_call_output":
			messages = append(messages, *group.toolOutput)
		}
	}

	return messages
}

// parseMessageItem extracts a Message from a message input item.
func (c *ResponsesToChatConverter) parseMessageItem(itemMap map[string]interface{}) *types.Message {
	role, _ := itemMap["role"].(string)
	if role == "" {
		role = "user"
	}
	if role == "developer" {
		role = "system"
	}

	msg := &types.Message{Role: role}

	if content, ok := itemMap["content"]; ok {
		msg.Content = c.convertContentToValue(content)
	}

	return msg
}

// parseFunctionCallItem extracts a ToolCall from a function_call input item.
func (c *ResponsesToChatConverter) parseFunctionCallItem(itemMap map[string]interface{}) *types.ToolCall {
	name, _ := itemMap["name"].(string)
	arguments, _ := itemMap["arguments"].(string)
	callID, _ := itemMap["call_id"].(string)
	if callID == "" {
		callID, _ = itemMap["id"].(string)
	}

	if name == "" {
		return nil
	}

	return &types.ToolCall{
		ID:   callID,
		Type: "function",
		Function: types.Function{
			Name:      name,
			Arguments: arguments,
		},
	}
}

// parseFunctionCallOutputItem extracts a tool Message from a function_call_output input item.
func (c *ResponsesToChatConverter) parseFunctionCallOutputItem(itemMap map[string]interface{}) *types.Message {
	callID, _ := itemMap["call_id"].(string)
	if callID == "" {
		callID, _ = itemMap["tool_call_id"].(string)
	}
	output, _ := itemMap["output"].(string)

	if callID == "" {
		return nil
	}

	return &types.Message{
		Role:       "tool",
		ToolCallID: callID,
		Content:    output,
	}
}

// extractToolCallsFromArray extracts ToolCall slice from a tool_calls array.
func (c *ResponsesToChatConverter) extractToolCallsFromArray(toolCallsRaw interface{}) []types.ToolCall {
	if toolCallsRaw == nil {
		return nil
	}

	var toolCalls []types.ToolCall

	switch v := toolCallsRaw.(type) {
	case []interface{}:
		for _, tc := range v {
			if tcMap, ok := tc.(map[string]interface{}); ok {
				if extracted := c.parseFunctionCallItem(tcMap); extracted != nil {
					toolCalls = append(toolCalls, *extracted)
				}
			}
		}
	}

	return toolCalls
}

// convertContentToValue converts content to an appropriate value for Message.Content.
func (c *ResponsesToChatConverter) convertContentToValue(content interface{}) interface{} {
	if content == nil {
		return nil
	}

	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		return c.convertContent(v)
	default:
		return content
	}
}

// convertInputItemsFromTyped converts typed InputItem array to messages.
// It uses the same merge logic as convertInputItems for consistency.
func (c *ResponsesToChatConverter) convertInputItemsFromTyped(items []types.InputItem) []types.Message {
	// Convert typed items to generic format for grouping
	genericItems := make([]interface{}, len(items))
	for i, item := range items {
		itemMap := map[string]interface{}{"type": item.Type}
		if item.Role != "" {
			itemMap["role"] = item.Role
		}
		if item.Content != nil {
			itemMap["content"] = item.Content
		}
		if item.CallID != "" {
			itemMap["call_id"] = item.CallID
		}
		if item.ID != "" {
			itemMap["id"] = item.ID
		}
		if item.Name != "" {
			itemMap["name"] = item.Name
		}
		if item.Arguments != "" {
			itemMap["arguments"] = item.Arguments
		}
		if item.Output != "" {
			itemMap["output"] = item.Output
		}
		genericItems[i] = itemMap
	}

	// Reuse the same grouping logic
	return c.convertInputItems(genericItems)
}

// convertContentParts converts Responses API content parts to Chat format.
func (c *ResponsesToChatConverter) extractTextFromParts(parts []interface{}) string {
	var result strings.Builder
	for _, part := range parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}
		partType, _ := partMap["type"].(string)
		switch partType {
		case "input_text", "text", "output_text", "refusal":
			if text, ok := partMap["text"].(string); ok {
				if result.Len() > 0 {
					result.WriteString("\n")
				}
				result.WriteString(text)
			}
		case "input_file":
			// Include file content if available, otherwise use placeholder
			if fileData, ok := partMap["file_data"].(map[string]interface{}); ok {
				if result.Len() > 0 {
					result.WriteString("\n")
				}
				// Try to include actual file data
				if content, ok := fileData["file_data"].(string); ok && content != "" {
					result.WriteString(content)
				} else if filename, ok := fileData["filename"].(string); ok {
					result.WriteString("[File attached: " + filename + "]")
				}
			}
		}
	}
	return result.String()
}

func (c *ResponsesToChatConverter) hasNonTextParts(parts []interface{}) bool {
	for _, part := range parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}
		partType, _ := partMap["type"].(string)
		// input_file, input_image need structured conversion
		if partType != "input_text" && partType != "text" && partType != "output_text" && partType != "refusal" {
			return true
		}
	}
	return false
}

func (c *ResponsesToChatConverter) convertContent(parts []interface{}) interface{} {
	if c.hasNonTextParts(parts) {
		return c.convertContentParts(parts)
	}
	return c.extractTextFromParts(parts)
}

func (c *ResponsesToChatConverter) convertContentParts(parts []interface{}) []interface{} {
	result := make([]interface{}, 0, len(parts))

	for _, part := range parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}

		partType, _ := partMap["type"].(string)
		switch partType {
		case "input_text", "output_text", "refusal":
			// Convert input_text, output_text, and refusal to text
			if text, ok := partMap["text"].(string); ok {
				result = append(result, map[string]interface{}{
					"type": "text",
					"text": text,
				})
			}
		case "input_image":
			// Convert input_image to image_url
			if imageURL, ok := partMap["image_url"].(string); ok {
				result = append(result, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": imageURL,
					},
				})
			}
		case "input_file":
			// Preserve file data in the content part
			// Chat Completions doesn't have a standard file type, but we pass through
			// the data so downstream providers can handle it
			if fileData, ok := partMap["file_data"].(map[string]interface{}); ok {
				filePart := map[string]interface{}{
					"type": "file",
				}
				if filename, ok := fileData["filename"].(string); ok {
					filePart["filename"] = filename
				}
				if content, ok := fileData["file_data"].(string); ok && content != "" {
					filePart["file_data"] = content
				}
				result = append(result, filePart)
			}
		default:
			// Pass through other content types
			result = append(result, part)
		}
	}

	return result
}

// convertTools transforms Responses API tools to Chat Completions tools.
func (c *ResponsesToChatConverter) convertTools(tools []types.ResponsesTool) []types.Tool {
	if len(tools) == 0 {
		return nil
	}

	result := make([]types.Tool, 0, len(tools))
	for _, tool := range tools {
		chatTool := c.convertTool(&tool)
		if chatTool != nil {
			result = append(result, *chatTool)
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// convertTool transforms a single ResponsesTool to Tool.
func (c *ResponsesToChatConverter) convertTool(tool *types.ResponsesTool) *types.Tool {
	if tool.Type != "function" {
		// Only function tools are supported in Chat Completions
		return nil
	}

	// Handle both flat and nested tool formats
	name := tool.Name
	description := tool.Description
	parameters := tool.Parameters

	// If nested function format, use those values
	if tool.Function != nil {
		name = tool.Function.Name
		description = tool.Function.Description
		parameters = tool.Function.Parameters
	}

	if name == "" {
		return nil
	}

	return &types.Tool{
		Type: "function",
		Function: types.ToolFunction{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
}

// ResponsesToChatTransformer converts OpenAI Responses SSE to Chat Completions format.
