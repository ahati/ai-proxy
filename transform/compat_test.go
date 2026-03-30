package transform

import (
	"errors"
	"strings"
	"testing"

	"github.com/tmaxmax/go-sse"
)

// compatMockSSE is a test mock for SSETransformer used only in compat tests.
// Named distinctly to avoid collision with mockSSETransformer in interface_test.go.
type compatMockSSE struct {
	transformed []*sse.Event
	initErr     error
	flushErr    error
	closeErr    error
}

func (m *compatMockSSE) Initialize() error { return m.initErr }

func (m *compatMockSSE) HandleCancel() error { return nil }

func (m *compatMockSSE) Transform(event *sse.Event) error {
	m.transformed = append(m.transformed, event)
	return nil
}

func (m *compatMockSSE) Flush() error { return m.flushErr }

func (m *compatMockSSE) Close() error { return m.closeErr }

// compatMockStage is a test mock for Stage used only in compat tests.
type compatMockStage struct {
	events   []PipelineEvent
	initErr  error
	flushErr error
	closeErr error
}

func (m *compatMockStage) Process(event PipelineEvent) error {
	m.events = append(m.events, event)
	return nil
}

func (m *compatMockStage) Initialize() error { return m.initErr }
func (m *compatMockStage) Flush() error      { return m.flushErr }
func (m *compatMockStage) Close() error      { return m.closeErr }

// --- StageFromSSETransformer tests ---

// TestStageFromSSETransformer_Process_Done verifies EventDone maps to Close.
//
// @brief Tests that EventDone triggers Close on the wrapped transformer.
func TestStageFromSSETransformer_Process_Done(t *testing.T) {
	mock := &compatMockSSE{}
	stage := StageFromSSETransformer(mock)

	err := stage.Process(PipelineEvent{Type: EventDone})
	if err != nil {
		t.Fatalf("Process(EventDone) error = %v", err)
	}
}

// TestStageFromSSETransformer_Process_OpenAIChunk verifies OpenAI chunk conversion.
//
// @brief Tests that EventOpenAIChunk maps to SSE data event.
func TestStageFromSSETransformer_Process_OpenAIChunk(t *testing.T) {
	mock := &compatMockSSE{}
	stage := StageFromSSETransformer(mock)

	data := []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk"}`)
	err := stage.Process(PipelineEvent{Type: EventOpenAIChunk, Data: data})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(mock.transformed) != 1 {
		t.Fatalf("expected 1 transformed event, got %d", len(mock.transformed))
	}
	if mock.transformed[0].Data != string(data) {
		t.Errorf("transformed data = %q, want %q", mock.transformed[0].Data, string(data))
	}
	if mock.transformed[0].Type != "" {
		t.Errorf("transformed type = %q, want empty (OpenAI has no event type)", mock.transformed[0].Type)
	}
}

// TestStageFromSSETransformer_Process_AnthropicEvent verifies Anthropic event conversion.
//
// @brief Tests that EventAnthropicEvent maps to SSE typed event.
func TestStageFromSSETransformer_Process_AnthropicEvent(t *testing.T) {
	mock := &compatMockSSE{}
	stage := StageFromSSETransformer(mock)

	data := []byte(`{"type":"message_start"}`)
	err := stage.Process(PipelineEvent{
		Type:    EventAnthropicEvent,
		Data:    data,
		SSEType: "message_start",
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(mock.transformed) != 1 {
		t.Fatalf("expected 1 transformed event, got %d", len(mock.transformed))
	}
	if mock.transformed[0].Type != "message_start" {
		t.Errorf("transformed type = %q, want %q", mock.transformed[0].Type, "message_start")
	}
	if mock.transformed[0].Data != string(data) {
		t.Errorf("transformed data = %q, want %q", mock.transformed[0].Data, string(data))
	}
}

// TestStageFromSSETransformer_Process_SSE verifies raw SSE passthrough.
//
// @brief Tests that EventSSE maps to SSE event with type.
func TestStageFromSSETransformer_Process_SSE(t *testing.T) {
	mock := &compatMockSSE{}
	stage := StageFromSSETransformer(mock)

	err := stage.Process(PipelineEvent{
		Type:    EventSSE,
		Data:    []byte("raw data"),
		SSEType: "ping",
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if len(mock.transformed) != 1 {
		t.Fatalf("expected 1 transformed event, got %d", len(mock.transformed))
	}
	if mock.transformed[0].Type != "ping" {
		t.Errorf("transformed type = %q, want %q", mock.transformed[0].Type, "ping")
	}
}

// TestStageFromSSETransformer_Initialize verifies Initialize delegation.
//
// @brief Tests that Initialize calls the inner transformer's Initialize.
func TestStageFromSSETransformer_Initialize(t *testing.T) {
	mock := &compatMockSSE{}
	stage := StageFromSSETransformer(mock)

	if err := stage.Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
}

// TestStageFromSSETransformer_Initialize_Error verifies error propagation.
//
// @brief Tests that Initialize propagates errors from inner transformer.
func TestStageFromSSETransformer_Initialize_Error(t *testing.T) {
	expected := errors.New("init error")
	mock := &compatMockSSE{initErr: expected}
	stage := StageFromSSETransformer(mock)

	err := stage.Initialize()
	if err == nil {
		t.Fatal("Initialize() should return error")
	}
	if err != expected {
		t.Errorf("Initialize() error = %v, want %v", err, expected)
	}
}

// TestStageFromSSETransformer_Flush verifies Flush delegation.
func TestStageFromSSETransformer_Flush(t *testing.T) {
	mock := &compatMockSSE{}
	stage := StageFromSSETransformer(mock)

	if err := stage.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
}

// TestStageFromSSETransformer_Flush_Error verifies Flush error propagation.
func TestStageFromSSETransformer_Flush_Error(t *testing.T) {
	mock := &compatMockSSE{flushErr: errors.New("flush error")}
	stage := StageFromSSETransformer(mock)

	err := stage.Flush()
	if err == nil {
		t.Fatal("Flush() should return error")
	}
}

// TestStageFromSSETransformer_Close verifies Close delegation.
func TestStageFromSSETransformer_Close(t *testing.T) {
	mock := &compatMockSSE{}
	stage := StageFromSSETransformer(mock)

	if err := stage.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

// TestStageFromSSETransformer_Close_Error verifies Close error propagation.
func TestStageFromSSETransformer_Close_Error(t *testing.T) {
	mock := &compatMockSSE{closeErr: errors.New("close error")}
	stage := StageFromSSETransformer(mock)

	err := stage.Close()
	if err == nil {
		t.Fatal("Close() should return error")
	}
}

// --- SSETransformerFromStage tests ---

// TestSSETransformerFromStage_Transform_DoneMarker verifies [DONE] detection.
//
// @brief Tests that Transform converts [DONE] data to EventDone.
func TestSSETransformerFromStage_Transform_DoneMarker(t *testing.T) {
	mock := &compatMockStage{}
	adapter := SSETransformerFromStage(mock)

	err := adapter.Transform(&sse.Event{Data: "[DONE]"})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}
	if mock.events[0].Type != EventDone {
		t.Errorf("event type = %v, want EventDone", mock.events[0].Type)
	}
}

// TestSSETransformerFromStage_Transform_OpenAIChunk verifies data-only SSE mapping.
//
// @brief Tests that data-only SSE events map to EventOpenAIChunk.
func TestSSETransformerFromStage_Transform_OpenAIChunk(t *testing.T) {
	mock := &compatMockStage{}
	adapter := SSETransformerFromStage(mock)

	data := `{"id":"chatcmpl-123"}`
	err := adapter.Transform(&sse.Event{Data: data})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}
	if mock.events[0].Type != EventOpenAIChunk {
		t.Errorf("event type = %v, want EventOpenAIChunk", mock.events[0].Type)
	}
	if string(mock.events[0].Data) != data {
		t.Errorf("event data = %q, want %q", string(mock.events[0].Data), data)
	}
}

// TestSSETransformerFromStage_Transform_AnthropicEvent verifies typed SSE mapping.
//
// @brief Tests that typed SSE events map to EventAnthropicEvent.
func TestSSETransformerFromStage_Transform_AnthropicEvent(t *testing.T) {
	mock := &compatMockStage{}
	adapter := SSETransformerFromStage(mock)

	data := `{"type":"content_block_delta"}`
	err := adapter.Transform(&sse.Event{Type: "content_block_delta", Data: data})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}
	if mock.events[0].Type != EventAnthropicEvent {
		t.Errorf("event type = %v, want EventAnthropicEvent", mock.events[0].Type)
	}
	if mock.events[0].SSEType != "content_block_delta" {
		t.Errorf("event SSEType = %q, want %q", mock.events[0].SSEType, "content_block_delta")
	}
}

// TestSSETransformerFromStage_Transform_NilEvent verifies nil event handling.
func TestSSETransformerFromStage_Transform_NilEvent(t *testing.T) {
	mock := &compatMockStage{}
	adapter := SSETransformerFromStage(mock)

	err := adapter.Transform(nil)
	if err != nil {
		t.Fatalf("Transform(nil) error = %v", err)
	}
	if len(mock.events) != 0 {
		t.Errorf("expected 0 events, got %d", len(mock.events))
	}
}

// TestSSETransformerFromStage_Initialize verifies Initialize delegation.
func TestSSETransformerFromStage_Initialize(t *testing.T) {
	mock := &compatMockStage{}
	adapter := SSETransformerFromStage(mock)

	if err := adapter.Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
}

// TestSSETransformerFromStage_Initialize_Error verifies error propagation.
func TestSSETransformerFromStage_Initialize_Error(t *testing.T) {
	expected := errors.New("init error")
	mock := &compatMockStage{initErr: expected}
	adapter := SSETransformerFromStage(mock)

	err := adapter.Initialize()
	if err == nil {
		t.Fatal("Initialize() should return error")
	}
}

// TestSSETransformerFromStage_HandleCancel verifies HandleCancel calls Flush.
func TestSSETransformerFromStage_HandleCancel(t *testing.T) {
	mock := &compatMockStage{}
	adapter := SSETransformerFromStage(mock)

	if err := adapter.HandleCancel(); err != nil {
		t.Fatalf("HandleCancel() error = %v", err)
	}
}

// TestSSETransformerFromStage_Flush verifies Flush delegation.
func TestSSETransformerFromStage_Flush(t *testing.T) {
	mock := &compatMockStage{}
	adapter := SSETransformerFromStage(mock)

	if err := adapter.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
}

// TestSSETransformerFromStage_Flush_Error verifies Flush error propagation.
func TestSSETransformerFromStage_Flush_Error(t *testing.T) {
	mock := &compatMockStage{flushErr: errors.New("flush error")}
	adapter := SSETransformerFromStage(mock)

	err := adapter.Flush()
	if err == nil {
		t.Fatal("Flush() should return error")
	}
}

// TestSSETransformerFromStage_Close verifies Close delegation.
func TestSSETransformerFromStage_Close(t *testing.T) {
	mock := &compatMockStage{}
	adapter := SSETransformerFromStage(mock)

	if err := adapter.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

// TestSSETransformerFromStage_Close_Error verifies Close error propagation.
func TestSSETransformerFromStage_Close_Error(t *testing.T) {
	mock := &compatMockStage{closeErr: errors.New("close error")}
	adapter := SSETransformerFromStage(mock)

	err := adapter.Close()
	if err == nil {
		t.Fatal("Close() should return error")
	}
}

// --- Round-trip tests ---

// TestCompat_RoundTrip verifies SSETransformer → Stage → SSETransformer round-trip.
//
// @brief Tests that wrapping through both adapters preserves events.
func TestCompat_RoundTrip(t *testing.T) {
	inner := &compatMockSSE{}
	stage := StageFromSSETransformer(inner)
	outer := SSETransformerFromStage(stage)

	data := `{"id":"test","object":"chat.completion.chunk"}`
	err := outer.Transform(&sse.Event{Data: data})
	if err != nil {
		t.Fatalf("round-trip Transform() error = %v", err)
	}

	if len(inner.transformed) != 1 {
		t.Fatalf("expected 1 event after round-trip, got %d", len(inner.transformed))
	}
	if inner.transformed[0].Data != data {
		t.Errorf("round-trip data = %q, want %q", inner.transformed[0].Data, data)
	}
}

// TestCompat_RoundTrip_Anthropic verifies round-trip with Anthropic events.
func TestCompat_RoundTrip_Anthropic(t *testing.T) {
	inner := &compatMockSSE{}
	stage := StageFromSSETransformer(inner)
	outer := SSETransformerFromStage(stage)

	err := outer.Transform(&sse.Event{
		Type: "message_start",
		Data: `{"type":"message_start","message":{"id":"msg_123"}}`,
	})
	if err != nil {
		t.Fatalf("round-trip Transform() error = %v", err)
	}

	if len(inner.transformed) != 1 {
		t.Fatalf("expected 1 event, got %d", len(inner.transformed))
	}
	if inner.transformed[0].Type != "message_start" {
		t.Errorf("round-trip type = %q, want %q", inner.transformed[0].Type, "message_start")
	}
}

// TestCompat_RoundTrip_Done verifies [DONE] marker triggers Close in round-trip.
func TestCompat_RoundTrip_Done(t *testing.T) {
	inner := &compatMockSSE{}
	stage := StageFromSSETransformer(inner)
	outer := SSETransformerFromStage(stage)

	err := outer.Transform(&sse.Event{Data: "[DONE]"})
	if err != nil {
		t.Fatalf("round-trip Transform([DONE]) error = %v", err)
	}
	// EventDone calls inner.Close(), not Transform — so no transformed events
	if len(inner.transformed) != 0 {
		t.Errorf("expected 0 transformed events (Close called), got %d", len(inner.transformed))
	}
}

// TestCompat_RoundTrip_Lifecycle verifies Initialize/Flush/Close through both adapters.
func TestCompat_RoundTrip_Lifecycle(t *testing.T) {
	inner := &compatMockSSE{}
	stage := StageFromSSETransformer(inner)
	outer := SSETransformerFromStage(stage)

	if err := outer.Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if err := outer.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if err := outer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

// TestCompat_PassthroughInPipeline verifies PassthroughTransformer via adapter in Pipeline.
//
// @brief Integration test: PassthroughTransformer wrapped via StageFromSSETransformer in a Pipeline.
func TestCompat_PassthroughInPipeline(t *testing.T) {
	var buf strings.Builder
	passthrough := NewPassthroughTransformer(&buf)
	stage := StageFromSSETransformer(passthrough)
	pipeline := NewPipeline(stage)

	if err := pipeline.Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	err := pipeline.Process(PipelineEvent{
		Type: EventSSE,
		Data: []byte(`{"id":"test"}`),
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if err := pipeline.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `{"id":"test"}`) {
		t.Errorf("output = %q, want to contain test data", output)
	}
}

// TestCompat_PipelineViaSSEAdapter verifies full Pipeline wrapped back as SSETransformer.
//
// @brief Integration test: Pipeline → SSETransformerFromStage → used as SSETransformer.
func TestCompat_PipelineViaSSEAdapter(t *testing.T) {
	var buf strings.Builder
	passthrough := NewPassthroughTransformer(&buf)
	stage := StageFromSSETransformer(passthrough)
	pipeline := NewPipeline(stage)
	adapter := SSETransformerFromStage(pipeline)

	if err := adapter.Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Send an OpenAI chunk as SSE event
	err := adapter.Transform(&sse.Event{Data: `{"id":"chatcmpl-1","object":"chat.completion.chunk"}`})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	if err := adapter.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	output := buf.String()
	// The SSETransformerFromStage converts to EventOpenAIChunk, then StageFromSSETransformer
	// converts back to an SSE event, and PassthroughTransformer writes it with SSE framing.
	if !strings.Contains(output, `"id":"chatcmpl-1"`) {
		t.Errorf("output = %q, want to contain chunk data", output)
	}
}
