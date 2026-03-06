package logging

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type recorder struct {
	mu      sync.Mutex
	data    *RequestRecorder
	started time.Time
}

func newRecorder(requestID, method, path, clientIP string) *recorder {
	now := time.Now()
	return &recorder{
		started: now,
		data: &RequestRecorder{
			RequestID: requestID,
			StartedAt: now,
			Method:    method,
			Path:      path,
			ClientIP:  clientIP,
		},
	}
}

func (r *recorder) RecordDownstreamRequest(headers http.Header, body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data.DownstreamRequest = &HTTPRequestCapture{
		At:      time.Now(),
		Headers: SanitizeHeaders(headers),
		Body:    body,
		RawBody: body,
	}
}

func (r *recorder) RecordUpstreamRequest(headers http.Header, body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data.UpstreamRequest = &HTTPRequestCapture{
		At:      time.Now(),
		Headers: SanitizeHeaders(headers),
		Body:    body,
		RawBody: body,
	}
}

func (r *recorder) RecordUpstreamResponse(statusCode int, headers http.Header) *responseRecorder {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data.UpstreamResponse = &SSEResponseCapture{
		StatusCode: statusCode,
		Headers:    SanitizeHeaders(headers),
		Chunks:     []SSEChunk{},
	}

	return &responseRecorder{
		capture: r.data.UpstreamResponse,
		started: r.started,
	}
}

func (r *recorder) RecordDownstreamResponse() *responseRecorder {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data.DownstreamResponse = &SSEResponseCapture{
		Chunks: []SSEChunk{},
	}

	return &responseRecorder{
		capture: r.data.DownstreamResponse,
		started: r.started,
	}
}

func (r *recorder) Data() *RequestRecorder {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.data
}

type responseRecorder struct {
	capture *SSEResponseCapture
	started time.Time
}

func (rr *responseRecorder) RecordChunk(event, raw string) {
	if rr == nil || rr.capture == nil {
		return
	}

	chunk := SSEChunk{
		OffsetMS: OffsetMS(rr.started),
		Event:    event,
		Raw:      raw,
	}

	var data json.RawMessage
	if err := json.Unmarshal([]byte(raw), &data); err == nil {
		chunk.Data = data
	}

	rr.capture.Chunks = append(rr.capture.Chunks, chunk)
}

func (rr *responseRecorder) RecordChunkBytes(event string, data []byte) {
	rr.RecordChunk(event, string(data))
}
