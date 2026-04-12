package capture

import (
	"net/http"
	"testing"
)

func TestRecorder_GetUpstreamResponseRecorder_NilResponse(t *testing.T) {
	r := NewRecorder("test-id", "GET", "/test", "127.0.0.1")

	recorder := r.GetUpstreamResponseRecorder()
	if recorder != nil {
		t.Errorf("GetUpstreamResponseRecorder() = %v, want nil when upstream response not initialized", recorder)
	}
}

func TestRecorder_GetUpstreamResponseRecorder_ExistingResponse(t *testing.T) {
	r := NewRecorder("test-id", "GET", "/test", "127.0.0.1")

	// Initialize upstream response
	r.RecordUpstreamResponse(200, http.Header{"Content-Type": []string{"application/json"}})

	// Get existing recorder
	recorder := r.GetUpstreamResponseRecorder()
	if recorder == nil {
		t.Fatal("GetUpstreamResponseRecorder() = nil, want non-nil when upstream response exists")
	}

	// Verify it writes to the same capture structure
	recorder.RecordChunk("test", "data")

	data := r.Data()
	if data.UpstreamResponse == nil {
		t.Fatal("UpstreamResponse should not be nil")
	}
	if data.UpstreamResponse.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", data.UpstreamResponse.StatusCode)
	}
	if len(data.UpstreamResponse.Chunks) != 1 {
		t.Errorf("Chunks length = %d, want 1", len(data.UpstreamResponse.Chunks))
	}
	if data.UpstreamResponse.Chunks[0].Event != "test" {
		t.Errorf("Chunk event = %q, want %q", data.UpstreamResponse.Chunks[0].Event, "test")
	}
}

func TestRecorder_GetUpstreamResponseRecorder_PreservesHeaders(t *testing.T) {
	r := NewRecorder("test-id", "GET", "/test", "127.0.0.1")

	// Initialize with headers
	headers := http.Header{
		"Content-Type":    []string{"application/json"},
		"X-Custom-Header": []string{"custom-value"},
		"Authorization":   []string{"Bearer secret-token"}, // Should be sanitized
	}
	r.RecordUpstreamResponse(200, headers)

	// Get existing recorder and add chunks
	recorder := r.GetUpstreamResponseRecorder()
	if recorder == nil {
		t.Fatal("GetUpstreamResponseRecorder() should return non-nil")
	}
	recorder.RecordChunk("", "chunk data")

	data := r.Data()
	// Verify headers are preserved (not overwritten)
	if data.UpstreamResponse.Headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type header = %q, want %q", data.UpstreamResponse.Headers["Content-Type"], "application/json")
	}
	if data.UpstreamResponse.Headers["X-Custom-Header"] != "custom-value" {
		t.Errorf("X-Custom-Header = %q, want %q", data.UpstreamResponse.Headers["X-Custom-Header"], "custom-value")
	}
	// Verify sensitive header was sanitized
	if data.UpstreamResponse.Headers["Authorization"] != "***" {
		t.Errorf("Authorization header = %q, want %q (sanitized)", data.UpstreamResponse.Headers["Authorization"], "***")
	}
}

func TestRecorder_GetDownstreamResponseRecorder_NilResponse(t *testing.T) {
	r := NewRecorder("test-id", "GET", "/test", "127.0.0.1")

	recorder := r.GetDownstreamResponseRecorder()
	if recorder != nil {
		t.Errorf("GetDownstreamResponseRecorder() = %v, want nil when downstream response not initialized", recorder)
	}
}

func TestRecorder_GetDownstreamResponseRecorder_ExistingResponse(t *testing.T) {
	r := NewRecorder("test-id", "GET", "/test", "127.0.0.1")

	// Initialize downstream response
	r.RecordDownstreamResponse(nil)

	// Get existing recorder
	recorder := r.GetDownstreamResponseRecorder()
	if recorder == nil {
		t.Fatal("GetDownstreamResponseRecorder() = nil, want non-nil when downstream response exists")
	}

	// Verify it writes to the same capture structure
	recorder.RecordChunk("test", "data")

	data := r.Data()
	if data.DownstreamResponse == nil {
		t.Fatal("DownstreamResponse should not be nil")
	}
	if len(data.DownstreamResponse.Chunks) != 1 {
		t.Errorf("Chunks length = %d, want 1", len(data.DownstreamResponse.Chunks))
	}
	if data.DownstreamResponse.Chunks[0].Event != "test" {
		t.Errorf("Chunk event = %q, want %q", data.DownstreamResponse.Chunks[0].Event, "test")
	}
}
