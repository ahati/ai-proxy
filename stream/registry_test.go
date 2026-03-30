package stream

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if r.streams == nil {
		t.Error("streams map not initialized")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := r.Register("test-id", cancel, nil)

	if stream == nil {
		t.Fatal("Register() returned nil")
	}
	if stream.ID != "test-id" {
		t.Errorf("ID = %q, want %q", stream.ID, "test-id")
	}
	if stream.Cancel == nil {
		t.Error("Cancel function is nil")
	}
	if stream.StartedAt.IsZero() {
		t.Error("StartedAt is zero")
	}
}

func TestRegistry_Cancel(t *testing.T) {
	r := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())

	r.Register("test-id", cancel, nil)

	// Verify context is not cancelled before Cancel
	select {
	case <-ctx.Done():
		t.Error("context cancelled before Cancel()")
	default:
	}

	// Cancel the stream
	result := r.Cancel("test-id")
	if !result {
		t.Error("Cancel() returned false, expected true")
	}

	// Verify context is cancelled
	select {
	case <-ctx.Done():
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Error("context not cancelled after Cancel()")
	}
}

func TestRegistry_Cancel_NotFound(t *testing.T) {
	r := NewRegistry()

	result := r.Cancel("nonexistent")
	if result {
		t.Error("Cancel() returned true for nonexistent stream")
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.Register("test-id", cancel, nil)
	r.Remove("test-id")

	// Verify stream is removed by checking Cancel returns false
	result := r.Cancel("test-id")
	if result {
		t.Error("Cancel() returned true after Remove(), expected false")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := string(rune('a'+n%26)) + string(rune('a'+n/26))
			_, cancel := context.WithCancel(context.Background())
			defer cancel()
			r.Register(id, cancel, nil)
		}(i)
	}

	// Concurrent removes
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := string(rune('a'+n%26)) + string(rune('a'+n/26))
			r.Remove(id)
		}(i)
	}

	wg.Wait()
}
