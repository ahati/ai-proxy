package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ai-proxy/capture"
	"ai-proxy/logging"
	"ai-proxy/proxy"
	"ai-proxy/transform"

	"github.com/gin-gonic/gin"
	"github.com/tmaxmax/go-sse"
)

type upstreamClient interface {
	BuildRequest(ctx context.Context, body []byte) (*http.Request, error)
	SetHeaders(req *http.Request)
	SetHeadersNonStreaming(req *http.Request)
	Do(req *http.Request) (*http.Response, error)
	Close()
}

var newUpstreamClient = func(baseURL, apiKey string) upstreamClient {
	return proxy.NewClient(baseURL, apiKey)
}

// timingCaptureWriter wraps an io.Writer and captures SSE events with accurate timing.
// It detects complete SSE events (ending with "\n\n") and records them to a CaptureWriter
// at the moment they are written, preserving correct offset_ms timing.
//
// Thread Safety: NOT thread-safe. Use from single goroutine.
type timingCaptureWriter struct {
	// underlying writer for client responses
	w io.Writer
	// capture writer for recording events with timing
	cw capture.CaptureWriter
	// buffer for accumulating partial SSE events
	buf bytes.Buffer
}

// newTimingCaptureWriter creates a writer that captures SSE events with timing.
func newTimingCaptureWriter(w io.Writer, cw capture.CaptureWriter) *timingCaptureWriter {
	return &timingCaptureWriter{
		w:  w,
		cw: cw,
	}
}

// Write implements io.Writer. It forwards data to the underlying writer and
// captures complete SSE events for timing-accurate recording.
func (tcw *timingCaptureWriter) Write(p []byte) (n int, err error) {
	// Write to underlying writer first
	n, err = tcw.w.Write(p)
	if err != nil {
		return n, err
	}

	// Accumulate data for SSE parsing
	tcw.buf.Write(p)

	// Parse and record any complete SSE events
	tcw.parseAndRecordEvents()

	return n, nil
}

// parseAndRecordEvents parses complete SSE events from the buffer and records them.
// SSE events are delimited by "\n\n". Each event may have "event:" and "data:" lines.
func (tcw *timingCaptureWriter) parseAndRecordEvents() {
	data := tcw.buf.Bytes()

	// Find complete events (ending with \n\n)
	for {
		idx := bytes.Index(data, []byte("\n\n"))
		if idx == -1 {
			break
		}

		// Extract the complete event
		event := data[:idx]
		data = data[idx+2:] // Skip past \n\n

		// Parse event type and data
		eventType, eventData := parseSSEEvent(event)
		if len(eventData) > 0 {
			tcw.cw.RecordChunk(eventType, eventData)
		}
	}

	// Keep remaining partial data in buffer
	tcw.buf.Reset()
	tcw.buf.Write(data)
}

// FlushRemaining flushes any remaining buffered data as a final chunk.
// This ensures partial events are captured when the stream ends unexpectedly.
func (tcw *timingCaptureWriter) FlushRemaining() {
	data := tcw.buf.Bytes()
	if len(data) > 0 {
		// First, try to parse any complete events
		tcw.parseAndRecordEvents()

		// If there's still data in buffer, it might be a partial event
		// Record it as raw data so it's not lost
		remaining := tcw.buf.Bytes()
		if len(remaining) > 0 {
			tcw.cw.RecordChunk("", remaining)
		}
	}
}

// parseSSEEvent extracts the event type and data from an SSE event string.
// SSE format: "event: type\ndata: {...}" or just "data: {...}"
func parseSSEEvent(event []byte) (eventType string, data []byte) {
	lines := bytes.Split(event, []byte("\n"))
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("event:")) {
			eventType = string(bytes.TrimSpace(line[6:]))
		} else if bytes.HasPrefix(line, []byte("data:")) {
			data = bytes.TrimSpace(line[5:])
		}
	}
	return eventType, data
}

// Handle wraps a Handler implementation and returns a Gin handler function.
// It orchestrates the full request pipeline: reading, validating, transforming, and proxying.
//
// The processing flow is:
//  1. Read request body from client
//  2. Record downstream request for capture
//  3. Validate request format
//  4. Transform request to upstream format
//  5. Forward to upstream and stream response back
//
// @param h - Handler implementation defining endpoint-specific behavior.
//
//	Must not be nil. Handler methods are called in sequence.
//
// @return Gin handler function that processes requests through the handler pipeline.
//
// @pre h != nil
// @post Response is fully written to client on return (success or error).
func Handle(h Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Step 1: Read the complete request body for processing
		body, err := readBody(c)
		if err != nil {
			// Body read failure indicates client disconnected or malformed request
			h.WriteError(c, http.StatusBadRequest, "Failed to read request body")
			return
		}

		// Step 2: Record downstream request for capture/logging
		// This captures the original request before any transformation
		capture.RecordDownstreamRequest(c.Request.Context(), c.Request, body)

		// Step 3: Validate request format and semantics
		if err := h.ValidateRequest(body); err != nil {
			// Validation failure indicates client error (400-level response)
			h.WriteError(c, http.StatusBadRequest, err.Error())
			return
		}

		// Step 4: Transform request to upstream format
		transformedBody, err := h.TransformRequest(c.Request.Context(), body)
		if err != nil {
			// Transformation failure indicates internal error
			h.WriteError(c, http.StatusInternalServerError, "Failed to transform request")
			return
		}

		// Step 5: Forward to upstream and stream response
		proxyRequest(c, h, transformedBody, body)
	}
}

// readBody reads and returns the entire request body.
// The body is consumed and cannot be read again.
//
// @param c - Gin context containing the HTTP request.
// @return Complete request body bytes, or error if read fails.
//
// @pre c.Request.Body != nil
// @post c.Request.Body is fully consumed and closed.
// @note Returns empty slice for empty body, not nil.
func readBody(c *gin.Context) ([]byte, error) {
	return io.ReadAll(c.Request.Body)
}

// upstreamResult holds the prepared upstream client, request, and metadata.
// Returned by prepareUpstreamRequest for use by the proxy pipeline.
//
// @note Caller must invoke cleanup() when done to release connection resources.
type upstreamResult struct {
	client    upstreamClient
	req       *http.Request
	streaming bool
}

// prepareUpstreamRequest resolves API credentials, creates the upstream HTTP client,
// builds the request with appropriate headers, and determines streaming mode.
//
// This consolidates client setup, request building, and header configuration into
// a single preparation step, keeping the orchestrator free of infrastructure details.
//
// @param c - Gin context for the current request.
// @param h - Handler providing upstream URL, API key, and header forwarding.
// @param body - Transformed request body to send upstream.
// @param originalBody - Original (pre-transform) body used to determine streaming mode.
// @return Prepared upstream result, cleanup function, or error if setup fails.
//
// @pre h != nil, body contains valid upstream-format JSON.
// @post On success, result.req has all headers set and is ready to execute.
// @post cleanup() must be called to release the upstream client connection pool.
func prepareUpstreamRequest(
	c *gin.Context, h Handler, body []byte, originalBody []byte,
) (*upstreamResult, func(), error) {
	apiKey := h.ResolveAPIKey(c)

	downstreamModel, upstreamModel := h.ModelInfo()
	logging.InfoMsg("Sending request to upstream: %s (downstream_model=%s, upstream_model=%s)",
		h.UpstreamURL(), downstreamModel, upstreamModel)

	// Capture upstream URL for logging
	if cc := capture.GetCaptureContext(c.Request.Context()); cc != nil {
		cc.Recorder.SetUpstreamURL(h.UpstreamURL())
	}

	client := newUpstreamClient(h.UpstreamURL(), apiKey)

	req, err := client.BuildRequest(c.Request.Context(), body)
	if err != nil {
		client.Close()
		return nil, func() {}, err
	}

	streaming := isStreamingRequest(originalBody)
	if streaming {
		client.SetHeaders(req)
	} else {
		client.SetHeadersNonStreaming(req)
	}
	h.ForwardHeaders(c, req)

	result := &upstreamResult{
		client:    client,
		req:       req,
		streaming: streaming,
	}
	cleanup := func() { client.Close() }

	return result, cleanup, nil
}

// executeUpstream sends the request to the upstream API and validates the response.
// On error, it writes an appropriate error response to the client via the handler.
//
// This eliminates duplication of the Do → status check → handleUpstreamError pattern
// between the streaming and non-streaming code paths.
//
// @param c - Gin context for writing error responses.
// @param h - Handler for formatting error responses.
// @param client - Configured upstream HTTP client.
// @param req - Fully prepared upstream request with headers set.
// @return Upstream HTTP response on success, or error with response already written.
//
// @pre req has all headers set and body populated.
// @post On success, caller must close resp.Body. On error, response is already written.
func executeUpstream(c *gin.Context, h Handler, client upstreamClient, req *http.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		// Upstream connection failure indicates gateway error
		h.WriteError(c, http.StatusBadGateway, "Upstream request failed")
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		handleUpstreamError(c, resp)
		return nil, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	return resp, nil
}

// streamingPipeline holds the initialized transformer and manages its lifecycle,
// including capture context coordination and stream cancellation registration.
//
// The pipeline consolidates three previously-scattered concerns:
//   - Transformer creation and initialization (capture vs non-capture)
//   - Stream cancellation registration with the global registry
//   - Resource cleanup via a single cleanup function
//
// @note cleanup() must be called when the pipeline is no longer needed.
type streamingPipeline struct {
	transformer transform.SSETransformer
	cc          *capture.CaptureContext
}

// initStreamingPipeline creates and initializes the streaming pipeline.
//
// Behavior depends on capture mode:
//   - Capture mode (cc != nil): records upstream request metadata; transformer
//     initialization is deferred to the streaming phase inside c.Stream.
//   - Non-capture mode: creates, configures, and initializes the transformer
//     immediately, and registers it for cancellation if it exposes a response ID.
//
// @param c - Gin context for the current request.
// @param h - Handler providing transformer creation.
// @param req - Upstream request (used to record headers in capture mode).
// @return Initialized pipeline, cleanup function, or error if setup fails.
//
// @pre The request headers have been fully set (SetHeaders + ForwardHeaders called).
// @post cleanup() will close transformer, unregister from stream registry, and cancel context.
func initStreamingPipeline(
	c *gin.Context, h Handler, req *http.Request,
) (*streamingPipeline, func(), error) {
	cc := capture.GetCaptureContext(c.Request.Context())
	pipeline := &streamingPipeline{cc: cc}

	var responseID string
	var transformerCleanup func()

	if cc != nil {
		// Record upstream request headers and body for capture now that all headers are set.
		// This must happen after both SetHeaders and ForwardHeaders to capture the complete set.
		var body []byte
		if cc.Recorder.Data().UpstreamRequest != nil {
			body = cc.Recorder.Data().UpstreamRequest.Body
		}
		cc.Recorder.RecordUpstreamRequest(req.Header, body)

		// Transformer is nil for capture mode; created inside c.Stream callback.
		transformerCleanup = func() {}
	} else {
		transformer := h.CreateTransformer(c.Writer)
		setContextOnTransformer(transformer, c.Request.Context())

		if err := transformer.Initialize(); err != nil {
			logging.ErrorMsg("Failed to initialize transformer: %v", err)
			return nil, func() {}, fmt.Errorf("transformer initialization failed: %w", err)
		}

		pipeline.transformer = transformer

		if getter, ok := transformer.(transform.ResponseIDGetter); ok {
			responseID = getter.GetResponseID()
		}

		transformerCleanup = func() { transformer.Close() }
	}

	// Register stream for cancellation support if we have a response ID
	var cancel context.CancelFunc
	var registryCleanup func()

	if responseID != "" {
		registry := GetGlobalRegistry()
		var streamCtx context.Context
		streamCtx, cancel = context.WithCancel(c.Request.Context())
		c.Request = c.Request.WithContext(streamCtx)
		registry.Register(responseID, cancel, pipeline.transformer)
		registryCleanup = func() {
			registry.Remove(responseID)
			if cancel != nil {
				cancel()
			}
		}
	} else {
		registryCleanup = func() {}
	}

	cleanup := func() {
		transformerCleanup()
		registryCleanup()
	}

	return pipeline, cleanup, nil
}

// stream dispatches to the appropriate streaming method based on capture mode.
//
// @param c - Gin context for writing the response.
// @param h - Handler providing transformer creation for capture mode.
// @param body - Reader for the upstream SSE response body.
func (p *streamingPipeline) stream(c *gin.Context, body io.Reader, h Handler) {
	if p.cc != nil {
		streamWithCapture(c, body, h, p.cc)
	} else {
		streamWithInitializedTransformer(c, body, p.transformer)
	}
}

// proxyRequest orchestrates the full upstream proxy flow: preparing the request,
// executing it, and streaming or returning the response.
//
// The function is a thin coordinator that delegates to focused helpers:
//   - prepareUpstreamRequest: client setup, request building, header configuration
//   - executeUpstream: HTTP execution with error handling (shared with non-streaming path)
//   - initStreamingPipeline: transformer lifecycle, capture, and cancellation
//
// @param c - Gin context for the current request.
// @param h - Handler defining endpoint-specific behavior.
// @param body - Transformed request body in upstream format.
// @param originalBody - Original request body (used to determine streaming mode).
//
// @pre body has been validated and transformed by the handler.
// @post Response is fully written to client (success, error, or streamed SSE).
func proxyRequest(c *gin.Context, h Handler, body []byte, originalBody []byte) {
	// Phase 1: Prepare upstream client, request, and headers
	result, cleanup, err := prepareUpstreamRequest(c, h, body, originalBody)
	if err != nil {
		h.WriteError(c, http.StatusInternalServerError, "Failed to create upstream request")
		return
	}
	defer cleanup()

	// Phase 2: Non-streaming path — simple JSON proxy
	if !result.streaming {
		proxyJSONRequest(c, h, result.client, result.req)
		return
	}

	// Phase 3: Initialize streaming pipeline (transformer + cancellation)
	pipeline, pipelineCleanup, err := initStreamingPipeline(c, h, result.req)
	if err != nil {
		h.WriteError(c, http.StatusInternalServerError, "Failed to initialize response stream")
		return
	}
	defer pipelineCleanup()

	// Phase 4: Execute upstream request
	resp, err := executeUpstream(c, h, result.client, result.req)
	if err != nil {
		return // error already written by executeUpstream
	}
	defer resp.Body.Close()

	// Phase 5: Configure response headers and stream to client
	forwardResponseHeaders(c, resp)
	setStreamHeaders(c)
	pipeline.stream(c, resp.Body, h)
}

func setStreamHeaders(c *gin.Context) {
	// Content-Type for Server-Sent Events
	c.Header("Content-Type", "text/event-stream")
	// Prevent caching to ensure real-time delivery
	c.Header("Cache-Control", "no-cache")
	// Keep connection open for streaming
	c.Header("Connection", "keep-alive")
}

// isStreamingRequest checks the transformed request body for the "stream" field.
// Returns true when streaming (default), false only when explicitly set to false.
//
// @param body - Transformed request body bytes (JSON).
// @return true for streaming (default), false only when stream:false is explicit.
func isStreamingRequest(body []byte) bool {
	var req struct {
		Stream *bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return true // Default to streaming on parse failure (backward compatible)
	}
	if req.Stream == nil {
		return true // Default to streaming when field absent (backward compatible)
	}
	return *req.Stream
}

// proxyJSONRequest handles non-streaming (stream:false) requests.
// It executes the upstream request and returns the plain JSON response
// directly to the client without SSE transformation.
//
// Uses executeUpstream for consistent error handling with the streaming path.
//
// @param c - Gin context for writing the response.
// @param h - Handler defining upstream URL, headers, and error handling.
// @param client - Configured upstream HTTP client.
// @param req - Pre-built upstream HTTP request with headers already set.
//
// @pre req was created by client.BuildRequest and headers set by caller.
// @post Response is returned to client as JSON or error response is sent.
func proxyJSONRequest(c *gin.Context, h Handler, client upstreamClient, req *http.Request) {
	resp, err := executeUpstream(c, h, client, req)
	if err != nil {
		return // error already written by executeUpstream
	}
	defer resp.Body.Close()

	// Forward upstream response headers to downstream
	forwardResponseHeaders(c, resp)

	// Read the full JSON response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		h.WriteError(c, http.StatusInternalServerError, "Failed to read upstream response")
		return
	}

	// Return JSON response directly to the client
	c.Data(resp.StatusCode, "application/json", responseBody)
}

// replacedHeaders lists headers that the proxy replaces with its own values.
// These are set by SetHeaders and must not be overwritten by client headers.
var replacedHeaders = map[string]bool{
	"authorization":     true, // Replaced with proxy API key
	"content-type":      true, // Set to application/json
	"accept":            true, // Set to text/event-stream
	"content-length":    true, // Set by Go from body
	"host":              true, // Set by Go from URL
	"connection":        true, // Hop-by-hop header
	"keep-alive":        true, // Hop-by-hop header
	"transfer-encoding": true, // Hop-by-hop header
	"upgrade":           true, // Hop-by-hop header
}

// forwardCustomHeaders copies all headers from the incoming request to the upstream
// request, except for headers that are replaced by the proxy (denylist).
// This ensures client metadata like User-Agent passes through automatically.
//
// @param c - Gin context containing the original request headers.
// @param req - Upstream request to receive forwarded headers.
func forwardCustomHeaders(c *gin.Context, req *http.Request) {
	for key, values := range c.Request.Header {
		if replacedHeaders[strings.ToLower(key)] {
			continue
		}
		req.Header[key] = values
	}
}

// forwardResponseHeaders copies all headers from the upstream response to the
// downstream response. Headers that the proxy needs to override (Content-Type,
// Cache-Control, etc.) are set afterwards by setStreamHeaders or c.Data.
//
// @param c - Gin context for writing downstream response headers.
// @param upstream - Upstream response whose headers to forward.
func forwardResponseHeaders(c *gin.Context, upstream *http.Response) {
	for key, values := range upstream.Header {
		c.Header(key, values[0])
	}
}

// streamWithCapture streams the response while capturing both upstream and downstream
// data for logging and analysis.
//
// @param c - Gin context for writing the response.
// @param body - Reader for upstream SSE response body.
// @param h - Handler providing the SSE transformer.
// @param cc - Capture context for recording request/response data.
//
// @pre body is a valid SSE stream reader.
// @pre cc != nil and is properly initialized.
// @post All events are captured in cc.Recorder.
func streamWithCapture(c *gin.Context, body io.Reader, h Handler, cc *capture.CaptureContext) {
	startTime := cc.StartTime
	// Create capture writer for downstream (transformed) events
	downstream := capture.NewCaptureWriter(startTime)
	// Create capture writer for upstream (original) events
	upstream := capture.NewCaptureWriter(startTime)

	// Stream events with capture
	// Get flusher from Gin response writer for immediate delivery
	flusher, canFlush := c.Writer.(http.Flusher)

	c.Stream(func(w io.Writer) bool {
		// Create timing-aware writer that captures downstream events with correct timing
		timingWriter := newTimingCaptureWriter(w, downstream)
		// Create transformer that writes to our timing-aware writer
		transformer := h.CreateTransformer(timingWriter)
		// Set context for cache status tracking
		setContextOnTransformer(transformer, c.Request.Context())
		defer func() {
			timingWriter.FlushRemaining()
			transformer.Close()
			// Flush final events (completion, [DONE]) to ensure they reach the client
			// before the response writer is closed.
			if canFlush {
				flusher.Flush()
			}
		}()

		// Initialize transformer and emit response.created BEFORE reading from upstream
		if err := transformer.Initialize(); err != nil {
			logging.ErrorMsg("Failed to initialize transformer in capture mode: %v", err)
			emitStreamError(transformer, err)
			return false
		}

		// Iterate over all SSE events from upstream
		for ev, err := range sse.Read(body, nil) {
			if err != nil {
				// Context canceled means client disconnected - can't send response.failed
				if errors.Is(err, context.Canceled) {
					logging.DebugMsg("Stream completed, client disconnected")
					return false
				}
				logging.ErrorMsg("SSE stream error (capture): %v", err)
				emitStreamError(transformer, err)
				return false
			}
			// Capture upstream events before transformation
			// Only record if event has data (skip empty keepalive events)
			if ev.Data != "" {
				recordUpstreamEvent(upstream, ev)
			}
			// Transform and send event to client (timing captured by timingWriter)
			if err := transformer.Transform(&ev); err != nil {
				logging.ErrorMsg("Transform error (capture): %v", err)
				emitStreamError(transformer, err)
				return false
			}

			// Flush after each event to ensure immediate delivery
			// This prevents buffering that causes clients to timeout
			if canFlush {
				flusher.Flush()
			}
		}
		return false
	})

	// Finalize capture by recording all captured data
	finalizeCapture(cc, downstream, upstream, c.Writer.Header())
}

// streamWithInitializedTransformer streams events using an already-initialized transformer.
// This is used in non-capture mode where Initialize() was called before the upstream request.
func streamWithInitializedTransformer(c *gin.Context, body io.Reader, transformer transform.SSETransformer) {
	// Stream events without capture overhead
	// Get flusher from Gin response writer for immediate delivery
	flusher, canFlush := c.Writer.(http.Flusher)

	c.Stream(func(w io.Writer) bool {
		for ev, err := range sse.Read(body, nil) {
			if err != nil {
				// Context canceled means client disconnected - can't send response.failed
				if errors.Is(err, context.Canceled) {
					logging.DebugMsg("Stream completed, client disconnected")
					return false
				}
				logging.ErrorMsg("SSE stream error (no-capture): %v", err)
				emitStreamError(transformer, err)
				return false
			}
			// Transform and send event directly to client
			if err := transformer.Transform(&ev); err != nil {
				logging.ErrorMsg("Transform error (no-capture): %v", err)
				emitStreamError(transformer, err)
				return false
			}

			// Flush after each event to ensure immediate delivery
			if canFlush {
				flusher.Flush()
			}
		}
		// Stream ended normally — close the transformer to emit final events
		// (output_item.done, response.completed, [DONE], etc.) and flush them
		// to the client before the response writer is released.
		transformer.Close()
		if canFlush {
			flusher.Flush()
		}
		return false
	})
}

func recordUpstreamEvent(w capture.CaptureWriter, ev sse.Event) {
	// Only record events with data - skip empty keepalive events
	if ev.Data != "" {
		w.RecordChunk(ev.Type, []byte(ev.Data))
	}
}

// emitStreamError sends a response.failed event to the client before closing.
// This ensures clients receive proper notification of stream failures.
func emitStreamError(transformer transform.SSETransformer, err error) {
	// Type assert to check if transformer supports error emission
	if et, ok := transformer.(interface{ EmitError(error) error }); ok {
		if emitErr := et.EmitError(err); emitErr != nil {
			logging.ErrorMsg("Failed to emit error event: %v", emitErr)
		}
	}
}

// setContextOnTransformer sets the context on transformers that support it.
// This enables cache status tracking during response transformation.
func setContextOnTransformer(transformer transform.SSETransformer, ctx context.Context) {
	if ct, ok := transformer.(interface{ SetContext(context.Context) }); ok {
		ct.SetContext(ctx)
	}
}

// finalizeCapture completes the capture process by recording response data
// and extracting request IDs from the SSE stream.
//
// @param cc - Capture context to finalize.
// @param downstream - Writer containing captured downstream events.
// @param upstream - Writer containing captured upstream events.
// @param headers - Downstream response headers to capture.
// @pre cc != nil and has been recording the request.
// @post cc.Recorder contains complete downstream and upstream response data.
// @post cc.RequestID is set if found in SSE stream.
func finalizeCapture(cc *capture.CaptureContext, downstream, upstream capture.CaptureWriter, headers http.Header) {
	// Get immutable snapshots of captured chunks
	// This is now thread-safe via atomic snapshot in Chunks()
	downstreamChunks := downstream.Chunks()
	upstreamChunks := upstream.Chunks()

	// Log chunk counts for debugging
	logging.DebugMsg("finalizeCapture: %d downstream chunks, %d upstream chunks",
		len(downstreamChunks), len(upstreamChunks))

	// Record downstream response (transformed events sent to client)
	// Use thread-safe method instead of direct field access
	downstreamRecorder := cc.Recorder.RecordDownstreamResponse(headers)
	// Transfer captured chunks directly, preserving their original timing
	// The chunks already have correct OffsetMS from when they were recorded during streaming
	for _, chunk := range downstreamChunks {
		downstreamRecorder.RecordChunkPreservingTiming(chunk)
	}

	// Record upstream response (original events from upstream)
	// The upstream response was already initialized in proxy/client.go with headers
	// We just need to add chunks to it, not recreate it
	if upstreamRecorder := cc.Recorder.GetUpstreamResponseRecorder(); upstreamRecorder != nil {
		// Transfer captured chunks directly, preserving their original timing
		for _, chunk := range upstreamChunks {
			upstreamRecorder.RecordChunkPreservingTiming(chunk)
		}
	} else {
		logging.InfoMsg("finalizeCapture: upstream recorder is nil - chunks may be lost")
	}

	// Extract request ID from SSE stream if not already found
	// Request ID is typically in the first SSE event from LLM APIs
	if !cc.IDExtracted {
		for _, chunk := range downstreamChunks {
			// Attempt to extract ID from each chunk until found
			if id := capture.ExtractRequestIDFromSSEChunk(chunk.Data); id != "" {
				cc.SetRequestID(id)
				// Stop after finding the first ID
				break
			}
		}
	}

	// Extract and log token usage from captured chunks
	// This provides a compact summary of request costs in a powerline-style format
	upstreamUsage := capture.ExtractTokenUsageFromChunks(upstreamChunks)
	downstreamUsage := capture.ExtractTokenUsageFromChunks(downstreamChunks)

	// Extract finish reasons from both upstream and downstream chunks
	// Upstream: reason from LLM API, Downstream: reason sent to client (may differ after transformation)
	upstreamReason := capture.ExtractFinishReasonFromChunks(upstreamChunks)
	downstreamReason := capture.ExtractFinishReasonFromChunks(downstreamChunks)

	// Build cache status indicators (separate items)
	var cacheParts []string
	if cc.CacheHit {
		cacheParts = append(cacheParts, "🗄️ cache-hit")
	}
	if cc.CacheCreated {
		cacheParts = append(cacheParts, "🗃️ cache-created")
	}
	cacheStatus := strings.Join(cacheParts, " ")

	// Compact one-line log with emojis:
	// 📤 = upstream (to LLM), 📥 = downstream (to client)
	// ⬆️ = input tokens, ⬇️ = output tokens, 📖 = cache read, 💾  = cache creation
	if cacheStatus != "" {
		logging.InfoMsg("|📤 ⬆️ %d ⬇️ %d 📖 %d 💾 %d r=%s|  |📥 ⬆️ %d ⬇️ %d 📖 %d 💾 %d r=%s| %s [%s] [%s]",
			upstreamUsage.InputTokens,
			upstreamUsage.OutputTokens,
			upstreamUsage.CacheReadTokens,
			upstreamUsage.CacheCreationTokens,
			upstreamReason,
			downstreamUsage.InputTokens,
			downstreamUsage.OutputTokens,
			downstreamUsage.CacheReadTokens,
			downstreamUsage.CacheCreationTokens,
			downstreamReason,
			cacheStatus,
			cc.SessionID,
			cc.RequestID,
		)
	} else {
		logging.InfoMsg("|📤 ⬆️ %d ⬇️ %d 📖 %d 💾 %d r=%s|  |📥 ⬆️ %d ⬇️ %d 📖 %d 💾 %d r=%s| [%s] [%s]",
			upstreamUsage.InputTokens,
			upstreamUsage.OutputTokens,
			upstreamUsage.CacheReadTokens,
			upstreamUsage.CacheCreationTokens,
			upstreamReason,
			downstreamUsage.InputTokens,
			downstreamUsage.OutputTokens,
			downstreamUsage.CacheReadTokens,
			downstreamUsage.CacheCreationTokens,
			downstreamReason,
			cc.SessionID,
			cc.RequestID,
		)
	}
}

// handleUpstreamError processes an error response from the upstream API
// and sends it to the client.
//
// @param c - Gin context for writing the error response.
// @param resp - Error response from upstream.
//
// @pre resp != nil and resp.Body is readable.
// @post Error response is sent to client in OpenAI error format.
// @post Capture is finalized with error response data if capture is enabled.
func handleUpstreamError(c *gin.Context, resp *http.Response) {
	// Read the error body for inclusion in client error message
	body, _ := io.ReadAll(resp.Body)
	msg := string(body)

	// Record the upstream error for capture
	c.Set("upstream_error_body", msg)
	c.Set("upstream_error_status", resp.StatusCode)

	// Finalize capture for error responses
	// This ensures error responses are logged with their status and body
	// Note: RecordUpstreamResponse was already called in proxy/client.go Do(),
	// so we only need to add the error body as a chunk.
	if c.Request != nil {
		cc := capture.GetCaptureContext(c.Request.Context())
		if cc != nil {
			// Append the error body to the already-recorded upstream response
			upstreamResp := cc.Recorder.GetUpstreamResponseRecorder()
			if upstreamResp != nil && len(body) > 0 {
				upstreamResp.RecordChunkBytes("", body)
			}

			// Create empty capture writers (no downstream streaming for errors)
			downstream := capture.NewCaptureWriter(cc.StartTime)
			upstream := capture.NewCaptureWriter(cc.StartTime)

			// Finalize capture to store the error response
			finalizeCapture(cc, downstream, upstream, c.Writer.Header())
		}
	}

	// Send error in OpenAI format with the original upstream status code
	// This preserves error details like 400 (bad request), 401 (auth), 429 (rate limit)
	sendOpenAIError(c, resp.StatusCode, msg)
}

// sendOpenAIError sends an error response in OpenAI API format.
// OpenAI format: {"error": {"message": "...", "type": "..."}}
//
// @param c - Gin context for writing the response.
// @param status - HTTP status code for the error.
// @param msg - Human-readable error message.
//
// @pre c != nil and response has not been written.
// @post JSON error response is written and flushed.
func sendOpenAIError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": msg,
			"type":    "invalid_request_error",
		},
	})
}

// sendAnthropicError sends an error response in Anthropic API format.
// Anthropic format: {"type": "error", "error": {"type": "...", "message": "..."}}
//
// @param c - Gin context for writing the response.
// @param status - HTTP status code for the error.
// @param msg - Human-readable error message.
//
// @pre c != nil and response has not been written.
// @post JSON error response is written and flushed.
func sendAnthropicError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    "invalid_request_error",
			"message": msg,
		},
	})
}

// sendOpenAIResponsesError sends an error response in OpenAI Responses API format.
// OpenAI Responses API uses SSE format for errors.
//
// @param c - Gin context for writing the response.
// @param status - HTTP status code for the error.
// @param msg - Human-readable error message.
//
// @pre c != nil and response has not been written.
// @post SSE error response is written and flushed.
func sendOpenAIResponsesError(c *gin.Context, status int, msg string) {
	event := map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"code":    "invalid_request_error",
			"message": msg,
		},
	}
	c.Header("Content-Type", "text/event-stream")
	data, _ := json.Marshal(event)
	c.String(status, "data: "+string(data)+"\n\n")
}
