// Package transform provides interfaces for transforming SSE events.
// This package defines the core abstractions for Server-Sent Events (SSE)
// transformation in the AI proxy system. Implementations process streaming
// data from LLM APIs and convert it to client-compatible formats.
package transform

import (
	"io"

	"github.com/tmaxmax/go-sse"
)

// ResponseIDGetter is an optional interface for transformers that can provide
// their response ID. This is used for stream cancellation registration.
type ResponseIDGetter interface {
	GetResponseID() string
}

// SSETransformer defines the interface for transforming server-sent events.
// Implementations process SSE events and write transformed output.
//
// @brief Interface for transforming server-sent events from upstream LLM APIs.
//
// @note Implementations must be safe for concurrent use if handlers are shared
//
//	across multiple requests. Each request should use its own transformer
//	instance unless explicitly documented as thread-safe.
//
// @note The transformation pipeline follows: Initialize -> Transform (multiple calls) -> Flush -> Close.
//
//	Callers must ensure Close() is called to release resources.
//
// @pre The output writer must be properly initialized before creating implementations.
// @post After Close(), the transformer must not be used for further transformations.
type SSETransformer interface {
	// Initialize prepares the transformer and emits initial events before upstream request.
	//
	// @brief Initializes the transformer and emits response.created before upstream call.
	//
	// @return error Returns nil on success.
	//               Returns error if initial event emission fails.
	//
	// @pre The transformer must not be initialized yet.
	// @post Initial events (response.created, response.in_progress) are emitted.
	//
	// @note Must be called BEFORE making the upstream API request to ensure
	//       response.created is sent before any upstream response arrives.
	Initialize() error

	// HandleCancel processes a cancellation request and emits response.cancelled.
	//
	// @brief Handles cancellation by flushing buffered content and emitting cancelled event.
	//
	// @return error Returns nil on success.
	//               Returns error if event emission fails.
	//
	// @pre The transformer must be initialized.
	// @post Buffered content is flushed and response.cancelled is emitted.
	//
	// @note Must be called when the client requests cancellation.
	//       Any buffered reasoning or tool calls should be flushed before emitting cancelled.
	HandleCancel() error

	// Transform processes a single SSE event.
	//
	// @brief Processes a single SSE event and writes transformed output.
	//
	// @param event The SSE event to process. Must not be nil.
	//              The event may contain data in various formats depending on
	//              the upstream API (OpenAI, Anthropic, proprietary formats).
	//
	// @return error Returns nil on success.
	//               Returns error if:
	//               - Event data cannot be parsed
	//               - Output write fails
	//               - Internal state is corrupted
	//
	// @pre event must not be nil.
	// @pre The transformer must not be in a closed state.
	// @post Output is written to the configured writer.
	// @post Internal parser state is updated for subsequent events.
	//
	// @note Implementations may buffer partial data until complete tokens
	//       are recognized. Callers should invoke Flush() after the last event.
	Transform(event *sse.Event) error

	// Flush writes any buffered data to the output.
	//
	// @brief Flushes all buffered transformation output to the underlying writer.
	//
	// @return error Returns nil on success.
	//               Returns error if:
	//               - Output write fails
	//               - Buffered data is malformed
	//
	// @pre Transform() must have been called for events requiring processing.
	// @pre The transformer must not be in a closed state.
	// @post All buffered data is written to output.
	// @post Internal buffers are cleared.
	//
	// @note Must be called after the last Transform() call to ensure all
	//       pending output is written. Not calling Flush() may result in
	//       truncated output at the client.
	Flush() error

	// Close flushes and releases resources.
	//
	// @brief Releases all resources held by the transformer.
	//
	// @return error Returns nil on success.
	//               Returns error if:
	//               - Flush() fails during close
	//               - Resources cannot be released cleanly
	//
	// @pre None (safe to call even if never used).
	// @post The transformer is in a closed state and must not be used.
	// @post All resources are released.
	//
	// @note Close() calls Flush() internally before releasing resources.
	//       After Close(), subsequent calls to Transform(), Flush(), or Close()
	//       may panic or return errors depending on implementation.
	Close() error
}

// FlushingWriter combines io.Writer with a Flush method for buffered output.
// This interface is used to ensure output destinations can be explicitly flushed.
//
// @brief Interface combining io.Writer with explicit flush capability.
//
// @note Implementations typically wrap buffered writers (e.g., bufio.Writer)
//
//	to provide explicit control over when data is written to the underlying
//	destination. This is critical for streaming responses where data must
//	be sent immediately rather than buffered.
//
// @pre The underlying writer must be properly initialized and not closed.
// @post After Flush(), all buffered data is written to the underlying destination.
type FlushingWriter interface {
	io.Writer

	// Flush writes buffered data to the underlying writer.
	//
	// @brief Flushes all buffered data to the underlying writer.
	//
	// @return error Returns nil on success.
	//               Returns error if:
	//               - Underlying write fails
	//               - Writer is closed
	//
	// @pre The writer must be open and accepting writes.
	// @post All buffered data is written to the underlying destination.
	//
	// @note For HTTP response writers, Flush() ensures data is sent to the
	//       client immediately rather than held in OS buffers. This is
	//       essential for real-time streaming of LLM responses.
	Flush() error
}

// OpenAIChatReceiver receives OpenAI Chat Completion chunk JSON strings.
// It enables chaining transformers with converters without intermediate SSE serialization.
//
// @brief Interface for receiving OpenAI Chat Completion chunks as JSON strings.
//
// @note This interface enables the chain pattern:
//
//	Transformer (parses upstream, extracts tool calls) → OpenAIChatReceiver (converts to target format)
//
// @note The chunkJSON is the raw JSON of a types.Chunk, WITHOUT SSE framing (no "data: " prefix).
//
// @pre The receiver must be properly initialized.
// @post After ReceiveDone(), the receiver should flush and finalize output.
type OpenAIChatReceiver interface {
	// Receive processes a single OpenAI Chat Completion chunk.
	//
	// @brief Processes a Chat Completion chunk JSON string.
	//
	// @param chunkJSON The raw JSON of a types.Chunk (without SSE framing).
	//                  Format: {"id":"...","object":"chat.completion.chunk",...}
	//                  Must be valid JSON.
	//
	// @return error Returns nil on success.
	//               Returns error if:
	//               - JSON is malformed
	//               - Output write fails
	//
	// @pre chunkJSON must be valid OpenAI Chunk JSON.
	// @post The chunk is converted and written to the target format.
	Receive(chunkJSON string) error

	// ReceiveDone signals the end of the stream.
	//
	// @brief Signals stream termination, triggering final events/cleanup.
	//
	// @return error Returns nil on success.
	//               Returns error if final event emission fails.
	//
	// @pre All chunks have been received via Receive().
	// @post Final events (message_stop, message_delta, etc.) are emitted.
	ReceiveDone() error

	// Flush writes any buffered data.
	//
	// @brief Flushes buffered content to the underlying writer.
	//
	// @return error Returns nil on success.
	//
	// @pre The receiver is not closed.
	// @post All buffered data is written.
	Flush() error
}

// AnthropicEventReceiver receives Anthropic SSE event JSON strings.
// It enables chaining transformers with converters for Anthropic-format streams.
//
// @brief Interface for receiving Anthropic SSE events as JSON strings.
//
// @note This interface enables the chain pattern for Anthropic→other conversions.
//
// @note The eventJSON is the raw JSON of a types.Event, WITHOUT SSE framing.
type AnthropicEventReceiver interface {
	// Receive processes a single Anthropic SSE event.
	//
	// @brief Processes an Anthropic event JSON string.
	//
	// @param eventJSON The raw JSON of a types.Event (without SSE framing).
	//                  Format: {"type":"message_start",...} or {"type":"content_block_delta",...}
	//                  Must be valid JSON.
	//
	// @return error Returns nil on success.
	//               Returns error if JSON is malformed or output write fails.
	//
	// @pre eventJSON must be valid Anthropic Event JSON.
	// @post The event is converted and written to the target format.
	Receive(eventJSON string) error

	// ReceiveDone signals the end of the stream.
	//
	// @brief Signals stream termination, triggering final events/cleanup.
	//
	// @return error Returns nil on success.
	ReceiveDone() error

	// Flush writes any buffered data.
	//
	// @brief Flushes buffered content to the underlying writer.
	Flush() error
}
