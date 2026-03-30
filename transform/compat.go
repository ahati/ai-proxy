package transform

import (
	"fmt"

	"github.com/tmaxmax/go-sse"
)

// SSETransformerFromStage wraps a Stage as an SSETransformer.
// Used during migration so handlers can still return SSETransformer
// while the pipeline internally uses Stage.
//
// @brief Adapter that wraps Stage as an SSETransformer.
//
// @note HandleCancel delegates to Flush (best-effort mapping).
func SSETransformerFromStage(s Stage) SSETransformer {
	return &stageAsSSETransformer{inner: s}
}

// stageAsSSETransformer wraps a Stage as an SSETransformer.
type stageAsSSETransformer struct {
	inner Stage
}

// Initialize delegates to the inner stage's Initialize.
func (a *stageAsSSETransformer) Initialize() error {
	return a.inner.Initialize()
}

// HandleCancel flushes the inner stage as a best-effort cancellation response.
func (a *stageAsSSETransformer) HandleCancel() error {
	return a.inner.Flush()
}

// Transform converts the *sse.Event to a PipelineEvent and delegates to Process.
//
// @brief Implements SSETransformer.Transform by converting to PipelineEvent.
func (a *stageAsSSETransformer) Transform(event *sse.Event) error {
	if event == nil {
		return nil
	}

	// Map SSE event to PipelineEvent
	var pe PipelineEvent

	if event.Data == "[DONE]" {
		pe = PipelineEvent{Type: EventDone}
	} else if event.Type != "" {
		// Has event type → Anthropic-style event
		pe = PipelineEvent{
			Type:    EventAnthropicEvent,
			Data:    []byte(event.Data),
			SSEType: event.Type,
		}
	} else {
		// No event type → OpenAI-style or raw SSE
		pe = PipelineEvent{
			Type: EventOpenAIChunk,
			Data: []byte(event.Data),
		}
	}

	if err := a.inner.Process(pe); err != nil {
		return fmt.Errorf("stage adapter: %w", err)
	}
	return nil
}

// Flush delegates to the inner stage's Flush.
func (a *stageAsSSETransformer) Flush() error {
	return a.inner.Flush()
}

// Close delegates to the inner stage's Close.
func (a *stageAsSSETransformer) Close() error {
	return a.inner.Close()
}
