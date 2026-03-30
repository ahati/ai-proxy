package transform

import (
	"errors"
	"testing"

	"github.com/tmaxmax/go-sse"
)

// compatMockStage is a test mock for Stage used in compat tests.
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
