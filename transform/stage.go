package transform

import (
	"fmt"
)

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
// @note Stages are chained via Pipeline. Each stage's output becomes the
//
//	next stage's input through direct PipelineEvent passing, avoiding
//	SSE text round-trips.
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

// Pipeline implements Stage by chaining multiple stages.
// Events flow through stages in order; the Pipeline itself is a Stage
// so pipelines can be nested.
//
// @brief Chains multiple Stage instances into a single Stage.
//
// @note Events are sent to the first stage. Each stage is responsible
//
//	for emitting transformed events to its downstream stage.
//	In practice, most stages write directly to an output sink
//	(the final stage in the chain).
//
// @note Pipeline implements Stage, enabling nested pipeline composition.
type Pipeline struct {
	stages []Stage
}

// NewPipeline creates a new Pipeline from the given stages.
//
// @brief Creates a Pipeline that chains the given stages.
//
// @param stages The stages to chain, in processing order.
//
// @return *Pipeline A new Pipeline instance.
//
// @pre stages must not be empty.
// @post The pipeline is ready for Initialize/Process/Flush/Close calls.
func NewPipeline(stages ...Stage) *Pipeline {
	return &Pipeline{stages: stages}
}

// Stages returns the slice of stages in this pipeline.
// Used for testing and introspection.
//
// @brief Returns the stages in this pipeline.
//
// @return []Stage The ordered stages.
func (p *Pipeline) Stages() []Stage {
	return p.stages
}

// Process sends the event through all stages in sequence.
//
// @brief Implements Stage.Process by forwarding to all stages.
//
// @param event The pipeline event to process.
//
// @return error Returns the first error encountered from any stage.
//
// @note Processing stops at the first error.
func (p *Pipeline) Process(event PipelineEvent) error {
	for _, stage := range p.stages {
		if err := stage.Process(event); err != nil {
			return fmt.Errorf("pipeline stage %T: %w", stage, err)
		}
	}
	return nil
}

// Initialize calls Initialize on all stages in order.
//
// @brief Implements Stage.Initialize.
//
// @return error Returns the first error from any stage.
func (p *Pipeline) Initialize() error {
	for _, stage := range p.stages {
		if err := stage.Initialize(); err != nil {
			return fmt.Errorf("pipeline stage %T: %w", stage, err)
		}
	}
	return nil
}

// Flush calls Flush on all stages in order.
//
// @brief Implements Stage.Flush.
//
// @return error Returns the first error from any stage.
func (p *Pipeline) Flush() error {
	for _, stage := range p.stages {
		if err := stage.Flush(); err != nil {
			return fmt.Errorf("pipeline stage %T: %w", stage, err)
		}
	}
	return nil
}

// Close calls Close on all stages in reverse order (last stage first).
// Reverse order ensures downstream stages are closed before upstream ones.
//
// @brief Implements Stage.Close.
//
// @return error Returns the first error from any stage.
func (p *Pipeline) Close() error {
	var firstErr error
	for i := len(p.stages) - 1; i >= 0; i-- {
		if err := p.stages[i].Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("pipeline stage %T: %w", p.stages[i], err)
		}
	}
	return firstErr
}
