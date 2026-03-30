package transform

import (
	"fmt"

	"github.com/tmaxmax/go-sse"
)

// StageFromSSETransformer wraps an existing SSETransformer as a Stage.
// This enables gradual migration: old transformers work in the new pipeline
// without modification.
//
// @brief Adapter that wraps SSETransformer as a Stage.
//
// @note The adapter converts PipelineEvents to *sse.Event before calling
//
//	Transform. This preserves full backward compatibility.
//
// @note Initialize, Flush, Close, and HandleCancel are delegated directly.
func StageFromSSETransformer(t SSETransformer) Stage {
	return &sseTransformerAdapter{inner: t}
}

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

// sseTransformerAdapter wraps SSETransformer as a Stage.
type sseTransformerAdapter struct {
	inner SSETransformer
}

// Process converts the PipelineEvent to *sse.Event and delegates to the inner transformer.
//
// @brief Implements Stage.Process by converting events and delegating.
func (a *sseTransformerAdapter) Process(event PipelineEvent) error {
	switch event.Type {
	case EventDone:
		return a.inner.Close()
	case EventSSE:
		sseEvent := &sse.Event{
			Type: event.SSEType,
			Data: string(event.Data),
		}
		return a.inner.Transform(sseEvent)
	case EventOpenAIChunk:
		// OpenAI chunks come as SSE "data: {json}\n\n" events
		sseEvent := &sse.Event{
			Data: string(event.Data),
		}
		return a.inner.Transform(sseEvent)
	case EventAnthropicEvent:
		// Anthropic events come as SSE "event: type\ndata: {json}\n\n" events
		sseEvent := &sse.Event{
			Type: event.SSEType,
			Data: string(event.Data),
		}
		return a.inner.Transform(sseEvent)
	default:
		return nil
	}
}

// Initialize delegates to the inner transformer's Initialize.
func (a *sseTransformerAdapter) Initialize() error {
	return a.inner.Initialize()
}

// Flush delegates to the inner transformer's Flush.
func (a *sseTransformerAdapter) Flush() error {
	return a.inner.Flush()
}

// Close delegates to the inner transformer's Close.
func (a *sseTransformerAdapter) Close() error {
	return a.inner.Close()
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
