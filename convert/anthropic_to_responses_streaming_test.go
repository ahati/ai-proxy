package convert

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"ai-proxy/transform/toolcall"
	"ai-proxy/types"

	"github.com/tmaxmax/go-sse"
)

// TestResponsesFormatter_FormatResponseCreated tests response.created event formatting.
func TestResponsesFormatter_FormatResponseCreated(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")

	result := formatter.FormatResponseCreated(1)
	resultStr := string(result)

	if !strings.HasPrefix(resultStr, "data: ") {
		t.Error("Result should start with 'data: '")
	}

	if !strings.Contains(resultStr, `"type":"response.created"`) {
		t.Error("Result should contain response.created type")
	}

	if !strings.Contains(resultStr, `"id":"resp_123"`) {
		t.Error("Result should contain response ID")
	}

	if !strings.Contains(resultStr, `"model":"gpt-4o"`) {
		t.Error("Result should contain model")
	}

	if !strings.Contains(resultStr, `"sequence_number":1`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestResponsesFormatter_FormatContentPartAdded tests content part added event.
func TestResponsesFormatter_FormatContentPartAdded(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")
	formatter.SetResponseID("resp_123")

	result := formatter.FormatContentPartAdded("msg_123", 0, "output_text", 1, 2)
	resultStr := string(result)

	if !strings.Contains(resultStr, `"type":"response.content_part.added"`) {
		t.Error("Result should contain response.content_part.added type")
	}

	if !strings.Contains(resultStr, `"content_index":0`) {
		t.Error("Result should contain content_index")
	}

	if !strings.Contains(resultStr, `"type":"output_text"`) {
		t.Error("Result should contain output_text type")
	}

	if !strings.Contains(resultStr, `"output_index":1`) {
		t.Error("Result should contain output_index")
	}

	if !strings.Contains(resultStr, `"sequence_number":2`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestResponsesFormatter_FormatOutputTextDelta tests text delta formatting.
func TestResponsesFormatter_FormatOutputTextDelta(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")
	formatter.SetResponseID("resp_123")

	result := formatter.FormatOutputTextDelta("msg_123", 0, "Hello world", 1, 2)
	resultStr := string(result)

	if !strings.Contains(resultStr, `"type":"response.output_text.delta"`) {
		t.Error("Result should contain response.output_text.delta type")
	}

	if !strings.Contains(resultStr, `"delta":"Hello world"`) {
		t.Error("Result should contain delta with text")
	}

	if !strings.Contains(resultStr, `"output_index":1`) {
		t.Error("Result should contain output_index")
	}

	if !strings.Contains(resultStr, `"sequence_number":2`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestResponsesFormatter_FormatFunctionCallItemAdded tests function call item added event.
func TestResponsesFormatter_FormatFunctionCallItemAdded(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")
	formatter.SetResponseID("resp_123")

	result := formatter.FormatFunctionCallItemAdded("toolu_abc", "get_weather", 1, 2)
	resultStr := string(result)

	if !strings.Contains(resultStr, `"type":"response.output_item.added"`) {
		t.Error("Result should contain response.output_item.added type")
	}

	if !strings.Contains(resultStr, `"type":"function_call"`) {
		t.Error("Result should contain function_call item type")
	}

	if !strings.Contains(resultStr, `"id":"toolu_abc"`) {
		t.Error("Result should contain call ID")
	}

	if !strings.Contains(resultStr, `"name":"get_weather"`) {
		t.Error("Result should contain function name")
	}

	if !strings.Contains(resultStr, `"output_index":1`) {
		t.Error("Result should contain output_index")
	}

	if !strings.Contains(resultStr, `"sequence_number":2`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestResponsesFormatter_FormatFunctionCallArgsDelta tests function args delta.
func TestResponsesFormatter_FormatFunctionCallArgsDelta(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")
	formatter.SetResponseID("resp_123")

	result := formatter.FormatFunctionCallArgsDelta("toolu_abc", "toolu_abc", `{"locat`, 1, 2)
	resultStr := string(result)

	if !strings.Contains(resultStr, `"type":"response.function_call_arguments.delta"`) {
		t.Error("Result should contain response.function_call_arguments.delta type")
	}

	if !strings.Contains(resultStr, `"call_id":"toolu_abc"`) {
		t.Error("Result should contain call_id")
	}

	// The delta is JSON-escaped in the output
	if !strings.Contains(resultStr, `"delta":"{\"locat"`) {
		t.Errorf("Result should contain escaped delta - got: %s", resultStr)
	}

	if !strings.Contains(resultStr, `"output_index":1`) {
		t.Error("Result should contain output_index")
	}

	if !strings.Contains(resultStr, `"sequence_number":2`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestResponsesFormatter_FormatContentPartDone tests content part done event.
func TestResponsesFormatter_FormatContentPartDone(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")
	formatter.SetResponseID("resp_123")

	// Test with output_text type
	result := formatter.FormatContentPartDone("msg_123", 0, "output_text", "Hello world", 1, 2)
	resultStr := string(result)

	if !strings.Contains(resultStr, `"type":"response.content_part.done"`) {
		t.Error("Result should contain response.content_part.done type")
	}

	if !strings.Contains(resultStr, `"text":"Hello world"`) {
		t.Error("Result should contain text content")
	}

	if !strings.Contains(resultStr, `"output_index":1`) {
		t.Error("Result should contain output_index")
	}

	if !strings.Contains(resultStr, `"sequence_number":2`) {
		t.Error("Result should contain sequence_number")
	}

	// Test with other type (no content)
	result2 := formatter.FormatContentPartDone("msg_123", 1, "function_call", "", 1, 3)
	resultStr2 := string(result2)

	if !strings.Contains(resultStr2, `"type":"function_call"`) {
		t.Error("Result should contain function_call type")
	}
}

// TestResponsesFormatter_FormatOutputItemDone tests output item done event.
func TestResponsesFormatter_FormatOutputItemDone(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")
	formatter.SetResponseID("resp_123")

	item := map[string]interface{}{
		"type":   "message",
		"id":     "msg_456",
		"status": "completed",
		"role":   "assistant",
		"content": []map[string]interface{}{{
			"type": "output_text",
			"text": "Hello",
		}},
	}

	result := formatter.FormatOutputItemDone("msg_456", item, 0, 1)
	resultStr := string(result)

	if !strings.Contains(resultStr, `"type":"response.output_item.done"`) {
		t.Error("Result should contain response.output_item.done type")
	}

	if !strings.Contains(resultStr, `"id":"msg_456"`) {
		t.Error("Result should contain item ID")
	}

	if !strings.Contains(resultStr, `"status":"completed"`) {
		t.Error("Result should contain completed status")
	}

	if !strings.Contains(resultStr, `"sequence_number":1`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestResponsesFormatter_FormatResponseCompleted tests response completed event.
func TestResponsesFormatter_FormatResponseCompleted(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")
	formatter.SetResponseID("resp_123")
	formatter.SetModel("gpt-4o")

	outputItems := []map[string]interface{}{{
		"type":   "message",
		"id":     "msg_456",
		"status": "completed",
		"role":   "assistant",
		"content": []map[string]interface{}{{
			"type": "output_text",
			"text": "Hello",
		}},
	}}

	result := formatter.FormatResponseCompleted(outputItems, nil, 1, "")
	resultStr := string(result)

	if !strings.Contains(resultStr, `"type":"response.completed"`) {
		t.Error("Result should contain response.completed type")
	}

	if !strings.Contains(resultStr, `"status":"completed"`) {
		t.Error("Result should contain completed status")
	}

	if !strings.Contains(resultStr, `"sequence_number":1`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestResponsesFormatter_FormatReasoningItemAdded tests reasoning item added event.
func TestResponsesFormatter_FormatReasoningItemAdded(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")

	result := formatter.FormatReasoningItemAdded("rs_abc", 0, 1)
	resultStr := string(result)

	if !strings.Contains(resultStr, `"type":"response.output_item.added"`) {
		t.Error("Result should contain response.output_item.added type")
	}

	if !strings.Contains(resultStr, `"type":"reasoning"`) {
		t.Error("Result should contain reasoning item type")
	}

	if !strings.Contains(resultStr, `"id":"rs_abc"`) {
		t.Error("Result should contain reasoning ID")
	}

	if !strings.Contains(resultStr, `"output_index":0`) {
		t.Error("Result should contain output_index")
	}

	if !strings.Contains(resultStr, `"sequence_number":1`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestResponsesFormatter_FormatReasoningSummaryPartAdded tests reasoning summary part added event.
func TestResponsesFormatter_FormatReasoningSummaryPartAdded(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")

	result := formatter.FormatReasoningSummaryPartAdded("rs_abc", 0, 0, 1)
	resultStr := string(result)

	if !strings.Contains(resultStr, `"type":"response.reasoning_summary_part.added"`) {
		t.Error("Result should contain response.reasoning_summary_part.added type")
	}

	if !strings.Contains(resultStr, `"item_id":"rs_abc"`) {
		t.Error("Result should contain item_id")
	}

	if !strings.Contains(resultStr, `"output_index":0`) {
		t.Error("Result should contain output_index")
	}

	if !strings.Contains(resultStr, `"summary_index":0`) {
		t.Error("Result should contain summary_index")
	}

	if !strings.Contains(resultStr, `"sequence_number":1`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestResponsesFormatter_FormatReasoningSummaryDelta tests reasoning summary delta.
func TestResponsesFormatter_FormatReasoningSummaryDelta(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")

	result := formatter.FormatReasoningSummaryDelta("rs_abc", "Analyzing...", 0, 0, 1)
	resultStr := string(result)

	if !strings.Contains(resultStr, `"type":"response.reasoning_summary_text.delta"`) {
		t.Error("Result should contain response.reasoning_summary_text.delta type")
	}

	if !strings.Contains(resultStr, `"delta":"Analyzing..."`) {
		t.Error("Result should contain delta")
	}

	if !strings.Contains(resultStr, `"output_index":0`) {
		t.Error("Result should contain output_index")
	}

	if !strings.Contains(resultStr, `"summary_index":0`) {
		t.Error("Result should contain summary_index")
	}

	if !strings.Contains(resultStr, `"sequence_number":1`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestResponsesFormatter_FormatReasoningSummaryTextDone tests reasoning summary text done event.
func TestResponsesFormatter_FormatReasoningSummaryTextDone(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")

	result := formatter.FormatReasoningSummaryTextDone("rs_abc", "Full reasoning text", 0, 0, 1)
	resultStr := string(result)

	if !strings.Contains(resultStr, `"type":"response.reasoning_summary_text.done"`) {
		t.Error("Result should contain response.reasoning_summary_text.done type")
	}

	if !strings.Contains(resultStr, `"text":"Full reasoning text"`) {
		t.Error("Result should contain text")
	}

	if !strings.Contains(resultStr, `"output_index":0`) {
		t.Error("Result should contain output_index")
	}

	if !strings.Contains(resultStr, `"summary_index":0`) {
		t.Error("Result should contain summary_index")
	}

	if !strings.Contains(resultStr, `"sequence_number":1`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestResponsesFormatter_FormatReasoningSummaryPartDone tests reasoning summary part done event.
func TestResponsesFormatter_FormatReasoningSummaryPartDone(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")

	result := formatter.FormatReasoningSummaryPartDone("rs_abc", "Full reasoning text", 0, 0, 1)
	resultStr := string(result)

	if !strings.Contains(resultStr, `"type":"response.reasoning_summary_part.done"`) {
		t.Error("Result should contain response.reasoning_summary_part.done type")
	}

	if !strings.Contains(resultStr, `"type":"summary_text"`) {
		t.Error("Result should contain summary_text type")
	}

	if !strings.Contains(resultStr, `"text":"Full reasoning text"`) {
		t.Error("Result should contain summary text")
	}

	if !strings.Contains(resultStr, `"output_index":0`) {
		t.Error("Result should contain output_index")
	}

	if !strings.Contains(resultStr, `"summary_index":0`) {
		t.Error("Result should contain summary_index")
	}

	if !strings.Contains(resultStr, `"sequence_number":1`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestResponsesFormatter_FormatReasoningItemDone tests reasoning item done event.
func TestResponsesFormatter_FormatReasoningItemDone(t *testing.T) {
	formatter := NewResponsesFormatter("resp_123", "gpt-4o")

	result := formatter.FormatReasoningItemDone("rs_abc", "Full reasoning text", 0, 1)
	resultStr := string(result)

	if !strings.Contains(resultStr, `"type":"response.output_item.done"`) {
		t.Error("Result should contain response.output_item.done type")
	}

	if !strings.Contains(resultStr, `"type":"summary_text"`) {
		t.Error("Result should contain summary_text type")
	}

	if !strings.Contains(resultStr, `"text":"Full reasoning text"`) {
		t.Error("Result should contain summary text")
	}

	if !strings.Contains(resultStr, `"output_index":0`) {
		t.Error("Result should contain output_index")
	}

	if !strings.Contains(resultStr, `"sequence_number":1`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestNewResponsesTransformer tests transformer creation.
func TestNewResponsesTransformer(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	if transformer == nil {
		t.Fatal("NewResponsesTransformer returned nil")
	}

	if transformer.sseWriter == nil {
		t.Error("Transformer sseWriter should not be nil")
	}

	if transformer.formatter == nil {
		t.Error("Transformer formatter should not be nil")
	}
}

// TestResponsesTransformer_Transform_EmptyData tests transform with empty data.
func TestResponsesTransformer_Transform_EmptyData(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	event := &sse.Event{Data: ""}
	err := transformer.Transform(event)

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	if buf.Len() != 0 {
		t.Error("Buffer should be empty for empty event data")
	}
}

// TestResponsesTransformer_Transform_Done tests transform with [DONE].
func TestResponsesTransformer_Transform_Done(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	event := &sse.Event{Data: "[DONE]"}
	err := transformer.Transform(event)

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, "data: [DONE]") {
		t.Error("Result should contain data: [DONE]")
	}
}

// TestResponsesTransformer_Transform_InvalidJSON tests transform with invalid JSON.
func TestResponsesTransformer_Transform_InvalidJSON(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	event := &sse.Event{Data: "not valid json"}
	err := transformer.Transform(event)

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, "data: not valid json") {
		t.Error("Result should pass through invalid JSON")
	}
}

// TestResponsesTransformer_HandleMessageStart tests message_start handling.
func TestResponsesTransformer_HandleMessageStart(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	err := transformer.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	anthropicEvent := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc123",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-3-opus",
		},
	}

	data, _ := json.Marshal(anthropicEvent)
	event := &sse.Event{Data: string(data)}
	err = transformer.Transform(event)

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, `"type":"response.created"`) {
		t.Error("Result should contain response.created event")
	}

	// response.created and response.in_progress are emitted by Initialize()
	// with a generated response ID and empty model (model is set later by message_start)
}

// TestResponsesTransformer_HandleContentBlockStart_Text tests text block start.
func TestResponsesTransformer_HandleContentBlockStart_Text(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// First send message_start
	msgStart := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(msgStart)
	transformer.Transform(&sse.Event{Data: string(data)})
	buf.Reset()

	// Then send content_block_start for text
	contentBlock := types.ContentBlock{Type: "text"}
	blockData, _ := json.Marshal(contentBlock)
	blockStart := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(0),
		ContentBlock: blockData,
	}
	data, _ = json.Marshal(blockStart)
	err := transformer.Transform(&sse.Event{Data: string(data)})

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, `"type":"response.content_part.added"`) {
		t.Error("Result should contain content_part.added event")
	}

	if !strings.Contains(result, `"type":"output_text"`) {
		t.Error("Result should contain output_text type")
	}
}

// TestResponsesTransformer_HandleContentBlockStart_Thinking tests thinking block start.
func TestResponsesTransformer_HandleContentBlockStart_Thinking(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// First send message_start
	msgStart := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(msgStart)
	transformer.Transform(&sse.Event{Data: string(data)})
	buf.Reset()

	// Then send content_block_start for thinking
	contentBlock := types.ContentBlock{Type: "thinking"}
	blockData, _ := json.Marshal(contentBlock)
	blockStart := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(0),
		ContentBlock: blockData,
	}
	data, _ = json.Marshal(blockStart)
	err := transformer.Transform(&sse.Event{Data: string(data)})

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	result := buf.String()
	// Should contain response.output_item.added for reasoning
	if !strings.Contains(result, `"type":"response.output_item.added"`) {
		t.Error("Result should contain response.output_item.added type")
	}

	if !strings.Contains(result, `"type":"reasoning"`) {
		t.Error("Result should contain reasoning item type")
	}

	if !strings.Contains(result, `"id":"rs_abc"`) {
		t.Error("Result should contain reasoning ID")
	}

	// Should also contain response.reasoning_summary_part.added
	if !strings.Contains(result, `"type":"response.reasoning_summary_part.added"`) {
		t.Error("Result should contain response.reasoning_summary_part.added type")
	}
}

// TestResponsesTransformer_HandleContentBlockStart_ToolUse tests tool_use block start.
func TestResponsesTransformer_HandleContentBlockStart_ToolUse(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// First send message_start
	msgStart := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(msgStart)
	transformer.Transform(&sse.Event{Data: string(data)})
	buf.Reset()

	// Then send content_block_start for tool_use
	contentBlock := types.ContentBlock{
		Type: "tool_use",
		ID:   "toolu_123",
		Name: "get_weather",
	}
	blockData, _ := json.Marshal(contentBlock)
	blockStart := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(0),
		ContentBlock: blockData,
	}
	data, _ = json.Marshal(blockStart)
	err := transformer.Transform(&sse.Event{Data: string(data)})

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, `"type":"function_call"`) {
		t.Error("Result should contain function_call type")
	}

	if !strings.Contains(result, `"id":"toolu_123"`) {
		t.Error("Result should contain tool ID")
	}

	if !strings.Contains(result, `"name":"get_weather"`) {
		t.Error("Result should contain function name")
	}
}

// TestResponsesTransformer_HandleContentBlockDelta_Text tests text delta.
func TestResponsesTransformer_HandleContentBlockDelta_Text(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// Setup
	msgStart := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(msgStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	contentBlock := types.ContentBlock{Type: "text"}
	blockData, _ := json.Marshal(contentBlock)
	blockStart := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(0),
		ContentBlock: blockData,
	}
	data, _ = json.Marshal(blockStart)
	transformer.Transform(&sse.Event{Data: string(data)})
	buf.Reset()

	// Send text delta
	delta := types.TextDelta{Type: "text_delta", Text: "Hello"}
	deltaData, _ := json.Marshal(delta)
	blockDelta := types.Event{
		Type:  "content_block_delta",
		Index: intPtr(0),
		Delta: deltaData,
	}
	data, _ = json.Marshal(blockDelta)
	err := transformer.Transform(&sse.Event{Data: string(data)})

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, `"type":"response.output_text.delta"`) {
		t.Error("Result should contain output_text.delta event")
	}

	if !strings.Contains(result, `"delta":"Hello"`) {
		t.Error("Result should contain delta text")
	}
}

// TestResponsesTransformer_HandleContentBlockDelta_Thinking tests thinking delta.
func TestResponsesTransformer_HandleContentBlockDelta_Thinking(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// Setup
	msgStart := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(msgStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	contentBlock := types.ContentBlock{Type: "thinking"}
	blockData, _ := json.Marshal(contentBlock)
	blockStart := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(0),
		ContentBlock: blockData,
	}
	data, _ = json.Marshal(blockStart)
	transformer.Transform(&sse.Event{Data: string(data)})
	buf.Reset()

	// Send thinking delta
	delta := types.ThinkingDelta{Type: "thinking_delta", Thinking: "Analyzing..."}
	deltaData, _ := json.Marshal(delta)
	blockDelta := types.Event{
		Type:  "content_block_delta",
		Index: intPtr(0),
		Delta: deltaData,
	}
	data, _ = json.Marshal(blockDelta)
	err := transformer.Transform(&sse.Event{Data: string(data)})

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, `"type":"response.reasoning_summary_text.delta"`) {
		t.Error("Result should contain response.reasoning_summary_text.delta type")
	}

	if !strings.Contains(result, `"delta":"Analyzing..."`) {
		t.Error("Result should contain thinking delta")
	}

	// Check for required fields
	if !strings.Contains(result, `"output_index":0`) {
		t.Error("Result should contain output_index")
	}

	if !strings.Contains(result, `"summary_index":0`) {
		t.Error("Result should contain summary_index")
	}

	if !strings.Contains(result, `"sequence_number"`) {
		t.Error("Result should contain sequence_number")
	}
}

// TestResponsesTransformer_HandleContentBlockDelta_ToolInput tests tool input delta.
func TestResponsesTransformer_HandleContentBlockDelta_ToolInput(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// Setup
	msgStart := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(msgStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	contentBlock := types.ContentBlock{
		Type: "tool_use",
		ID:   "toolu_123",
		Name: "get_weather",
	}
	blockData, _ := json.Marshal(contentBlock)
	blockStart := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(0),
		ContentBlock: blockData,
	}
	data, _ = json.Marshal(blockStart)
	transformer.Transform(&sse.Event{Data: string(data)})
	buf.Reset()

	// Send input_json delta
	delta := types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"loc`}
	deltaData, _ := json.Marshal(delta)
	blockDelta := types.Event{
		Type:  "content_block_delta",
		Index: intPtr(0),
		Delta: deltaData,
	}
	data, _ = json.Marshal(blockDelta)
	err := transformer.Transform(&sse.Event{Data: string(data)})

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, `"type":"response.function_call_arguments.delta"`) {
		t.Error("Result should contain function_call_arguments.delta event")
	}
}

// TestResponsesTransformer_HandleContentBlockStop tests block stop handling.
func TestResponsesTransformer_HandleContentBlockStop(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// Setup
	msgStart := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(msgStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	contentBlock := types.ContentBlock{Type: "text"}
	blockData, _ := json.Marshal(contentBlock)
	blockStart := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(0),
		ContentBlock: blockData,
	}
	data, _ = json.Marshal(blockStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	delta := types.TextDelta{Type: "text_delta", Text: "Hello world"}
	deltaData, _ := json.Marshal(delta)
	blockDelta := types.Event{
		Type:  "content_block_delta",
		Index: intPtr(0),
		Delta: deltaData,
	}
	data, _ = json.Marshal(blockDelta)
	transformer.Transform(&sse.Event{Data: string(data)})
	buf.Reset()

	// Send block stop
	blockStop := types.Event{
		Type:  "content_block_stop",
		Index: intPtr(0),
	}
	data, _ = json.Marshal(blockStop)
	err := transformer.Transform(&sse.Event{Data: string(data)})

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, `"type":"response.content_part.done"`) {
		t.Error("Result should contain content_part.done event")
	}

	if !strings.Contains(result, `"text":"Hello world"`) {
		t.Error("Result should contain accumulated text")
	}
}

// TestResponsesTransformer_HandleMessageStop tests message stop.
func TestResponsesTransformer_HandleMessageStop(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// Setup
	msgStart := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(msgStart)
	transformer.Transform(&sse.Event{Data: string(data)})
	buf.Reset()

	// Send message_stop
	msgStop := types.Event{Type: "message_stop"}
	data, _ = json.Marshal(msgStop)
	err := transformer.Transform(&sse.Event{Data: string(data)})

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, `"type":"response.completed"`) {
		t.Error("Result should contain response.completed event")
	}
}

// TestResponsesTransformer_HandleMessageStop_OnlyToolCalls tests message stop with only tool calls (no text).
// This verifies that assistant messages are properly emitted even when the model only returns tool calls.
func TestResponsesTransformer_HandleMessageStop_OnlyToolCalls(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// Setup: message_start
	msgStart := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(msgStart)
	transformer.Transform(&sse.Event{Data: string(data)})
	buf.Reset()

	// Send tool_use content block (no text)
	cbStart := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(0),
		ContentBlock: json.RawMessage(`{"type":"tool_use","id":"tool_123","name":"exec_command"}`),
	}
	data, _ = json.Marshal(cbStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	// Send tool input delta
	cbDelta := types.Event{
		Type:  "content_block_delta",
		Index: intPtr(0),
		Delta: json.RawMessage(`{"type":"input_json_delta","partial_json":"{\"cmd\":\"ls\"}"}`),
	}
	data, _ = json.Marshal(cbDelta)
	transformer.Transform(&sse.Event{Data: string(data)})

	// End content block
	cbStop := types.Event{
		Type:  "content_block_stop",
		Index: intPtr(0),
	}
	data, _ = json.Marshal(cbStop)
	transformer.Transform(&sse.Event{Data: string(data)})

	buf.Reset()

	// Send message_stop
	msgStop := types.Event{Type: "message_stop"}
	data, _ = json.Marshal(msgStop)
	err := transformer.Transform(&sse.Event{Data: string(data)})

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	result := buf.String()

	// Should contain the assistant message output_item.done
	if !strings.Contains(result, `"type":"response.output_item.done"`) {
		t.Error("Result should contain response.output_item.done event for assistant message")
	}

	// Should contain the function_call output_item.done
	if !strings.Contains(result, `"type":"function_call"`) {
		t.Error("Result should contain function_call in output")
	}

	// Should contain response.completed
	if !strings.Contains(result, `"type":"response.completed"`) {
		t.Error("Result should contain response.completed event")
	}

	// Verify the assistant message is included in response.completed output
	if !strings.Contains(result, `"role":"assistant"`) {
		t.Error("Result should contain assistant role in output")
	}
}

// TestResponsesTransformer_HandlePing tests ping handling.
func TestResponsesTransformer_HandlePing(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	ping := types.Event{Type: "ping"}
	data, _ := json.Marshal(ping)
	err := transformer.Transform(&sse.Event{Data: string(data)})

	if err != nil {
		t.Errorf("Transform returned error: %v", err)
	}

	// Ping should produce no output
	if buf.Len() != 0 {
		t.Error("Buffer should be empty for ping events")
	}
}

// TestResponsesTransformer_Flush tests flush operation.
func TestResponsesTransformer_Flush(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	err := transformer.Flush()
	if err != nil {
		t.Errorf("Flush returned error: %v", err)
	}
}

// TestResponsesTransformer_Close tests close operation.
func TestResponsesTransformer_Close(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	err := transformer.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

// TestResponsesTransformer_FullFlow tests a complete streaming flow.
func TestResponsesTransformer_FullFlow(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	err := transformer.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_abc123",
				Type:  "message",
				Role:  "assistant",
				Model: "claude-3-opus",
			},
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(0),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "Hello"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: " world"}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(0),
		},
		{
			Type: "message_stop",
		},
	}

	for i, e := range events {
		data, _ := json.Marshal(e)
		err := transformer.Transform(&sse.Event{Data: string(data)})
		if err != nil {
			t.Errorf("Transform event %d returned error: %v", i, err)
		}
	}

	result := buf.String()

	// Verify all expected events are present
	expectedEvents := []string{
		`"type":"response.created"`,
		`"type":"response.content_part.added"`,
		`"type":"response.output_text.delta"`,
		`"type":"response.content_part.done"`,
		`"type":"response.completed"`,
	}

	for _, expected := range expectedEvents {
		if !strings.Contains(result, expected) {
			t.Errorf("Result should contain %s", expected)
		}
	}
}

// TestResponsesTransformer_FullFlowWithTool tests complete flow with tool call.
func TestResponsesTransformer_FullFlowWithTool(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_abc",
				Model: "claude-3",
			},
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(0),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "get_weather"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"location":`}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `"San Francisco"}`}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(0),
		},
		{
			Type: "message_stop",
		},
	}

	for _, e := range events {
		data, _ := json.Marshal(e)
		transformer.Transform(&sse.Event{Data: string(data)})
	}

	result := buf.String()

	// Verify tool call events
	if !strings.Contains(result, `"type":"function_call"`) {
		t.Error("Result should contain function_call type")
	}

	if !strings.Contains(result, `"type":"response.function_call_arguments.delta"`) {
		t.Error("Result should contain function_call_arguments.delta")
	}
}

// BenchmarkResponsesTransformer_Transform benchmarks the transformer.
func BenchmarkResponsesTransformer_Transform(b *testing.B) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	event := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(event)
	sseEvent := &sse.Event{Data: string(data)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		transformer.Transform(sseEvent)
	}
}

// Helper function to marshal to RawMessage
func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return json.RawMessage(data)
}

// TestResponsesTransformer_MultipleToolCalls tests multiple parallel tool calls.
func TestResponsesTransformer_MultipleToolCalls(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_abc",
				Model: "claude-3",
			},
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(0),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_1", Name: "get_weather"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"city": "SF"}`}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(0),
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(1),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_2", Name: "get_time"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(1),
			Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"timezone": "PST"}`}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(1),
		},
		{
			Type: "message_stop",
		},
	}

	for _, e := range events {
		data, _ := json.Marshal(e)
		err := transformer.Transform(&sse.Event{Data: string(data)})
		if err != nil {
			t.Errorf("Transform returned error: %v", err)
		}
	}

	result := buf.String()

	// Should contain both function_call items
	if !strings.Contains(result, `"name":"get_weather"`) {
		t.Error("Expected get_weather in output")
	}
	if !strings.Contains(result, `"name":"get_time"`) {
		t.Error("Expected get_time in output")
	}

	// Should contain both tool IDs
	if !strings.Contains(result, `"id":"toolu_1"`) {
		t.Error("Expected toolu_1 in output")
	}
	if !strings.Contains(result, `"id":"toolu_2"`) {
		t.Error("Expected toolu_2 in output")
	}
}

// TestResponsesTransformer_NestedJSONArgs tests nested JSON in tool arguments.
func TestResponsesTransformer_NestedJSONArgs(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_abc",
				Model: "claude-3",
			},
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(0),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "search"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"query": "test", "filters": {"date": "2024-01"}}`}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(0),
		},
		{
			Type: "message_stop",
		},
	}

	for _, e := range events {
		data, _ := json.Marshal(e)
		transformer.Transform(&sse.Event{Data: string(data)})
	}

	result := buf.String()

	// Should contain arguments field
	if !strings.Contains(result, `"arguments"`) {
		t.Error("Expected arguments in output")
	}
}

// TestResponsesTransformer_EmptyToolArgs tests empty tool arguments.
func TestResponsesTransformer_EmptyToolArgs(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_abc",
				Model: "claude-3",
			},
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(0),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "get_time"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{}`}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(0),
		},
		{
			Type: "message_stop",
		},
	}

	for _, e := range events {
		data, _ := json.Marshal(e)
		transformer.Transform(&sse.Event{Data: string(data)})
	}

	result := buf.String()

	// Should contain empty arguments
	if !strings.Contains(result, `"arguments":"{}"`) {
		t.Error("Expected empty arguments in output")
	}
}

// ============================================================================
// PHASE 2 HIGH PRIORITY TESTS
// ============================================================================

// TestResponsesTransformer_ReasoningSummaryTextDelta tests response.reasoning_summary_text.delta.
// Category B2 (Responses → Chat transformation): HIGH priority
func TestResponsesTransformer_ReasoningSummaryTextDelta(t *testing.T) {
	tests := []struct {
		name     string
		events   []types.Event
		validate func(t *testing.T, output string)
	}{
		{
			name: "reasoning summary text delta streaming",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_abc123",
						Model: "claude-3-opus",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "Let me"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: " analyze this"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: " step by step..."}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{
					Type: "message_stop",
				},
			},
			validate: func(t *testing.T, output string) {
				// Should contain response.reasoning_summary_text.delta events
				if !strings.Contains(output, `"type":"response.reasoning_summary_text.delta"`) {
					t.Error("Expected response.reasoning_summary_text.delta in output")
				}
				// Should have all deltas
				if !strings.Contains(output, `"delta":"Let me"`) {
					t.Error("Expected first reasoning delta")
				}
				if !strings.Contains(output, `"delta":" analyze this"`) {
					t.Error("Expected second reasoning delta")
				}
				if !strings.Contains(output, `"delta":" step by step..."`) {
					t.Error("Expected third reasoning delta")
				}
				// Should have correct output_index
				if !strings.Contains(output, `"output_index":0`) {
					t.Error("Expected output_index 0 for reasoning")
				}
				// Should have summary_index
				if !strings.Contains(output, `"summary_index":0`) {
					t.Error("Expected summary_index")
				}
			},
		},
		{
			name: "reasoning followed by text message",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_def456",
						Model: "claude-3-opus",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "Thinking..."}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(1),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(1),
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "Answer: 42"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(1),
				},
				{
					Type: "message_stop",
				},
			},
			validate: func(t *testing.T, output string) {
				// Should have both reasoning and text events
				if !strings.Contains(output, `"type":"response.reasoning_summary_text.delta"`) {
					t.Error("Expected reasoning summary delta")
				}
				if !strings.Contains(output, `"type":"response.output_text.delta"`) {
					t.Error("Expected output text delta")
				}
				// Reasoning should come before text (output_index 0 vs 1)
				reasoningIdx := strings.Index(output, `"type":"response.reasoning_summary_text.delta"`)
				textIdx := strings.Index(output, `"type":"response.output_text.delta"`)
				if reasoningIdx == -1 || textIdx == -1 {
					t.Fatal("Missing expected events")
				}
				if reasoningIdx > textIdx {
					t.Error("Reasoning should come before text in output")
				}
				// Final output should have both items
				if !strings.Contains(output, `"type":"reasoning"`) {
					t.Error("Expected reasoning item in final output")
				}
				if !strings.Contains(output, `"type":"message"`) {
					t.Error("Expected message item in final output")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, e := range tt.events {
				data, _ := json.Marshal(e)
				err := transformer.Transform(&sse.Event{Data: string(data)})
				if err != nil {
					t.Errorf("Transform returned error: %v", err)
				}
			}

			output := buf.String()
			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

// TestResponsesTransformer_ResponseCompletedWithReasoning tests response.completed with reasoning item.
// Category B2 (Responses → Chat transformation): HIGH priority
func TestResponsesTransformer_ResponseCompletedWithReasoning(t *testing.T) {
	tests := []struct {
		name     string
		events   []types.Event
		validate func(t *testing.T, output string)
	}{
		{
			name: "response.completed with only reasoning",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_abc",
						Type:  "message",
						Role:  "assistant",
						Model: "claude-3-opus",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "Internal reasoning"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{
					Type: "message_stop",
				},
			},
			validate: func(t *testing.T, output string) {
				// Should have response.completed
				if !strings.Contains(output, `"type":"response.completed"`) {
					t.Error("Expected response.completed event")
				}
				// Should have reasoning output item
				if !strings.Contains(output, `"type":"reasoning"`) {
					t.Error("Expected reasoning item in output")
				}
				// Should have the reasoning ID
				if !strings.Contains(output, `"id":"rs_abc"`) {
					t.Error("Expected reasoning ID rs_abc")
				}
				// Should have summary text
				if !strings.Contains(output, `"text":"Internal reasoning"`) {
					t.Error("Expected reasoning summary text")
				}
				// Should have correct structure
				if !strings.Contains(output, `"type":"summary_text"`) {
					t.Error("Expected summary_text type in reasoning")
				}
			},
		},
		{
			name: "response.completed with reasoning and tool call",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_xyz",
						Model: "claude-3-opus",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "Need to check weather"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(1),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "get_weather"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(1),
					Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"city": "SF"}`}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(1),
				},
				{
					Type: "message_stop",
				},
			},
			validate: func(t *testing.T, output string) {
				// Should have both reasoning and function_call in final output
				if !strings.Contains(output, `"type":"reasoning"`) {
					t.Error("Expected reasoning item")
				}
				if !strings.Contains(output, `"type":"function_call"`) {
					t.Error("Expected function_call item")
				}
				// Reasoning should come before function_call
				reasoningIdx := strings.Index(output, `"type":"reasoning"`)
				functionIdx := strings.Index(output, `"type":"function_call"`)
				if reasoningIdx == -1 || functionIdx == -1 {
					t.Fatal("Missing expected items")
				}
				if reasoningIdx > functionIdx {
					t.Error("Reasoning should come before function_call in output")
				}
				// Verify reasoning content
				if !strings.Contains(output, `"text":"Need to check weather"`) {
					t.Error("Expected reasoning text content")
				}
				// Verify function_call content
				if !strings.Contains(output, `"name":"get_weather"`) {
					t.Error("Expected function name")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, e := range tt.events {
				data, _ := json.Marshal(e)
				err := transformer.Transform(&sse.Event{Data: string(data)})
				if err != nil {
					t.Errorf("Transform returned error: %v", err)
				}
			}

			output := buf.String()
			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

// TestResponsesTransformer_FunctionCallArgumentsDelta tests response.function_call_arguments.delta.
// Category B2 (Responses → Chat transformation): HIGH priority
func TestResponsesTransformer_FunctionCallArgumentsDelta(t *testing.T) {
	tests := []struct {
		name     string
		events   []types.Event
		validate func(t *testing.T, output string)
	}{
		{
			name: "function_call_arguments.delta chunked streaming",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_tool123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_abc", Name: "search"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"q`}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `uery": "`}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `hello`}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `"}`}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{
					Type: "message_stop",
				},
			},
			validate: func(t *testing.T, output string) {
				// Should have function_call_arguments.delta events
				if !strings.Contains(output, `"type":"response.function_call_arguments.delta"`) {
					t.Error("Expected response.function_call_arguments.delta")
				}
				// Should have all chunks
				count := strings.Count(output, `"type":"response.function_call_arguments.delta"`)
				if count != 4 {
					t.Errorf("Expected 4 argument delta events, got %d", count)
				}
				// Each chunk should have correct call_id
				if !strings.Contains(output, `"call_id":"toolu_abc"`) {
					t.Error("Expected call_id in argument deltas")
				}
				// Final arguments should be complete
				if !strings.Contains(output, `"arguments":"{\"query\": \"hello\"}"`) {
					t.Error("Expected complete arguments in final output")
				}
			},
		},
		{
			name: "function_call_arguments.delta with special characters",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_special",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_special", Name: "process_text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"text": "Hello
World"}`}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{
					Type: "message_stop",
				},
			},
			validate: func(t *testing.T, output string) {
				// Special characters should be handled correctly
				if !strings.Contains(output, `"type":"response.function_call_arguments.delta"`) {
					t.Error("Expected function_call_arguments.delta")
				}
				// Arguments should contain the text
				if !strings.Contains(output, `"arguments"`) {
					t.Error("Expected arguments field")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, e := range tt.events {
				data, _ := json.Marshal(e)
				err := transformer.Transform(&sse.Event{Data: string(data)})
				if err != nil {
					t.Errorf("Transform returned error: %v", err)
				}
			}

			output := buf.String()
			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

// TestResponsesTransformer_ToolCallsInThinkingContent tests tool call extraction from thinking content.
// This verifies that when Kimi models emit tool calls inside thinking blocks using proprietary markup,
// they are correctly extracted and converted to function_call output items.
func TestResponsesTransformer_ToolCallsInThinkingContent(t *testing.T) {
	tests := []struct {
		name     string
		events   []types.Event
		validate func(t *testing.T, output string)
	}{
		{
			name: "single tool call in thinking block",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_abc",
						Model: "kimi-k2.5",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "Let me help you.<|tool_calls_section_begin|><|tool_call_begin|>bash:32<|tool_call_argument_begin|>{\"cmd\":\"ls\"}<|tool_call_end|><|tool_calls_section_end|>"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{
					Type: "message_stop",
				},
			},
			validate: func(t *testing.T, output string) {
				// Should contain function_call output item
				if !strings.Contains(output, `"type":"function_call"`) {
					t.Error("Expected function_call in output")
				}
				// Should contain function name
				if !strings.Contains(output, `"name":"bash"`) {
					t.Error("Expected function name 'bash' in output")
				}
				// Should contain function_call_arguments.delta for the args
				if !strings.Contains(output, `"type":"response.function_call_arguments.delta"`) {
					t.Error("Expected response.function_call_arguments.delta")
				}
				// Should contain the arguments
				if !strings.Contains(output, `"cmd\":\"ls\"`) {
					t.Error("Expected arguments in output")
				}
			},
		},
		{
			name: "reasoning before tool call",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_xyz",
						Model: "kimi-k2.5",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "I need to run a command."}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_calls_section_begin|><|tool_call_begin|>exec_command<|tool_call_argument_begin|>{\"cmd\":\"pwd\"}<|tool_call_end|><|tool_calls_section_end|>"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{
					Type: "message_stop",
				},
			},
			validate: func(t *testing.T, output string) {
				// Should contain reasoning content
				if !strings.Contains(output, `"delta":"I need to run a command."`) {
					t.Error("Expected reasoning delta in output")
				}
				// Should contain function_call
				if !strings.Contains(output, `"type":"function_call"`) {
					t.Error("Expected function_call in output")
				}
				if !strings.Contains(output, `"name":"exec_command"`) {
					t.Error("Expected function name 'exec_command' in output")
				}
			},
		},
		{
			name: "multiple tool calls in thinking",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_multi",
						Model: "kimi-k2.5",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_calls_section_begin|><|tool_call_begin|>read<|tool_call_argument_begin|>{\"file\":\"a.txt\"}<|tool_call_end|><|tool_call_begin|>write<|tool_call_argument_begin|>{\"file\":\"b.txt\"}<|tool_call_end|><|tool_calls_section_end|>"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{
					Type: "message_stop",
				},
			},
			validate: func(t *testing.T, output string) {
				// Should contain both function calls
				if !strings.Contains(output, `"name":"read"`) {
					t.Error("Expected function name 'read' in output")
				}
				if !strings.Contains(output, `"name":"write"`) {
					t.Error("Expected function name 'write' in output")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			responsesConverter := NewResponsesTransformer(&buf)
			transformer := toolcall.NewAnthropicTransformerWithReceiver(responsesConverter)
			transformer.SetKimiToolCallTransform(true)

			for _, e := range tt.events {
				data, _ := json.Marshal(e)
				err := transformer.Transform(&sse.Event{Data: string(data)})
				if err != nil {
					t.Errorf("Transform returned error: %v", err)
				}
			}
			transformer.Close()

			output := buf.String()
			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

// ============================================================================
// HandleCancel Tests
// ============================================================================

// TestResponsesTransformer_HandleCancel_WithBufferedReasoning tests HandleCancel with buffered reasoning content.
func TestResponsesTransformer_HandleCancel_WithBufferedReasoning(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// Setup: send message_start
	msgStart := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(msgStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	// Start a thinking block
	contentBlock := types.ContentBlock{Type: "thinking"}
	blockData, _ := json.Marshal(contentBlock)
	blockStart := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(0),
		ContentBlock: blockData,
	}
	data, _ = json.Marshal(blockStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	// Send some thinking content
	delta := types.ThinkingDelta{Type: "thinking_delta", Thinking: "Analyzing the request..."}
	deltaData, _ := json.Marshal(delta)
	blockDelta := types.Event{
		Type:  "content_block_delta",
		Index: intPtr(0),
		Delta: deltaData,
	}
	data, _ = json.Marshal(blockDelta)
	transformer.Transform(&sse.Event{Data: string(data)})

	buf.Reset()

	// Call HandleCancel
	err := transformer.HandleCancel()
	if err != nil {
		t.Errorf("HandleCancel returned error: %v", err)
	}

	result := buf.String()

	// Should contain response.reasoning_summary_text.done
	if !strings.Contains(result, `"type":"response.reasoning_summary_text.done"`) {
		t.Error("Result should contain response.reasoning_summary_text.done")
	}

	// Should contain response.reasoning_summary_part.done
	if !strings.Contains(result, `"type":"response.reasoning_summary_part.done"`) {
		t.Error("Result should contain response.reasoning_summary_part.done")
	}

	// Should contain response.output_item.done for reasoning
	if !strings.Contains(result, `"type":"response.output_item.done"`) {
		t.Error("Result should contain response.output_item.done")
	}

	// Should contain reasoning type in output
	if !strings.Contains(result, `"type":"reasoning"`) {
		t.Error("Result should contain reasoning item type")
	}

	// Should contain the reasoning text
	if !strings.Contains(result, `"text":"Analyzing the request..."`) {
		t.Error("Result should contain reasoning text")
	}

	// Should contain response.cancelled
	if !strings.Contains(result, `"type":"response.cancelled"`) {
		t.Error("Result should contain response.cancelled")
	}

	// Should contain status: cancelled
	if !strings.Contains(result, `"status":"cancelled"`) {
		t.Error("Result should contain status: cancelled")
	}
}

// TestResponsesTransformer_HandleCancel_WithInProgressToolCall tests HandleCancel with in-progress tool call.
func TestResponsesTransformer_HandleCancel_WithInProgressToolCall(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// Setup: send message_start
	msgStart := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(msgStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	// Start a tool_use block
	contentBlock := types.ContentBlock{
		Type: "tool_use",
		ID:   "toolu_123",
		Name: "get_weather",
	}
	blockData, _ := json.Marshal(contentBlock)
	blockStart := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(0),
		ContentBlock: blockData,
	}
	data, _ = json.Marshal(blockStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	// Send partial tool arguments
	delta := types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"location": "S`}
	deltaData, _ := json.Marshal(delta)
	blockDelta := types.Event{
		Type:  "content_block_delta",
		Index: intPtr(0),
		Delta: deltaData,
	}
	data, _ = json.Marshal(blockDelta)
	transformer.Transform(&sse.Event{Data: string(data)})

	buf.Reset()

	// Call HandleCancel
	err := transformer.HandleCancel()
	if err != nil {
		t.Errorf("HandleCancel returned error: %v", err)
	}

	result := buf.String()

	// Should contain response.output_item.done for function_call
	if !strings.Contains(result, `"type":"response.output_item.done"`) {
		t.Error("Result should contain response.output_item.done")
	}

	// Should contain function_call type
	if !strings.Contains(result, `"type":"function_call"`) {
		t.Error("Result should contain function_call item type")
	}

	// Should contain the tool ID
	if !strings.Contains(result, `"id":"toolu_123"`) {
		t.Error("Result should contain tool ID")
	}

	// Should contain the function name
	if !strings.Contains(result, `"name":"get_weather"`) {
		t.Error("Result should contain function name")
	}

	// Should contain the partial arguments
	if !strings.Contains(result, `"arguments"`) {
		t.Error("Result should contain arguments field")
	}

	// Should contain response.cancelled
	if !strings.Contains(result, `"type":"response.cancelled"`) {
		t.Error("Result should contain response.cancelled")
	}
}

// TestResponsesTransformer_HandleCancel_WithReasoningAndToolCall tests HandleCancel with both reasoning and tool call.
func TestResponsesTransformer_HandleCancel_WithReasoningAndToolCall(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// Setup: send message_start
	msgStart := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(msgStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	// Start a thinking block
	thinkingBlock := types.ContentBlock{Type: "thinking"}
	thinkingData, _ := json.Marshal(thinkingBlock)
	thinkingStart := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(0),
		ContentBlock: thinkingData,
	}
	data, _ = json.Marshal(thinkingStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	// Send thinking content
	delta := types.ThinkingDelta{Type: "thinking_delta", Thinking: "Need to check weather"}
	deltaData, _ := json.Marshal(delta)
	blockDelta := types.Event{
		Type:  "content_block_delta",
		Index: intPtr(0),
		Delta: deltaData,
	}
	data, _ = json.Marshal(blockDelta)
	transformer.Transform(&sse.Event{Data: string(data)})

	// Start a tool_use block (this will finalize the reasoning)
	toolBlock := types.ContentBlock{
		Type: "tool_use",
		ID:   "toolu_456",
		Name: "get_weather",
	}
	toolData, _ := json.Marshal(toolBlock)
	toolStart := types.Event{
		Type:         "content_block_start",
		Index:        intPtr(1),
		ContentBlock: toolData,
	}
	data, _ = json.Marshal(toolStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	// Send partial tool arguments
	toolDelta := types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"city":`}
	toolDeltaData, _ := json.Marshal(toolDelta)
	toolBlockDelta := types.Event{
		Type:  "content_block_delta",
		Index: intPtr(1),
		Delta: toolDeltaData,
	}
	data, _ = json.Marshal(toolBlockDelta)
	transformer.Transform(&sse.Event{Data: string(data)})

	buf.Reset()

	// Call HandleCancel
	err := transformer.HandleCancel()
	if err != nil {
		t.Errorf("HandleCancel returned error: %v", err)
	}

	result := buf.String()

	// Should contain function_call type (from the in-progress tool call)
	if !strings.Contains(result, `"type":"function_call"`) {
		t.Error("Result should contain function_call item type")
	}

	// Should contain response.cancelled
	if !strings.Contains(result, `"type":"response.cancelled"`) {
		t.Error("Result should contain response.cancelled")
	}

	// Should have output with both items (reasoning was finalized when tool started)
	if !strings.Contains(result, `"output"`) {
		t.Error("Result should contain output field")
	}
}

// TestResponsesTransformer_HandleCancel_EmptyState tests HandleCancel with empty state.
func TestResponsesTransformer_HandleCancel_EmptyState(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// Setup: send message_start to set response ID
	msgStart := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_abc",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(msgStart)
	transformer.Transform(&sse.Event{Data: string(data)})

	buf.Reset()

	// Call HandleCancel without any content
	err := transformer.HandleCancel()
	if err != nil {
		t.Errorf("HandleCancel returned error: %v", err)
	}

	result := buf.String()

	// Should still contain response.cancelled
	if !strings.Contains(result, `"type":"response.cancelled"`) {
		t.Error("Result should contain response.cancelled")
	}

	// Should have empty output
	if !strings.Contains(result, `"output":[]`) {
		t.Error("Result should have empty output array")
	}

	// Should contain status: cancelled
	if !strings.Contains(result, `"status":"cancelled"`) {
		t.Error("Result should contain status: cancelled")
	}
}

// ============================================================================
// Incomplete Response Tests
// ============================================================================

// TestResponsesTransformer_MaxTokensStopReason tests that stop_reason:"max_tokens" produces response.incomplete.
func TestResponsesTransformer_MaxTokensStopReason(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_abc",
				Model: "claude-3",
			},
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(0),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "Hello"}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(0),
		},
		{
			Type:       "message_delta",
			StopReason: "max_tokens",
		},
		{
			Type: "message_stop",
		},
	}

	for _, e := range events {
		data, _ := json.Marshal(e)
		err := transformer.Transform(&sse.Event{Data: string(data)})
		if err != nil {
			t.Errorf("Transform returned error: %v", err)
		}
	}

	result := buf.String()

	// Should contain response.incomplete event
	if !strings.Contains(result, `"type":"response.incomplete"`) {
		t.Error("Result should contain response.incomplete event for max_tokens stop reason")
	}

	// Should have status: incomplete
	if !strings.Contains(result, `"status":"incomplete"`) {
		t.Error("Result should contain status: incomplete")
	}

	// Should have incomplete_details with reason
	if !strings.Contains(result, `"incomplete_details"`) {
		t.Error("Result should contain incomplete_details")
	}

	if !strings.Contains(result, `"reason":"max_output_tokens"`) {
		t.Error("Result should contain reason: max_output_tokens")
	}
}

// TestResponsesTransformer_NormalCompletion tests that normal stop produces response.completed.
func TestResponsesTransformer_NormalCompletion(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_abc",
				Model: "claude-3",
			},
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(0),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "Hello"}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(0),
		},
		{
			Type:       "message_delta",
			StopReason: "end_turn",
		},
		{
			Type: "message_stop",
		},
	}

	for _, e := range events {
		data, _ := json.Marshal(e)
		err := transformer.Transform(&sse.Event{Data: string(data)})
		if err != nil {
			t.Errorf("Transform returned error: %v", err)
		}
	}

	result := buf.String()

	// Should contain response.completed event
	if !strings.Contains(result, `"type":"response.completed"`) {
		t.Error("Result should contain response.completed event for end_turn stop reason")
	}

	// Should have status: completed
	if !strings.Contains(result, `"status":"completed"`) {
		t.Error("Result should contain status: completed")
	}

	// Should NOT have incomplete_details
	if strings.Contains(result, `"incomplete_details"`) {
		t.Error("Result should NOT contain incomplete_details for normal completion")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// error tests (merged from anthropic_to_responses_error_test.go)
// ─────────────────────────────────────────────────────────────────────────────

// TestErrorTransform_InvalidJSONInSSE tests handling of invalid JSON in SSE events.
// Category D2: Incomplete JSON in SSE (HIGH)
func TestErrorTransform_InvalidJSONInSSE(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "incomplete JSON object",
			data: `{"type": "message_start", "message": {`,
		},
		{
			name: "invalid JSON syntax",
			data: `{"type": "message_start", "message": undefined}`,
		},
		{
			name: "truncated event",
			data: `{"type": "content_block_delta", "delta": {"type": "text_delta", "text": "hel`,
		},
		{
			name: "malformed JSON array",
			data: `{"type": "message_start", "message": {"content": [}}`,
		},
		{
			name: "invalid escape in JSON",
			data: `{"type": "message_start", "message": {"id": "msg_\x00"}}`,
		},
		{
			name: "binary data in JSON",
			data: `{"type": "message_start", "message": {"id": "msg_` + string([]byte{0x00, 0x01, 0x02}) + `"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			event := &sse.Event{Data: tt.data}
			err := transformer.Transform(event)
			if err != nil {
				// Error is acceptable, as long as it doesn't panic
				t.Logf("Transform returned error (acceptable): %v", err)
			}

			// Verify output is valid or empty
			_ = buf.String()
		})
	}
}

// TestErrorTransform_MalformedSSEEvents tests handling of malformed SSE events.
// Category D2: Malformed SSE event (HIGH)
func TestErrorTransform_MalformedSSEEvents(t *testing.T) {
	tests := []struct {
		name  string
		event *sse.Event
	}{
		{
			name:  "empty event data",
			event: &sse.Event{Data: ""},
		},
		{
			name:  "only whitespace",
			event: &sse.Event{Data: "   \n\t  "},
		},
		{
			name:  "event with only newlines",
			event: &sse.Event{Data: "\n\n\n"},
		},
		{
			name:  "event with null bytes",
			event: &sse.Event{Data: string([]byte{0x00, 0x00, 0x00})},
		},
		{
			name:  "very long event data",
			event: &sse.Event{Data: strings.Repeat("a", 1000000)},
		},
		{
			name:  "event with special characters",
			event: &sse.Event{Data: "\x00\x01\x02\x03\x04\x05"},
		},
		{
			name:  "event type without data",
			event: &sse.Event{Type: "message", Data: ""},
		},
		{
			name:  "event ID without data",
			event: &sse.Event{LastEventID: "123", Data: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			err := transformer.Transform(tt.event)
			if err != nil {
				t.Logf("Transform returned error (acceptable): %v", err)
			}

			// Should not panic
		})
	}
}

// TestErrorTransform_MissingRequiredFields tests handling of events with missing fields.
func TestErrorTransform_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		events []types.Event
	}{
		{
			name: "message_start without message",
			events: []types.Event{
				{Type: "message_start"},
			},
		},
		{
			name: "message_start with null message",
			events: []types.Event{
				{Type: "message_start", Message: nil},
			},
		},
		{
			name: "message_start with empty message ID",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "",
						Type:  "message",
						Role:  "assistant",
						Model: "claude-3",
					},
				},
			},
		},
		{
			name: "content_block_start without index",
			events: []types.Event{
				{
					Type:         "content_block_start",
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
			},
		},
		{
			name: "content_block_start without content_block",
			events: []types.Event{
				{
					Type:  "content_block_start",
					Index: intPtr(0),
				},
			},
		},
		{
			name: "content_block_delta without index",
			events: []types.Event{
				{
					Type:  "content_block_delta",
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "hello"}),
				},
			},
		},
		{
			name: "content_block_delta without delta",
			events: []types.Event{
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
				},
			},
		},
		{
			name: "content_block_stop without index",
			events: []types.Event{
				{Type: "content_block_stop"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				err := transformer.Transform(&sse.Event{Data: string(data)})
				_ = err
			}

			// Should handle missing fields without panic
			_ = buf.String()
		})
	}
}

// TestErrorTransform_InvalidContentBlockTypes tests handling of invalid content block types.
func TestErrorTransform_InvalidContentBlockTypes(t *testing.T) {
	tests := []struct {
		name   string
		events []types.Event
	}{
		{
			name: "unknown content block type",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "unknown_type"}),
				},
			},
		},
		{
			name: "empty content block type",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: ""}),
				},
			},
		},
		{
			name: "tool_use without ID",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", Name: "test_tool"}),
				},
			},
		},
		{
			name: "tool_use without name",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123"}),
				},
			},
		},
		{
			name: "text block with null content",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: json.RawMessage(`{"type": "text", "text": null}`),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				err := transformer.Transform(&sse.Event{Data: string(data)})
				_ = err
			}

			// Should handle invalid content blocks without panic
			_ = buf.String()
		})
	}
}

// TestErrorTransform_InvalidDeltaTypes tests handling of invalid delta types.
func TestErrorTransform_InvalidDeltaTypes(t *testing.T) {
	tests := []struct {
		name   string
		events []types.Event
	}{
		{
			name: "unknown delta type",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: json.RawMessage(`{"type": "unknown_delta", "text": "hello"}`),
				},
			},
		},
		{
			name: "delta with null text",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: json.RawMessage(`{"type": "text_delta", "text": null}`),
				},
			},
		},
		{
			name: "thinking delta without thinking field",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: json.RawMessage(`{"type": "thinking_delta", "thinking": null}`),
				},
			},
		},
		{
			name: "input_json_delta without partial_json",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "test"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: json.RawMessage(`{"type": "input_json_delta"}`),
				},
			},
		},
		{
			name: "delta with mismatched index",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(999), // Mismatched index
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "hello"}),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				err := transformer.Transform(&sse.Event{Data: string(data)})
				_ = err
			}

			// Should handle invalid deltas without panic
			_ = buf.String()
		})
	}
}

// TestErrorTransform_UpstreamTimeoutSimulates tests handling that simulates upstream timeout.
// Category D2: Upstream timeout (HIGH)
func TestErrorTransform_UpstreamTimeoutSimulates(t *testing.T) {
	t.Run("incomplete stream without message_stop", func(t *testing.T) {
		var buf bytes.Buffer
		transformer := NewResponsesTransformer(&buf)

		// Send message_start and some deltas but no message_stop
		events := []types.Event{
			{
				Type: "message_start",
				Message: &types.MessageInfo{
					ID:    "msg_123",
					Model: "claude-3",
				},
			},
			{
				Type:         "content_block_start",
				Index:        intPtr(0),
				ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
			},
			{
				Type:  "content_block_delta",
				Index: intPtr(0),
				Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "Hello"}),
			},
			// Missing content_block_stop and message_stop - simulates timeout
		}

		for _, event := range events {
			data, _ := json.Marshal(event)
			_ = transformer.Transform(&sse.Event{Data: string(data)})
		}

		// Verify transformer is still in consistent state
		if transformer.messageID != "msg_123" {
			t.Error("Expected messageID to be set")
		}
		if !transformer.inText {
			t.Error("Expected inText to be true (block not stopped)")
		}
	})

	t.Run("partial tool call without completion", func(t *testing.T) {
		var buf bytes.Buffer
		transformer := NewResponsesTransformer(&buf)

		events := []types.Event{
			{
				Type: "message_start",
				Message: &types.MessageInfo{
					ID:    "msg_123",
					Model: "claude-3",
				},
			},
			{
				Type:         "content_block_start",
				Index:        intPtr(0),
				ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "test"}),
			},
			{
				Type:  "content_block_delta",
				Index: intPtr(0),
				Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"partial": `}),
			},
			// Missing rest of JSON and content_block_stop
		}

		for _, event := range events {
			data, _ := json.Marshal(event)
			_ = transformer.Transform(&sse.Event{Data: string(data)})
		}

		// Verify partial state
		if !transformer.inToolCall {
			t.Error("Expected inToolCall to be true")
		}
		if transformer.currentID != "toolu_123" {
			t.Errorf("Expected currentID to be 'toolu_123', got '%s'", transformer.currentID)
		}
	})
}

// TestErrorTransform_InvalidUsageData tests handling of invalid usage data.
func TestErrorTransform_InvalidUsageData(t *testing.T) {
	tests := []struct {
		name   string
		events []types.Event
	}{
		{
			name: "negative token counts",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
						Usage: &types.AnthropicUsage{
							InputTokens:  -100,
							OutputTokens: -50,
						},
					},
				},
			},
		},
		{
			name: "zero token counts",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
						Usage: &types.AnthropicUsage{
							InputTokens:  0,
							OutputTokens: 0,
						},
					},
				},
			},
		},
		{
			name: "very large token counts",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
						Usage: &types.AnthropicUsage{
							InputTokens:  2147483647,
							OutputTokens: 2147483647,
						},
					},
				},
			},
		},
		{
			name: "negative cache tokens",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
						Usage: &types.AnthropicUsage{
							InputTokens:              100,
							OutputTokens:             50,
							CacheReadInputTokens:     -10,
							CacheCreationInputTokens: -5,
						},
					},
				},
			},
		},
		{
			name: "cache tokens greater than input tokens",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
						Usage: &types.AnthropicUsage{
							InputTokens:          100,
							OutputTokens:         50,
							CacheReadInputTokens: 200, // More than input tokens
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				err := transformer.Transform(&sse.Event{Data: string(data)})
				_ = err
			}

			// Send message_stop to complete
			data, _ := json.Marshal(types.Event{Type: "message_stop"})
			_ = transformer.Transform(&sse.Event{Data: string(data)})

			// Should handle invalid usage data without panic
			_ = buf.String()
		})
	}
}

// TestErrorTransform_InvalidStopReason tests handling of invalid stop reasons.
func TestErrorTransform_InvalidStopReason(t *testing.T) {
	tests := []struct {
		name       string
		stopReason string
	}{
		{
			name:       "unknown stop reason",
			stopReason: "unknown_reason",
		},
		{
			name:       "empty stop reason",
			stopReason: "",
		},
		{
			name:       "whitespace stop reason",
			stopReason: "   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			// First send message_start
			msgStart := types.Event{
				Type: "message_start",
				Message: &types.MessageInfo{
					ID:    "msg_123",
					Model: "claude-3",
				},
			}
			data, _ := json.Marshal(msgStart)
			_ = transformer.Transform(&sse.Event{Data: string(data)})

			// Then send message_delta with stop reason
			msgDelta := types.Event{
				Type:       "message_delta",
				StopReason: tt.stopReason,
			}
			data, _ = json.Marshal(msgDelta)
			err := transformer.Transform(&sse.Event{Data: string(data)})
			_ = err

			// Should handle invalid stop reason without panic
		})
	}
}

// TestErrorTransform_UnknownEventTypes tests handling of unknown event types.
func TestErrorTransform_UnknownEventTypes(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
	}{
		{
			name:      "completely unknown event",
			eventType: "unknown_event_type",
		},
		{
			name:      "event with special characters",
			eventType: "message<start>",
		},
		{
			name:      "empty event type",
			eventType: "",
		},
		{
			name:      "null-like event type",
			eventType: "null",
		},
		{
			name:      "event type with spaces",
			eventType: "message start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			event := types.Event{Type: tt.eventType}
			data, _ := json.Marshal(event)
			err := transformer.Transform(&sse.Event{Data: string(data)})
			_ = err

			// Should handle unknown event types without panic
			_ = buf.String()
		})
	}
}

// TestErrorTransform_MalformedToolCallData tests handling of malformed tool call data.
func TestErrorTransform_MalformedToolCallData(t *testing.T) {
	tests := []struct {
		name   string
		events []types.Event
	}{
		{
			name: "tool call with invalid JSON in arguments",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "test"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{invalid json`}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
		},
		{
			name: "tool call with very long arguments",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "test"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"text": "` + strings.Repeat("a", 100000) + `"}`}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
		},
		{
			name: "tool call with empty tool name",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: ""}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				err := transformer.Transform(&sse.Event{Data: string(data)})
				_ = err
			}

			// Should handle malformed tool call data without panic
			output := buf.String()
			// Verify output contains expected function_call structure
			if !strings.Contains(output, "function_call") {
				t.Error("Expected output to contain function_call")
			}
		})
	}
}

// TestErrorTransform_RapidEventSequence tests handling of rapid event sequences.
func TestErrorTransform_RapidEventSequence(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// Send many events rapidly
	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_123",
				Model: "claude-3",
			},
		},
	}

	// Add many content blocks
	for i := 0; i < 100; i++ {
		events = append(events, types.Event{
			Type:         "content_block_start",
			Index:        intPtr(i),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
		})
		events = append(events, types.Event{
			Type:  "content_block_delta",
			Index: intPtr(i),
			Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "a"}),
		})
		events = append(events, types.Event{
			Type:  "content_block_stop",
			Index: intPtr(i),
		})
	}

	events = append(events, types.Event{Type: "message_stop"})

	for _, event := range events {
		data, _ := json.Marshal(event)
		err := transformer.Transform(&sse.Event{Data: string(data)})
		if err != nil {
			t.Errorf("Transform returned error: %v", err)
		}
	}

	// Verify output
	output := buf.String()
	if !strings.Contains(output, "response.completed") {
		t.Error("Expected output to contain response.completed")
	}
}

// TestErrorTransform_StateConsistency tests that transformer maintains consistent state.
func TestErrorTransform_StateConsistency(t *testing.T) {
	t.Run("sequence number always increases", func(t *testing.T) {
		var buf bytes.Buffer
		transformer := NewResponsesTransformer(&buf)

		// Send events and track sequence numbers
		events := []types.Event{
			{
				Type: "message_start",
				Message: &types.MessageInfo{
					ID:    "msg_123",
					Model: "claude-3",
				},
			},
			{
				Type:         "content_block_start",
				Index:        intPtr(0),
				ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
			},
			{
				Type:  "content_block_delta",
				Index: intPtr(0),
				Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "Hello"}),
			},
			{
				Type:  "content_block_stop",
				Index: intPtr(0),
			},
			{Type: "message_stop"},
		}

		prevSeqNum := 0
		for _, event := range events {
			buf.Reset()
			data, _ := json.Marshal(event)
			_ = transformer.Transform(&sse.Event{Data: string(data)})
			output := buf.String()

			// Extract sequence number from output
			if strings.Contains(output, "sequence_number") {
				var result struct{ Seq int }
				if err := json.Unmarshal([]byte(`{"seq":`+extractSequenceNumber(output)+`}`), &result); err == nil {
					if result.Seq <= prevSeqNum {
						t.Errorf("Sequence number did not increase: %d -> %d", prevSeqNum, result.Seq)
					}
					prevSeqNum = result.Seq
				}
			}
		}
	})
}

// Helper function to extract sequence number from output
func extractSequenceNumber(output string) string {
	start := strings.Index(output, `"sequence_number":`)
	if start == -1 {
		return "0"
	}
	start += len(`"sequence_number":`)
	end := start
	for end < len(output) && (output[end] >= '0' && output[end] <= '9') {
		end++
	}
	if end > start {
		return output[start:end]
	}
	return "0"
}

// TestErrorTransform_MultipleMessageStarts tests handling of multiple message_start events.
func TestErrorTransform_MultipleMessageStarts(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// First message_start
	event1 := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_123",
			Model: "claude-3",
		},
	}
	data, _ := json.Marshal(event1)
	_ = transformer.Transform(&sse.Event{Data: string(data)})

	// Second message_start (should reset state)
	event2 := types.Event{
		Type: "message_start",
		Message: &types.MessageInfo{
			ID:    "msg_456",
			Model: "claude-3-opus",
		},
	}
	data, _ = json.Marshal(event2)
	_ = transformer.Transform(&sse.Event{Data: string(data)})

	// Verify state was reset
	if transformer.messageID != "msg_456" {
		t.Errorf("Expected messageID to be 'msg_456', got '%s'", transformer.messageID)
	}
}

// TestErrorTransform_EmptyToolCallID tests handling of empty tool call IDs.
func TestErrorTransform_EmptyToolCallID(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_123",
				Model: "claude-3",
			},
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(0),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "", Name: "test_tool"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{}`}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(0),
		},
		{Type: "message_stop"},
	}

	for _, event := range events {
		data, _ := json.Marshal(event)
		err := transformer.Transform(&sse.Event{Data: string(data)})
		_ = err
	}

	// Should handle empty tool call ID
	output := buf.String()
	if !strings.Contains(output, "function_call") {
		t.Error("Expected output to contain function_call")
	}
}

// TestErrorTransform_RateLimitError tests rate limit error handling.
// Category D1: Rate limit error (HIGH)
func TestErrorTransform_RateLimitError(t *testing.T) {
	tests := []struct {
		name   string
		events []types.Event
	}{
		{
			name: "rate limit error event",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type: "error",
					Message: &types.MessageInfo{
						ID:    "error_123",
						Type:  "error",
						Role:  "assistant",
						Model: "claude-3",
					},
				},
			},
		},
		{
			name: "rate limit during streaming",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "Hello"}),
				},
				// Rate limit error occurs before completion
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				_ = transformer.Transform(&sse.Event{Data: string(data)})
			}

			// Should handle rate limit error without panic
			_ = buf.String()
		})
	}
}

// TestErrorTransform_AuthenticationError tests authentication error handling.
// Category D1: Authentication error (HIGH)
func TestErrorTransform_AuthenticationError(t *testing.T) {
	tests := []struct {
		name   string
		events []types.Event
	}{
		{
			name: "authentication error at start",
			events: []types.Event{
				{
					Type: "error",
					Message: &types.MessageInfo{
						ID:    "error_auth",
						Type:  "error",
						Role:  "assistant",
						Model: "claude-3",
					},
				},
			},
		},
		{
			name: "authentication error with message start",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type: "error",
					Message: &types.MessageInfo{
						ID:    "error_auth",
						Type:  "error",
						Role:  "assistant",
						Model: "claude-3",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				_ = transformer.Transform(&sse.Event{Data: string(data)})
			}

			// Should handle authentication error without panic
			_ = buf.String()
		})
	}
}

// TestErrorTransform_UpstreamTimeout tests upstream timeout handling.
// Category D1: Upstream timeout handling (HIGH)
func TestErrorTransform_UpstreamTimeout(t *testing.T) {
	tests := []struct {
		name   string
		events []types.Event
	}{
		{
			name: "timeout during text generation",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "Hello"}),
				},
				// Timeout before content_block_stop
			},
		},
		{
			name: "timeout during reasoning",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "Let me think"}),
				},
				// Timeout before reasoning completion
			},
		},
		{
			name: "timeout during tool call",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "get_weather"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"location": "NYC"}`}),
				},
				// Timeout before tool call completion
			},
		},
		{
			name: "timeout after partial message_stop",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:  "message_delta",
					Usage: &types.AnthropicUsage{InputTokens: 100, OutputTokens: 50},
				},
				// Timeout before message_stop
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				_ = transformer.Transform(&sse.Event{Data: string(data)})
			}

			// Verify transformer is in consistent state despite timeout
			if transformer.messageID != "msg_123" {
				t.Error("Expected messageID to be set")
			}
		})
	}
}

// TestErrorTransform_UpstreamConnectionReset tests upstream connection reset.
// Category D1: Upstream connection reset (HIGH)
func TestErrorTransform_UpstreamConnectionReset(t *testing.T) {
	tests := []struct {
		name   string
		events []types.Event
	}{
		{
			name: "connection reset after message_start",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				// Connection reset - no more events
			},
		},
		{
			name: "connection reset during content streaming",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "Partial "}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "content"}),
				},
				// Connection reset before content_block_stop
			},
		},
		{
			name: "connection reset during tool streaming",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "search"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"query": "in`}),
				},
				// Connection reset before completion
			},
		},
		{
			name: "connection reset during reasoning",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "Analyzing..."}),
				},
				// Connection reset before reasoning completion
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				_ = transformer.Transform(&sse.Event{Data: string(data)})
			}

			// Should handle incomplete stream without panic
			_ = buf.String()
		})
	}
}

// TestErrorTransform_ContentOnlyNewlines tests content with only newlines.
// Category E1: Content with only newlines (HIGH)
func TestErrorTransform_ContentOnlyNewlines(t *testing.T) {
	tests := []struct {
		name   string
		events []types.Event
		expect string
	}{
		{
			name: "single newline in text",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "\n"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: "\n",
		},
		{
			name: "multiple newlines",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "\n\n\n"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: "\\n\\n\\n", // JSON-escaped newlines
		},
		{
			name: "windows line endings",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "\r\n\r\n"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: "\\r\\n\\r\\n", // JSON-escaped Windows line endings
		},
		{
			name: "newlines in thinking",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "\n\n"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: "reasoning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				_ = transformer.Transform(&sse.Event{Data: string(data)})
			}

			output := buf.String()
			if tt.expect != "" && !strings.Contains(output, tt.expect) {
				t.Errorf("Expected output to contain '%s', got: %s", tt.expect, output)
			}
		})
	}
}

// TestErrorTransform_UnicodeEmojiHandling tests Unicode emoji handling.
// Category E1: Unicode emoji handling (HIGH)
func TestErrorTransform_UnicodeEmojiHandling(t *testing.T) {
	tests := []struct {
		name   string
		events []types.Event
		expect string
	}{
		{
			name: "basic emojis",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "👋"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: "👋",
		},
		{
			name: "complex emoji - flag",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "🇺🇸"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: "🇺🇸",
		},
		{
			name: "emoji in thinking",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "🤔"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: "🤔",
		},
		{
			name: "emoji in tool arguments",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "get_weather"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"city": "🌆"}`}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: "🌆",
		},
		{
			name: "CJK characters",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "世界你好"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: "世界你好",
		},
		{
			name: "RTL text",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "text"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.TextDelta{Type: "text_delta", Text: "مرحبا"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: "مرحبا",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				_ = transformer.Transform(&sse.Event{Data: string(data)})
			}

			output := buf.String()
			if !strings.Contains(output, tt.expect) {
				t.Errorf("Expected output to contain '%s', got: %s", tt.expect, output)
			}
		})
	}
}

// TestErrorTransform_ToolWithNoParameters tests tool with no parameters.
// Category E2: Tool with no parameters (MEDIUM)
func TestErrorTransform_ToolWithNoParameters(t *testing.T) {
	// This tests the tool call conversion in responses_to_anthropic
	tests := []struct {
		name   string
		events []types.Event
		expect string
	}{
		{
			name: "tool call with empty JSON arguments",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "get_time"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{}`}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: `"arguments":"{}"`,
		},
		{
			name: "tool call with no arguments streamed",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "ping"}),
				},
				// No content_block_delta for arguments
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: "function_call",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				_ = transformer.Transform(&sse.Event{Data: string(data)})
			}

			output := buf.String()
			if !strings.Contains(output, tt.expect) {
				t.Errorf("Expected output to contain '%s', got: %s", tt.expect, output)
			}
		})
	}
}

// TestErrorTransform_ToolNameWithSpaces tests tool name with spaces.
// Category E2: Tool name with spaces (MEDIUM)
func TestErrorTransform_ToolNameWithSpaces(t *testing.T) {
	tests := []struct {
		name   string
		events []types.Event
		expect string
	}{
		{
			name: "tool name with internal space",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "get weather"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: `"name":"get weather"`,
		},
		{
			name: "tool name with leading space",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: " get_weather"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: `"name":" get_weather"`,
		},
		{
			name: "tool name with trailing space",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "get_weather "}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: `"name":"get_weather "`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				_ = transformer.Transform(&sse.Event{Data: string(data)})
			}

			output := buf.String()
			if !strings.Contains(output, tt.expect) {
				t.Errorf("Expected output to contain '%s', got: %s", tt.expect, output)
			}
		})
	}
}

// TestErrorTransform_ToolNameWithSpecialChars tests tool name with special characters.
// Category E2: Tool name with special chars (MEDIUM)
func TestErrorTransform_ToolNameWithSpecialChars(t *testing.T) {
	tests := []struct {
		name   string
		events []types.Event
		expect string
	}{
		{
			name: "tool name with hyphen",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "get-weather"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: `"name":"get-weather"`,
		},
		{
			name: "tool name with dot",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "get.weather"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: `"name":"get.weather"`,
		},
		{
			name: "tool name with underscore",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "get_weather"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: `"name":"get_weather"`,
		},
		{
			name: "tool name with number",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_123",
						Model: "claude-3",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "tool_use", ID: "toolu_123", Name: "get_weather_v2"}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{Type: "message_stop"},
			},
			expect: `"name":"get_weather_v2"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewResponsesTransformer(&buf)

			for _, event := range tt.events {
				data, _ := json.Marshal(event)
				_ = transformer.Transform(&sse.Event{Data: string(data)})
			}

			output := buf.String()
			if !strings.Contains(output, tt.expect) {
				t.Errorf("Expected output to contain '%s', got: %s", tt.expect, output)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// extracted tests (merged from anthropic_to_responses_extracted_test.go)
// ─────────────────────────────────────────────────────────────────────────────

// TestExtractedToolCallsPreserveIDAndName reproduces the bug where tool calls
// extracted from thinking content lose their ID and Name in the output_item.done event.
func TestExtractedToolCallsPreserveIDAndName(t *testing.T) {
	var buf bytes.Buffer
	responsesConverter := NewResponsesTransformer(&buf)
	transformer := toolcall.NewAnthropicTransformerWithReceiver(responsesConverter)
	transformer.SetKimiToolCallTransform(true)

	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_test123",
				Model: "kimi-k2.5",
			},
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(0),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking", Thinking: ""}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_calls_section_begin|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_begin|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "exec_command:4"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_argument_begin|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `{"cmd":"ls -la"}`}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_end|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_calls_section_end|>"}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(0),
		},
		{
			Type:       "message_delta",
			StopReason: "end_turn",
			Usage:      &types.AnthropicUsage{OutputTokens: 10},
		},
		{
			Type: "message_stop",
		},
	}

	for _, event := range events {
		if err := func() error {
			data, _ := json.Marshal(event)
			return transformer.Transform(&sse.Event{Data: string(data)})
		}(); err != nil {
			t.Fatalf("handleEvent error: %v", err)
		}
	}

	transformer.Close()

	output := buf.String()
	t.Logf("Output:\n%s", output)

	// Parse each SSE event and check for the bug
	lines := strings.Split(output, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			eventType, _ := event["type"].(string)
			if eventType == "response.output_item.done" {
				item, _ := event["item"].(map[string]interface{})
				if item == nil {
					continue
				}
				itemType, _ := item["type"].(string)
				if itemType == "function_call" {
					// Check that name, id, call_id, and item_id are not empty
					name, _ := item["name"].(string)
					id, _ := item["id"].(string)
					callID, _ := item["call_id"].(string)
					itemID, _ := event["item_id"].(string)

					if name == "" {
						t.Errorf("BUG: function_call output_item.done has empty 'name' field")
					}
					if id == "" {
						t.Errorf("BUG: function_call output_item.done has empty 'id' field")
					}
					if callID == "" {
						t.Errorf("BUG: function_call output_item.done has empty 'call_id' field")
					}
					if itemID == "" {
						t.Errorf("BUG: function_call output_item.done has empty 'item_id' field")
					}

					// Verify the function name is correct
					if name != "exec_command" {
						t.Errorf("Expected function name 'exec_command', got '%s'", name)
					}

					t.Logf("SUCCESS: function_call output_item.done has correct values: id=%s, name=%s, call_id=%s, item_id=%s", id, name, callID, itemID)
				}
			}
		}
	}
}

// TestExtractedToolCallsPreserveIDAndName_MultipleToolCalls tests multiple tool calls
// extracted from thinking content.
func TestExtractedToolCallsPreserveIDAndName_MultipleToolCalls(t *testing.T) {
	var buf bytes.Buffer
	responsesConverter := NewResponsesTransformer(&buf)
	transformer := toolcall.NewAnthropicTransformerWithReceiver(responsesConverter)
	transformer.SetKimiToolCallTransform(true)

	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_test456",
				Model: "kimi-k2.5",
			},
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(0),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking", Thinking: ""}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_calls_section_begin|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_begin|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "exec_command:0"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_argument_begin|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `{"cmd":"ls"}`}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_end|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_begin|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "read_file:1"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_argument_begin|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `{"path":"/tmp/test.txt"}`}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_end|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_calls_section_end|>"}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(0),
		},
		{
			Type:       "message_delta",
			StopReason: "end_turn",
			Usage:      &types.AnthropicUsage{OutputTokens: 10},
		},
		{
			Type: "message_stop",
		},
	}

	for _, event := range events {
		if err := func() error {
			data, _ := json.Marshal(event)
			return transformer.Transform(&sse.Event{Data: string(data)})
		}(); err != nil {
			t.Fatalf("handleEvent error: %v", err)
		}
	}

	transformer.Close()

	output := buf.String()
	t.Logf("Output:\n%s", output)

	// Count tool calls and verify they all have correct names
	toolCallCount := 0
	toolNames := make(map[string]bool)

	lines := strings.Split(output, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			eventType, _ := event["type"].(string)
			if eventType == "response.output_item.done" {
				item, _ := event["item"].(map[string]interface{})
				if item == nil {
					continue
				}
				itemType, _ := item["type"].(string)
				if itemType == "function_call" {
					toolCallCount++
					name, _ := item["name"].(string)
					id, _ := item["id"].(string)

					if name == "" {
						t.Errorf("BUG: function_call %d has empty 'name' field", toolCallCount)
					} else {
						toolNames[name] = true
						t.Logf("Tool call %d: name=%s, id=%s", toolCallCount, name, id)
					}
				}
			}
		}
	}

	if toolCallCount != 2 {
		t.Errorf("Expected 2 tool calls, got %d", toolCallCount)
	}

	// Verify we have both exec_command and read_file
	if !toolNames["exec_command"] {
		t.Errorf("Missing tool call 'exec_command'")
	}
	if !toolNames["read_file"] {
		t.Errorf("Missing tool call 'read_file'")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// mixed tests (merged from anthropic_to_responses_mixed_test.go)
// ─────────────────────────────────────────────────────────────────────────────

// TestToolUseBlockWithExtractedArgs reproduces a scenario where the model
// sends a tool_use block (generating output_item.added with correct ID)
// but then the arguments come embedded in thinking content (which would
// need to extract them properly).
//
// This tests the case where Kimi outputs:
// 1. An explicit tool_use block start (with correct ID)
// 2. Arguments inside thinking_delta instead of input_json_delta
func TestToolUseBlockWithExtractedArgs(t *testing.T) {
	var buf bytes.Buffer
	responsesConverter := NewResponsesTransformer(&buf)
	transformer := toolcall.NewAnthropicTransformerWithReceiver(responsesConverter)
	transformer.SetKimiToolCallTransform(true)

	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_test123",
				Model: "kimi-k2.5",
			},
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(0),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking", Thinking: ""}),
		},
		// Thinking with tool call markup - tool call gets extracted
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_calls_section_begin|><|tool_call_begin|>exec_command:0<|tool_call_argument_begin|>"}),
		},
		// First args chunk
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `{"cmd":"ls"}`}),
		},
		// End tool call
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_end|><|tool_calls_section_end|>"}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(0),
		},
		// Now an explicit tool_use block (this is another tool call)
		{
			Type:         "content_block_start",
			Index:        intPtr(1),
			ContentBlock: json.RawMessage(`{"type":"tool_use","id":"toolu_abc123","name":"read_file"}`),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(1),
			Delta: mustMarshal(types.InputJSONDelta{Type: "input_json_delta", PartialJSON: `{"path":"/tmp/test.txt"}`}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(1),
		},
		{
			Type:       "message_delta",
			StopReason: "end_turn",
			Usage:      &types.AnthropicUsage{OutputTokens: 10},
		},
		{
			Type: "message_stop",
		},
	}

	for _, event := range events {
		data, _ := json.Marshal(event)
		err := transformer.Transform(&sse.Event{Data: string(data)})
		if err != nil {
			t.Fatalf("Transform error: %v", err)
		}
	}

	transformer.Close()

	output := buf.String()
	t.Logf("Output:\n%s", output)

	// Check that all function_call_arguments.delta events have proper IDs
	lines := strings.Split(output, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			eventType, _ := event["type"].(string)
			if eventType == "response.function_call_arguments.delta" {
				itemID, _ := event["item_id"].(string)
				callID, _ := event["call_id"].(string)
				delta, _ := event["delta"].(string)

				t.Logf("delta: item_id=%s, call_id=%s, delta=%s", itemID, callID, delta)

				if itemID == "" {
					t.Errorf("BUG: function_call_arguments.delta has empty 'item_id' field")
				}
				if callID == "" {
					t.Errorf("BUG: function_call_arguments.delta has empty 'call_id' field")
				}
			}
		}
	}
}

// TestToolCallIDConsistency verifies that the ID in output_item.added
// matches the ID in all subsequent delta events for the same tool call.
func TestToolCallIDConsistency(t *testing.T) {
	var buf bytes.Buffer
	responsesConverter := NewResponsesTransformer(&buf)
	transformer := toolcall.NewAnthropicTransformerWithReceiver(responsesConverter)
	transformer.SetKimiToolCallTransform(true)

	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_consistency",
				Model: "kimi-k2.5",
			},
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(0),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking", Thinking: ""}),
		},
		// Single tool call split across multiple chunks
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_calls_section_begin|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_begin|>bash"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_argument_begin|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `{"com`}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `mand":"echo hello"}`}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_end|>"}),
		},
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_calls_section_end|>"}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(0),
		},
		{
			Type:       "message_delta",
			StopReason: "end_turn",
			Usage:      &types.AnthropicUsage{OutputTokens: 10},
		},
		{
			Type: "message_stop",
		},
	}

	for _, event := range events {
		data, _ := json.Marshal(event)
		err := transformer.Transform(&sse.Event{Data: string(data)})
		if err != nil {
			t.Fatalf("Transform error: %v", err)
		}
	}

	transformer.Close()

	output := buf.String()
	t.Logf("Output:\n%s", output)

	// Extract the tool call ID from output_item.added
	var expectedID string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			eventType, _ := event["type"].(string)
			if eventType == "response.output_item.added" {
				item, _ := event["item"].(map[string]interface{})
				if item != nil {
					itemType, _ := item["type"].(string)
					if itemType == "function_call" {
						expectedID, _ = item["id"].(string)
						t.Logf("Found tool call ID: %s", expectedID)
						break
					}
				}
			}
		}
	}

	if expectedID == "" {
		t.Fatal("Could not find tool call ID in output_item.added")
	}

	// Verify all delta events use the same ID
	lines = strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			eventType, _ := event["type"].(string)
			if eventType == "response.function_call_arguments.delta" {
				itemID, _ := event["item_id"].(string)
				callID, _ := event["call_id"].(string)

				if itemID != expectedID {
					t.Errorf("BUG: function_call_arguments.delta has item_id=%s, expected %s", itemID, expectedID)
				}
				if callID != expectedID {
					t.Errorf("BUG: function_call_arguments.delta has call_id=%s, expected %s", callID, expectedID)
				}
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// issue tests (merged from anthropic_to_responses_issue_test.go)
// ─────────────────────────────────────────────────────────────────────────────

// TestExtractedToolCalls_SplitAcrossChunks reproduces the bug where tool calls
// extracted from thinking content have empty item_id/call_id in delta events
// when the markup is split across multiple streaming chunks.
func TestExtractedToolCalls_SplitAcrossChunks(t *testing.T) {
	var buf bytes.Buffer
	responsesConverter := NewResponsesTransformer(&buf)
	transformer := toolcall.NewAnthropicTransformerWithReceiver(responsesConverter)
	transformer.SetKimiToolCallTransform(true)

	events := []types.Event{
		{
			Type: "message_start",
			Message: &types.MessageInfo{
				ID:    "msg_test123",
				Model: "kimi-k2.5",
			},
		},
		{
			Type:         "content_block_start",
			Index:        intPtr(0),
			ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking", Thinking: ""}),
		},
		// Chunk 1: Section begin
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_calls_section_begin|>"}),
		},
		// Chunk 2: Tool call begin
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_begin|>"}),
		},
		// Chunk 3: ID and name (split)
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "exec_"}),
		},
		// Chunk 4: Rest of name
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "command:0"}),
		},
		// Chunk 5: Arg begin
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_argument_begin|>"}),
		},
		// Chunk 6: First part of args
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `{"`}),
		},
		// Chunk 7: More args
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `cmd":"ls -la"}`}),
		},
		// Chunk 8: Tool call end
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_call_end|>"}),
		},
		// Chunk 9: Section end
		{
			Type:  "content_block_delta",
			Index: intPtr(0),
			Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: "<|tool_calls_section_end|>"}),
		},
		{
			Type:  "content_block_stop",
			Index: intPtr(0),
		},
		{
			Type:       "message_delta",
			StopReason: "end_turn",
			Usage:      &types.AnthropicUsage{OutputTokens: 10},
		},
		{
			Type: "message_stop",
		},
	}

	for _, event := range events {
		if err := func() error {
			data, _ := json.Marshal(event)
			return transformer.Transform(&sse.Event{Data: string(data)})
		}(); err != nil {
			t.Fatalf("handleEvent error: %v", err)
		}
	}

	transformer.Close()

	output := buf.String()
	t.Logf("Output:\n%s", output)

	// Parse each SSE event and check for the bug
	lines := strings.Split(output, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			eventType, _ := event["type"].(string)

			// Check output_item.added event
			if eventType == "response.output_item.added" {
				item, _ := event["item"].(map[string]interface{})
				if item != nil {
					itemType, _ := item["type"].(string)
					if itemType == "function_call" {
						id, _ := item["id"].(string)
						name, _ := item["name"].(string)
						callID, _ := item["call_id"].(string)
						itemID, _ := event["item_id"].(string)

						t.Logf("output_item.added: id=%s, name=%s, call_id=%s, item_id=%s", id, name, callID, itemID)

						if id == "" {
							t.Errorf("BUG: output_item.added has empty 'id' field")
						}
						if name == "" {
							t.Errorf("BUG: output_item.added has empty 'name' field")
						}
						if callID == "" {
							t.Errorf("BUG: output_item.added has empty 'call_id' field")
						}
						if itemID == "" {
							t.Errorf("BUG: output_item.added has empty 'item_id' field")
						}
					}
				}
			}

			// Check function_call_arguments.delta event
			if eventType == "response.function_call_arguments.delta" {
				itemID, _ := event["item_id"].(string)
				callID, _ := event["call_id"].(string)
				delta, _ := event["delta"].(string)

				t.Logf("function_call_arguments.delta: item_id=%s, call_id=%s, delta=%s", itemID, callID, delta)

				if itemID == "" {
					t.Errorf("BUG: function_call_arguments.delta has empty 'item_id' field")
				}
				if callID == "" {
					t.Errorf("BUG: function_call_arguments.delta has empty 'call_id' field")
				}
			}

			// Check output_item.done event
			if eventType == "response.output_item.done" {
				item, _ := event["item"].(map[string]interface{})
				if item != nil {
					itemType, _ := item["type"].(string)
					if itemType == "function_call" {
						id, _ := item["id"].(string)
						name, _ := item["name"].(string)
						callID, _ := item["call_id"].(string)
						arguments, _ := item["arguments"].(string)
						itemID, _ := event["item_id"].(string)

						t.Logf("output_item.done: id=%s, name=%s, call_id=%s, arguments=%s, item_id=%s", id, name, callID, arguments, itemID)

						if id == "" {
							t.Errorf("BUG: output_item.done has empty 'id' field")
						}
						if name == "" {
							t.Errorf("BUG: output_item.done has empty 'name' field")
						}
						if callID == "" {
							t.Errorf("BUG: output_item.done has empty 'call_id' field")
						}
						if arguments == "" {
							t.Errorf("BUG: output_item.done has empty 'arguments' field")
						}
						if itemID == "" {
							t.Errorf("BUG: output_item.done has empty 'item_id' field")
						}
					}
				}
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// reproduce tests (merged from anthropic_to_responses_reproduce_test.go)
// ─────────────────────────────────────────────────────────────────────────────

// TestReproduceIssueFromCapturedLogs reproduces the bug from the captured logs
// where tool calls extracted from thinking content have empty id/name in output_item.done
func TestReproduceIssueFromCapturedLogs(t *testing.T) {
	var buf bytes.Buffer
	transformer := NewResponsesTransformer(&buf)

	// Exact sequence from captured logs: function.exec_command:2
	// The upstream sends: "function" ".exec" "_command" ":" "2"
	// Which gets concatenated to "function.exec_command:2" before ArgBegin
	upstreamEvents := []string{
		`event: message_start
data: {"message":{"model":"kimi-k2.5","id":"msg_13c8256b-60de-4eee-a2a2-e0e17d16e1e2","role":"assistant","type":"message","content":[],"usage":{"input_tokens":5942,"output_tokens":0}},"type":"message_start"}`,
		`event: ping
data: {"type":"ping"}`,
		`event: content_block_start
data: {"type":"content_block_start","content_block":{"type":"thinking","signature":"","thinking":""},"index":0}`,
		`event: content_block_delta
data: {"delta":{"type":"thinking_delta","thinking":"<|tool_calls_section_begin|>"},"type":"content_block_delta","index":0}`,
		`event: content_block_delta
data: {"delta":{"type":"thinking_delta","thinking":"<|tool_call_begin|>"},"type":"content_block_delta","index":0}`,
		`event: content_block_delta
data: {"delta":{"type":"thinking_delta","thinking":"function"},"type":"content_block_delta","index":0}`,
		`event: content_block_delta
data: {"delta":{"type":"thinking_delta","thinking":".exec"},"type":"content_block_delta","index":0}`,
		`event: content_block_delta
data: {"delta":{"type":"thinking_delta","thinking":"_command"},"type":"content_block_delta","index":0}`,
		`event: content_block_delta
data: {"delta":{"type":"thinking_delta","thinking":":"},"type":"content_block_delta","index":0}`,
		`event: content_block_delta
data: {"delta":{"type":"thinking_delta","thinking":"2"},"type":"content_block_delta","index":0}`,
		`event: content_block_delta
data: {"delta":{"type":"thinking_delta","thinking":"<|tool_call_argument_begin|>"},"type":"content_block_delta","index":0}`,
		`event: content_block_delta
data: {"delta":{"type":"thinking_delta","thinking":"{\"cmd\": \"ls -d /usr/include/*/ 2>/dev/null | head -30\", \"max_output_tokens\": 1000}"},"type":"content_block_delta","index":0}`,
		`event: content_block_delta
data: {"delta":{"type":"thinking_delta","thinking":"<|tool_call_end|>"},"type":"content_block_delta","index":0}`,
		`event: content_block_delta
data: {"delta":{"type":"thinking_delta","thinking":"<|tool_calls_section_end|>"},"type":"content_block_delta","index":0}`,
		`event: content_block_stop
data: {"type":"content_block_stop","index":0}`,
		`event: message_delta
data: {"delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":100}}`,
		`event: message_stop
data: {"type":"message_stop"}`,
	}

	// Process events using handleEvent (internal method)
	for _, eventStr := range upstreamEvents {
		// Parse the SSE event
		lines := strings.Split(eventStr, "\n")
		var eventType, dataStr string
		for _, line := range lines {
			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				dataStr = strings.TrimPrefix(line, "data: ")
			}
		}

		var event types.Event
		if dataStr != "" {
			if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
				t.Fatalf("Failed to parse event: %v", err)
			}
		}
		event.Type = eventType

		if err := transformer.handleEvent(event); err != nil {
			t.Fatalf("handleEvent error: %v", err)
		}
	}

	output := buf.String()
	t.Logf("Output:\n%s", output)

	// Parse output and check for the bug
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			eventType, _ := event["type"].(string)
			t.Logf("Event: %s", eventType)

			if eventType == "response.output_item.done" {
				item, _ := event["item"].(map[string]interface{})
				if item == nil {
					continue
				}
				itemType, _ := item["type"].(string)
				t.Logf("  item type: %s", itemType)
				if itemType == "function_call" {
					name, _ := item["name"].(string)
					id, _ := item["id"].(string)
					callID, _ := item["call_id"].(string)
					itemID, _ := event["item_id"].(string)

					t.Logf("  function_call: name=%q, id=%q, call_id=%q, item_id=%q", name, id, callID, itemID)

					if name == "" {
						t.Errorf("BUG: function_call output_item.done has empty 'name' field")
					}
					if id == "" {
						t.Errorf("BUG: function_call output_item.done has empty 'id' field")
					}
					if callID == "" {
						t.Errorf("BUG: function_call output_item.done has empty 'call_id' field")
					}
					if itemID == "" {
						t.Errorf("BUG: function_call output_item.done has empty 'item_id' field")
					}

					// The function name should be exec_command (stripped of module prefix)
					if name != "" && name != "exec_command" {
						t.Errorf("Expected function name 'exec_command', got '%s'", name)
					}
				}
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// test helpers tests (merged from anthropic_to_responses_test_helpers_test.go)
// ─────────────────────────────────────────────────────────────────────────────

// intPtr creates a pointer to an int value.
// Used in tests to create pointer fields for API types.
func intPtr(i int) *int {
	return &i
}
