package downstream

import (
	"net/http"
	"regexp"

	"ai-proxy/logging"
)

var eventTypeRegex = regexp.MustCompile(`^event:\s*(\S+)`)

// ResponseRecorder wraps an http.ResponseWriter to capture SSE chunks
// It implements the CaptureWriter interface for homogeneous logging
type ResponseRecorder struct {
	writer  http.ResponseWriter
	capture logging.CaptureWriter
}

func NewResponseRecorder(writer http.ResponseWriter, capture logging.CaptureWriter) *ResponseRecorder {
	return &ResponseRecorder{
		writer:  writer,
		capture: capture,
	}
}

func (r *ResponseRecorder) Write(data []byte) (int, error) {
	if len(data) > 0 {
		// Extract just the JSON data part from SSE format (remove "data: " prefix and newlines)
		dataForLogging := extractDataPart(data)
		event := extractEventType(data)
		r.capture.RecordChunk(event, dataForLogging)
	}
	return r.writer.Write(data)
}

func extractDataPart(data []byte) []byte {
	// SSE format: "data: {...}\n\n" or "event: xxx\ndata: {...}\n\n"
	s := string(data)

	// Find "data: " prefix
	if idx := findDataPrefix(s); idx >= 0 {
		start := idx + 6 // len("data: ")
		// Find the end (before the double newline)
		end := len(s)
		if nlIdx := indexOfDoubleNewline(s[start:]); nlIdx >= 0 {
			end = start + nlIdx
		}
		return []byte(s[start:end])
	}
	return data
}

func findDataPrefix(s string) int {
	// Look for "data: " in the string
	for i := 0; i < len(s)-5; i++ {
		if s[i:i+6] == "data: " {
			return i
		}
	}
	return -1
}

func indexOfDoubleNewline(s string) int {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '\n' && s[i+1] == '\n' {
			return i
		}
	}
	return -1
}

func (r *ResponseRecorder) Header() http.Header {
	return r.writer.Header()
}

func (r *ResponseRecorder) WriteHeader(statusCode int) {
	r.writer.WriteHeader(statusCode)
}

func extractEventType(data []byte) string {
	matches := eventTypeRegex.FindSubmatch(data)
	if len(matches) > 1 {
		return string(matches[1])
	}
	return "message"
}
