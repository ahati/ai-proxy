package convert

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"ai-proxy/transform"
	"ai-proxy/types"
)

// AnthropicToChatStreamingConverter converts Anthropic SSE events to OpenAI Chat chunks.
// It implements AnthropicEventReceiver for chaining with AnthropicTransformer.
//
// @brief Streaming converter from Anthropic SSE events to OpenAI Chat chunks.
//
// @note This enables the chain: Anthropic SSE → AnthropicTransformer → AnthropicToChatStreamingConverter → OpenAI Chat SSE
type AnthropicToChatStreamingConverter struct {
	writer io.Writer

	// State tracking
	messageID string
	model     string
	started   bool
	blockType string // Current block type: "text", "thinking", "tool_use"
	blockIdx  int

	// Tool call tracking
	toolCallID   string
	toolCallName string
	toolCallArgs strings.Builder

	// Usage tracking
	promptTokens     int
	completionTokens int

	// Finish reason
	finishReason string

	// Optional output to another receiver for further chaining
	receiver transform.OpenAIChatReceiver
}

// NewAnthropicToChatStreamingConverter creates a converter that writes to an io.Writer.
//
// @brief Creates a new streaming converter writing OpenAI chunks to w.
//
// @param w The io.Writer to write OpenAI Chat chunks to.
//
// @return *AnthropicToChatStreamingConverter A new converter instance.
func NewAnthropicToChatStreamingConverter(w io.Writer) *AnthropicToChatStreamingConverter {
	return &AnthropicToChatStreamingConverter{
		writer: w,
	}
}

// NewAnthropicToChatStreamingConverterWithReceiver creates a converter that sends to a receiver.
//
// @brief Creates a new streaming converter sending chunks to a receiver.
//
// @param receiver The OpenAIChatReceiver to send chunk JSON to.
//
// @return *AnthropicToChatStreamingConverter A new converter instance.
func NewAnthropicToChatStreamingConverterWithReceiver(receiver transform.OpenAIChatReceiver) *AnthropicToChatStreamingConverter {
	return &AnthropicToChatStreamingConverter{
		receiver: receiver,
	}
}

// Receive processes an Anthropic event JSON string.
//
// @brief Implements AnthropicEventReceiver.Receive.
//
// @param eventJSON The raw JSON of an Anthropic event.
//
// @return error Returns nil on success.
func (c *AnthropicToChatStreamingConverter) Receive(eventJSON string) error {
	var event types.Event
	if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
		return nil // Skip unparseable events
	}

	return c.handleEvent(event, eventJSON)
}

// ReceiveDone signals the end of the stream.
//
// @brief Implements AnthropicEventReceiver.ReceiveDone.
func (c *AnthropicToChatStreamingConverter) ReceiveDone() error {
	return c.Flush()
}

// Flush writes any buffered data.
//
// @brief Implements AnthropicEventReceiver.Flush.
func (c *AnthropicToChatStreamingConverter) Flush() error {
	// Emit final chunk with finish reason if we have content
	if c.finishReason != "" {
		chunk := c.createChunk()
		chunk.Choices = []types.Choice{{
			Index:        0,
			FinishReason: &c.finishReason,
		}}
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
		return c.receiver.ReceiveDone()
	}
	if c.writer != nil {
		_, err := c.writer.Write([]byte("data: [DONE]\n\n"))
		return err
	}
	return nil
}

func (c *AnthropicToChatStreamingConverter) handleEvent(event types.Event, rawJSON string) error {
	switch event.Type {
	case "message_start":
		return c.handleMessageStart(event, rawJSON)
	case "content_block_start":
		return c.handleContentBlockStart(event)
	case "content_block_delta":
		return c.handleContentBlockDelta(event)
	case "content_block_stop":
		return c.handleContentBlockStop()
	case "message_delta":
		return c.handleMessageDelta(event, rawJSON)
	case "message_stop":
		return c.handleMessageStop()
	default:
		return nil
	}
}

func (c *AnthropicToChatStreamingConverter) handleMessageStart(event types.Event, rawJSON string) error {
	if event.Message != nil {
		c.messageID = event.Message.ID
		c.model = event.Message.Model
		c.started = true

		// Extract usage if present
		if event.Message.Usage != nil {
			c.promptTokens = event.Message.Usage.InputTokens
		}
	}
	return nil
}

func (c *AnthropicToChatStreamingConverter) handleContentBlockStart(event types.Event) error {
	if event.ContentBlock == nil {
		return nil
	}

	var block types.ContentBlock
	if err := json.Unmarshal(event.ContentBlock, &block); err != nil {
		return nil
	}

	c.blockType = block.Type
	c.blockIdx++

	if block.Type == "tool_use" {
		c.toolCallID = block.ID
		c.toolCallName = block.Name
		c.toolCallArgs.Reset()

		// Emit tool call start chunk
		chunk := c.createChunk()
		chunk.Choices = []types.Choice{{
			Index: 0,
			Delta: types.Delta{
				ToolCalls: []types.ToolCall{{
					ID:    block.ID,
					Type:  "function",
					Index: c.blockIdx - 1,
					Function: types.Function{
						Name: block.Name,
					},
				}},
			},
		}}
		return c.writeChunk(chunk)
	}

	return nil
}

func (c *AnthropicToChatStreamingConverter) handleContentBlockDelta(event types.Event) error {
	if event.Delta == nil {
		return nil
	}

	switch c.blockType {
	case "text":
		return c.handleTextDelta(event.Delta)
	case "thinking":
		return c.handleThinkingDelta(event.Delta)
	case "tool_use":
		return c.handleToolArgsDelta(event.Delta)
	}
	return nil
}

func (c *AnthropicToChatStreamingConverter) handleTextDelta(delta json.RawMessage) error {
	var textDelta types.TextDelta
	if err := json.Unmarshal(delta, &textDelta); err != nil {
		return nil
	}
	if textDelta.Text == "" {
		return nil
	}

	chunk := c.createChunk()
	chunk.Choices = []types.Choice{{
		Index: 0,
		Delta: types.Delta{
			Content: textDelta.Text,
		},
	}}
	return c.writeChunk(chunk)
}

func (c *AnthropicToChatStreamingConverter) handleThinkingDelta(delta json.RawMessage) error {
	var thinkingDelta types.ThinkingDelta
	if err := json.Unmarshal(delta, &thinkingDelta); err != nil {
		return nil
	}
	if thinkingDelta.Thinking == "" {
		return nil
	}

	// Map thinking to reasoning_content (OpenAI extended field)
	chunk := c.createChunk()
	chunk.Choices = []types.Choice{{
		Index: 0,
		Delta: types.Delta{
			ReasoningContent: thinkingDelta.Thinking,
		},
	}}
	return c.writeChunk(chunk)
}

func (c *AnthropicToChatStreamingConverter) handleToolArgsDelta(delta json.RawMessage) error {
	var inputDelta types.InputJSONDelta
	if err := json.Unmarshal(delta, &inputDelta); err != nil {
		return nil
	}
	if inputDelta.PartialJSON == "" {
		return nil
	}

	c.toolCallArgs.WriteString(inputDelta.PartialJSON)

	chunk := c.createChunk()
	chunk.Choices = []types.Choice{{
		Index: 0,
		Delta: types.Delta{
			ToolCalls: []types.ToolCall{{
				Index: c.blockIdx - 1,
				Function: types.Function{
					Arguments: inputDelta.PartialJSON,
				},
			}},
		},
	}}
	return c.writeChunk(chunk)
}

func (c *AnthropicToChatStreamingConverter) handleContentBlockStop() error {
	// For tool_use, we don't need to emit anything special
	// The tool call is complete when we stop receiving args
	c.blockType = ""
	return nil
}

func (c *AnthropicToChatStreamingConverter) handleMessageDelta(event types.Event, rawJSON string) error {
	// Parse raw JSON to get usage
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &rawData); err == nil {
		if usage, ok := rawData["usage"].(map[string]interface{}); ok {
			if ot, ok := usage["output_tokens"].(float64); ok {
				c.completionTokens = int(ot)
			}
		}
		if delta, ok := rawData["delta"].(map[string]interface{}); ok {
			if sr, ok := delta["stop_reason"].(string); ok {
				c.finishReason = c.mapStopReason(sr)
			}
		}
	}
	return nil
}

func (c *AnthropicToChatStreamingConverter) handleMessageStop() error {
	// Set default finish reason if not already set
	if c.finishReason == "" {
		c.finishReason = "stop"
	}
	return nil
}

func (c *AnthropicToChatStreamingConverter) mapStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	default:
		return "stop"
	}
}

func (c *AnthropicToChatStreamingConverter) createChunk() *types.Chunk {
	return &types.Chunk{
		ID:      c.messageID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   c.model,
	}
}

func (c *AnthropicToChatStreamingConverter) writeChunk(chunk *types.Chunk) error {
	data, err := json.Marshal(chunk)
	if err != nil {
		return err
	}

	if c.receiver != nil {
		return c.receiver.Receive(string(data))
	}

	if c.writer != nil {
		_, err = fmt.Fprintf(c.writer, "data: %s\n\n", string(data))
		return err
	}

	return nil
}
