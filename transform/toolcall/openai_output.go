package toolcall

import (
	"encoding/json"
	"io"

	"ai-proxy/types"
)

type OpenAIOutput struct {
	writer  io.Writer
	base    types.StreamChunk
	current string
}

func NewOpenAIOutput(writer io.Writer, base types.StreamChunk) *OpenAIOutput {
	return &OpenAIOutput{
		writer: writer,
		base:   base,
	}
}

func (o *OpenAIOutput) OnText(text string) {
	chunk := o.shallowCopy()
	chunk.Choices[0].Delta = types.StreamDelta{Content: text}
	o.writeChunk(chunk)
}

func (o *OpenAIOutput) OnToolCallStart(id, name string, index int) {
	o.current = id
	chunk := o.shallowCopy()
	chunk.Choices[0].Delta = types.StreamDelta{
		ToolCalls: []types.StreamToolCall{{
			ID:       id,
			Type:     "function",
			Index:    index,
			Function: types.StreamFunction{Name: name},
		}},
	}
	o.writeChunk(chunk)
}

func (o *OpenAIOutput) OnToolCallArgs(args string, index int) {
	chunk := o.shallowCopy()
	chunk.Choices[0].Delta = types.StreamDelta{
		ToolCalls: []types.StreamToolCall{{
			Index:    index,
			Function: types.StreamFunction{Arguments: args},
		}},
	}
	o.writeChunk(chunk)
}

func (o *OpenAIOutput) OnToolCallEnd(index int) {
}

func (o *OpenAIOutput) shallowCopy() types.StreamChunk {
	cp := o.base
	cp.Choices = []types.StreamChoice{{Index: 0}}
	if len(o.base.Choices) > 0 {
		cp.Choices[0].Index = o.base.Choices[0].Index
		cp.Choices[0].FinishReason = o.base.Choices[0].FinishReason
	}
	return cp
}

func (o *OpenAIOutput) writeChunk(chunk types.StreamChunk) {
	b, err := json.Marshal(chunk)
	if err != nil {
		return
	}
	o.writer.Write([]byte("data: "))
	o.writer.Write(b)
	o.writer.Write([]byte("\n\n"))
}
