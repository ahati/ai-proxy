package capture

import (
	"encoding/json"
	"sync"
	"testing"
)

func TestNewMemoryStore(t *testing.T) {
	tests := []struct {
		name         string
		capacity     int
		wantCapacity int
	}{
		{name: "normal capacity", capacity: 100, wantCapacity: 100},
		{name: "below minimum", capacity: 5, wantCapacity: 10},
		{name: "exactly minimum", capacity: 10, wantCapacity: 10},
		{name: "large capacity", capacity: 5000, wantCapacity: 5000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := NewMemoryStore(tt.capacity)
			if ms.Capacity() != tt.wantCapacity {
				t.Errorf("NewMemoryStore(%d).Capacity() = %d, want %d", tt.capacity, ms.Capacity(), tt.wantCapacity)
			}
			if ms.Count() != 0 {
				t.Errorf("NewMemoryStore(%d).Count() = %d, want 0", tt.capacity, ms.Count())
			}
		})
	}
}

func TestMemoryStore_StoreAndGet(t *testing.T) {
	ms := NewMemoryStore(100)
	r := NewRecorder("test-id-1", "POST", "/v1/chat/completions", "127.0.0.1")
	r.RecordDownstreamRequest(nil, json.RawMessage(`{"model":"gpt-4"}`))

	ms.Store(r)

	if ms.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", ms.Count())
	}

	entry, found := ms.Get("test-id-1")
	if !found {
		t.Fatal("Get() not found")
	}
	if entry.RequestID != "test-id-1" {
		t.Errorf("RequestID = %q, want %q", entry.RequestID, "test-id-1")
	}
	if entry.Method != "POST" {
		t.Errorf("Method = %q, want %q", entry.Method, "POST")
	}
	if entry.Path != "/v1/chat/completions" {
		t.Errorf("Path = %q, want %q", entry.Path, "/v1/chat/completions")
	}
}

func TestMemoryStore_Get_NotFound(t *testing.T) {
	ms := NewMemoryStore(100)
	_, found := ms.Get("nonexistent")
	if found {
		t.Error("Get() should return false for nonexistent ID")
	}
}

func TestMemoryStore_RingBuffer(t *testing.T) {
	// Use capacity 10 (the minimum) and store 15 entries to trigger eviction
	ms := NewMemoryStore(10)

	// Store 15 entries — only last 10 should remain
	for i := 0; i < 15; i++ {
		id := string(rune('a' + i))
		r := NewRecorder(id, "GET", "/test", "localhost")
		ms.Store(r)
	}

	if ms.Count() != 10 {
		t.Fatalf("Count() = %d, want 10", ms.Count())
	}

	// First 5 entries (a-e) should be evicted
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		_, found := ms.Get(id)
		if found {
			t.Errorf("entry %q should have been evicted", id)
		}
	}

	// Last 10 entries (f-o) should exist
	for i := 5; i < 15; i++ {
		id := string(rune('a' + i))
		entry, found := ms.Get(id)
		if !found {
			t.Errorf("entry %q not found", id)
		}
		if entry.RequestID != id {
			t.Errorf("RequestID = %q, want %q", entry.RequestID, id)
		}
	}
}

func TestMemoryStore_List_Pagination(t *testing.T) {
	ms := NewMemoryStore(100)

	// Store 10 entries
	for i := 0; i < 10; i++ {
		r := NewRecorder(string(rune('0'+i)), "GET", "/test", "localhost")
		ms.Store(r)
	}

	// Page 1: limit 3, newest first
	entries, total := ms.List(0, 3, LogFilter{})
	if total != 10 {
		t.Errorf("total = %d, want 10", total)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}
	if entries[0].RequestID != "9" {
		t.Errorf("first entry = %q, want %q (newest first)", entries[0].RequestID, "9")
	}

	// Page 2: offset 3, limit 3
	entries, _ = ms.List(3, 3, LogFilter{})
	if entries[0].RequestID != "6" {
		t.Errorf("first entry = %q, want %q", entries[0].RequestID, "6")
	}

	// Beyond range
	entries, _ = ms.List(100, 3, LogFilter{})
	if len(entries) != 0 {
		t.Errorf("len(entries) = %d, want 0 for beyond-range offset", len(entries))
	}
}

func TestMemoryStore_List_FilterSearch(t *testing.T) {
	ms := NewMemoryStore(100)

	r1 := NewRecorder("abc-123", "GET", "/v1/models", "10.0.0.1")
	r2 := NewRecorder("def-456", "POST", "/v1/chat/completions", "10.0.0.2")
	r3 := NewRecorder("ghi-789", "POST", "/v1/messages", "10.0.0.1")
	ms.Store(r1)
	ms.Store(r2)
	ms.Store(r3)

	// Search by ID
	entries, total := ms.List(0, 10, LogFilter{Search: "abc"})
	if total != 1 || len(entries) != 1 {
		t.Errorf("Search='abc': total=%d, len=%d, want 1,1", total, len(entries))
	}

	// Search by path
	entries, total = ms.List(0, 10, LogFilter{Search: "chat"})
	if total != 1 || len(entries) != 1 {
		t.Errorf("Search='chat': total=%d, len=%d, want 1,1", total, len(entries))
	}

	// Search by IP
	entries, total = ms.List(0, 10, LogFilter{Search: "10.0.0.1"})
	if total != 2 || len(entries) != 2 {
		t.Errorf("Search='10.0.0.1': total=%d, len=%d, want 2,2", total, len(entries))
	}
}

func TestMemoryStore_List_FilterMethod(t *testing.T) {
	ms := NewMemoryStore(100)

	ms.Store(NewRecorder("1", "GET", "/test1", "localhost"))
	ms.Store(NewRecorder("2", "POST", "/test2", "localhost"))
	ms.Store(NewRecorder("3", "GET", "/test3", "localhost"))

	entries, total := ms.List(0, 10, LogFilter{Method: "POST"})
	if total != 1 || len(entries) != 1 {
		t.Fatalf("Method='POST': total=%d, len=%d, want 1,1", total, len(entries))
	}
	if entries[0].Method != "POST" {
		t.Errorf("Method = %q, want POST", entries[0].Method)
	}
}

func TestMemoryStore_List_FilterStatus(t *testing.T) {
	ms := NewMemoryStore(100)

	r1 := NewRecorder("ok", "POST", "/test", "localhost")
	resp1 := r1.RecordUpstreamResponse(200, nil)
	resp1.RecordChunk("message", `{"id":"1"}`)
	ms.Store(r1)

	r2 := NewRecorder("err", "POST", "/test", "localhost")
	r2.RecordUpstreamResponse(500, nil)
	ms.Store(r2)

	r3 := NewRecorder("notfound", "GET", "/test", "localhost")
	r3.RecordUpstreamResponse(404, nil)
	ms.Store(r3)

	entries, total := ms.List(0, 10, LogFilter{StatusPrefix: "2xx"})
	if total != 1 {
		t.Errorf("StatusPrefix='2xx': total=%d, want 1", total)
	}
	if len(entries) > 0 && entries[0].RequestID != "ok" {
		t.Errorf("RequestID = %q, want %q", entries[0].RequestID, "ok")
	}

	entries, total = ms.List(0, 10, LogFilter{StatusPrefix: "5xx"})
	if total != 1 {
		t.Errorf("StatusPrefix='5xx': total=%d, want 1", total)
	}

	entries, total = ms.List(0, 10, LogFilter{StatusPrefix: "4xx"})
	if total != 1 {
		t.Errorf("StatusPrefix='4xx': total=%d, want 1", total)
	}
}

func TestMemoryStore_GetByIDs(t *testing.T) {
	ms := NewMemoryStore(100)

	ms.Store(NewRecorder("id1", "GET", "/test", "localhost"))
	ms.Store(NewRecorder("id2", "POST", "/test", "localhost"))
	ms.Store(NewRecorder("id3", "GET", "/test", "localhost"))

	entries := ms.GetByIDs([]string{"id1", "id3"})
	if len(entries) != 2 {
		t.Fatalf("GetByIDs() returned %d entries, want 2", len(entries))
	}

	ids := make(map[string]bool)
	for _, e := range entries {
		ids[e.RequestID] = true
	}
	if !ids["id1"] || !ids["id3"] {
		t.Error("GetByIDs() missing expected IDs")
	}
}

func TestMemoryStore_GetAll(t *testing.T) {
	ms := NewMemoryStore(100)

	ms.Store(NewRecorder("a", "GET", "/test", "localhost"))
	ms.Store(NewRecorder("b", "POST", "/test", "localhost"))
	ms.Store(NewRecorder("c", "GET", "/test", "localhost"))

	entries := ms.GetAll()
	if len(entries) != 3 {
		t.Fatalf("GetAll() returned %d entries, want 3", len(entries))
	}
	// GetAll returns entries in ring-buffer order
	// Verify all entries are present regardless of ordering
	ids := make(map[string]bool)
	for _, e := range entries {
		ids[e.RequestID] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !ids[want] {
			t.Errorf("GetAll() missing entry %q", want)
		}
	}
}

func TestMemoryStore_Clear(t *testing.T) {
	ms := NewMemoryStore(100)

	ms.Store(NewRecorder("1", "GET", "/test", "localhost"))
	ms.Store(NewRecorder("2", "POST", "/test", "localhost"))
	if ms.Count() != 2 {
		t.Fatalf("Count() before clear = %d, want 2", ms.Count())
	}

	ms.Clear()
	if ms.Count() != 0 {
		t.Errorf("Count() after clear = %d, want 0", ms.Count())
	}

	_, found := ms.Get("1")
	if found {
		t.Error("Get() should return false after clear")
	}
}

func TestMemoryStore_Store_NilRecorder(t *testing.T) {
	ms := NewMemoryStore(100)
	ms.Store(nil) // should not panic
	if ms.Count() != 0 {
		t.Errorf("Count() = %d, want 0 after nil store", ms.Count())
	}
}

func TestInitMemoryStore(t *testing.T) {
	// Save and restore
	orig := DefaultMemoryStore
	defer func() { DefaultMemoryStore = orig }()

	InitMemoryStore(true, 500)
	if DefaultMemoryStore == nil {
		t.Fatal("DefaultMemoryStore should not be nil when enabled")
	}
	if DefaultMemoryStore.Capacity() != 500 {
		t.Errorf("Capacity() = %d, want 500", DefaultMemoryStore.Capacity())
	}

	InitMemoryStore(false, 100)
	if DefaultMemoryStore != nil {
		t.Error("DefaultMemoryStore should be nil when disabled")
	}
}

func TestMemoryStore_Concurrent(t *testing.T) {
	ms := NewMemoryStore(1000)
	var wg sync.WaitGroup

	// Concurrent stores
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r := NewRecorder(string(rune('A'+n%26))+string(rune('0'+n%10)), "GET", "/test", "localhost")
			ms.Store(r)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ms.List(0, 10, LogFilter{})
			ms.Count()
			ms.GetAll()
		}()
	}

	wg.Wait()

	// Should not panic and count should be valid
	count := ms.Count()
	if count > 100 {
		t.Errorf("Count() = %d, should be <= 100", count)
	}
}
