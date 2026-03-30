package toolcall

import (
	"encoding/json"
	"fmt"
	"io"

	"ai-proxy/logging"
	"ai-proxy/transform"
	"ai-proxy/types"

	"github.com/tmaxmax/go-sse"
)

type AnthropicTransformer struct {
	sseWriter  *transform.SSEWriter
	formatter  *AnthropicFormatter
	toolIndex  int
	blockIndex int
	messageID  string

	inThinking       bool
	inText           bool
	thinkingIndex    int
	textIndex        int
	needThinkingStop bool
	needTextStop     bool
	toolsEmitted     bool

	// GLM-5 tool call extraction
	glm5Parser            *GLM5Parser
	glm5ToolCallTransform bool

	// Kimi tool call extraction
	kmiParser             *KimiParser
	kimiToolCallTransform bool

	// receiver is an optional output destination for Anthropic event JSON
	// When set, events are sent here instead of sseWriter
	receiver transform.AnthropicEventReceiver
}

func NewAnthropicTransformer(output io.Writer) *AnthropicTransformer {
	return &AnthropicTransformer{
		sseWriter:             transform.NewSSEWriter(output),
		formatter:             NewAnthropicFormatter("", ""),
		glm5Parser:            NewGLM5Parser(),
		glm5ToolCallTransform: false,
		kmiParser:             NewKimiParser(),
		kimiToolCallTransform: false,
	}
}

// NewAnthropicTransformerWithReceiver creates a transformer that sends events to a receiver.
// This enables chaining: AnthropicTransformer → AnthropicEventReceiver → target format.
//
// @brief Creates a transformer that outputs to an AnthropicEventReceiver.
//
// @param receiver The receiver to send event JSON to.
//
// @return *AnthropicTransformer A new transformer instance.
func NewAnthropicTransformerWithReceiver(receiver transform.AnthropicEventReceiver) *AnthropicTransformer {
	return &AnthropicTransformer{
		formatter:             NewAnthropicFormatter("", ""),
		glm5Parser:            NewGLM5Parser(),
		glm5ToolCallTransform: false,
		kmiParser:             NewKimiParser(),
		kimiToolCallTransform: false,
		receiver:              receiver,
	}
}

// SetGLM5ToolCallTransform enables or disables GLM-5 XML tool call extraction.
func (t *AnthropicTransformer) SetGLM5ToolCallTransform(enabled bool) {
	t.glm5ToolCallTransform = enabled
}

// SetKimiToolCallTransform enables or disables Kimi-style tool call extraction.
func (t *AnthropicTransformer) SetKimiToolCallTransform(enabled bool) {
	t.kimiToolCallTransform = enabled
}

func (t *AnthropicTransformer) Transform(event *sse.Event) error {
	if event.Data == "" {
		return nil
	}

	if event.Data == "[DONE]" {
		return t.writeDone()
	}

	var anthropicEvent types.Event
	if err := json.Unmarshal([]byte(event.Data), &anthropicEvent); err != nil {
		return t.writePassthrough("error", []byte(event.Data))
	}
	if anthropicEvent.Type == "" {
		return t.writeData([]byte(event.Data))
	}

	return t.handleEvent(anthropicEvent, []byte(event.Data))
}

func (t *AnthropicTransformer) handleEvent(event types.Event, rawJSON []byte) error {
	switch event.Type {
	case "message_start":
		return t.handleMessageStart(event, rawJSON)
	case "content_block_start":
		return t.handleContentBlockStart(event)
	case "content_block_delta":
		return t.handleContentBlockDelta(event)
	case "content_block_stop":
		return t.handleContentBlockStop(event)
	case "message_delta":
		return t.handleMessageDelta(event, rawJSON)
	case "message_stop", "ping":
		return t.writePassthrough(event.Type, rawJSON)
	default:
		return t.writePassthrough(event.Type, rawJSON)
	}
}

func (t *AnthropicTransformer) handleMessageStart(event types.Event, rawJSON []byte) error {
	if event.Message != nil && event.Message.ID != "" {
		t.messageID = event.Message.ID
		t.blockIndex = 0
		t.formatter.SetMessageID(event.Message.ID)
		t.formatter.SetModel(event.Message.Model)
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal(rawJSON, &rawData); err != nil {
		return t.writePassthrough(event.Type, rawJSON)
	}

	if message, ok := rawData["message"].(map[string]interface{}); ok {
		if usage, ok := message["usage"].(map[string]interface{}); ok {
			if _, hasInputTokens := usage["input_tokens"]; !hasInputTokens {
				if promptTokens, exists := usage["prompt_tokens"]; exists {
					usage["input_tokens"] = promptTokens
					delete(usage, "prompt_tokens")
				}
			}
			if _, hasOutputTokens := usage["output_tokens"]; !hasOutputTokens {
				if completionTokens, exists := usage["completion_tokens"]; exists {
					usage["output_tokens"] = completionTokens
					delete(usage, "completion_tokens")
				}
			}
			delete(usage, "total_tokens")
		}
	}

	return t.writePassthrough(event.Type, marshalJSON(rawData))
}

func (t *AnthropicTransformer) handleContentBlockStart(event types.Event) error {
	if event.ContentBlock != nil {
		var block types.ContentBlock
		if err := json.Unmarshal(event.ContentBlock, &block); err == nil {
			if block.Type == "thinking" {
				t.inThinking = true
				if event.Index != nil {
					t.thinkingIndex = *event.Index
				}
				t.needThinkingStop = true
				return t.writePassthrough(event.Type, marshalJSON(event))
			}
			if block.Type == "text" {
				t.inText = true
				if event.Index != nil {
					t.textIndex = *event.Index
				}
				t.needTextStop = true
				return t.writePassthrough(event.Type, marshalJSON(event))
			}
		}
	}
	if event.Index != nil && *event.Index >= t.blockIndex {
		t.blockIndex = *event.Index + 1
	}
	return t.writePassthrough(event.Type, marshalJSON(event))
}

func (t *AnthropicTransformer) handleContentBlockDelta(event types.Event) error {
	if t.inThinking {
		return t.handleThinkingDelta(event)
	}
	if t.inText {
		return t.handleTextDelta(event)
	}
	return t.writePassthrough(event.Type, marshalJSON(event))
}

func (t *AnthropicTransformer) handleThinkingDelta(event types.Event) error {
	var delta types.ThinkingDelta
	if err := json.Unmarshal(event.Delta, &delta); err != nil {
		return t.writePassthrough(event.Type, marshalJSON(event))
	}
	if delta.Type != "thinking_delta" {
		return t.writePassthrough(event.Type, marshalJSON(event))
	}

	idx := 0
	if event.Index != nil {
		idx = *event.Index
	}

	chunks := t.processThinking(delta.Thinking, idx)
	for _, chunk := range chunks {
		if err := t.write(chunk); err != nil {
			return err
		}
	}
	return nil
}

func (t *AnthropicTransformer) handleTextDelta(event types.Event) error {
	var delta types.TextDelta
	if err := json.Unmarshal(event.Delta, &delta); err != nil {
		return t.writePassthrough(event.Type, marshalJSON(event))
	}
	if delta.Type != "text_delta" {
		return t.writePassthrough(event.Type, marshalJSON(event))
	}

	idx := 0
	if event.Index != nil {
		idx = *event.Index
	}

	chunks := t.processText(delta.Text, idx)
	for _, chunk := range chunks {
		if err := t.write(chunk); err != nil {
			return err
		}
	}
	return nil
}

func (t *AnthropicTransformer) handleContentBlockStop(event types.Event) error {
	if t.inThinking {
		return t.handleThinkingBlockStop(event)
	}
	if t.inText {
		return t.handleTextBlockStop(event)
	}
	return t.writePassthrough(event.Type, marshalJSON(event))
}

func (t *AnthropicTransformer) handleThinkingBlockStop(event types.Event) error {
	t.inThinking = false
	t.flushActiveParser(true, t.thinkingIndex)
	if t.needThinkingStop {
		t.write(t.makeThinkingBlockStop(t.thinkingIndex))
		t.needThinkingStop = false
	}
	return nil
}

func (t *AnthropicTransformer) handleTextBlockStop(event types.Event) error {
	t.inText = false
	t.flushActiveParser(false, t.textIndex)
	if t.needTextStop {
		t.write(t.makeTextBlockStop(t.textIndex))
		t.needTextStop = false
	}
	return nil
}

func (t *AnthropicTransformer) flushActiveParser(isThinking bool, index int) {
	var events []Event
	if t.glm5ToolCallTransform {
		events = t.glm5Parser.ForceFlush()
	} else if t.kimiToolCallTransform {
		events = t.kmiParser.ForceFlush()
	}
	if len(events) == 0 {
		return
	}
	for _, chunk := range t.convertEventsToAnthropic(events, isThinking, index) {
		t.write(chunk)
	}
}

func (t *AnthropicTransformer) handleMessageDelta(event types.Event, rawJSON []byte) error {
	var rawData map[string]interface{}
	if err := json.Unmarshal(rawJSON, &rawData); err != nil {
		return t.writePassthrough(event.Type, rawJSON)
	}

	if usage, ok := rawData["usage"].(map[string]interface{}); ok {
		if _, hasOutputTokens := usage["output_tokens"]; !hasOutputTokens {
			if completionTokens, exists := usage["completion_tokens"]; exists {
				usage["output_tokens"] = completionTokens
				delete(usage, "completion_tokens")
			}
		}
		if _, hasInputTokens := usage["input_tokens"]; !hasInputTokens {
			if promptTokens, exists := usage["prompt_tokens"]; exists {
				usage["input_tokens"] = promptTokens
				delete(usage, "prompt_tokens")
			}
		}
		delete(usage, "total_tokens")
	}

	if t.toolsEmitted {
		if delta, ok := rawData["delta"].(map[string]interface{}); ok {
			if stopReason, exists := delta["stop_reason"].(string); exists && stopReason == "end_turn" {
				logging.InfoMsg("[%s] Changing stop_reason from 'end_turn' to 'tool_use' due to emitted tool calls", t.messageID)
				delta["stop_reason"] = "tool_use"
				return t.writePassthrough(event.Type, marshalJSON(rawData))
			}
		}
	}

	return t.writePassthrough(event.Type, marshalJSON(rawData))
}

func (t *AnthropicTransformer) processThinking(text string, index int) [][]byte {
	if t.glm5ToolCallTransform {
		events := t.glm5Parser.Parse(text)
		if len(events) > 0 {
			return t.convertEventsToAnthropic(events, true, index)
		}
		return nil
	}

	if t.kimiToolCallTransform {
		events := t.kmiParser.Parse(text)
		if len(events) > 0 {
			return t.convertEventsToAnthropic(events, true, index)
		}
		return nil
	}

	return [][]byte{t.makeThinkingDelta(index, text)}
}

func (t *AnthropicTransformer) processText(text string, index int) [][]byte {
	// if t.glm5ToolCallTransform {
	// 	events := t.glm5Parser.Parse(text)
	// 	if len(events) > 0 {
	// 		return t.convertEventsToAnthropic(events, false, index)
	// 	}
	// 	return nil
	// }

	// if t.kimiToolCallTransform {
	// 	events := t.kmiParser.Parse(text)
	// 	if len(events) > 0 {
	// 		return t.convertEventsToAnthropic(events, false, index)
	// 	}
	// 	return nil
	// }

	return [][]byte{t.makeTextDelta(index, text)}
}

func (t *AnthropicTransformer) convertEventsToAnthropic(events []Event, isThinking bool, index int) [][]byte {
	var out [][]byte

	for _, e := range events {
		switch e.Type {
		case EventContent:
			if e.Text != "" {
				if isThinking {
					out = append(out, t.makeThinkingDelta(index, e.Text))
				} else {
					out = append(out, t.makeTextDelta(index, e.Text))
				}
			}
		case EventToolStart:
			logging.InfoMsg("[%s] Tool call extracted: name=%s, id=%s, blockIndex=%d", t.messageID, e.Name, e.ID, t.blockIndex+1)
			if isThinking && t.needThinkingStop {
				out = append(out, t.makeThinkingBlockStop(t.thinkingIndex))
				t.needThinkingStop = false
			} else if !isThinking && t.needTextStop {
				out = append(out, t.makeTextBlockStop(t.textIndex))
				t.needTextStop = false
			}
			out = append(out, t.makeToolUseBlockStart(e.ID, e.Name))
		case EventToolArgs:
			out = append(out, t.makeInputJSONDelta(e.Args))
		case EventToolEnd:
			out = append(out, t.makeContentBlockStop())
			t.toolIndex++
		case EventSectionEnd:
			if isThinking {
				t.thinkingIndex = t.blockIndex
				t.blockIndex++
				out = append(out, t.makeThinkingBlockStart(t.thinkingIndex))
				t.needThinkingStop = true
			} else {
				t.textIndex = t.blockIndex
				t.blockIndex++
				out = append(out, t.makeTextBlockStart(t.textIndex))
				t.needTextStop = true
			}
		}
	}

	return out
}

func (t *AnthropicTransformer) makeThinkingDelta(index int, thinking string) []byte {
	event := types.Event{
		Type:  "content_block_delta",
		Index: intPtr(index),
		Delta: json.RawMessage(fmt.Sprintf(`{"type":"thinking_delta","thinking":%q}`, thinking)),
	}
	return serializeAnthropicEvent(event)
}

func (t *AnthropicTransformer) makeThinkingBlockStart(index int) []byte {
	event := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(index),
		ContentBlock: json.RawMessage(`{"type":"thinking","thinking":""}`),
	}
	return serializeAnthropicEvent(event)
}

func (t *AnthropicTransformer) makeThinkingBlockStop(index int) []byte {
	event := types.Event{
		Type:  "content_block_stop",
		Index: intPtr(index),
	}
	return serializeAnthropicEvent(event)
}

func (t *AnthropicTransformer) makeTextDelta(index int, text string) []byte {
	event := types.Event{
		Type:  "content_block_delta",
		Index: intPtr(index),
		Delta: json.RawMessage(fmt.Sprintf(`{"type":"text_delta","text":%q}`, text)),
	}
	return serializeAnthropicEvent(event)
}

func (t *AnthropicTransformer) makeTextBlockStart(index int) []byte {
	event := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(index),
		ContentBlock: json.RawMessage(`{"type":"text","text":""}`),
	}
	return serializeAnthropicEvent(event)
}

func (t *AnthropicTransformer) makeTextBlockStop(index int) []byte {
	event := types.Event{
		Type:  "content_block_stop",
		Index: intPtr(index),
	}
	return serializeAnthropicEvent(event)
}

func (t *AnthropicTransformer) makeToolUseBlockStart(id, name string) []byte {
	t.toolsEmitted = true
	event := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(t.blockIndex),
		ContentBlock: json.RawMessage(fmt.Sprintf(`{"type":"tool_use","id":%q,"name":%q,"input":{}}`, id, name)),
	}
	return serializeAnthropicEvent(event)
}

func (t *AnthropicTransformer) makeInputJSONDelta(partialJSON string) []byte {
	event := types.Event{
		Type:  "content_block_delta",
		Index: intPtr(t.blockIndex),
		Delta: json.RawMessage(fmt.Sprintf(`{"type":"input_json_delta","partial_json":%q}`, partialJSON)),
	}
	return serializeAnthropicEvent(event)
}

func (t *AnthropicTransformer) makeContentBlockStop() []byte {
	event := types.Event{
		Type:  "content_block_stop",
		Index: intPtr(t.blockIndex),
	}
	return serializeAnthropicEvent(event)
}

func (t *AnthropicTransformer) write(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if t.receiver != nil {
		// Extract event JSON from SSE format "event: type\ndata: {...}\n\n"
		_, jsonData := transform.ExtractAnthropicEventFromSSE(data)
		if jsonData != "" {
			return t.receiver.Receive(jsonData)
		}
		return nil
	}
	_, err := t.sseWriter.WriteRaw(data)
	return err
}

func (t *AnthropicTransformer) writeData(data []byte) error {
	if t.receiver != nil {
		return t.receiver.Receive(string(data))
	}
	return t.sseWriter.WriteData(data)
}

// writeDone signals stream end.
//
// @brief Writes the [DONE] marker or signals completion to receiver.
//
// @return error Returns nil on success.
//
// @note When receiver is set, calls ReceiveDone() instead of writing [DONE].
// This is critical for converter chains to properly emit final output.
func (t *AnthropicTransformer) writeDone() error {
	if t.receiver != nil {
		return t.receiver.ReceiveDone()
	}
	return t.sseWriter.WriteData([]byte("[DONE]"))
}

func (t *AnthropicTransformer) writePassthrough(eventType string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if t.receiver != nil {
		// For passthrough, send the JSON data as-is
		return t.receiver.Receive(string(data))
	}
	return t.sseWriter.WriteEvent(eventType, data)
}

func (t *AnthropicTransformer) Flush() error {
	isThinking := t.inThinking || !t.inText
	index := t.thinkingIndex
	if t.inText {
		index = t.textIndex
	}

	if t.glm5ToolCallTransform {
		events := t.glm5Parser.ForceFlush()
		if len(events) > 0 {
			for _, chunk := range t.convertEventsToAnthropic(events, isThinking, index) {
				t.write(chunk)
			}
		}
	}

	if t.kimiToolCallTransform {
		events := t.kmiParser.ForceFlush()
		if len(events) > 0 {
			for _, chunk := range t.convertEventsToAnthropic(events, isThinking, index) {
				t.write(chunk)
			}
		}
	}

	if t.needThinkingStop {
		t.write(t.makeThinkingBlockStop(t.thinkingIndex))
		t.needThinkingStop = false
	}
	if t.needTextStop {
		t.write(t.makeTextBlockStop(t.textIndex))
		t.needTextStop = false
	}

	// Flush receiver if present
	if t.receiver != nil {
		return t.receiver.Flush()
	}
	return nil
}

func (t *AnthropicTransformer) Close() error {
	return t.Flush()
}

func (t *AnthropicTransformer) Initialize() error {
	return nil
}

func (t *AnthropicTransformer) HandleCancel() error {
	return nil
}
