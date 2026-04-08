package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"ai-proxy/capture"

	"github.com/gin-gonic/gin"
)

// LogsHandler handles in-memory log API endpoints.
// Provides listing, detail, flush, clear, and configuration endpoints.
type LogsHandler struct{}

// NewLogsHandler creates a new LogsHandler.
func NewLogsHandler() *LogsHandler {
	return &LogsHandler{}
}

// logsListResponse is the response for GET /ui/api/logs.
type logsListResponse struct {
	Logs    []capture.LogEntry `json:"logs"`
	Total   int                `json:"total"`
	Page    int                `json:"page"`
	PerPage int                `json:"per_page"`
}

// logsConfigResponse is the response for GET /ui/api/logs/config.
type logsConfigResponse struct {
	Enabled  bool `json:"enabled"`
	Capacity int  `json:"capacity"`
	Count    int  `json:"count"`
}

// logsConfigUpdate is the request for PUT /ui/api/logs/config.
type logsConfigUpdate struct {
	Enabled  *bool `json:"enabled"`
	Capacity *int  `json:"capacity"`
}

// flushRequest is the request for POST /ui/api/logs/flush.
type flushRequest struct {
	Directory string   `json:"directory"`
	IDs       []string `json:"ids"`
}

// getStore returns the current in-memory store or responds with 503 and returns nil.
func getStore(c *gin.Context) *capture.MemoryStore {
	store := capture.GetDefaultMemoryStore()
	if store == nil {
		c.JSON(http.StatusServiceUnavailable, apiResponse{
			OK:    false,
			Error: "In-memory logging is disabled",
		})
		return nil
	}
	return store
}

// isValidFlushPath validates that a flush target directory is not a dangerous path.
// Rejects empty paths, relative paths, and known sensitive system directories.
func isValidFlushPath(dir string) bool {
	if dir == "" {
		return false
	}
	// Must be an absolute path
	if !filepath.IsAbs(dir) {
		return false
	}
	// Clean the path to prevent traversal
	cleaned := filepath.Clean(dir)
	// Reject known sensitive paths
	lower := strings.ToLower(cleaned)
	for _, prefix := range []string{"/etc", "/proc", "/sys", "/dev", "/boot", "/root", "/bin", "/sbin", "/usr", "/lib"} {
		if lower == prefix || strings.HasPrefix(lower, prefix+"/") {
			return false
		}
	}
	return true
}

// List returns a paginated list of in-memory log entries.
//
// GET /ui/api/logs?page=1&per_page=50&search=&path=&method=&status=
func (h *LogsHandler) List(c *gin.Context) {
	store := getStore(c)
	if store == nil {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}

	filter := capture.LogFilter{
		Search:       c.Query("search"),
		Path:         c.Query("path"),
		Method:       c.Query("method"),
		StatusPrefix: c.Query("status"),
	}

	offset := (page - 1) * perPage
	entries, total := store.List(offset, perPage, filter)

	c.JSON(http.StatusOK, logsListResponse{
		Logs:    entries,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

// Get returns a single log entry by request ID.
//
// GET /ui/api/logs/:id
func (h *LogsHandler) Get(c *gin.Context) {
	store := getStore(c)
	if store == nil {
		return
	}

	id := c.Param("id")
	entry, found := store.Get(id)
	if !found {
		c.JSON(http.StatusNotFound, apiResponse{
			OK:    false,
			Error: fmt.Sprintf("Log entry '%s' not found", id),
		})
		return
	}

	c.JSON(http.StatusOK, entry)
}

// Flush writes in-memory log entries to disk.
//
// POST /ui/api/logs/flush
func (h *LogsHandler) Flush(c *gin.Context) {
	store := getStore(c)
	if store == nil {
		return
	}

	var req flushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			OK:    false,
			Error: fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	if !isValidFlushPath(req.Directory) {
		c.JSON(http.StatusBadRequest, apiResponse{
			OK:    false,
			Error: "Invalid directory: must be an absolute path and not a system directory",
		})
		return
	}

	var entries []capture.LogEntry
	if len(req.IDs) > 0 {
		entries = store.GetByIDs(req.IDs)
	} else {
		entries = store.GetAll()
	}

	count, err := capture.FlushToDisk(entries, req.Directory)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiResponse{
			OK:    false,
			Error: fmt.Sprintf("Flush partially failed: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		OK:      true,
		Message: fmt.Sprintf("Flushed %d entries to %s", count, req.Directory),
	})
}

// Clear removes all in-memory log entries without writing to disk.
//
// DELETE /ui/api/logs
func (h *LogsHandler) Clear(c *gin.Context) {
	store := getStore(c)
	if store == nil {
		return
	}

	count := store.Count()
	store.Clear()

	c.JSON(http.StatusOK, apiResponse{
		OK:      true,
		Message: fmt.Sprintf("Cleared %d log entries", count),
	})
}

// GetConfig returns the current in-memory log configuration.
//
// GET /ui/api/logs/config
func (h *LogsHandler) GetConfig(c *gin.Context) {
	store := capture.GetDefaultMemoryStore()
	if store == nil {
		c.JSON(http.StatusOK, logsConfigResponse{
			Enabled:  false,
			Capacity: 0,
			Count:    0,
		})
		return
	}

	c.JSON(http.StatusOK, logsConfigResponse{
		Enabled:  true,
		Capacity: store.Capacity(),
		Count:    store.Count(),
	})
}

// UpdateConfig updates the in-memory log configuration at runtime.
// Disabling discards all entries. Changing capacity creates a new store.
//
// PUT /ui/api/logs/config
func (h *LogsHandler) UpdateConfig(c *gin.Context) {
	var req logsConfigUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			OK:    false,
			Error: fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Determine new state
	currentStore := capture.GetDefaultMemoryStore()
	enabled := currentStore != nil
	capacity := 2000
	if currentStore != nil {
		capacity = currentStore.Capacity()
	}

	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.Capacity != nil && *req.Capacity >= 10 {
		capacity = *req.Capacity
	}

	capture.InitMemoryStore(enabled, capacity)

	c.JSON(http.StatusOK, logsConfigResponse{
		Enabled:  enabled,
		Capacity: capacity,
		Count:    0,
	})
}
