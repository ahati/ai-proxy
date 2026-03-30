package transform

import (
	"io"

	"github.com/tmaxmax/go-sse"
)

// PassthroughTransformer is an SSE transformer that passes events through unchanged.
// It is used when the upstream already returns data in the desired format.
//
// @brief SSE transformer that passes events through without modification.
//
// @note Use this transformer when upstream format matches downstream format.
// @note This transformer is lightweight and has no internal state.
// @note The transformer is thread-safe for concurrent Transform calls.
//
// Use cases:
//   - Anthropic upstream → Anthropic downstream (no transformation needed)
//   - OpenAI upstream → OpenAI downstream (no transformation needed)
type PassthroughTransformer struct {
	// output is the destination writer for SSE data.
	output io.Writer
}

// NewPassthroughTransformer creates a transformer that passes events through unchanged.
//
// @brief Creates a new PassthroughTransformer.
//
// @param output The destination writer for SSE data.
//
//	Must be non-nil and writable.
//	Should support flushing for streaming responses.
//
// @return *PassthroughTransformer A new transformer ready for Transform calls.
//
// @pre output must be non-nil and writable.
// @post Transformer is ready to process SSE events.
//
// @note This is the simplest transformer - it just writes data as-is.
func NewPassthroughTransformer(output io.Writer) *PassthroughTransformer {
	return &PassthroughTransformer{output: output}
}

// Process handles a PipelineEvent by writing it as SSE to the output.
// This implements the Stage interface.
//
// @brief Implements Stage.Process by converting PipelineEvent to SSE text.
//
// @param event The pipeline event to process.
//
// @return error Returns nil on success, error if write fails.
func (t *PassthroughTransformer) Process(event PipelineEvent) error {
	switch event.Type {
	case EventDone:
		return nil
	case EventSSE, EventOpenAIChunk, EventAnthropicEvent:
		return t.writeSSE(event)
	default:
		return nil
	}
}

// writeSSE writes a PipelineEvent as SSE text to the output.
func (t *PassthroughTransformer) writeSSE(event PipelineEvent) error {
	if len(event.Data) == 0 && event.Type != EventSSE {
		return nil
	}

	var err error
	if event.SSEType != "" {
		_, err = t.output.Write([]byte("event: " + event.SSEType + "\n"))
		if err != nil {
			return err
		}
	}
	_, err = t.output.Write([]byte("data: " + string(event.Data) + "\n\n"))
	return err
}

// Transform writes the SSE event data to the output unchanged.
//
// @brief Passes the event data through without modification.
//
// @param event The SSE event to pass through.
//
//	Must not be nil.
//	Data field is written as-is.
//
// @return error Returns nil on success.
//
//	Returns error if output write fails.
//
// @pre event must not be nil.
// @pre output writer must be writable.
// @post Event data is written to output unchanged.
//
// @note The SSE format with both event type and data is maintained.
func (t *PassthroughTransformer) Transform(event *sse.Event) error {
	// Skip empty events
	if event.Data == "" {
		return nil
	}

	// Delegate to Process via PipelineEvent
	return t.Process(PipelineEvent{
		Type:    EventSSE,
		Data:    []byte(event.Data),
		SSEType: event.Type,
	})
}

// Flush is a no-op for passthrough transformer.
// There is no buffered content to flush.
//
// @brief No-op flush for passthrough transformer.
//
// @return Always returns nil.
//
// @note Included for interface compatibility.
func (t *PassthroughTransformer) Flush() error {
	return nil
}

// Close is a no-op for passthrough transformer.
// There are no resources to release.
//
// @brief No-op close for passthrough transformer.
//
// @return Always returns nil.
//
// @note Included for interface compatibility.
func (t *PassthroughTransformer) Close() error {
	return nil
}

// Initialize is a no-op for passthrough transformer.
// There is no initialization needed.
//
// @brief No-op initialization for passthrough transformer.
//
// @return Always returns nil.
//
// @note Included for interface compatibility.
func (t *PassthroughTransformer) Initialize() error {
	return nil
}

// HandleCancel handles cancellation requests.
// For PassthroughTransformer, this is a no-op as there is no buffered state.
//
// @brief No-op cancellation handler for passthrough transformer.
//
// @return Always returns nil.
//
// @note Included for interface compatibility.
func (t *PassthroughTransformer) HandleCancel() error {
	return nil
}
