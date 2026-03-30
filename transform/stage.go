package transform

// Stage is the unified interface for all pipeline stages.
// Every transformer and converter implements this single interface,
// replacing the separate SSETransformer, OpenAIChatReceiver, and
// AnthropicEventReceiver interfaces with one common shape.
//
// @brief Unified interface for all transformer and converter stages.
//
// @note The Stage interface follows the same lifecycle as SSETransformer:
//
//	Initialize -> Process (multiple calls) -> Flush -> Close
//
// @note Implementations must be safe for single-goroutine use per instance.
//
//	Each request should use its own Stage instance.
type Stage interface {
	// Process handles a single pipeline event.
	//
	// @brief Processes a PipelineEvent and optionally emits events to the next stage.
	//
	// @param event The pipeline event to process. Must not be zero-value.
	//
	// @return error Returns nil on success.
	//               Returns error if:
	//               - Event data cannot be parsed
	//               - Output write fails
	//               - Internal state is corrupted
	//
	// @pre The stage must be initialized (Initialize called).
	// @post The event is consumed, transformed, or passed through.
	//
	// @note Implementations should handle all EventType values they expect
	//       and return nil for unknown types (passthrough).
	Process(event PipelineEvent) error

	// Initialize prepares the stage before the upstream request.
	//
	// @brief Initializes the stage and optionally emits initial events.
	//
	// @return error Returns nil on success.
	//
	// @note Must be called BEFORE any Process calls.
	Initialize() error

	// Flush writes any buffered data to the output.
	//
	// @brief Flushes all buffered transformation output.
	//
	// @return error Returns nil on success.
	//
	// @note Must be called after the last Process() call.
	Flush() error

	// Close releases resources and writes final output.
	//
	// @brief Releases all resources held by the stage.
	//
	// @return error Returns nil on success.
	//
	// @note Close() calls Flush() internally before releasing resources.
	Close() error
}
