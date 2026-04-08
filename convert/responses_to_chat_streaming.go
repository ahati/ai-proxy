// Package convert provides converters between different API formats.
// This file implements OpenAI Responses API to OpenAI Chat Completions streaming conversion.
package convert

import (
	"encoding/json"
	"io"
	"strings"

	"ai-proxy/logging"
	"ai-proxy/transform"
	"ai-proxy/types"
)

// ResponsesToChatStreamingConverter converts Responses API SSE events to OpenAI Chat Completions format.
// It implements the transform.Stage interface for pipeline integration.
//
// @brief Streaming converter from Responses API SSE events to OpenAI Chat chunks.
//
// @note This enables the chain: Responses SSE → ResponsesToChatStreamingConverter → OpenAI Chat SSE
type ResponsesToChatStreamingConverter struct {
	writer io.Writer

	// State tracked during streaming
	responseID string
	model      string
	started    bool

	// Content tracking
	textContent strings.Builder
	toolCallIdx int

	// Current tool call state
	currentToolCallID   string
	currentToolCallName string
	toolCallArgs        strings.Builder

	// Usage tracking
	promptTokens     int
	completionTokens int

	// Finish reason
	finishReason string

	// Optional output to another receiver for further chaining
	receiver transform.Stage
}

// NewResponsesToChatStreamingConverter creates a converter that writes to an io.Writer.
//
// @brief Creates a new streaming converter writing OpenAI Chat chunks to w.
//
// @param w The io.Writer to write OpenAI Chat chunks to.
//
// @return *ResponsesToChatStreamingConverter A new converter instance.
func NewResponsesToChatStreamingConverter(w io.Writer) *ResponsesToChatStreamingConverter {
	return &ResponsesToChatStreamingConverter{
		writer: w,
	}
}

// SetOutputStage sets the next stage in the pipeline for chaining.
//
// @brief Configures the converter to emit events to another stage instead of writing directly.
//
// @param stage The next stage to receive PipelineEvents.
func (c *ResponsesToChatStreamingConverter) SetOutputStage(stage transform.Stage) {
	c.receiver = stage
}

// Initialize prepares the converter before processing events.
// It implements the transform.Stage interface.
//
// @brief Implements transform.Stage.Initialize. No-op for this converter.
func (c *ResponsesToChatStreamingConverter) Initialize() error {
	return nil
}

// Close flushes and releases resources.
// It implements the transform.Stage interface.
//
// @brief Implements transform.Stage.Close by flushing buffered data.
func (c *ResponsesToChatStreamingConverter) Close() error {
	return c.Flush()
}

// Process handles a pipeline event for the ResponsesToChatStreamingConverter.
// It implements the transform.Stage interface.
//
// @brief Implements transform.Stage.Process for Responses events and done events.
//
// @param event The pipeline event to process.
//
// @return error Returns nil on success.
func (c *ResponsesToChatStreamingConverter) Process(event transform.PipelineEvent) error {
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

// Flush writes any buffered data and the final chunk.
//
// @brief Flushes any pending output and writes the final OpenAI chunk.
func (c *ResponsesToChatStreamingConverter) Flush() error {
	// Emit final chunk with finish reason if we have one
	if c.finishReason != "" {
		chunk := types.Chunk{
			ID:      c.responseID,
			Object:  "chat.completion.chunk",
			Created: 0,
			Model:   c.model,
			Choices: []types.Choice{
				{
					Index:        0,
					FinishReason: &c.finishReason,
				},
			},
		}

		// Add usage if available
		if c.promptTokens > 0 || c.completionTokens > 0 {
			chunk.Usage = &types.Usage{
				PromptTokens:     c.promptTokens,
				CompletionTokens: c.completionTokens,
				TotalTokens:      c.promptTokens + c.completionTokens,
			}
		}

		if err := c.writeChunk(chunk); err != nil {
			return err
		}
		c.finishReason = ""
	}

	// Write [DONE]
	if c.receiver != nil {
		return c.receiver.Process(transform.PipelineEvent{Type: transform.EventDone})
	}
	if c.writer != nil {
		_, err := c.writer.Write([]byte("data: [DONE]\n\n"))
		return err
	}
	return nil
}

// handleEvent processes a single Responses API event and emits corresponding OpenAI chunks.
func (c *ResponsesToChatStreamingConverter) handleEvent(event *types.ResponsesStreamEvent) error {
	switch event.Type {
	case "response.created":
		return c.handleResponseCreated(event)
	case "response.output_item.added":
		return c.handleOutputItemAdded(event)
	case "response.output_text.delta":
		return c.handleOutputTextDelta(event)
	case "response.function_call_arguments.delta":
		return c.handleFunctionCallArgsDelta(event)
	case "response.function_call_arguments.done":
		return c.handleFunctionCallArgsDone(event)
	case "response.completed":
		return c.handleResponseCompleted(event)
	case "response.incomplete":
		return c.handleResponseIncomplete(event)
	default:
		// Skip unknown events
		return nil
	}
}

// handleResponseCreated handles response.created event → stores metadata.
func (c *ResponsesToChatStreamingConverter) handleResponseCreated(event *types.ResponsesStreamEvent) error {
	if event.Response == nil {
		return nil
	}

	c.responseID = event.Response.ID
	c.model = event.Response.Model
	c.started = true

	return nil
}

// handleOutputItemAdded handles response.output_item.added event.
// For function_call type, initializes tool call tracking.
func (c *ResponsesToChatStreamingConverter) handleOutputItemAdded(event *types.ResponsesStreamEvent) error {
	if event.OutputItem == nil {
		return nil
	}

	if event.OutputItem.Type == "function_call" {
		c.currentToolCallID = event.OutputItem.CallID
		c.currentToolCallName = event.OutputItem.Name
		c.toolCallArgs.Reset()
	}

	return nil
}

// handleOutputTextDelta handles response.output_text.delta event → emits OpenAI chunk with text delta.
func (c *ResponsesToChatStreamingConverter) handleOutputTextDelta(event *types.ResponsesStreamEvent) error {
	if event.Delta == "" {
		return nil
	}

	chunk := types.Chunk{
		ID:      c.responseID,
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   c.model,
		Choices: []types.Choice{
			{
				Index: 0,
				Delta: types.Delta{
					Content: event.Delta,
				},
			},
		},
	}

	return c.writeChunk(chunk)
}

// handleFunctionCallArgsDelta handles response.function_call_arguments.delta event
// → emits OpenAI chunk with tool call arguments delta.
func (c *ResponsesToChatStreamingConverter) handleFunctionCallArgsDelta(event *types.ResponsesStreamEvent) error {
	if event.Delta == "" {
		return nil
	}

	c.toolCallArgs.WriteString(event.Delta)

	chunk := types.Chunk{
		ID:      c.responseID,
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   c.model,
		Choices: []types.Choice{
			{
				Index: 0,
				Delta: types.Delta{
					ToolCalls: []types.ToolCall{
						{
							Index: c.toolCallIdx,
							Function: types.Function{
								Arguments: event.Delta,
							},
						},
					},
				},
			},
		},
	}

	return c.writeChunk(chunk)
}

// handleFunctionCallArgsDone handles response.function_call_arguments.done event
// → emits final tool call chunk with ID and name.
func (c *ResponsesToChatStreamingConverter) handleFunctionCallArgsDone(event *types.ResponsesStreamEvent) error {
	// Emit chunk with full tool call info
	chunk := types.Chunk{
		ID:      c.responseID,
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   c.model,
		Choices: []types.Choice{
			{
				Index: 0,
				Delta: types.Delta{
					ToolCalls: []types.ToolCall{
						{
							Index: c.toolCallIdx,
							ID:    c.currentToolCallID,
							Type:  "function",
							Function: types.Function{
								Name:      c.currentToolCallName,
								Arguments: c.toolCallArgs.String(),
							},
						},
					},
				},
			},
		},
	}

	if err := c.writeChunk(chunk); err != nil {
		return err
	}

	c.toolCallIdx++
	c.currentToolCallID = ""
	c.currentToolCallName = ""
	c.toolCallArgs.Reset()

	return nil
}

// handleResponseCompleted handles response.completed event → sets finish reason.
func (c *ResponsesToChatStreamingConverter) handleResponseCompleted(event *types.ResponsesStreamEvent) error {
	if event.Response == nil {
		return nil
	}

	// Determine finish_reason based on output items
	c.finishReason = "stop"
	for _, item := range event.Response.Output {
		if item.Type == "function_call" {
			c.finishReason = "tool_calls"
			break
		}
	}

	// Update usage if available
	if event.Response.Usage != nil {
		c.promptTokens = event.Response.Usage.InputTokens
		c.completionTokens = event.Response.Usage.OutputTokens
	}

	return nil
}

// handleResponseIncomplete handles response.incomplete event → sets finish reason to length.
func (c *ResponsesToChatStreamingConverter) handleResponseIncomplete(event *types.ResponsesStreamEvent) error {
	c.finishReason = "length"
	return nil
}

// writeChunk writes an OpenAI Chat chunk as SSE data.
func (c *ResponsesToChatStreamingConverter) writeChunk(chunk types.Chunk) error {
	dataBytes, err := json.Marshal(chunk)
	if err != nil {
		return err
	}

	// Emit to receiver if configured
	if c.receiver != nil {
		return c.receiver.Process(transform.PipelineEvent{
			Type: transform.EventOpenAIChunk,
			Data: dataBytes,
		})
	}

	// Write directly to writer
	if c.writer != nil {
		_, err := c.writer.Write([]byte("data: " + string(dataBytes) + "\n\n"))
		return err
	}

	return nil
}

// Verify interface compliance
var _ transform.Stage = (*ResponsesToChatStreamingConverter)(nil)
