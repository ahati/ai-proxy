package toolcall

import (
	"encoding/json"
	"io"

	"ai-proxy/types"

	"github.com/tmaxmax/go-sse"
)

// Transformer combines a Parser and OutputFormatter to transform tool call markup
// into API-specific streaming output. It implements SSETransformer.
//
// @brief SSE transformer that converts proprietary tool call markup to API-specific formats.
//
// @note The transformer processes SSE events from upstream LLM APIs and extracts
//
//	tool call information from the "reasoning" or "reasoning_content" fields (OpenAI)
//	or from "text" in content_block_delta events (Anthropic).
//	This is specific to models that embed tool calls in text/reasoning content.
//
// @note The transformer is NOT thread-safe. Create a new instance for each request.
// @note The transformer maintains internal state and must be properly closed to
//
//	release resources and flush remaining content.
//
// @pre Must be initialized via NewOpenAITransformer or NewAnthropicTransformer.
// @post After Close(), the transformer must not be used for further transformations.
//
// Data Flow:
// 1. SSE event received from upstream
// 2. Event data parsed as JSON (OpenAI or Anthropic format)
// 3. Text content extracted from delta
// 4. Parser extracts tool call events from markup
// 5. Formatter converts events to API-specific format
// 6. Formatted output written to destination
type Transformer struct {
	// parser extracts tool call events from text content.
	// Initialized with DefaultTokens for Kimi model compatibility.
	parser *Parser

	// formatter converts parsed events to API-specific format.
	// Either OpenAIFormatter or AnthropicFormatter depending on constructor used.
	formatter OutputFormatter

	// output is the destination for formatted SSE data.
	// Must be a valid io.Writer that accepts concurrent writes.
	output io.Writer

	// messageID is the unique identifier for the response.
	// Extracted from the first upstream chunk and propagated to output.
	messageID string

	// model is the model name for the response.
	// Extracted from the first upstream chunk and propagated to output.
	model string

	// buf is an internal buffer for accumulating output.
	// Currently unused but reserved for future optimization.
	buf []byte

	// blockIndex tracks the current content block index for Anthropic format.
	// Incremented when processing tool_use blocks.
	blockIndex int

	// inToolUse indicates if we're currently processing tool call markup.
	// Used to track state across multiple events.
	inToolUse bool
}

// NewOpenAITransformer creates a transformer that outputs OpenAI streaming format.
//
// @brief Creates a new Transformer configured for OpenAI streaming output.
//
// @param output The destination writer for formatted output.
//
//	Must be non-nil and writable.
//	Should support flushing for streaming responses.
//
// @param messageID The unique identifier for the chat completion.
//
//	May be empty (will be extracted from upstream response).
//	Format: typically "chatcmpl-xxx" or similar.
//
// @param model The model name for the response.
//
//	May be empty (will be extracted from upstream response).
//	Examples: "gpt-4", "moonshotai/Kimi-K2.5-TEE".
//
// @return *Transformer A new transformer ready for Transform calls.
//
// @pre output must be non-nil and writable.
// @post Transformer is ready to process SSE events.
// @post Parser is initialized with DefaultTokens.
// @post Formatter is OpenAIFormatter.
//
// @note Use this constructor for OpenAI-compatible clients.
func NewOpenAITransformer(output io.Writer, messageID, model string) *Transformer {
	return &Transformer{
		parser:    NewParser(DefaultTokens),
		formatter: NewOpenAIFormatter(messageID, model),
		output:    output,
		messageID: messageID,
		model:     model,
	}
}

// NewAnthropicTransformer creates a transformer that outputs Anthropic streaming format.
//
// @brief Creates a new Transformer configured for Anthropic streaming output.
//
// @param output The destination writer for formatted output.
//
//	Must be non-nil and writable.
//	Should support flushing for streaming responses.
//
// @param messageID The unique identifier for the message.
//
//	May be empty (Anthropic format may not require it).
//	Format: typically "msg_xxx" or similar.
//
// @param model The model name for the response.
//
//	May be empty (will be extracted from upstream response).
//	Examples: "claude-3-opus-20240229", "kimi-k2.5".
//
// @return *Transformer A new transformer ready for Transform calls.
//
// @pre output must be non-nil and writable.
// @post Transformer is ready to process SSE events.
// @post Parser is initialized with DefaultTokens.
// @post Formatter is AnthropicFormatter.
//
// @note Use this constructor for Anthropic-compatible clients.
func NewAnthropicTransformer(output io.Writer, messageID, model string) *Transformer {
	return &Transformer{
		parser:    NewParser(DefaultTokens),
		formatter: NewAnthropicFormatter(messageID, model),
		output:    output,
		messageID: messageID,
		model:     model,
	}
}

// Transform processes an SSE event and writes formatted output.
// It handles both OpenAI and Anthropic format events, extracting tool calls
// from text content and converting them to the appropriate format.
//
// @brief Processes a single SSE event and writes transformed output.
//
// @param event The SSE event to process.
//
//	Must not be nil.
//	Data field should contain JSON chunk or "[DONE]" marker.
//
// @return error Returns nil on success.
//
// @note The transformer handles both OpenAI and Anthropic streaming formats.
func (t *Transformer) Transform(event *sse.Event) error {
	// Skip empty events and stream termination markers.
	if event.Data == "" || event.Data == "[DONE]" {
		return nil
	}

	data := []byte(event.Data)

	// Try to detect event format and process accordingly.
	// First, try as Anthropic format event.
	var anthropicEvent types.Event
	if err := json.Unmarshal(data, &anthropicEvent); err == nil && anthropicEvent.Type != "" {
		return t.transformAnthropicEvent(anthropicEvent, data)
	}

	// Try as OpenAI format chunk.
	var chunk types.Chunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		// JSON parsing failed - write as SSE formatted raw data.
		return t.writeSSE(data)
	}

	return t.transformOpenAIChunk(chunk, data)
}

// transformAnthropicEvent processes an Anthropic format streaming event.
// It extracts tool call markup from text content and converts it to tool_use events.
//
// @brief Transforms Anthropic format streaming events.
//
// @param event The parsed Anthropic event.
// @param rawData The raw JSON data for passthrough events.
//
// @return error Returns nil on success.
func (t *Transformer) transformAnthropicEvent(event types.Event, rawData []byte) error {
	switch event.Type {
	case "message_start":
		// Extract message ID from message_start event.
		if event.Message != nil && event.Message.ID != "" {
			t.messageID = event.Message.ID
			t.model = event.Message.Model
			t.setMessageID(event.Message.ID, event.Message.Model)
		}
		// Pass through message_start unchanged with SSE formatting.
		return t.writeSSE(rawData)

	case "content_block_start":
		// Pass through content_block_start unchanged with SSE formatting.
		return t.writeSSE(rawData)

	case "content_block_delta":
		// Check if delta contains text with tool call markup.
		return t.transformAnthropicDelta(event, rawData)

	case "content_block_stop", "message_delta", "message_stop", "ping":
		// Pass through these events unchanged with SSE formatting.
		return t.writeSSE(rawData)

	default:
		// Pass through unknown events unchanged with SSE formatting.
		return t.writeSSE(rawData)
	}
}

// transformAnthropicDelta processes a content_block_delta event.
// It checks for tool call markup in the text and converts it appropriately.
//
// @brief Transforms Anthropic content_block_delta events.
//
// @param event The Anthropic event with delta.
// @param rawData The raw JSON data for passthrough.
//
// @return error Returns nil on success.
func (t *Transformer) transformAnthropicDelta(event types.Event, rawData []byte) error {
	// Parse the delta to extract text.
	var delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(event.Delta, &delta); err != nil {
		// Failed to parse delta - pass through unchanged with SSE formatting.
		return t.writeSSE(rawData)
	}

	// Only process text_delta events with text content.
	if delta.Type != "text_delta" || delta.Text == "" {
		return t.writeSSE(rawData)
	}

	// Check if text contains tool call markup.
	if !t.parser.tokens.ContainsAny(delta.Text) {
		// No tool call markup - pass through unchanged with SSE formatting.
		return t.writeSSE(rawData)
	}

	// Extract tool call markup and convert to tool_use events.
	return t.extractAndConvertToolCalls(delta.Text, event.Index)
}

// transformOpenAIChunk processes an OpenAI format streaming chunk.
// It extracts tool call markup from reasoning content.
//
// @brief Transforms OpenAI format streaming chunks.
//
// @param chunk The parsed OpenAI chunk.
// @param rawData The raw JSON data for passthrough.
//
// @return error Returns nil on success.
func (t *Transformer) transformOpenAIChunk(chunk types.Chunk, rawData []byte) error {
	// Extract messageID and model from first chunk if not already set.
	if t.messageID == "" && chunk.ID != "" {
		t.messageID = chunk.ID
		t.model = chunk.Model
		t.setMessageID(chunk.ID, chunk.Model)
	}

	// Skip chunks without choices.
	if len(chunk.Choices) == 0 {
		return nil
	}

	delta := chunk.Choices[0].Delta

	// Handle regular content in the delta.
	if delta.Content != "" {
		return t.write(t.formatter.FormatContent(delta.Content))
	}

	// Extract reasoning content where tool calls are embedded.
	text := delta.Reasoning
	if text == "" {
		text = delta.ReasoningContent
	}

	if text == "" {
		return nil
	}

	// Check if we're in the middle of parsing tool calls OR if text contains new tool call markup.
	// If parser is not idle, we need to feed it more text regardless of whether it contains tokens.
	if !t.parser.IsIdle() || t.parser.tokens.ContainsAny(text) {
		// Parse and convert tool calls.
		events := t.parser.Parse(text)
		for _, e := range events {
			if err := t.writeEvent(e); err != nil {
				return err
			}
		}
		return nil
	}

	// No tool call markup and parser is idle - write as content.
	return t.write(t.formatter.FormatContent(text))
}

// extractAndConvertToolCalls parses tool call markup from text and writes
// proper tool_use events for Anthropic format.
//
// @brief Extracts tool calls from markup and converts to Anthropic format.
//
// @param text The text containing tool call markup.
// @param blockIndex The current block index from the event.
//
// @return error Returns nil on success.
func (t *Transformer) extractAndConvertToolCalls(text string, blockIndex *int) error {
	// Parse the text for tool call events.
	events := t.parser.Parse(text)

	for _, e := range events {
		switch e.Type {
		case EventContent:
			// Regular text content - write as text_delta.
			if e.Text != "" {
				delta := map[string]string{"type": "text_delta", "text": e.Text}
				deltaJSON, _ := json.Marshal(delta)
				ev := types.Event{
					Type:  "content_block_delta",
					Delta: deltaJSON,
				}
				if blockIndex != nil {
					ev.Index = blockIndex
				}
				t.writeEvent(Event{Type: EventContent, Text: e.Text})
			}
		case EventToolStart:
			// Start of tool call - emit content_block_start for tool_use.
			t.inToolUse = true
			t.blockIndex++
			if err := t.writeEvent(e); err != nil {
				return err
			}
		case EventToolArgs:
			// Tool arguments - emit as input_json_delta.
			if err := t.writeEvent(e); err != nil {
				return err
			}
		case EventToolEnd, EventSectionEnd:
			// End of tool call.
			if err := t.writeEvent(e); err != nil {
				return err
			}
			t.inToolUse = false
		}
	}
	return nil
}

// writeEvent formats and writes a single parsed event to the output.
//
// @brief Internal method to format and write a parser event.
//
// @param e The event to write.
//
//	Must have a valid Type field.
//
// @return error Returns nil on success.
//
//	Returns error if output write fails.
//
// @pre e.Type must be a valid EventType.
// @post Event is formatted and written to output.
func (t *Transformer) writeEvent(e Event) error {
	switch e.Type {
	case EventContent:
		return t.write(t.formatter.FormatContent(e.Text))
	case EventToolStart:
		return t.write(t.formatter.FormatToolStart(e.ID, e.Name, e.Index))
	case EventToolArgs:
		return t.write(t.formatter.FormatToolArgs(e.Args, e.Index))
	case EventToolEnd:
		return t.write(t.formatter.FormatToolEnd(e.Index))
	case EventSectionEnd:
		return t.write(t.formatter.FormatSectionEnd())
	}
	return nil
}

// write writes data to the output writer.
// If data doesn't already have SSE format, it's written as-is.
//
// @brief Internal method to write data to output.
//
// @param data The data to write.
//
//	May be nil or empty (no-op).
//
// @return error Returns nil on success or if data is empty.
//
// @pre output writer must be writable.
// @post Data is written to output (if non-empty).
func (t *Transformer) write(data []byte) error {
	// Skip nil or empty data to avoid unnecessary writes.
	if len(data) == 0 {
		return nil
	}
	_, err := t.output.Write(data)
	return err
}

// writeSSE writes data in SSE format: "data: <content>\n\n".
//
// @brief Writes data with SSE formatting.
//
// @param data The data to write in SSE format.
//
// @return error Returns nil on success.
func (t *Transformer) writeSSE(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	_, err := t.output.Write([]byte("data: "))
	if err != nil {
		return err
	}
	_, err = t.output.Write(data)
	if err != nil {
		return err
	}
	_, err = t.output.Write([]byte("\n\n"))
	return err
}

// setMessageID updates the message ID in the formatter.
// This method uses type assertion to call the appropriate setter.
//
// @brief Internal method to propagate message ID to the formatter.
//
// @param id The message ID to set.
//
//	Must be valid UTF-8 for JSON encoding.
//
// @param model The model name to set.
//
//	Must be valid UTF-8 for JSON encoding.
//
// @return None (method returns no value).
//
// @pre Formatter must be OpenAIFormatter or AnthropicFormatter.
// @post Formatter's message ID and model are updated.
//
// @note Uses type assertion to handle different formatter types.
//
//	This is a design trade-off for flexibility.
func (t *Transformer) setMessageID(id, model string) {
	switch f := t.formatter.(type) {
	case *OpenAIFormatter:
		f.SetMessageID(id)
		f.SetModel(model)
	case *AnthropicFormatter:
		f.SetMessageID(id)
		f.SetModel(model)
	}
}

// Flush processes any remaining buffered content in the parser.
// This ensures all pending data is written to the output.
//
// @brief Flushes all buffered content from the parser to output.
//
// @return error Returns nil on success.
//
//	Returns error if:
//	- Parser produces events that fail to write
//	- Output write fails
//
// @pre Transform() should have been called for all events.
// @post All buffered content is written to output.
// @post Parser buffer is empty.
//
// @note Must be called after the last Transform() call to ensure
//
//	all pending content is written. Not calling Flush() may
//	result in truncated output at the client.
//
// @note Flush() is idempotent - calling multiple times has no effect
//
//	after the first call (parser buffer is empty).
func (t *Transformer) Flush() error {
	for {
		// Parse with empty string to flush remaining buffer.
		// The parser will emit any complete events from buffered data.
		events := t.parser.Parse("")
		if len(events) == 0 {
			// No more events - buffer is empty.
			return nil
		}
		// Write all flushed events to output.
		for _, e := range events {
			if err := t.writeEvent(e); err != nil {
				return err
			}
		}
	}
}

// Close flushes remaining content and releases resources.
// After Close(), the transformer must not be used.
//
// @brief Releases all resources held by the transformer.
//
// @return error Returns nil on success.
//
//	Returns error if Flush() fails.
//
// @pre None (safe to call even if never used).
// @post Transformer is in a closed state and must not be used.
// @post All buffered content is flushed.
//
// @note Close() calls Flush() internally before releasing resources.
//
//	After Close(), subsequent calls to Transform(), Flush(), or Close()
//	may panic or return errors.
func (t *Transformer) Close() error {
	if err := t.Flush(); err != nil {
		return err
	}
	return nil
}

// Parser returns the internal parser for testing and inspection.
//
// @brief Returns the internal Parser instance.
//
// @return *Parser The parser used by this transformer.
//
// @pre None.
// @post No state is modified.
//
// @note This method is primarily for testing and debugging.
//
//	Production code should not need to access the parser directly.
func (t *Transformer) Parser() *Parser {
	return t.parser
}

// Formatter returns the output formatter for testing and inspection.
//
// @brief Returns the internal OutputFormatter instance.
//
// @return OutputFormatter The formatter used by this transformer.
//
// @pre None.
// @post No state is modified.
//
// @note This method is primarily for testing and debugging.
//
//	Production code should not need to access the formatter directly.
func (t *Transformer) Formatter() OutputFormatter {
	return t.formatter
}
