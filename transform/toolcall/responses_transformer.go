package toolcall

import (
	"encoding/json"
	"io"
	"strings"

	"ai-proxy/logging"
	"ai-proxy/types"

	"github.com/tmaxmax/go-sse"
)

// ResponsesTransformer converts Anthropic SSE events to OpenAI Responses API format.
type ResponsesTransformer struct {
	output     io.Writer
	formatter  *ResponsesFormatter
	parser     *Parser
	messageID  string
	model      string
	blockIndex int
	toolIndex  int
	currentID  string
	responseID string

	// State flags
	inToolCall  bool
	inText      bool
	inReasoning bool

	// Content builders
	textContent      strings.Builder
	toolArgs         strings.Builder
	reasoningContent strings.Builder

	// Output tracking
	outputIndex     int    // Current output index counter (0-indexed)
	reasoningID     string // Cached reasoning item ID
	currentToolName string // Current tool name for function_call output item

	// Output items for final response
	outputItems []map[string]interface{}
	currentItem map[string]interface{}
	itemAdded   bool
}

// ResponsesFormatter formats events in OpenAI Responses API format.
type ResponsesFormatter struct {
	responseID string
	model      string
}

// NewResponsesFormatter creates a new formatter for OpenAI Responses API.
func NewResponsesFormatter(responseID, model string) *ResponsesFormatter {
	return &ResponsesFormatter{
		responseID: responseID,
		model:      model,
	}
}

// SetResponseID sets the response ID.
func (f *ResponsesFormatter) SetResponseID(id string) {
	f.responseID = id
}

// SetModel sets the model name.
func (f *ResponsesFormatter) SetModel(model string) {
	f.model = model
}

// getReasoningID generates a consistent reasoning item ID from the message ID.
// Converts "msg_xxx" to "rs_xxx" format per OpenAI Responses API convention.
func (t *ResponsesTransformer) getReasoningID() string {
	if t.reasoningID == "" && t.messageID != "" {
		if len(t.messageID) > 4 {
			t.reasoningID = "rs_" + t.messageID[4:]
		} else {
			t.reasoningID = "rs_" + t.messageID
		}
	}
	return t.reasoningID
}

// emitMessageItemAdded emits the response.output_item.added event for the message.
// This is called when the first text content is encountered, after all reasoning
// and tool calls have already emitted their output_item.added events.
// Returns nil if the message was already emitted or there's no message.
func (t *ResponsesTransformer) emitMessageItemAdded() error {
	// Check if we've already set up the message item - we emit it lazily
	// The actual emission happens in handleMessageStop after all output items are known
	return nil
}

// getOutputIndexForReasoning returns the output index for a reasoning item.
// Since reasoning items come first, the index is simply the count of reasoning items.
func (t *ResponsesTransformer) getOutputIndexForReasoning() int {
	count := 0
	for _, item := range t.outputItems {
		if item["type"] == "reasoning" {
			count++
		}
	}
	return count - 1 // -1 because we just appended the item
}

// getOutputIndexForToolCall returns the output index for a tool call item.
// It's positioned after all reasoning items and previous tool calls.
func (t *ResponsesTransformer) getOutputIndexForToolCall() int {
	count := 0
	for _, item := range t.outputItems {
		if item["type"] == "reasoning" || item["type"] == "function_call" {
			count++
		}
	}
	return count - 1 // -1 because we just appended the item
}

// FormatResponseCreated formats a response.created event.
func (f *ResponsesFormatter) FormatResponseCreated() []byte {
	event := map[string]interface{}{
		"type": "response.created",
		"response": map[string]interface{}{
			"id":         f.responseID,
			"object":     "response",
			"created_at": 0,
			"model":      f.model,
			"status":     "in_progress",
			"output":     []interface{}{},
		},
	}
	data, _ := json.Marshal(event)
	return []byte("data: " + string(data) + "\n\n")
}

// FormatOutputItemAdded formats a response.output_item.added event for message.
// The outputIndex is the 0-indexed position in the output array.
func (f *ResponsesFormatter) FormatOutputItemAdded(itemID string, outputIndex int) []byte {
	event := map[string]interface{}{
		"type":         "response.output_item.added",
		"item_id":      itemID,
		"output_index": outputIndex,
		"item": map[string]interface{}{
			"type":    "message",
			"id":      itemID,
			"status":  "in_progress",
			"role":    "assistant",
			"content": []interface{}{},
		},
	}
	data, _ := json.Marshal(event)
	return []byte("data: " + string(data) + "\n\n")
}

// FormatContentPartAdded formats a response.content_part.added event for text.
func (f *ResponsesFormatter) FormatContentPartAdded(contentIndex int, partType string) []byte {
	event := map[string]interface{}{
		"type":          "response.content_part.added",
		"item_id":       f.responseID,
		"content_index": contentIndex,
		"part": map[string]interface{}{
			"type": partType,
		},
	}
	if partType == "output_text" {
		event["part"] = map[string]interface{}{
			"type": "output_text",
			"text": "",
		}
	}
	data, _ := json.Marshal(event)
	return []byte("data: " + string(data) + "\n\n")
}

// FormatOutputTextDelta formats a response.output_text.delta event.
func (f *ResponsesFormatter) FormatOutputTextDelta(contentIndex int, delta string) []byte {
	event := map[string]interface{}{
		"type":          "response.output_text.delta",
		"item_id":       f.responseID,
		"content_index": contentIndex,
		"delta":         delta,
	}
	data, _ := json.Marshal(event)
	return []byte("data: " + string(data) + "\n\n")
}

// FormatFunctionCallItemAdded emits a response.output_item.added event for a function_call item.
// Function calls are separate output items in the Responses API, not content parts.
// The outputIndex is the 0-indexed position in the output array.
func (f *ResponsesFormatter) FormatFunctionCallItemAdded(itemID, name string, outputIndex int) []byte {
	event := map[string]interface{}{
		"type":         "response.output_item.added",
		"item_id":      itemID,
		"output_index": outputIndex,
		"item": map[string]interface{}{
			"type":      "function_call",
			"id":        itemID,
			"call_id":   itemID,
			"name":      name,
			"arguments": "",
		},
	}
	data, _ := json.Marshal(event)
	return []byte("data: " + string(data) + "\n\n")
}

// FormatFunctionCallItemDone emits a response.output_item.done event for a function_call item.
// This signals the completion of a function call output item with the full arguments.
func (f *ResponsesFormatter) FormatFunctionCallItemDone(itemID, name, arguments string, outputIndex int) []byte {
	event := map[string]interface{}{
		"type":         "response.output_item.done",
		"item_id":      itemID,
		"output_index": outputIndex,
		"item": map[string]interface{}{
			"type":      "function_call",
			"id":        itemID,
			"call_id":   itemID,
			"name":      name,
			"arguments": arguments,
		},
	}
	data, _ := json.Marshal(event)
	return []byte("data: " + string(data) + "\n\n")
}

// FormatFunctionCallArgsDelta emits a response.function_call_arguments.delta event.
// The itemID is the function_call item ID, and callID is used for the call_id field.
func (f *ResponsesFormatter) FormatFunctionCallArgsDelta(itemID, callID, delta string) []byte {
	event := map[string]interface{}{
		"type":    "response.function_call_arguments.delta",
		"item_id": itemID,
		"call_id": callID,
		"delta":   delta,
	}
	data, _ := json.Marshal(event)
	return []byte("data: " + string(data) + "\n\n")
}

// FormatContentPartDone formats a response.content_part.done event.
func (f *ResponsesFormatter) FormatContentPartDone(contentIndex int, partType string, content string) []byte {
	part := map[string]interface{}{
		"type": partType,
	}
	if partType == "output_text" {
		part["text"] = content
	}
	event := map[string]interface{}{
		"type":          "response.content_part.done",
		"item_id":       f.responseID,
		"content_index": contentIndex,
		"part":          part,
	}
	data, _ := json.Marshal(event)
	return []byte("data: " + string(data) + "\n\n")
}

// FormatOutputItemDone formats a response.output_item.done event.
// The outputIndex is the 0-indexed position in the output array.
func (f *ResponsesFormatter) FormatOutputItemDone(itemID string, item map[string]interface{}, outputIndex int) []byte {
	event := map[string]interface{}{
		"type":         "response.output_item.done",
		"item_id":      itemID,
		"output_index": outputIndex,
		"item":         item,
	}
	data, _ := json.Marshal(event)
	return []byte("data: " + string(data) + "\n\n")
}

// FormatResponseCompleted formats a response.completed event.
func (f *ResponsesFormatter) FormatResponseCompleted(outputItems []map[string]interface{}) []byte {
	// Build output with the message item
	output := []interface{}{}
	if len(outputItems) > 0 {
		for _, item := range outputItems {
			output = append(output, item)
		}
	}
	event := map[string]interface{}{
		"type": "response.completed",
		"response": map[string]interface{}{
			"id":     f.responseID,
			"object": "response",
			"model":  f.model,
			"status": "completed",
			"output": output,
		},
	}
	data, _ := json.Marshal(event)
	return []byte("data: " + string(data) + "\n\n")
}

// FormatReasoningItemAdded emits a response.output_item.added event for a reasoning item.
// This signals the start of a reasoning output item in the streaming response.
// The outputIndex is the 0-indexed position in the output array (always 0 for reasoning).
func (f *ResponsesFormatter) FormatReasoningItemAdded(itemID string, outputIndex int) []byte {
	event := map[string]interface{}{
		"type":         "response.output_item.added",
		"item_id":      itemID,
		"output_index": outputIndex,
		"item": map[string]interface{}{
			"type":    "reasoning",
			"id":      itemID,
			"summary": []interface{}{},
		},
	}
	data, _ := json.Marshal(event)
	return []byte("data: " + string(data) + "\n\n")
}

// FormatReasoningSummaryDelta emits a response.reasoning_summary_text.delta event.
// This streams incremental reasoning summary text to the client.
func (f *ResponsesFormatter) FormatReasoningSummaryDelta(itemID, delta string) []byte {
	event := map[string]interface{}{
		"type":    "response.reasoning_summary_text.delta",
		"item_id": itemID,
		"delta":   delta,
	}
	data, _ := json.Marshal(event)
	return []byte("data: " + string(data) + "\n\n")
}

// FormatReasoningItemDone emits a response.output_item.done event with the full summary.
// This signals the completion of a reasoning output item.
// The outputIndex is the 0-indexed position in the output array.
func (f *ResponsesFormatter) FormatReasoningItemDone(itemID, summaryText string, outputIndex int) []byte {
	event := map[string]interface{}{
		"type":         "response.output_item.done",
		"item_id":      itemID,
		"output_index": outputIndex,
		"item": map[string]interface{}{
			"type": "reasoning",
			"id":   itemID,
			"summary": []map[string]interface{}{
				{"type": "summary_text", "text": summaryText},
			},
		},
	}
	data, _ := json.Marshal(event)
	return []byte("data: " + string(data) + "\n\n")
}

// NewResponsesTransformer creates a new transformer for OpenAI Responses API.
func NewResponsesTransformer(output io.Writer) *ResponsesTransformer {
	return &ResponsesTransformer{
		output:      output,
		formatter:   NewResponsesFormatter("", ""),
		parser:      NewParser(DefaultTokens),
		outputItems: make([]map[string]interface{}, 0),
	}
}

// Transform processes an SSE event and converts it to OpenAI Responses format.
func (t *ResponsesTransformer) Transform(event *sse.Event) error {
	if event.Data == "" {
		return nil
	}

	if event.Data == "[DONE]" {
		return t.write([]byte("data: [DONE]\n\n"))
	}

	var anthropicEvent types.Event
	if err := json.Unmarshal([]byte(event.Data), &anthropicEvent); err != nil {
		return t.write([]byte("data: " + event.Data + "\n\n"))
	}

	return t.handleEvent(anthropicEvent)
}

// handleEvent processes an Anthropic event and converts it to Responses format.
func (t *ResponsesTransformer) handleEvent(event types.Event) error {
	switch event.Type {
	case "message_start":
		return t.handleMessageStart(event)
	case "content_block_start":
		return t.handleContentBlockStart(event)
	case "content_block_delta":
		return t.handleContentBlockDelta(event)
	case "content_block_stop":
		return t.handleContentBlockStop(event)
	case "message_delta":
		return t.handleMessageDelta(event)
	case "message_stop":
		return t.handleMessageStop(event)
	case "ping":
		// Pass through ping events
		return nil
	default:
		// Pass through unknown events
		return nil
	}
}

func (t *ResponsesTransformer) handleMessageStart(event types.Event) error {
	if event.Message != nil && event.Message.ID != "" {
		t.messageID = event.Message.ID
		t.responseID = "resp_" + event.Message.ID[4:] // Convert msg_xxx to resp_xxx
		t.model = event.Message.Model
		t.formatter.SetResponseID(t.responseID)
		t.formatter.SetModel(t.model)
	}

	// Send response.created event
	if err := t.write(t.formatter.FormatResponseCreated()); err != nil {
		return err
	}

	// Create the message item but don't emit it yet - we need to know the output_index
	// which depends on how many reasoning and tool_call items come before it.
	t.itemAdded = true
	// Use messageID (msg_xxx) for the message item, NOT responseID (resp_xxx)
	// to avoid ID collision with the response itself
	messageItemID := t.messageID
	if messageItemID == "" {
		messageItemID = "msg_" + t.responseID[5:] // fallback: derive from response ID
	}
	t.currentItem = map[string]interface{}{
		"type":    "message",
		"id":      messageItemID,
		"status":  "in_progress",
		"role":    "assistant",
		"content": []map[string]interface{}{},
	}
	return nil
}

func (t *ResponsesTransformer) handleContentBlockStart(event types.Event) error {
	if event.Index == nil {
		return nil
	}

	if event.ContentBlock != nil {
		var block types.ContentBlock
		if err := json.Unmarshal(event.ContentBlock, &block); err == nil {
			t.blockIndex = *event.Index

			switch block.Type {
			case "text":
				t.inText = true
				t.textContent.Reset()
				// Emit message item if not yet emitted
				if err := t.emitMessageItemAdded(); err != nil {
					return err
				}
				return t.write(t.formatter.FormatContentPartAdded(t.blockIndex, "output_text"))
			case "thinking":
				t.inReasoning = true
				t.reasoningContent.Reset()
				reasoningID := t.getReasoningID()
				outputIdx := t.outputIndex
				t.outputIndex++
				return t.write(t.formatter.FormatReasoningItemAdded(reasoningID, outputIdx))
			case "tool_use":
				t.inToolCall = true
				t.currentID = block.ID
				t.currentToolName = block.Name
				t.toolArgs.Reset()
				outputIdx := t.outputIndex
				t.outputIndex++
				return t.write(t.formatter.FormatFunctionCallItemAdded(block.ID, block.Name, outputIdx))
			}
		}
	}

	return nil
}

func (t *ResponsesTransformer) handleContentBlockDelta(event types.Event) error {
	if event.Index == nil {
		return nil
	}

	if event.Delta != nil {
		// Try to parse as text_delta
		var textDelta types.TextDelta
		if err := json.Unmarshal(event.Delta, &textDelta); err == nil && textDelta.Type == "text_delta" {
			if t.inText {
				t.textContent.WriteString(textDelta.Text)
				return t.write(t.formatter.FormatOutputTextDelta(*event.Index, textDelta.Text))
			}
		}

		// Try to parse as thinking_delta
		var thinkingDelta types.ThinkingDelta
		if err := json.Unmarshal(event.Delta, &thinkingDelta); err == nil && thinkingDelta.Type == "thinking_delta" {
			if t.inReasoning {
				t.reasoningContent.WriteString(thinkingDelta.Thinking)
				reasoningID := t.getReasoningID()
				return t.write(t.formatter.FormatReasoningSummaryDelta(reasoningID, thinkingDelta.Thinking))
			}
		}

		// Try to parse as input_json_delta
		var inputDelta types.InputJSONDelta
		if err := json.Unmarshal(event.Delta, &inputDelta); err == nil && inputDelta.Type == "input_json_delta" {
			if t.inToolCall {
				t.toolArgs.WriteString(inputDelta.PartialJSON)
				return t.write(t.formatter.FormatFunctionCallArgsDelta(t.currentID, t.currentID, inputDelta.PartialJSON))
			}
		}
	}

	return nil
}

func (t *ResponsesTransformer) handleContentBlockStop(event types.Event) error {
	if event.Index == nil {
		return nil
	}

	if t.inText {
		t.inText = false
		content := t.textContent.String()
		// Track content for output item
		if t.currentItem != nil {
			if contents, ok := t.currentItem["content"].([]map[string]interface{}); ok {
				t.currentItem["content"] = append(contents, map[string]interface{}{
					"type": "output_text",
					"text": content,
				})
			}
		}
		return t.write(t.formatter.FormatContentPartDone(*event.Index, "output_text", content))
	}

	if t.inReasoning {
		t.inReasoning = false
		summary := t.reasoningContent.String()
		reasoningID := t.getReasoningID()

		reasoningItem := map[string]interface{}{
			"type": "reasoning",
			"id":   reasoningID,
			"summary": []map[string]interface{}{
				{"type": "summary_text", "text": summary},
			},
		}
		t.outputItems = append(t.outputItems, reasoningItem)

		// Get output index: 0 for first reasoning item, 1 for second, etc.
		outputIdx := t.getOutputIndexForReasoning()
		return t.write(t.formatter.FormatReasoningItemDone(reasoningID, summary, outputIdx))
	}

	if t.inToolCall {
		t.inToolCall = false
		args := t.toolArgs.String()

		toolItem := map[string]interface{}{
			"type":      "function_call",
			"id":        t.currentID,
			"call_id":   t.currentID,
			"name":      t.currentToolName,
			"arguments": args,
		}
		t.outputItems = append(t.outputItems, toolItem)

		outputIdx := t.getOutputIndexForToolCall()
		return t.write(t.formatter.FormatFunctionCallItemDone(t.currentID, t.currentToolName, args, outputIdx))
	}

	return nil
}

func (t *ResponsesTransformer) handleMessageDelta(event types.Event) error {
	// Handle stop_reason - convert Anthropic stop_reason to Responses API format
	if event.StopReason != "" {
		// Map Anthropic stop_reason to appropriate Responses API output
		// Anthropic: "end_turn", "max_tokens", "stop_sequence", "tool_use"
		switch event.StopReason {
		case "tool_use":
			// Tool use is handled via content blocks, no special event needed
			logging.InfoMsg("[%s] Stop reason: tool_use", t.messageID)
		case "max_tokens":
			logging.InfoMsg("[%s] Stop reason: max_tokens", t.messageID)
		case "end_turn":
			logging.InfoMsg("[%s] Stop reason: end_turn", t.messageID)
		}
	}
	return nil
}

func (t *ResponsesTransformer) handleMessageStop(event types.Event) error {
	// Only emit message item if there's actual text content.
	// When stop_reason is tool_use with no text, output should only contain
	// reasoning and function_call items - no empty message.
	if t.itemAdded && t.currentItem != nil && t.textContent.Len() > 0 {
		// Calculate output index for message (after all reasoning and tool calls)
		outputIdx := t.outputIndex

		// Get the message item ID from currentItem (set in handleMessageStart)
		// This ensures we use msg_xxx format, not resp_xxx
		messageItemID, _ := t.currentItem["id"].(string)
		if messageItemID == "" {
			messageItemID = t.messageID
		}

		// Emit response.output_item.added for the message
		if err := t.write(t.formatter.FormatOutputItemAdded(messageItemID, outputIdx)); err != nil {
			return err
		}

		// Emit content_part events for the message content
		if content, ok := t.currentItem["content"].([]map[string]interface{}); ok && len(content) > 0 {
			for i, part := range content {
				// Emit content_part.added and content_part.done for each part
				if partType, ok := part["type"].(string); ok {
					t.write(t.formatter.FormatContentPartAdded(i, partType))
					if text, ok := part["text"].(string); ok {
						t.write(t.formatter.FormatContentPartDone(i, partType, text))
					}
				}
			}
		}

		// Mark message as completed and emit output_item.done
		t.currentItem["status"] = "completed"
		if err := t.write(t.formatter.FormatOutputItemDone(messageItemID, t.currentItem, outputIdx)); err != nil {
			return err
		}
		// Add to output items for response.completed
		t.outputItems = append(t.outputItems, t.currentItem)
	}

	// Send response.completed event with populated output
	return t.write(t.formatter.FormatResponseCompleted(t.outputItems))
}

func (t *ResponsesTransformer) write(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	_, err := t.output.Write(data)
	return err
}

// Flush writes any buffered data.
func (t *ResponsesTransformer) Flush() error {
	return nil
}

// Close flushes and releases resources.
func (t *ResponsesTransformer) Close() error {
	return t.Flush()
}
