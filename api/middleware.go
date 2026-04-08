// Package api provides middleware for request capture and logging.
// This file implements the capture middleware that records request and response
// data for debugging, auditing, and analysis purposes.
package api

import (
	"strings"

	"ai-proxy/capture"
	"ai-proxy/logging"

	"github.com/gin-gonic/gin"
)

// CaptureMiddleware captures request and response data for logging purposes.
// It integrates with the capture package to asynchronously persist captured data
// to storage for later analysis.
//
// @invariant If m.storage is nil, capture is disabled and no writes occur.
type CaptureMiddleware struct {
	// storage is the backend for persisting captured request/response data.
	// May be nil to disable capture. Thread-safe for concurrent writes.
	storage *capture.Storage
}

// NewCaptureMiddleware creates a new CaptureMiddleware with the given storage backend.
//
// @param storage - Storage backend for persisting captured data.
//
//	May be nil to disable capture functionality.
//	Caller retains ownership; middleware does not close it.
//
// @return Pointer to newly allocated CaptureMiddleware instance. Never returns nil.
//
// @post Returned middleware is ready to use with Handler() method.
func NewCaptureMiddleware(storage *capture.Storage) *CaptureMiddleware {
	return &CaptureMiddleware{storage: storage}
}

// isUIPath returns true if the request path starts with /ui/, which should
// be excluded from in-memory logging to avoid noise from the admin UI itself.
func isUIPath(path string) bool {
	return strings.HasPrefix(path, "/ui/") || path == "/ui"
}

// Handler returns a Gin middleware function that captures request context
// and writes to storage asynchronously after request processing completes.
//
// The capture flow is:
//  1. Create capture context before request processing
//  2. Attach capture context to request context for downstream access
//  3. Process request (handlers can access and populate capture context)
//  4. After request completes, asynchronously write captured data to storage
//
// @return Gin middleware function that captures request/response data.
//
// @pre m != nil
// @post Capture context is available via capture.GetCaptureContext().
// @post Captured data is written to storage asynchronously after request completes.
// @note Asynchronous write ensures request latency is not affected by disk I/O.
// @note If m.storage is nil, capture is disabled and no data is written.
// @note Requests to /ui/* paths are excluded from in-memory logging.
func (m *CaptureMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create capture context to hold all request/response data
		cc := capture.NewCaptureContext(c.Request)

		// Attach capture context to request context so handlers can access it
		// This enables downstream code to populate capture data
		ctx := capture.WithCaptureContext(c.Request.Context(), cc)

		// Record the downstream request so non-API routes (e.g. /api/ui/logs)
		// also capture headers. API handlers will overwrite this with the full
		// body when they process the request.
		cc.Recorder.RecordDownstreamRequest(c.Request.Header, nil)
		c.Request = c.Request.WithContext(ctx)

		// Capture the request path synchronously before c.Next() and goroutines.
		// After c.Next() returns, Gin may recycle the context, making c.Request
		// unsafe to access from the goroutine.
		requestPath := c.Request.URL.Path

		// Process the request - control returns here after handler completes
		c.Next()

		// Write captured data asynchronously to avoid blocking the response.
		// Goroutine is safe because capture context is self-contained and
		// requestPath was captured synchronously above.
		go func() {
			// Write to in-memory store if available.
			// Exclude /ui/* paths to avoid logging the admin UI's own requests.
			if store := capture.GetDefaultMemoryStore(); store != nil && !isUIPath(requestPath) {
				store.Store(cc.Recorder)
			}
			// Write to file storage if configured
			if m.storage != nil {
				if err := m.storage.Write(cc.Recorder); err != nil {
					logging.ErrorMsg("Failed to write capture: %v", err)
				}
			}
		}()
	}
}

// InitStorage creates a new capture storage instance if a base directory is provided.
// Returns nil if no directory is specified, effectively disabling capture.
//
// @param baseDir - Directory path for storing captured request/response data.
//
//	Empty string disables capture. Directory is created if needed.
//
// @return Pointer to Storage if baseDir is non-empty, nil otherwise.
//
// @post If baseDir is non-empty, storage is initialized and ready for writes.
// @note Caller is responsible for ensuring directory exists or is creatable.
func InitStorage(baseDir string) *capture.Storage {
	// Only create storage if a directory is specified
	// Empty directory string indicates capture should be disabled
	if baseDir != "" {
		return capture.NewStorage(baseDir)
	}
	// Return nil to signal that capture is disabled
	// Middleware checks for nil storage before attempting writes
	return nil
}
