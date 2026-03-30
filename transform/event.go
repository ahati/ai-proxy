package transform

// EventType identifies the kind of data carried by a PipelineEvent.
// It replaces the separate output interfaces (OpenAIChatReceiver,
// AnthropicEventReceiver, SSETransformer) with a single discriminated type.
//
// @brief Discriminated event type for pipeline stage communication.
//
// @note Each EventType corresponds to a specific data format:
//
//   - EventOpenAIChunk: JSON-serialized types.Chunk (OpenAI Chat Completion chunk)
//   - EventAnthropicEvent: JSON-serialized types.Event (Anthropic SSE event)
//   - EventSSE: Raw SSE event with optional type (passthrough)
//   - EventDone: Stream termination signal
type EventType int

const (
	// EventOpenAIChunk carries a JSON-serialized OpenAI ChatCompletion chunk.
	// Data field contains the raw JSON of a types.Chunk (no SSE framing).
	EventOpenAIChunk EventType = iota

	// EventAnthropicEvent carries a JSON-serialized Anthropic SSE event.
	// Data field contains the raw JSON of a types.Event (no SSE framing).
	EventAnthropicEvent

	// EventSSE carries a raw SSE event for passthrough.
	// SSEType field contains the event type (e.g., "message_start").
	// Data field contains the event payload.
	EventSSE

	// EventDone signals stream completion.
	// No data is carried; this triggers finalization in stages.
	EventDone
)

// String returns a human-readable name for the EventType.
//
// @brief Returns the name of the EventType for logging and debugging.
//
// @return string representation of the EventType.
func (t EventType) String() string {
	switch t {
	case EventOpenAIChunk:
		return "OpenAIChunk"
	case EventAnthropicEvent:
		return "AnthropicEvent"
	case EventSSE:
		return "SSE"
	case EventDone:
		return "Done"
	default:
		return "Unknown"
	}
}

// PipelineEvent is the unified internal event passed between pipeline stages.
// It carries typed data, avoiding SSE text serialization round-trips when
// chaining transformers and converters.
//
// @brief Unified event type for inter-stage communication in pipelines.
//
// @note PipelineEvent eliminates the need for separate receiver interfaces.
// Instead of calling receiver.Receive(chunkJSON), stages emit:
//
//	PipelineEvent{Type: EventOpenAIChunk, Data: []byte(chunkJSON)}
//
// @note The Data field is raw JSON without SSE framing (no "data: " prefix).
// Stages that write to an io.Writer add SSE framing in their output stage.
type PipelineEvent struct {
	// Type identifies the event format carried by Data.
	Type EventType

	// Data contains the raw JSON payload (no SSE framing).
	// Interpretation depends on Type:
	//   - EventOpenAIChunk: types.Chunk JSON
	//   - EventAnthropicEvent: types.Event JSON
	//   - EventSSE: raw SSE data payload
	//   - EventDone: nil
	Data []byte

	// SSEType is the SSE event type, used only for EventSSE.
	// Examples: "message_start", "content_block_delta", "" (for data-only events).
	SSEType string
}
