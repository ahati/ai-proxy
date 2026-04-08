package capture

import (
	"time"
)

// LogEntry represents a captured request/response pair for logging and analysis.
// It contains the full request lifecycle data including timing, request/response
// details, and SSE streaming chunks.
//
// Thread Safety: Value type; safe for concurrent read access after creation.
type LogEntry struct {
	// RequestID is the unique identifier for this request.
	RequestID string `json:"request_id"`
	// StartedAt is when the request was initiated.
	StartedAt time.Time `json:"started_at"`
	// DurationMS is the total request duration in milliseconds.
	DurationMS int64 `json:"duration_ms,omitempty"`
	// Method is the HTTP method of the request.
	Method string `json:"method"`
	// Path is the URL path of the request.
	Path string `json:"path"`
	// ClientIP is the remote address of the client.
	ClientIP string `json:"client_ip,omitempty"`
	// DownstreamRequest is the captured client request.
	DownstreamRequest *HTTPRequestCapture `json:"downstream_request,omitempty"`
	// UpstreamRequest is the captured upstream API request.
	UpstreamRequest *HTTPRequestCapture `json:"upstream_request,omitempty"`
	// UpstreamResponse is the captured upstream API response.
	UpstreamResponse *SSEResponseCapture `json:"upstream_response,omitempty"`
	// DownstreamResponse is the captured response sent to client.
	DownstreamResponse *SSEResponseCapture `json:"downstream_response,omitempty"`
}

// LogFilter defines filtering criteria for querying log entries.
// Empty fields are ignored (match all).
type LogFilter struct {
	// Search matches against request ID, path, and client IP (case-insensitive substring).
	Search string
	// Path is an exact match filter on the request path.
	Path string
	// Method is an exact match filter on the HTTP method.
	Method string
	// StatusPrefix matches HTTP status code ranges: "2xx", "4xx", "5xx", etc.
	StatusPrefix string
}
