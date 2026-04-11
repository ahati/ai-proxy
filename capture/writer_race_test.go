package capture

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestCaptureWriter_ConcurrentRecordChunk tests thread-safe concurrent chunk recording.
// This test verifies that the atomic compare-and-swap implementation correctly handles
// multiple goroutines appending chunks simultaneously without data loss.
func TestCaptureWriter_ConcurrentRecordChunk(t *testing.T) {
	cw := NewCaptureWriter(time.Now())

	// Simulate concurrent chunk recording
	var wg sync.WaitGroup
	numGoroutines := 10
	chunksPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < chunksPerGoroutine; j++ {
				data := fmt.Sprintf(`{"goroutine":%d,"chunk":%d}`, id, j)
				cw.RecordChunk("message", []byte(data))
			}
		}(i)
	}

	// Concurrently read chunks during recording
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			chunks := cw.Chunks()
			// Verify snapshot is safe to iterate
			for _, chunk := range chunks {
				_ = chunk.OffsetMS // Access fields to verify validity
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Verify all chunks were captured
	finalChunks := cw.Chunks()
	expectedTotal := numGoroutines * chunksPerGoroutine
	if len(finalChunks) != expectedTotal {
		t.Errorf("Expected %d chunks, got %d - data was lost due to race condition",
			expectedTotal, len(finalChunks))
	}
}

// TestCaptureWriter_ConcurrentChunksRead tests that Chunks() returns a consistent
// snapshot that doesn't change even if more chunks are added.
func TestCaptureWriter_ConcurrentChunksRead(t *testing.T) {
	cw := NewCaptureWriter(time.Now())

	// Add some initial chunks
	for i := 0; i < 10; i++ {
		cw.RecordChunk("message", []byte(fmt.Sprintf(`{"initial":%d}`, i)))
	}

	// Get snapshot
	snapshot := cw.Chunks()
	initialLen := len(snapshot)

	// Add more chunks in background
	go func() {
		for i := 0; i < 100; i++ {
			cw.RecordChunk("message", []byte(fmt.Sprintf(`{"additional":%d}`, i)))
		}
	}()

	// Wait a bit for goroutine to add chunks
	time.Sleep(10 * time.Millisecond)

	// Verify snapshot hasn't changed
	if len(snapshot) != initialLen {
		t.Errorf("Snapshot length changed from %d to %d - snapshot is not immutable",
			initialLen, len(snapshot))
	}

	// Verify new chunks were actually added
	currentChunks := cw.Chunks()
	if len(currentChunks) <= initialLen {
		t.Errorf("Expected more than %d chunks after concurrent adds, got %d",
			initialLen, len(currentChunks))
	}
}

// TestCaptureWriter_RaceDetector runs multiple concurrent operations to trigger
// the race detector. This test should pass without any race warnings.
func TestCaptureWriter_RaceDetector(t *testing.T) {
	cw := NewCaptureWriter(time.Now())

	var wg sync.WaitGroup

	// Writer goroutine 1
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			cw.RecordChunk("event1", []byte(`{"writer":1}`))
		}
	}()

	// Writer goroutine 2
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			cw.RecordChunk("event2", []byte(`{"writer":2}`))
		}
	}()

	// Reader goroutine 1
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			chunks := cw.Chunks()
			_ = len(chunks)
		}
	}()

	// Reader goroutine 2
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			chunks := cw.Chunks()
			for _, chunk := range chunks {
				_ = chunk.OffsetMS
			}
		}
	}()

	wg.Wait()

	// Verify no data was lost
	chunks := cw.Chunks()
	if len(chunks) != 200 {
		t.Errorf("Expected 200 chunks, got %d - race condition caused data loss", len(chunks))
	}
}

// BenchmarkCaptureWriter_RecordChunk measures the performance of thread-safe chunk recording.
// Expected: ~100-200 ns/op (acceptable for streaming)
func BenchmarkCaptureWriter_RecordChunk(b *testing.B) {
	cw := NewCaptureWriter(time.Now())
	data := []byte(`{"test":"benchmark data"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cw.RecordChunk("message", data)
	}

	// Report allocation stats
	b.ReportAllocs()
}

// BenchmarkCaptureWriter_RecordChunk_Parallel measures performance under concurrent load.
func BenchmarkCaptureWriter_RecordChunk_Parallel(b *testing.B) {
	cw := NewCaptureWriter(time.Now())
	data := []byte(`{"test":"benchmark data"}`)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cw.RecordChunk("message", data)
		}
	})
}

// BenchmarkCaptureWriter_Chunks measures the performance of getting a snapshot.
func BenchmarkCaptureWriter_Chunks(b *testing.B) {
	cw := NewCaptureWriter(time.Now())

	// Add 100 chunks
	for i := 0; i < 100; i++ {
		cw.RecordChunk("message", []byte(`{"chunk":`+fmt.Sprint(i)+`}`))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunks := cw.Chunks()
		_ = chunks
	}

	b.ReportAllocs()
}
