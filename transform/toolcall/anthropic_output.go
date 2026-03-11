package toolcall

import (
	"encoding/json"
	"fmt"
	"io"

	"ai-proxy/types"
)

type OutputContext int

const (
	ContextThinking OutputContext = iota
	ContextText
)

type AnthropicOutput struct {
	writer       io.Writer
	context      OutputContext
	blockIndex   int
	currentIndex int
	toolsEmitted bool
	blockOpen    bool
}

func NewAnthropicOutput(writer io.Writer, context OutputContext, initialBlockIndex int) *AnthropicOutput {
	return &AnthropicOutput{
		writer:     writer,
		context:    context,
		blockIndex: initialBlockIndex,
	}
}

func (o *AnthropicOutput) SetBlockOpen(open bool) {
	o.blockOpen = open
}

func (o *AnthropicOutput) ToolsEmitted() bool {
	return o.toolsEmitted
}

func (o *AnthropicOutput) BlockIndex() int {
	return o.blockIndex
}

func (o *AnthropicOutput) OnText(text string) {
	if text == "" {
		return
	}

	if o.context == ContextThinking {
		delta := types.ThinkingDelta{
			Type:     "thinking_delta",
			Thinking: text,
		}
		o.writeContentBlockDelta(o.currentIndex, delta)
	} else {
		delta := types.TextDelta{
			Type: "text_delta",
			Text: text,
		}
		o.writeContentBlockDelta(o.currentIndex, delta)
	}
}

func (o *AnthropicOutput) OnToolCallStart(id, name string, index int) {
	if o.blockOpen {
		o.writeContentBlockStop(o.currentIndex)
		o.blockIndex++
	}

	o.toolsEmitted = true
	o.currentIndex = o.blockIndex

	block := types.ContentBlock{
		Type:  "tool_use",
		ID:    id,
		Name:  name,
		Input: json.RawMessage("{}"),
	}
	o.writeContentBlockStart(o.currentIndex, block)
	o.blockOpen = true
}

func (o *AnthropicOutput) OnToolCallArgs(args string, index int) {
	delta := types.InputJSONDelta{
		Type:        "input_json_delta",
		PartialJSON: args,
	}
	o.writeContentBlockDelta(o.currentIndex, delta)
}

func (o *AnthropicOutput) OnToolCallEnd(index int) {
	o.writeContentBlockStop(o.currentIndex)
	o.blockIndex++
	o.blockOpen = false
}

func (o *AnthropicOutput) writeContentBlockStart(index int, block types.ContentBlock) {
	blockJSON, _ := json.Marshal(block)
	event := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(index),
		ContentBlock: blockJSON,
	}
	o.writeSSE(event)
}

func (o *AnthropicOutput) writeContentBlockDelta(index int, delta interface{}) {
	deltaJSON, _ := json.Marshal(delta)
	event := types.Event{
		Type:  "content_block_delta",
		Index: intPtr(index),
		Delta: deltaJSON,
	}
	o.writeSSE(event)
}

func (o *AnthropicOutput) writeContentBlockStop(index int) {
	event := types.Event{
		Type:  "content_block_stop",
		Index: intPtr(index),
	}
	o.writeSSE(event)
}

func (o *AnthropicOutput) writeSSE(event types.Event) {
	data, _ := json.Marshal(event)
	line := fmt.Sprintf("event: %s\ndata: %s\n\n", event.Type, string(data))
	o.writer.Write([]byte(line))
}

func intPtr(i int) *int {
	return &i
}
