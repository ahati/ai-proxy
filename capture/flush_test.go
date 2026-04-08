package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFlushToDisk_AllEntries(t *testing.T) {
	tmpDir := t.TempDir()

	entries := []LogEntry{
		{
			RequestID:  "flush-test-1",
			StartedAt:  time.Now(),
			DurationMS: 150,
			Method:     "POST",
			Path:       "/v1/chat/completions",
			ClientIP:   "127.0.0.1",
		},
		{
			RequestID:  "flush-test-2",
			StartedAt:  time.Now(),
			DurationMS: 200,
			Method:     "GET",
			Path:       "/v1/models",
			ClientIP:   "127.0.0.1",
		},
	}

	count, err := FlushToDisk(entries, tmpDir)
	if err != nil {
		t.Fatalf("FlushToDisk() error: %v", err)
	}
	if count != 2 {
		t.Errorf("FlushToDisk() count = %d, want 2", count)
	}

	// Verify files were created
	dateDir := filepath.Join(tmpDir, time.Now().Format("2006-01-02"))
	files, err := os.ReadDir(dateDir)
	if err != nil {
		t.Fatalf("failed to read date directory: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	// Verify file content
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dateDir, f.Name()))
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		var entry LogEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		if entry.RequestID == "" {
			t.Error("RequestID should not be empty in flushed file")
		}
	}
}

func TestFlushToDisk_EmptyBuffer(t *testing.T) {
	tmpDir := t.TempDir()

	count, err := FlushToDisk([]LogEntry{}, tmpDir)
	if err != nil {
		t.Fatalf("FlushToDisk() error: %v", err)
	}
	if count != 0 {
		t.Errorf("FlushToDisk() count = %d, want 0", count)
	}
}

func TestFlushToDisk_NilEntries(t *testing.T) {
	tmpDir := t.TempDir()

	count, err := FlushToDisk(nil, tmpDir)
	if err != nil {
		t.Fatalf("FlushToDisk() error: %v", err)
	}
	if count != 0 {
		t.Errorf("FlushToDisk() count = %d, want 0", count)
	}
}

func TestFlushToDisk_InvalidDir(t *testing.T) {
	entries := []LogEntry{
		{RequestID: "test", Method: "GET", Path: "/test"},
	}

	_, err := FlushToDisk(entries, "/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for invalid directory")
	}
}

func TestFlushToDisk_SelectiveIDs(t *testing.T) {
	tmpDir := t.TempDir()

	// Flush specific entries
	entries := []LogEntry{
		{RequestID: "keep-1", Method: "POST", Path: "/v1/chat"},
		{RequestID: "keep-2", Method: "GET", Path: "/v1/models"},
	}

	count, err := FlushToDisk(entries, tmpDir)
	if err != nil {
		t.Fatalf("FlushToDisk() error: %v", err)
	}
	if count != 2 {
		t.Errorf("FlushToDisk() count = %d, want 2", count)
	}

	// Verify both entries are in the output
	dateDir := filepath.Join(tmpDir, time.Now().Format("2006-01-02"))
	files, _ := os.ReadDir(dateDir)

	found1, found2 := false, false
	for _, f := range files {
		if strings.Contains(f.Name(), "keep-1") {
			found1 = true
		}
		if strings.Contains(f.Name(), "keep-2") {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Errorf("expected both files, found keep-1=%v, keep-2=%v", found1, found2)
	}
}
