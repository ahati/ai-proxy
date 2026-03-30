package transform

import (
	"github.com/tmaxmax/go-sse"
)

// WriterToSSEAdapter wraps an SSETransformer and provides an io.Writer interface.
// It parses SSE-formatted text and converts it back to *sse.Event objects for the base transformer.
//
// @brief Adapter that converts SSE text output back to SSETransformer events.
//
// @note This adapter enables the chain pattern for output interception:
//
//	Writer (SSE text) → WriterToSSEAdapter → SSETransformer (event-based)
//
// @note The input is expected to be SSE-formatted: "event: type\ndata: {...}\n\n" or "data: {...}\n\n"
// The adapter parses the text and calls base.Transform(&sse.Event{...}).
type WriterToSSEAdapter struct {
	base   SSETransformer
	buffer []byte
}

// NewWriterToSSEAdapter creates an adapter that converts SSE text to events.
//
// @brief Creates a new WriterToSSEAdapter wrapping the given transformer.
//
// @param base The SSETransformer to send parsed events to.
//
// @return *WriterToSSEAdapter A new adapter instance.
//
// @pre base must not be nil.
// @post The adapter is ready to receive SSE-formatted text.
func NewWriterToSSEAdapter(base SSETransformer) *WriterToSSEAdapter {
	return &WriterToSSEAdapter{
		base: base,
	}
}

// Write parses SSE-formatted text and sends events to the base transformer.
//
// @brief Implements io.Writer by parsing SSE text and calling base.Transform.
//
// @param p SSE-formatted text, typically "event: type\ndata: {...}\n\n".
//
// @return int The number of bytes accepted (always len(p)).
// @return error Returns error if parsing or base.Transform fails.
//
// @note This method handles:
//   - "event: type\ndata: {...}\n\n" → creates event with type and data
//   - "data: {...}\n\n" → creates event with empty type and data
//   - Buffers incomplete events until \n\n is received
func (a *WriterToSSEAdapter) Write(p []byte) (int, error) {
	a.buffer = append(a.buffer, p...)

	for {
		// Look for complete SSE event (ends with \n\n)
		idx := findSSEEnd(a.buffer)
		if idx == -1 {
			break
		}

		eventBytes := a.buffer[:idx]
		a.buffer = a.buffer[idx:]

		// Parse the SSE event
		eventType, jsonData := ExtractAnthropicEventFromSSE(eventBytes)
		if jsonData != "" {
			event := &sse.Event{
				Type: eventType,
				Data: jsonData,
			}
			if err := a.base.Transform(event); err != nil {
				return len(p), err
			}
		}
	}

	return len(p), nil
}

// Flush flushes the underlying transformer.
//
// @brief Flushes any buffered data in the base transformer.
func (a *WriterToSSEAdapter) Flush() error {
	return a.base.Flush()
}

// Close closes the underlying transformer.
//
// @brief Closes the base transformer and releases resources.
func (a *WriterToSSEAdapter) Close() error {
	return a.base.Close()
}
