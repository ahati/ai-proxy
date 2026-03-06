package downstream

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"ai-proxy/logging"
	"github.com/tmaxmax/go-sse"
)

const (
	oaTokSectionBegin = "<|tool_calls_section_begin|>"
	oaTokCallBegin    = "<|tool_call_begin|>"
	oaTokArgBegin     = "<|tool_call_argument_begin|>"
	oaTokCallEnd      = "<|tool_call_end|>"
	oaTokSectionEnd   = "<|tool_calls_section_end|>"
)

type oaState int

const (
	oaStateIdle oaState = iota
	oaStateInSection
	oaStateReadingID
	oaStateReadingArgs
	oaStateTrailing
)

type OpenAIToAnthropicTransformer struct {
	output           io.Writer
	state            oaState
	buf              string
	toolIndex        int
	blockIndex       int
	messageID        string
	model            string
	messageSent      bool
	inThinking       bool
	inText           bool
	inRole           bool
	thinkingIndex    int
	textIndex        int
	roleIndex        int
	needThinkingStop bool
	needTextStop     bool
	needRoleStop     bool
	processAsText    bool
	hasContent       bool
	stopReason       string
	usage            *OpenAIUsage
	messageDeltaSent bool
	messageStopSent  bool
	currentToolIndex int
	currentToolID    string
	inToolUse        bool
}

func NewOpenAIToAnthropicTransformer(output io.Writer) *OpenAIToAnthropicTransformer {
	return &OpenAIToAnthropicTransformer{
		output: output,
		state:  oaStateIdle,
	}
}

func (t *OpenAIToAnthropicTransformer) Transform(event *sse.Event) {
	if event.Data == "" {
		return
	}
	if event.Data == "[DONE]" {
		return
	}

	var chunk OAChunk
	if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
		t.writeSSE([]byte("event: error\ndata: {\"type\": \"error\", \"error\": {\"type\": \"invalid_request_error\", \"message\": \"Failed to parse event\"}}\n\n"))
		return
	}

	if !t.messageSent {
		t.messageID = chunk.ID
		t.model = chunk.Model
		t.sendMessageStart()
	}

	if chunk.Usage != nil {
		t.usage = &OpenAIUsage{
			PromptTokens:     chunk.Usage.PromptTokens,
			CompletionTokens: chunk.Usage.CompletionTokens,
			TotalTokens:      chunk.Usage.TotalTokens,
		}
	}

	if len(chunk.Choices) == 0 {
		if chunk.Usage != nil {
			t.sendMessageDelta()
		}
		return
	}

	delta := chunk.Choices[0].Delta
	if chunk.Choices[0].FinishReason != nil {
		t.stopReason = *chunk.Choices[0].FinishReason
	}
	if delta.FinishReason != nil {
		t.stopReason = *delta.FinishReason
	}

	if delta.Role != "" && !t.inRole {
		t.inRole = true
		t.needRoleStop = true
	}

	reasoning := delta.Reasoning
	if reasoning == "" {
		reasoning = delta.ReasoningContent
	}

	if reasoning != "" {
		t.processAsText = false
		idx := t.thinkingIndex
		if !t.inThinking {
			t.inThinking = true
			t.needThinkingStop = true
			idx = t.blockIndex
			t.thinkingIndex = idx
			t.blockIndex++
			t.sendThinkingBlockStart(idx)
		}
		chunks, err := t.processThinking(reasoning, idx)
		if err != nil {
			logging.ErrorMsg("Process thinking error: %v", err)
			return
		}
		for _, c := range chunks {
			t.writeSSE(c)
		}
		return
	}

	if delta.Content != "" {
		t.processAsText = true
		idx := t.textIndex
		if !t.inText {
			t.inText = true
			t.needTextStop = true
			idx = t.blockIndex
			t.textIndex = idx
			t.blockIndex++
			t.sendTextBlockStart(idx)
		}
		chunks, err := t.processText(delta.Content, idx)
		if err != nil {
			logging.ErrorMsg("Process text error: %v", err)
			return
		}
		for _, c := range chunks {
			t.writeSSE(c)
		}
		return
	}

	if len(delta.ToolCalls) > 0 {
		for _, tc := range delta.ToolCalls {
			if tc.Function.Name != "" {
				if t.inToolUse {
					t.closeToolUse()
				}
				t.sendToolUseBlock(tc)
			} else if tc.Function.Arguments != "" {
				t.sendToolUseBlock(tc)
			}
		}
		return
	}

	if t.stopReason == "tool_calls" && t.inToolUse {
		t.closeToolUse()
	}
}

func (t *OpenAIToAnthropicTransformer) sendMessageStart() {
	t.messageSent = true
	event := AnthropicEvent{
		Type: "message_start",
		Message: &AnthropicMessage{
			ID:      t.messageID,
			Type:    "message",
			Role:    "assistant",
			Content: []AnthropicContentBlock{},
			Model:   t.model,
			Usage: &AnthropicUsage{
				InputTokens:  0,
				OutputTokens: 0,
			},
		},
	}
	t.writeEvent(&event)
}

func (t *OpenAIToAnthropicTransformer) sendRoleBlockStart(role string) {
	event := AnthropicEvent{
		Type:  "content_block_start",
		Index: intPtr(t.roleIndex),
		ContentBlock: mustMarshal(AnthropicContentBlock{
			Type: "text",
			Text: role,
		}),
	}
	t.writeEvent(&event)
}

func (t *OpenAIToAnthropicTransformer) sendThinkingBlockStart(index int) {
	event := AnthropicEvent{
		Type:  "content_block_start",
		Index: intPtr(index),
		ContentBlock: mustMarshal(AnthropicContentBlock{
			Type:     "thinking",
			Thinking: "",
		}),
	}
	t.writeEvent(&event)
}

func (t *OpenAIToAnthropicTransformer) sendTextBlockStart(index int) {
	event := AnthropicEvent{
		Type:  "content_block_start",
		Index: intPtr(index),
		ContentBlock: mustMarshal(AnthropicContentBlock{
			Type: "text",
			Text: "",
		}),
	}
	t.writeEvent(&event)
}

func (t *OpenAIToAnthropicTransformer) sendToolUseBlock(tc OpenAIToolCall) {
	if tc.Function.Name != "" {
		t.inToolUse = true
		t.currentToolIndex = tc.Index
		t.currentToolID = tc.ID
		name := t.parseFunctionName(tc.Function.Name)
		if name == "" {
			name = tc.Function.Name
		}
		id := tc.ID
		if id == "" {
			id = fmt.Sprintf("toolu_%d_%d", t.toolIndex, time.Now().UnixMilli())
		}
		logging.InfoMsg("[OpenAIToAnthropicTransformer] chat_id=%s tool_call_id=%s function=%s", t.messageID, id, name)
		event := AnthropicEvent{
			Type:  "content_block_start",
			Index: intPtr(t.blockIndex),
			ContentBlock: mustMarshal(AnthropicContentBlock{
				Type:  "tool_use",
				ID:    id,
				Name:  name,
				Input: json.RawMessage("{}"),
			}),
		}
		t.writeEvent(&event)
		t.toolIndex++
		t.blockIndex++
	}

	if tc.Function.Arguments != "" {
		deltaEvent := AnthropicEvent{
			Type:  "content_block_delta",
			Index: intPtr(t.blockIndex - 1),
			Delta: mustMarshal(InputJSONDelta{
				Type:        "input_json_delta",
				PartialJSON: tc.Function.Arguments,
			}),
		}
		t.writeEvent(&deltaEvent)
	}
}

func (t *OpenAIToAnthropicTransformer) closeToolUse() {
	if !t.inToolUse {
		return
	}
	t.inToolUse = false
	event := AnthropicEvent{
		Type:  "content_block_stop",
		Index: intPtr(t.blockIndex - 1),
	}
	t.writeEvent(&event)
}

func (t *OpenAIToAnthropicTransformer) sendMessageDelta() {
	if t.messageDeltaSent {
		return
	}
	t.messageDeltaSent = true
	stopReason := t.mapStopReason(t.stopReason)
	event := AnthropicEvent{
		Type: "message_delta",
		Delta: mustMarshal(struct {
			StopReason   string  `json:"stop_reason"`
			StopSequence *string `json:"stop_sequence"`
		}{
			StopReason:   stopReason,
			StopSequence: nil,
		}),
	}
	if t.usage != nil {
		event.MessageUsage = &AnthropicUsage{
			InputTokens:  t.usage.PromptTokens,
			OutputTokens: t.usage.CompletionTokens,
		}
	}
	t.writeEvent(&event)
}

func (t *OpenAIToAnthropicTransformer) sendMessageStop() {
	if t.messageStopSent {
		return
	}
	t.messageStopSent = true
	event := AnthropicEvent{
		Type: "message_stop",
	}
	t.writeEvent(&event)
}

func (t *OpenAIToAnthropicTransformer) mapStopReason(reason string) string {
	switch reason {
	case "tool_calls":
		return "tool_use"
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "content_filter":
		return "content_filter"
	default:
		if reason != "" {
			return reason
		}
		return "end_turn"
	}
}

func (t *OpenAIToAnthropicTransformer) processThinking(text string, index int) ([][]byte, error) {
	t.buf += text
	var out [][]byte

	for {
		switch t.state {
		case oaStateIdle:
			idx := strings.Index(t.buf, oaTokSectionBegin)
			if idx < 0 {
				if t.buf != "" {
					out = append(out, t.makeThinkingDelta(index, t.buf))
					t.buf = ""
				}
				return out, nil
			}
			if idx > 0 {
				out = append(out, t.makeThinkingDelta(index, t.buf[:idx]))
			}
			t.buf = t.buf[idx+len(oaTokSectionBegin):]
			t.state = oaStateInSection
			if t.needThinkingStop {
				out = append(out, t.makeThinkingBlockStop(t.thinkingIndex))
				t.needThinkingStop = false
				t.blockIndex++
			}

		case oaStateInSection:
			idx := strings.Index(t.buf, oaTokCallBegin)
			endIdx := strings.Index(t.buf, oaTokSectionEnd)

			if endIdx >= 0 && (idx < 0 || endIdx < idx) {
				t.buf = t.buf[endIdx+len(oaTokSectionEnd):]
				t.state = oaStateTrailing
				if t.buf != "" {
					t.thinkingIndex = t.blockIndex
					t.blockIndex++
					out = append(out, t.makeThinkingBlockStart(t.thinkingIndex))
					out = append(out, t.makeThinkingDelta(t.thinkingIndex, t.buf))
					t.needThinkingStop = true
					t.buf = ""
				}
				logging.InfoMsg("[OpenAIToAnthropicTransformer] chat_id=%s Tool calls section ended (thinking context)", t.messageID)
				return out, nil
			}
			if idx < 0 {
				return out, nil
			}
			logging.InfoMsg("[OpenAIToAnthropicTransformer] chat_id=%s Tool call section begin detected (thinking context)", t.messageID)
			t.buf = t.buf[idx+len(oaTokCallBegin):]
			t.state = oaStateReadingID

		case oaStateReadingID:
			argIdx := strings.Index(t.buf, oaTokArgBegin)
			if argIdx < 0 {
				return out, nil
			}
			rawID := strings.TrimSpace(t.buf[:argIdx])
			id := t.parseToolCallID(rawID, t.toolIndex)
			name := t.parseFunctionName(rawID)
			logging.InfoMsg("[OpenAIToAnthropicTransformer] chat_id=%s tool_call_id=%s function=%s (thinking context)", t.messageID, id, name)
			t.buf = t.buf[argIdx+len(oaTokArgBegin):]
			t.state = oaStateReadingArgs
			out = append(out, t.makeToolUseBlockStart(id, name))

		case oaStateReadingArgs:
			endIdx := strings.Index(t.buf, oaTokCallEnd)
			if endIdx < 0 {
				if t.buf != "" {
					out = append(out, t.makeInputJSONDelta(t.buf))
					t.buf = ""
				}
				return out, nil
			}
			args := t.buf[:endIdx]
			if args != "" {
				out = append(out, t.makeInputJSONDelta(args))
			}
			out = append(out, t.makeContentBlockStop(t.blockIndex))
			t.buf = t.buf[endIdx+len(oaTokCallEnd):]
			t.toolIndex++
			t.blockIndex++
			t.state = oaStateInSection

		case oaStateTrailing:
			idx := strings.Index(t.buf, oaTokSectionBegin)
			if idx >= 0 {
				if idx > 0 {
					out = append(out, t.makeThinkingDelta(index, t.buf[:idx]))
				}
				t.buf = t.buf[idx+len(oaTokSectionBegin):]
				t.state = oaStateInSection
				continue
			}
			if t.buf != "" {
				out = append(out, t.makeThinkingDelta(index, t.buf))
			}
			return out, nil
		}
	}
}

func (t *OpenAIToAnthropicTransformer) processText(text string, index int) ([][]byte, error) {
	t.buf += text
	var out [][]byte

	for {
		switch t.state {
		case oaStateIdle:
			idx := strings.Index(t.buf, oaTokSectionBegin)
			if idx < 0 {
				if t.buf != "" {
					out = append(out, t.makeTextDelta(index, t.buf))
					t.buf = ""
				}
				return out, nil
			}
			if idx > 0 {
				out = append(out, t.makeTextDelta(index, t.buf[:idx]))
			}
			t.buf = t.buf[idx+len(oaTokSectionBegin):]
			t.state = oaStateInSection
			if t.needTextStop {
				out = append(out, t.makeTextBlockStop(t.textIndex))
				t.needTextStop = false
				t.blockIndex++
			}

		case oaStateInSection:
			idx := strings.Index(t.buf, oaTokCallBegin)
			endIdx := strings.Index(t.buf, oaTokSectionEnd)

			if endIdx >= 0 && (idx < 0 || endIdx < idx) {
				t.buf = t.buf[endIdx+len(oaTokSectionEnd):]
				t.state = oaStateTrailing
				if t.buf != "" {
					t.textIndex = t.blockIndex
					t.blockIndex++
					out = append(out, t.makeTextBlockStart(t.textIndex))
					out = append(out, t.makeTextDelta(t.textIndex, t.buf))
					t.needTextStop = true
					t.buf = ""
				}
				logging.InfoMsg("[OpenAIToAnthropicTransformer] chat_id=%s Tool calls section ended (text context)", t.messageID)
				return out, nil
			}
			if idx < 0 {
				return out, nil
			}
			logging.InfoMsg("[OpenAIToAnthropicTransformer] chat_id=%s Tool call section begin detected (text context)", t.messageID)
			t.buf = t.buf[idx+len(oaTokCallBegin):]
			t.state = oaStateReadingID

		case oaStateReadingID:
			argIdx := strings.Index(t.buf, oaTokArgBegin)
			if argIdx < 0 {
				return out, nil
			}
			rawID := strings.TrimSpace(t.buf[:argIdx])
			id := t.parseToolCallID(rawID, t.toolIndex)
			name := t.parseFunctionName(rawID)
			logging.InfoMsg("[OpenAIToAnthropicTransformer] chat_id=%s tool_call_id=%s function=%s (text context)", t.messageID, id, name)
			t.buf = t.buf[argIdx+len(oaTokArgBegin):]
			t.state = oaStateReadingArgs
			out = append(out, t.makeToolUseBlockStart(id, name))

		case oaStateReadingArgs:
			endIdx := strings.Index(t.buf, oaTokCallEnd)
			if endIdx < 0 {
				if t.buf != "" {
					out = append(out, t.makeInputJSONDelta(t.buf))
					t.buf = ""
				}
				return out, nil
			}
			args := t.buf[:endIdx]
			if args != "" {
				out = append(out, t.makeInputJSONDelta(args))
			}
			out = append(out, t.makeContentBlockStop(t.blockIndex))
			t.buf = t.buf[endIdx+len(oaTokCallEnd):]
			t.toolIndex++
			t.blockIndex++
			t.state = oaStateInSection

		case oaStateTrailing:
			idx := strings.Index(t.buf, oaTokSectionBegin)
			if idx >= 0 {
				if idx > 0 {
					out = append(out, t.makeTextDelta(index, t.buf[:idx]))
				}
				t.buf = t.buf[idx+len(oaTokSectionBegin):]
				t.state = oaStateInSection
				continue
			}
			if t.buf != "" {
				out = append(out, t.makeTextDelta(index, t.buf))
			}
			return out, nil
		}
	}
}

func (t *OpenAIToAnthropicTransformer) makeThinkingDelta(index int, thinking string) []byte {
	delta := ThinkingDelta{
		Type:     "thinking_delta",
		Thinking: thinking,
	}
	deltaJSON, _ := json.Marshal(delta)
	event := AnthropicEvent{
		Type:  "content_block_delta",
		Index: intPtr(index),
		Delta: deltaJSON,
	}
	return t.serializeEvent(event)
}

func (t *OpenAIToAnthropicTransformer) makeThinkingBlockStart(index int) []byte {
	block := AnthropicContentBlock{
		Type:     "thinking",
		Thinking: "",
	}
	blockJSON, _ := json.Marshal(block)
	event := AnthropicEvent{
		Type:         "content_block_start",
		Index:        intPtr(index),
		ContentBlock: blockJSON,
	}
	return t.serializeEvent(event)
}

func (t *OpenAIToAnthropicTransformer) makeThinkingBlockStop(index int) []byte {
	event := AnthropicEvent{
		Type:  "content_block_stop",
		Index: intPtr(index),
	}
	return t.serializeEvent(event)
}

func (t *OpenAIToAnthropicTransformer) makeTextDelta(index int, text string) []byte {
	delta := TextDelta{
		Type: "text_delta",
		Text: text,
	}
	deltaJSON, _ := json.Marshal(delta)
	event := AnthropicEvent{
		Type:  "content_block_delta",
		Index: intPtr(index),
		Delta: deltaJSON,
	}
	return t.serializeEvent(event)
}

func (t *OpenAIToAnthropicTransformer) makeTextBlockStart(index int) []byte {
	block := AnthropicContentBlock{
		Type: "text",
		Text: "",
	}
	blockJSON, _ := json.Marshal(block)
	event := AnthropicEvent{
		Type:         "content_block_start",
		Index:        intPtr(index),
		ContentBlock: blockJSON,
	}
	return t.serializeEvent(event)
}

func (t *OpenAIToAnthropicTransformer) makeTextBlockStop(index int) []byte {
	event := AnthropicEvent{
		Type:  "content_block_stop",
		Index: intPtr(index),
	}
	return t.serializeEvent(event)
}

func (t *OpenAIToAnthropicTransformer) makeToolUseBlockStart(id, name string) []byte {
	toolBlock := AnthropicContentBlock{
		Type:  "tool_use",
		ID:    id,
		Name:  name,
		Input: json.RawMessage("{}"),
	}
	blockJSON, _ := json.Marshal(toolBlock)
	event := AnthropicEvent{
		Type:         "content_block_start",
		Index:        intPtr(t.blockIndex),
		ContentBlock: blockJSON,
	}
	return t.serializeEvent(event)
}

func (t *OpenAIToAnthropicTransformer) makeInputJSONDelta(partialJSON string) []byte {
	delta := InputJSONDelta{
		Type:        "input_json_delta",
		PartialJSON: partialJSON,
	}
	deltaJSON, _ := json.Marshal(delta)
	event := AnthropicEvent{
		Type:  "content_block_delta",
		Index: intPtr(t.blockIndex),
		Delta: deltaJSON,
	}
	return t.serializeEvent(event)
}

func (t *OpenAIToAnthropicTransformer) makeContentBlockStop(index int) []byte {
	event := AnthropicEvent{
		Type:  "content_block_stop",
		Index: intPtr(index),
	}
	return t.serializeEvent(event)
}

func (t *OpenAIToAnthropicTransformer) parseToolCallID(raw string, index int) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "call_") || strings.HasPrefix(raw, "toolu_") {
		return raw
	}
	return fmt.Sprintf("toolu_%d_%d", index, time.Now().UnixMilli())
}

func (t *OpenAIToAnthropicTransformer) parseFunctionName(raw string) string {
	raw = strings.TrimSpace(raw)
	if i := strings.LastIndex(raw, ":"); i >= 0 {
		raw = raw[:i]
	}
	if i := strings.LastIndex(raw, "."); i >= 0 {
		raw = raw[i+1:]
	}
	return raw
}

func (t *OpenAIToAnthropicTransformer) flushFinalEvents() {
	if !t.messageSent {
		return
	}
	if t.inThinking && t.needThinkingStop {
		t.writeEvent(&AnthropicEvent{
			Type:  "content_block_stop",
			Index: intPtr(t.thinkingIndex),
		})
		t.inThinking = false
		t.needThinkingStop = false
	}
	if t.inText && t.needTextStop {
		t.writeEvent(&AnthropicEvent{
			Type:  "content_block_stop",
			Index: intPtr(t.textIndex),
		})
		t.inText = false
		t.needTextStop = false
	}
	if t.inToolUse {
		t.closeToolUse()
	}
	t.sendMessageDelta()
	t.sendMessageStop()
}

func (t *OpenAIToAnthropicTransformer) writeEvent(event *AnthropicEvent) {
	data := t.serializeEvent(*event)
	if len(data) > 0 {
		t.writeSSE(data)
	}
}

func (t *OpenAIToAnthropicTransformer) serializeEvent(event AnthropicEvent) []byte {
	data, err := json.Marshal(event)
	if err != nil {
		logging.ErrorMsg("Failed to serialize event: %v", err)
		return nil
	}
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event.Type, string(data)))
}

func (t *OpenAIToAnthropicTransformer) writeSSE(data []byte) {
	if len(data) == 0 {
		return
	}
	if _, err := t.output.Write(data); err != nil {
		logging.ErrorMsg("Failed to write to output: %v", err)
	}
}

func (t *OpenAIToAnthropicTransformer) Close() {}

func (t *OpenAIToAnthropicTransformer) Flush() {
	t.flushFinalEvents()
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
