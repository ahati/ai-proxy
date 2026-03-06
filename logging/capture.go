package logging

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type contextKey string

const captureContextKey contextKey = "capture_context"

type CaptureContext struct {
	RequestID   string
	StartTime   time.Time
	Recorder    *RequestRecorder
	IDExtracted bool
}

type HTTPRequestCapture struct {
	At      time.Time         `json:"at"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
	RawBody []byte            `json:"-"`
}

type SSEResponseCapture struct {
	StatusCode int               `json:"status_code,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Chunks     []SSEChunk        `json:"chunks,omitempty"`
	RawBody    []byte            `json:"-"`
}

type SSEChunk struct {
	OffsetMS int64           `json:"offset_ms"`
	Event    string          `json:"event,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
	Raw      string          `json:"raw,omitempty"`
}

func NewSSEChunk(offsetMS int64, event string, data []byte) SSEChunk {
	chunk := SSEChunk{
		OffsetMS: offsetMS,
		Event:    event,
	}

	var jsonData json.RawMessage
	if err := json.Unmarshal(data, &jsonData); err == nil {
		chunk.Data = make(json.RawMessage, len(jsonData))
		copy(chunk.Data, jsonData)
	} else {
		chunk.Raw = string(data)
	}

	return chunk
}

type RequestRecorder struct {
	RequestID          string
	StartedAt          time.Time
	Method             string
	Path               string
	ClientIP           string
	DownstreamRequest  *HTTPRequestCapture
	UpstreamRequest    *HTTPRequestCapture
	UpstreamResponse   *SSEResponseCapture
	DownstreamResponse *SSEResponseCapture
}

func NewCaptureContext(r *http.Request) *CaptureContext {
	return &CaptureContext{
		StartTime: time.Now(),
		Recorder: &RequestRecorder{
			StartedAt: time.Now(),
			Method:    r.Method,
			Path:      r.URL.Path,
			ClientIP:  r.RemoteAddr,
		},
		IDExtracted: false,
	}
}

func (cc *CaptureContext) SetRequestID(id string) {
	cc.RequestID = id
	cc.Recorder.RequestID = id
	cc.IDExtracted = true
}

func WithCaptureContext(ctx context.Context, cc *CaptureContext) context.Context {
	return context.WithValue(ctx, captureContextKey, cc)
}

func GetCaptureContext(ctx context.Context) *CaptureContext {
	if cc, ok := ctx.Value(captureContextKey).(*CaptureContext); ok {
		return cc
	}
	return nil
}

func extractRequestID(r *http.Request) string {
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	if id := r.Header.Get("x-request-id"); id != "" {
		return id
	}

	if r.Method == "POST" && r.Body != nil {
		return ""
	}

	return ""
}

func ExtractRequestIDFromSSEChunk(body []byte) string {
	var openAIChunk struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &openAIChunk); err == nil && openAIChunk.ID != "" {
		return openAIChunk.ID
	}

	var anthropicChunk struct {
		Type    string `json:"type"`
		Message struct {
			ID string `json:"id"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &anthropicChunk); err == nil && anthropicChunk.Message.ID != "" {
		return anthropicChunk.Message.ID
	}

	return ""
}

func SanitizeHeaders(headers http.Header) map[string]string {
	sensitive := map[string]bool{
		"authorization": true,
		"x-api-key":     true,
		"cookie":        true,
		"set-cookie":    true,
		"x-auth-token":  true,
	}

	result := make(map[string]string)
	for k, v := range headers {
		keyLower := strings.ToLower(k)
		if sensitive[keyLower] {
			result[k] = "***"
		} else if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

func OffsetMS(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

type CaptureWriter interface {
	RecordChunk(event string, data []byte)
	Chunks() []SSEChunk
}

type captureWriter struct {
	start  time.Time
	chunks []SSEChunk
}

func NewCaptureWriter(start time.Time) CaptureWriter {
	return &captureWriter{
		start:  start,
		chunks: []SSEChunk{},
	}
}

func (cw *captureWriter) RecordChunk(event string, data []byte) {
	if len(data) == 0 {
		return
	}
	chunk := NewSSEChunk(OffsetMS(cw.start), event, data)
	cw.chunks = append(cw.chunks, chunk)
}

func (cw *captureWriter) Chunks() []SSEChunk {
	return cw.chunks
}
