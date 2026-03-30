package transform

import (
	"testing"
)

// TestEventType_String verifies the String method for all EventType values.
//
// @brief Tests EventType.String() returns correct names.
func TestEventType_String(t *testing.T) {
	tests := []struct {
		name     string
		eventType EventType
		want     string
	}{
		{"OpenAIChunk", EventOpenAIChunk, "OpenAIChunk"},
		{"AnthropicEvent", EventAnthropicEvent, "AnthropicEvent"},
		{"SSE", EventSSE, "SSE"},
		{"Done", EventDone, "Done"},
		{"Unknown", EventType(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.eventType.String(); got != tt.want {
				t.Errorf("EventType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPipelineEvent_Type verifies the PipelineEvent struct fields.
//
// @brief Tests PipelineEvent construction and field access.
func TestPipelineEvent_Type(t *testing.T) {
	tests := []struct {
		name    string
		event   PipelineEvent
		wantType EventType
		wantData string
		wantSSE  string
	}{
		{
			name:    "OpenAIChunk with data",
			event:   PipelineEvent{Type: EventOpenAIChunk, Data: []byte(`{"id":"chatcmpl-123"}`)},
			wantType: EventOpenAIChunk,
			wantData: `{"id":"chatcmpl-123"}`,
		},
		{
			name:    "AnthropicEvent with SSE type",
			event:   PipelineEvent{Type: EventAnthropicEvent, Data: []byte(`{"type":"message_start"}`), SSEType: "message_start"},
			wantType: EventAnthropicEvent,
			wantData: `{"type":"message_start"}`,
			wantSSE:  "message_start",
		},
		{
			name:    "SSE event",
			event:   PipelineEvent{Type: EventSSE, Data: []byte("hello"), SSEType: "ping"},
			wantType: EventSSE,
			wantData: "hello",
			wantSSE:  "ping",
		},
		{
			name:     "Done event",
			event:    PipelineEvent{Type: EventDone},
			wantType: EventDone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.event.Type != tt.wantType {
				t.Errorf("PipelineEvent.Type = %v, want %v", tt.event.Type, tt.wantType)
			}
			if tt.wantData != "" && string(tt.event.Data) != tt.wantData {
				t.Errorf("PipelineEvent.Data = %q, want %q", string(tt.event.Data), tt.wantData)
			}
			if tt.event.SSEType != tt.wantSSE {
				t.Errorf("PipelineEvent.SSEType = %q, want %q", tt.event.SSEType, tt.wantSSE)
			}
		})
	}
}
