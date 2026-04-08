// Package convert provides converters between different API formats.
// This file implements OpenAI Responses API to Anthropic Messages streaming conversion.
package convert

import (
	"encoding/json"
	"io"

	"ai-proxy/logging"
	"ai-proxy/transform"
	"ai-proxy/types"
)

// ResponsesToAnthropicStreamingConverter converts Responses API SSE events to Anthropic Messages format.
// It implements the transform.Stage interface for pipeline integration.
//
// @brief Streaming converter from Responses API SSE events to Anthropic SSE format.
//
// @note This enables the chain: Responses SSE → ResponsesToAnthropicStreamingConverter → Anthropic SSE
type ResponsesToAnthropicStreamingConverter struct {
	writer io.Writer

	// State tracked during streaming
	responseID string
	model      string

	// Block index tracking
	blockIndex         int
	outputIndexToBlock map[int]int // Maps output_index from Responses to block index in Anthropic

	// Token usage tracking
	inputTokens  int
	outputTokens int

	// Optional output to another receiver for further chaining
	receiver transform.Stage
}

// NewResponsesToAnthropicStreamingConverter creates a converter that writes to an io.Writer.
//
// @brief Creates a new streaming converter writing Anthropic events to w.
//
// @param w The io.Writer to write Anthropic SSE events to.
//
// @return *ResponsesToAnthropicStreamingConverter A new converter instance.
func NewResponsesToAnthropicStreamingConverter(w io.Writer) *ResponsesToAnthropicStreamingConverter {
	return &ResponsesToAnthropicStreamingConverter{
		writer:             w,
		outputIndexToBlock: make(map[int]int),
	}
}

// SetOutputStage sets the next stage in the pipeline for chaining.
//
// @brief Configures the converter to emit events to another stage instead of writing directly.
//
// @param stage The next stage to receive PipelineEvents.
func (c *ResponsesToAnthropicStreamingConverter) SetOutputStage(stage transform.Stage) {
	c.receiver = stage
}

// Initialize prepares the converter before processing events.
// It implements the transform.Stage interface.
//
// @brief Implements transform.Stage.Initialize. No-op for this converter.
func (c *ResponsesToAnthropicStreamingConverter) Initialize() error {
	return nil
}

// Close flushes and releases resources.
// It implements the transform.Stage interface.
//
// @brief Implements transform.Stage.Close by flushing buffered data.
func (c *ResponsesToAnthropicStreamingConverter) Close() error {
	return c.Flush()
}

// Process handles a pipeline event for the ResponsesToAnthropicStreamingConverter.
// It implements the transform.Stage interface.
//
// @brief Implements transform.Stage.Process for Responses events and done events.
//
// @param event The pipeline event to process.
//
// @return error Returns nil on success.
func (c *ResponsesToAnthropicStreamingConverter) Process(event transform.PipelineEvent) error {
	switch event.Type {
	case transform.EventResponsesEvent:
		if len(event.Data) == 0 {
			return nil
		}
		var respEvent types.ResponsesStreamEvent
		if err := json.Unmarshal(event.Data, &respEvent); err != nil {
			logging.DebugMsg("Failed to parse Responses event: %v", err)
			return nil
		}
		return c.handleEvent(&respEvent)
	case transform.EventDone:
		return c.Flush()
	default:
		return nil
	}
}

// Flush writes any buffered data.
//
// @brief Flushes any pending output to the writer or next stage.
func (c *ResponsesToAnthropicStreamingConverter) Flush() error {
	// No buffered data for this converter
	if c.receiver != nil {
		return c.receiver.Process(transform.PipelineEvent{Type: transform.EventDone})
	}
	return nil
}

// handleEvent processes a single Responses API event and emits corresponding Anthropic events.
func (c *ResponsesToAnthropicStreamingConverter) handleEvent(event *types.ResponsesStreamEvent) error {
	switch event.Type {
	case "response.created":
		return c.handleResponseCreated(event)
	case "response.output_item.added":
		return c.handleOutputItemAdded(event)
	case "response.content_part.added":
		return c.handleContentPartAdded(event)
	case "response.output_text.delta":
		return c.handleOutputTextDelta(event)
	case "response.function_call_arguments.delta":
		return c.handleFunctionCallArgsDelta(event)
	case "response.output_text.done":
		return c.handleOutputTextDone(event)
	case "response.function_call_arguments.done":
		return c.handleFunctionCallArgsDone(event)
	case "response.output_item.done":
		return c.handleOutputItemDone(event)
	case "response.completed":
		return c.handleResponseCompleted(event)
	case "response.incomplete":
		return c.handleResponseIncomplete(event)
	case "response.failed":
		return c.handleResponseFailed(event)
	default:
		// Skip unknown events
		return nil
	}
}

// handleResponseCreated handles response.created event → emits message_start.
func (c *ResponsesToAnthropicStreamingConverter) handleResponseCreated(event *types.ResponsesStreamEvent) error {
	if event.Response == nil {
		return nil
	}

	c.responseID = event.Response.ID
	c.model = event.Response.Model

	// Emit message_start event
	msgStart := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":    c.responseID,
			"type":  "message",
			"role":  "assistant",
			"model": c.model,
			"usage": map[string]int{
				"input_tokens": c.inputTokens,
			},
		},
	}
	return c.emitEvent("message_start", msgStart)
}

// handleOutputItemAdded handles response.output_item.added event.
// For function_call type, emits content_block_start with tool_use.
func (c *ResponsesToAnthropicStreamingConverter) handleOutputItemAdded(event *types.ResponsesStreamEvent) error {
	if event.OutputItem == nil {
		return nil
	}

	// Track the mapping from output_index to block_index
	c.outputIndexToBlock[event.ContentIndex] = c.blockIndex

	if event.OutputItem.Type == "function_call" {
		// Emit content_block_start for tool_use
		blockStart := map[string]interface{}{
			"type":  "content_block_start",
			"index": c.blockIndex,
			"content_block": map[string]interface{}{
				"type": "tool_use",
				"id":   event.OutputItem.CallID,
				"name": event.OutputItem.Name,
			},
		}
		if err := c.emitEvent("content_block_start", blockStart); err != nil {
			return err
		}
		c.blockIndex++
	}

	return nil
}

// handleContentPartAdded handles response.content_part.added event.
// For output_text type, emits content_block_start with text.
func (c *ResponsesToAnthropicStreamingConverter) handleContentPartAdded(event *types.ResponsesStreamEvent) error {
	if event.OutputItem == nil {
		return nil
	}

	// Check if this is an output_text content part being added to a message
	// The OutputItem here represents the content part
	if event.OutputItem.Type == "output_text" || (event.OutputItem.Content != nil && len(event.OutputItem.Content) > 0) {
		// For content_part.added with output_text, emit content_block_start for text
		// Track the mapping
		c.outputIndexToBlock[event.ContentIndex] = c.blockIndex

		blockStart := map[string]interface{}{
			"type":  "content_block_start",
			"index": c.blockIndex,
			"content_block": map[string]interface{}{
				"type": "text",
				"text": "",
			},
		}
		if err := c.emitEvent("content_block_start", blockStart); err != nil {
			return err
		}
		c.blockIndex++
	}

	return nil
}

// handleOutputTextDelta handles response.output_text.delta event → emits content_block_delta with text_delta.
func (c *ResponsesToAnthropicStreamingConverter) handleOutputTextDelta(event *types.ResponsesStreamEvent) error {
	if event.Delta == "" {
		return nil
	}

	idx := c.outputIndexToBlock[event.ContentIndex]

	delta := map[string]interface{}{
		"type":  "content_block_delta",
		"index": idx,
		"delta": map[string]interface{}{
			"type": "text_delta",
			"text": event.Delta,
		},
	}
	return c.emitEvent("content_block_delta", delta)
}

// handleFunctionCallArgsDelta handles response.function_call_arguments.delta event
// → emits content_block_delta with input_json_delta.
func (c *ResponsesToAnthropicStreamingConverter) handleFunctionCallArgsDelta(event *types.ResponsesStreamEvent) error {
	if event.Delta == "" {
		return nil
	}

	idx := c.outputIndexToBlock[event.ContentIndex]

	delta := map[string]interface{}{
		"type":  "content_block_delta",
		"index": idx,
		"delta": map[string]interface{}{
			"type":         "input_json_delta",
			"partial_json": event.Delta,
		},
	}
	return c.emitEvent("content_block_delta", delta)
}

// handleOutputTextDone handles response.output_text.done event → emits content_block_stop.
func (c *ResponsesToAnthropicStreamingConverter) handleOutputTextDone(event *types.ResponsesStreamEvent) error {
	idx := c.outputIndexToBlock[event.ContentIndex]

	blockStop := map[string]interface{}{
		"type":  "content_block_stop",
		"index": idx,
	}
	return c.emitEvent("content_block_stop", blockStop)
}

// handleFunctionCallArgsDone handles response.function_call_arguments.done event → emits content_block_stop.
func (c *ResponsesToAnthropicStreamingConverter) handleFunctionCallArgsDone(event *types.ResponsesStreamEvent) error {
	idx := c.outputIndexToBlock[event.ContentIndex]

	blockStop := map[string]interface{}{
		"type":  "content_block_stop",
		"index": idx,
	}
	return c.emitEvent("content_block_stop", blockStop)
}

// handleOutputItemDone handles response.output_item.done event.
// Currently no specific action needed as content_block_stop is emitted on text/function done events.
func (c *ResponsesToAnthropicStreamingConverter) handleOutputItemDone(event *types.ResponsesStreamEvent) error {
	// No action needed - content_block_stop already emitted
	return nil
}

// handleResponseCompleted handles response.completed event → emits message_delta + message_stop.
func (c *ResponsesToAnthropicStreamingConverter) handleResponseCompleted(event *types.ResponsesStreamEvent) error {
	if event.Response == nil {
		return nil
	}

	// Determine stop_reason based on output items
	stopReason := "end_turn"
	for _, item := range event.Response.Output {
		if item.Type == "function_call" {
			stopReason = "tool_use"
			break
		}
	}

	// Update usage if available
	if event.Response.Usage != nil {
		c.inputTokens = event.Response.Usage.InputTokens
		c.outputTokens = event.Response.Usage.OutputTokens
	}

	// Emit message_delta
	msgDelta := map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason": stopReason,
		},
		"usage": map[string]int{
			"output_tokens": c.outputTokens,
		},
	}
	if err := c.emitEvent("message_delta", msgDelta); err != nil {
		return err
	}

	// Emit message_stop
	return c.emitEvent("message_stop", map[string]string{"type": "message_stop"})
}

// handleResponseIncomplete handles response.incomplete event → emits message_delta with max_tokens + message_stop.
func (c *ResponsesToAnthropicStreamingConverter) handleResponseIncomplete(event *types.ResponsesStreamEvent) error {
	// Emit message_delta with max_tokens stop reason
	msgDelta := map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason": "max_tokens",
		},
		"usage": map[string]int{
			"output_tokens": c.outputTokens,
		},
	}
	if err := c.emitEvent("message_delta", msgDelta); err != nil {
		return err
	}

	// Emit message_stop
	return c.emitEvent("message_stop", map[string]string{"type": "message_stop"})
}

// handleResponseFailed handles response.failed event → emits error event.
func (c *ResponsesToAnthropicStreamingConverter) handleResponseFailed(event *types.ResponsesStreamEvent) error {
	errEvent := map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    "api_error",
			"message": "upstream response failed",
		},
	}
	return c.emitEvent("error", errEvent)
}

// emitEvent writes an Anthropic SSE event with the given name and data.
func (c *ResponsesToAnthropicStreamingConverter) emitEvent(name string, data interface{}) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Emit to receiver if configured
	if c.receiver != nil {
		return c.receiver.Process(transform.PipelineEvent{
			Type:    transform.EventAnthropicEvent,
			Data:    dataBytes,
			SSEType: name,
		})
	}

	// Write directly to writer
	if c.writer != nil {
		// Write event name
		if _, err := c.writer.Write([]byte("event: " + name + "\n")); err != nil {
			return err
		}

		// Write data
		if _, err := c.writer.Write([]byte("data: " + string(dataBytes) + "\n\n")); err != nil {
			return err
		}
	}

	return nil
}

// Verify interface compliance
var _ transform.Stage = (*ResponsesToAnthropicStreamingConverter)(nil)
