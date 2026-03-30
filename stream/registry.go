package stream

import (
	"context"
	"sync"
	"time"

	"ai-proxy/transform"
)

// ActiveStream represents an in-progress streaming response.
type ActiveStream struct {
	ID          string
	Cancel      context.CancelFunc
	Transformer transform.SSETransformer
	StartedAt   time.Time
}

// Registry tracks active streams for cancellation support.
type Registry struct {
	mu      sync.RWMutex
	streams map[string]*ActiveStream
}

// NewRegistry creates a new stream registry.
func NewRegistry() *Registry {
	return &Registry{
		streams: make(map[string]*ActiveStream),
	}
}

// Register adds a new active stream to the registry.
// The transformer is stored to enable proper cancellation handling.
func (r *Registry) Register(id string, cancel context.CancelFunc, transformer transform.SSETransformer) *ActiveStream {
	r.mu.Lock()
	defer r.mu.Unlock()

	stream := &ActiveStream{
		ID:          id,
		Cancel:      cancel,
		Transformer: transformer,
		StartedAt:   time.Now(),
	}
	r.streams[id] = stream
	return stream
}

// Cancel attempts to cancel an active stream by ID.
// Returns true if the stream was found and cancelled.
// Calls HandleCancel() on the transformer to flush buffered content and emit response.cancelled.
func (r *Registry) Cancel(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	stream, ok := r.streams[id]
	if !ok {
		return false
	}

	// Call HandleCancel to flush buffered content and emit response.cancelled
	if stream.Transformer != nil {
		stream.Transformer.HandleCancel()
	}

	stream.Cancel()
	delete(r.streams, id)
	return true
}

// Remove removes a stream from the registry.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.streams, id)
}
