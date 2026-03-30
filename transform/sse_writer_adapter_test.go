package transform

import (
	"bytes"
	"testing"

	"github.com/tmaxmax/go-sse"
)

func TestWriterToSSEAdapter_EventWithType(t *testing.T) {
	var receivedEvents []*sse.Event
	mockTransformer := &mockEventCollector{events: &receivedEvents}
	adapter := NewWriterToSSEAdapter(mockTransformer)

	sseText := "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0}\n\n"
	n, err := adapter.Write([]byte(sseText))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(sseText) {
		t.Errorf("Write returned %d, expected %d", n, len(sseText))
	}

	if len(receivedEvents) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(receivedEvents))
	}

	if receivedEvents[0].Type != "content_block_start" {
		t.Errorf("Event type = %q, expected %q", receivedEvents[0].Type, "content_block_start")
	}
	if receivedEvents[0].Data != "{\"type\":\"content_block_start\",\"index\":0}" {
		t.Errorf("Event data = %q, expected %q", receivedEvents[0].Data, "{\"type\":\"content_block_start\",\"index\":0}")
	}
}

func TestWriterToSSEAdapter_EventWithoutType(t *testing.T) {
	var receivedEvents []*sse.Event
	mockTransformer := &mockEventCollector{events: &receivedEvents}
	adapter := NewWriterToSSEAdapter(mockTransformer)

	sseText := "data: {\"type\":\"message_start\"}\n\n"
	n, err := adapter.Write([]byte(sseText))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(sseText) {
		t.Errorf("Write returned %d, expected %d", n, len(sseText))
	}

	if len(receivedEvents) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(receivedEvents))
	}

	if receivedEvents[0].Type != "" {
		t.Errorf("Event type = %q, expected empty", receivedEvents[0].Type)
	}
	if receivedEvents[0].Data != "{\"type\":\"message_start\"}" {
		t.Errorf("Event data = %q, expected %q", receivedEvents[0].Data, "{\"type\":\"message_start\"}")
	}
}

func TestWriterToSSEAdapter_MultipleEvents(t *testing.T) {
	var receivedEvents []*sse.Event
	mockTransformer := &mockEventCollector{events: &receivedEvents}
	adapter := NewWriterToSSEAdapter(mockTransformer)

	sseText := "event: message_start\ndata: {}\n\ndata: {\"type\":\"content_block_start\"}\n\n"
	n, err := adapter.Write([]byte(sseText))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(sseText) {
		t.Errorf("Write returned %d, expected %d", n, len(sseText))
	}

	if len(receivedEvents) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(receivedEvents))
	}

	if receivedEvents[0].Type != "message_start" {
		t.Errorf("First event type = %q, expected %q", receivedEvents[0].Type, "message_start")
	}
	if receivedEvents[1].Type != "" {
		t.Errorf("Second event type = %q, expected empty", receivedEvents[1].Type)
	}
}

func TestWriterToSSEAdapter_Buffering(t *testing.T) {
	var receivedEvents []*sse.Event
	mockTransformer := &mockEventCollector{events: &receivedEvents}
	adapter := NewWriterToSSEAdapter(mockTransformer)

	part1 := "event: test\ndata: {"
	n, err := adapter.Write([]byte(part1))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(part1) {
		t.Errorf("Write returned %d, expected %d", n, len(part1))
	}
	if len(receivedEvents) != 0 {
		t.Errorf("Expected 0 events before terminator, got %d", len(receivedEvents))
	}

	part2 := "}\n\n"
	n, err = adapter.Write([]byte(part2))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(part2) {
		t.Errorf("Write returned %d, expected %d", n, len(part2))
	}

	if len(receivedEvents) != 1 {
		t.Fatalf("Expected 1 event after terminator, got %d", len(receivedEvents))
	}
}

func TestWriterToSSEAdapter_FullAnthropicChain(t *testing.T) {
	buf := &bytes.Buffer{}
	passthrough := NewPassthroughTransformer(buf)
	adapter := NewWriterToSSEAdapter(passthrough)

	events := []string{
		"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_123\"}}\n\n",
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n",
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n",
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
	}

	for _, evt := range events {
		_, err := adapter.Write([]byte(evt))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	for _, expected := range events {
		if !bytes.Contains(buf.Bytes(), []byte(expected)) {
			t.Errorf("Output missing expected event: %q", expected)
		}
	}
}

func TestWriterToSSEAdapter_FlushAndClose(t *testing.T) {
	mockTransformer := &mockEventCollector{}
	adapter := NewWriterToSSEAdapter(mockTransformer)

	if err := adapter.Flush(); err != nil {
		t.Errorf("Flush failed: %v", err)
	}
	if err := adapter.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

type mockEventCollector struct {
	events *[]*sse.Event
}

func (m *mockEventCollector) Initialize() error { return nil }
func (m *mockEventCollector) Transform(event *sse.Event) error {
	if m.events != nil {
		*m.events = append(*m.events, event)
	}
	return nil
}
func (m *mockEventCollector) Flush() error        { return nil }
func (m *mockEventCollector) Close() error        { return nil }
func (m *mockEventCollector) HandleCancel() error { return nil }
