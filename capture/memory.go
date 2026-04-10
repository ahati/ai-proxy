package capture

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultMemoryCapacity is the default number of log entries to keep in memory.
const DefaultMemoryCapacity = 2000

// minMemoryCapacity is the minimum allowed capacity for the memory store.
const minMemoryCapacity = 10

// defaultStore is the global in-memory log store, accessed atomically.
// Use GetDefaultMemoryStore() to read and InitMemoryStore() to write.
var defaultStore atomic.Pointer[MemoryStore]

// GetDefaultMemoryStore returns the current global in-memory log store.
// Returns nil if in-memory logging is disabled.
// Thread-safe: uses atomic load.
func GetDefaultMemoryStore() *MemoryStore {
	return defaultStore.Load()
}

// DefaultMemoryStore is kept for backward compatibility but delegates to the atomic store.
// Deprecated: Use GetDefaultMemoryStore() instead.
var DefaultMemoryStore *MemoryStore

// MemoryStore is a thread-safe ring buffer that stores captured request logs in memory.
// When the buffer is full, the oldest entries are overwritten.
//
// Thread Safety: All methods are safe for concurrent use.
type MemoryStore struct {
	mu       sync.RWMutex
	entries  []LogEntry
	head     int // next write position
	count    int // number of entries stored (up to capacity)
	capacity int
}

// NewMemoryStore creates a new MemoryStore with the given capacity.
// If capacity is less than minMemoryCapacity, it is set to minMemoryCapacity.
//
// @param capacity - Maximum number of entries to store. Must be >= 10.
// @return *MemoryStore - Initialized memory store.
func NewMemoryStore(capacity int) *MemoryStore {
	if capacity < minMemoryCapacity {
		capacity = minMemoryCapacity
	}
	return &MemoryStore{
		entries:  make([]LogEntry, capacity),
		capacity: capacity,
	}
}

// InitMemoryStore initializes the global in-memory log store.
// If enabled is false, the store is set to nil (feature disabled).
// Thread-safe: uses atomic store.
//
// @param enabled - Whether in-memory logging is enabled.
// @param capacity - Maximum number of entries (used only if enabled).
func InitMemoryStore(enabled bool, capacity int) {
	if !enabled {
		defaultStore.Store(nil)
		DefaultMemoryStore = nil
		return
	}
	store := NewMemoryStore(capacity)
	defaultStore.Store(store)
	DefaultMemoryStore = store
}

// Store serializes the recorder data into a LogEntry and adds it to the ring buffer.
// If the buffer is full, the oldest entry is overwritten.
//
// @param recorder - The request recorder to store. Must not be nil.
func (m *MemoryStore) Store(recorder *Recorder) {
	if recorder == nil {
		return
	}

	data := recorder.Data()
	entry := LogEntry{
		RequestID:          data.RequestID,
		StartedAt:          data.StartedAt,
		DurationMS:         time.Since(data.StartedAt).Milliseconds(),
		Method:             data.Method,
		Path:               data.Path,
		ClientIP:           data.ClientIP,
		DownstreamRequest:  data.DownstreamRequest,
		UpstreamRequest:    data.UpstreamRequest,
		UpstreamResponse:   data.UpstreamResponse,
		DownstreamResponse: data.DownstreamResponse,
	}

	m.mu.Lock()
	m.entries[m.head] = entry
	m.head = (m.head + 1) % m.capacity
	if m.count < m.capacity {
		m.count++
	}
	m.mu.Unlock()
}

// List returns a slice of log entries matching the filter, ordered newest-first.
// Also returns the total number of matching entries for pagination.
//
// @param offset - Number of entries to skip (for pagination).
// @param limit - Maximum number of entries to return.
// @param filter - Filter criteria. Empty filter matches all entries.
// @return entries - Slice of matching LogEntry values (may be empty).
// @return total - Total number of matching entries.
func (m *MemoryStore) List(offset, limit int, filter LogFilter) ([]LogEntry, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect entries in chronological order (oldest first)
	var all []LogEntry
	if m.count < m.capacity {
		all = m.entries[:m.count]
	} else {
		// Ring buffer is full; start from oldest (head position)
		all = make([]LogEntry, m.capacity)
		for i := 0; i < m.capacity; i++ {
			all[i] = m.entries[(m.head+i)%m.capacity]
		}
	}

	// Apply filters and reverse to newest-first
	filtered := make([]LogEntry, 0)
	for i := len(all) - 1; i >= 0; i-- {
		if matchesFilter(all[i], filter) {
			filtered = append(filtered, all[i])
		}
	}

	total := len(filtered)

	// Apply pagination
	if offset >= total {
		return []LogEntry{}, total
	}
	if offset+limit > total {
		limit = total - offset
	}

	return filtered[offset : offset+limit], total
}

// Get returns a single log entry by request ID.
//
// @param requestID - The request ID to search for.
// @return entry - The matching log entry, or nil if not found.
// @return found - True if an entry was found.
func (m *MemoryStore) Get(requestID string) (*LogEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := 0; i < m.count; i++ {
		idx := i
		if m.count == m.capacity {
			// Buffer is full - calculate actual ring buffer position
			// Ring buffer: head is next write position, so oldest entry is at head
			idx = (m.head + i) % m.capacity
		}
		if m.entries[idx].RequestID == requestID {
			entry := m.entries[idx]
			return &entry, true
		}
	}
	return nil, false
}

// GetByIDs returns all log entries matching the given request IDs.
//
// @param ids - Slice of request IDs to retrieve.
// @return Slice of matching LogEntry values.
func (m *MemoryStore) GetByIDs(ids []string) []LogEntry {
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []LogEntry
	for i := 0; i < m.count; i++ {
		idx := i
		if m.count == m.capacity {
			// Buffer is full - calculate actual ring buffer position
			idx = (m.head + i) % m.capacity
		}
		if idSet[m.entries[idx].RequestID] {
			result = append(result, m.entries[idx])
		}
	}
	return result
}

// GetAll returns all entries in the store (newest first).
//
// @return Slice of all LogEntry values.
func (m *MemoryStore) GetAll() []LogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]LogEntry, 0, m.count)
	// Iterate in reverse from head to get newest first
	for i := m.count - 1; i >= 0; i-- {
		idx := (m.head - 1 - i + m.capacity) % m.capacity
		result = append(result, m.entries[idx])
	}
	return result
}

// Clear removes all entries from the store.
func (m *MemoryStore) Clear() {
	m.mu.Lock()
	m.entries = make([]LogEntry, m.capacity)
	m.head = 0
	m.count = 0
	m.mu.Unlock()
}

// Count returns the current number of entries in the store.
func (m *MemoryStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.count
}

// Capacity returns the maximum capacity of the store.
func (m *MemoryStore) Capacity() int {
	return m.capacity
}

// matchesFilter checks if a log entry matches the given filter criteria.
func matchesFilter(entry LogEntry, f LogFilter) bool {
	if f.Search != "" {
		searchLower := strings.ToLower(f.Search)
		if !strings.Contains(strings.ToLower(entry.RequestID), searchLower) &&
			!strings.Contains(strings.ToLower(entry.Path), searchLower) &&
			!strings.Contains(strings.ToLower(entry.ClientIP), searchLower) {
			return false
		}
	}
	if f.Path != "" && entry.Path != f.Path {
		return false
	}
	if f.Method != "" && entry.Method != f.Method {
		return false
	}
	if f.StatusPrefix != "" {
		statusCode := getStatusCode(entry)
		if !matchesStatusPrefix(statusCode, f.StatusPrefix) {
			return false
		}
	}
	return true
}

// getStatusCode extracts the HTTP status code from a log entry.
// Checks upstream response first, then downstream response.
func getStatusCode(entry LogEntry) int {
	if entry.UpstreamResponse != nil && entry.UpstreamResponse.StatusCode > 0 {
		return entry.UpstreamResponse.StatusCode
	}
	if entry.DownstreamResponse != nil && entry.DownstreamResponse.StatusCode > 0 {
		return entry.DownstreamResponse.StatusCode
	}
	return 0
}

// matchesStatusPrefix checks if a status code matches a prefix like "2xx", "4xx", "5xx".
func matchesStatusPrefix(code int, prefix string) bool {
	if code == 0 {
		return false
	}
	if len(prefix) != 3 {
		return false
	}
	classDigit := prefix[0]
	return code/100 == int(classDigit-'0')
}
