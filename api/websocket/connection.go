package websocket

import (
	"ai-proxy/api/pipeline"
	"ai-proxy/capture"
	"ai-proxy/config"
	"ai-proxy/conversation"
	"ai-proxy/logging"
	"ai-proxy/router"
	"ai-proxy/transform"
	"ai-proxy/types"
	"ai-proxy/websearch"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tmaxmax/go-sse"
)

const (
	ConnectionDurationLimit = 60 * time.Minute
	ReadDeadline            = 120 * time.Second
	WriteDeadline           = 30 * time.Second
	WarningTimeBeforeLimit  = 55 * time.Minute
)

// Connection bridges the client's WebSocket to the upstream HTTP/SSE API.
type Connection struct {
	ID              string
	clientConn      *websocket.Conn
	config          *config.Config
	manager         *config.ConfigManager
	router          router.Router
	authHeader      string
	defaultModel    string
	captureRecorder *capture.Recorder

	previousResponseID string
	currentModel       string
	startTime          time.Time
	mu                 sync.Mutex
	active             bool
	done               chan struct{}

	// history accumulates input+output items across turns within this connection.
	// Each turn's input and assistant output are appended so the next turn has
	// full conversation context without hitting the conversation store.
	history []types.InputItem

	// responseIDs tracks all conversation IDs generated in this connection.
	// Used for cleanup of non-persisted conversations on Close().
	responseIDs []string
}

type ConnectionManager struct {
	connections map[string]*Connection
	mu          sync.RWMutex
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{connections: make(map[string]*Connection)}
}

func (cm *ConnectionManager) NewConnection(
	clientConn *websocket.Conn,
	config *config.Config,
	manager *config.ConfigManager,
	router router.Router,
	authHeader string,
	model string,
	recorder *capture.Recorder,
) *Connection {
	conn := &Connection{
		ID:              generateConnectionID(),
		clientConn:      clientConn,
		config:          config,
		manager:         manager,
		router:          router,
		authHeader:      authHeader,
		defaultModel:    model,
		captureRecorder: recorder,
		startTime:       time.Now(),
		active:          true,
		done:            make(chan struct{}),
	}
	cm.mu.Lock()
	cm.connections[conn.ID] = conn
	cm.mu.Unlock()
	return conn
}

func (cm *ConnectionManager) Remove(id string) {
	cm.mu.Lock()
	delete(cm.connections, id)
	cm.mu.Unlock()
}

func (cm *ConnectionManager) Count() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.connections)
}

func (c *Connection) Run() {
	defer c.Close()
	go c.monitorDuration()
	c.readFromClient()
}

func (c *Connection) monitorDuration() {
	durationTimer := time.NewTimer(ConnectionDurationLimit)
	warningTimer := time.NewTimer(WarningTimeBeforeLimit)
	defer durationTimer.Stop()
	defer warningTimer.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-warningTimer.C:
			logging.InfoMsg("connection %s: approaching 60-minute limit", c.ID)
		case <-durationTimer.C:
			c.sendWSError(types.NewWSError(
				types.WSErrorWebsocketConnectionLimitReached,
				"Responses websocket connection limit reached (60 minutes). Create a new websocket connection to continue.",
				http.StatusBadRequest,
			))
			c.Close()
			return
		}
	}
}

func (c *Connection) readFromClient() {
	for {
		select {
		case <-c.done:
			return
		default:
		}
		c.clientConn.SetReadDeadline(time.Now().Add(ReadDeadline))
		_, message, err := c.clientConn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logging.ErrorMsg("websocket read error: %v", err)
			}
			c.Close()
			return
		}
		if err := c.handleClientMessage(message); err != nil {
			logging.ErrorMsg("handle client message error: %v", err)
		}
	}
}

func (c *Connection) handleClientMessage(message []byte) error {
	var req types.WSRequest
	if err := json.Unmarshal(message, &req); err != nil {
		return c.sendWSError(types.NewWSError(
			types.WSErrorInvalidRequest, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest,
		))
	}
	if req.Type != "response.create" {
		return c.sendWSError(types.NewWSError(
			types.WSErrorInvalidRequest, fmt.Sprintf("unknown event type: %s", req.Type), http.StatusBadRequest,
		))
	}

	model := req.Model
	if model == "" {
		model = c.defaultModel
	}
	if model == "" {
		return c.sendWSError(types.NewWSError(
			types.WSErrorInvalidRequest, "model is required", http.StatusBadRequest,
		))
	}
	c.mu.Lock()
	c.currentModel = model
	c.mu.Unlock()

	// Handle warmup (generate: false) — return synthetic response ID locally.
	// The upstream (Anthropic etc.) doesn't support warmup mode.
	if req.Generate != nil && !*req.Generate {
		return c.handleWarmup(&req)
	}

	if c.router == nil {
		return c.sendWSError(types.NewWSError(
			types.WSErrorInvalidRequest, "no router configured", http.StatusInternalServerError,
		))
	}

	route, err := c.router.ResolveWithProtocol(model, "responses")
	if err != nil {
		return c.sendWSError(types.NewWSError(
			types.WSErrorInvalidRequest, fmt.Sprintf("model resolution failed: %v", err), http.StatusBadRequest,
		))
	}

	// Prepend accumulated conversation history for within-connection multi-turn.
	// This gives the upstream model full context without hitting the store.
	// Only auto-fill previousResponseID for cross-connection resume (when history
	// is empty and the client didn't provide one).
	turnInput := req.Input
	c.mu.Lock()
	if len(c.history) > 0 {
		req.Input = append(c.history, req.Input...)
	} else if req.PreviousResponseID == "" && c.previousResponseID != "" {
		req.PreviousResponseID = c.previousResponseID
	}
	c.mu.Unlock()

	shouldStore := req.Store == nil || *req.Store

	responsesBody := c.buildResponsesBody(&req, route)
	upstreamBody, err := c.transformRequest(responsesBody, route, shouldStore)
	if err != nil {
		return c.sendWSError(types.NewWSError(
			types.WSErrorInvalidRequest, fmt.Sprintf("request transform failed: %v", err), http.StatusInternalServerError,
		))
	}

	upstreamURL := route.Provider.GetEndpoint(route.OutputProtocol)
	if upstreamURL == "" {
		return c.sendWSError(types.NewWSError(
			types.WSErrorInvalidRequest,
			fmt.Sprintf("no endpoint for protocol %s on provider %s", route.OutputProtocol, route.Provider.Name),
			http.StatusBadGateway,
		))
	}

	apiKey := route.Provider.GetAPIKey()

	logging.InfoMsg("[ws:%s] Sending request to upstream: %s (model=%s, upstream_model=%s, protocol=%s)",
		c.ID, upstreamURL, model, route.Model, route.OutputProtocol)

	c.recordDownstreamRequest(message, upstreamURL)

	return c.proxyRequest(upstreamURL, apiKey, upstreamBody, message, route, shouldStore, req.PreviousResponseID, turnInput)
}

// handleWarmup synthesizes a response ID for generate:false (warmup).
func (c *Connection) handleWarmup(req *types.WSRequest) error {
	warmupID := "resp_warmup_" + generateConnectionID()

	c.mu.Lock()
	c.previousResponseID = warmupID
	c.mu.Unlock()

	c.sendWSEvent(types.WSEvent{
		Type:       types.WSEventResponseCreated,
		ResponseID: warmupID,
	})

	c.sendWSEvent(types.WSEvent{
		Type:       types.WSEventResponseCompleted,
		ResponseID: warmupID,
		Response: &types.ResponsesResponse{
			ID:     warmupID,
			Object: "response",
			Model:  c.currentModel,
			Status: "completed",
		},
	})

	logging.DebugMsg("[ws:%s] warmup completed, response_id=%s", c.ID, warmupID)
	return nil
}

// sendWSEvent sends a single JSON event over the WebSocket connection.
// WebSocket mode uses raw JSON (not SSE wire format) per OpenAI spec.
func (c *Connection) sendWSEvent(data interface{}) {
	if c.clientConn == nil {
		return
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	c.clientConn.SetWriteDeadline(time.Now().Add(WriteDeadline))
	c.clientConn.WriteMessage(websocket.TextMessage, jsonBytes)
}

func (c *Connection) recordDownstreamRequest(wsMessage []byte, upstreamURL string) {
	if c.captureRecorder == nil {
		return
	}
	c.captureRecorder.SetUpstreamURL(upstreamURL)
	headers := http.Header{}
	headers.Set("Authorization", c.authHeader)
	c.captureRecorder.RecordDownstreamRequest(headers, wsMessage)
}

func (c *Connection) buildResponsesBody(req *types.WSRequest, route *router.ResolvedRoute) []byte {
	stream := true
	httpReq := types.ResponsesRequest{
		Model:              route.Model,
		Input:              inputItemsToInterface(req.Input),
		Instructions:       req.Instructions,
		MaxOutputTokens:    req.MaxOutputTokens,
		Store:              req.Store,
		PreviousResponseID: req.PreviousResponseID,
		Reasoning:          req.Reasoning,
		ResponseFormat:     req.ResponseFormat,
		Metadata:           req.Metadata,
		EncryptedReasoning: req.EncryptedReasoning,
		ToolChoice:         req.ToolChoice,
		Stream:             &stream,
	}
	if len(req.Tools) > 0 {
		httpReq.Tools = req.Tools
	}
	if req.Temperature != nil {
		httpReq.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		httpReq.TopP = *req.TopP
	}
	if route.SamplingParams != nil {
		if route.SamplingParams.Temperature != nil && req.Temperature == nil {
			httpReq.Temperature = *route.SamplingParams.Temperature
		}
		if route.SamplingParams.TopP != nil && req.TopP == nil {
			httpReq.TopP = *route.SamplingParams.TopP
		}
	}
	body, err := json.Marshal(httpReq)
	if err != nil {
		logging.ErrorMsg("[ws:%s] failed to marshal body: %v", c.ID, err)
		return nil
	}
	return body
}

func (c *Connection) transformRequest(body []byte, route *router.ResolvedRoute, store bool) ([]byte, error) {
	t, err := pipeline.BuildRequestPipeline(pipeline.RequestConfig{
		DownstreamFormat: "responses",
		UpstreamFormat:   route.OutputProtocol,
		ResolvedModel:    route.Model,
		IsPassthrough:    route.IsPassthrough,
		ReasoningSplit:   route.ReasoningSplit,
		Store:            store,
		SamplingParams:   route.SamplingParams,
	})
	if err != nil {
		return nil, err
	}
	return t(context.Background(), body)
}

func (c *Connection) proxyRequest(
	upstreamURL, apiKey string,
	upstreamBody []byte,
	downstreamWSMessage []byte,
	route *router.ResolvedRoute,
	store bool,
	previousResponseID string,
	turnInput []types.InputItem,
) error {
	// Generate per-turn unique request ID for capture logs so each turn
	// within a WS connection gets a distinct ID in /ui/api/logs.
	if c.captureRecorder != nil {
		c.captureRecorder.SetRequestID(uuid.New().String()[:8])
	}

	timeout := c.config.UpstreamTimeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(upstreamBody))
	if err != nil {
		return c.sendWSError(types.NewWSError(
			types.WSErrorInvalidRequest, fmt.Sprintf("failed to create upstream request: %v", err), http.StatusInternalServerError,
		))
	}

	upstreamHeaders := http.Header{}
	upstreamHeaders.Set("Content-Type", "application/json")
	upstreamHeaders.Set("Authorization", "Bearer "+apiKey)
	upstreamHeaders.Set("Accept", "text/event-stream")
	req.Header = upstreamHeaders

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.sendWSError(types.NewWSError(
			types.WSErrorInvalidRequest, fmt.Sprintf("upstream request failed: %v", err), http.StatusBadGateway,
		))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return c.sendWSError(types.NewWSError(
			types.WSErrorInvalidRequest,
			fmt.Sprintf("upstream returned status %d: %s", resp.StatusCode, string(respBody)),
			resp.StatusCode,
		))
	}

	c.recordUpstreamRequest(upstreamHeaders, upstreamBody)

	now := time.Now()
	upstreamCW := capture.NewCaptureWriter(now)
	downstreamCW := capture.NewCaptureWriter(now)

	if c.captureRecorder != nil {
		c.captureRecorder.RecordUpstreamResponse(resp.StatusCode, resp.Header)
	}

	wsWriter := &wsMessageWriter{
		conn:          c,
		captureWriter: downstreamCW,
	}

	t, err := pipeline.BuildPipeline(wsWriter, transform.Config{
		UpstreamFormat:        route.OutputProtocol,
		DownstreamFormat:      "responses",
		KimiToolCallTransform: route.KimiToolCallTransform,
		GLM5ToolCallTransform: route.GLM5ToolCallTransform,
		ReasoningSplit:        route.ReasoningSplit,
		WebSearchEnabled:      websearch.GetDefaultAdapter() != nil && websearch.GetDefaultAdapter().IsEnabled(),
		InputItems:            turnInput,
		Store:                 store,
		PreviousResponseID:    previousResponseID,
	})
	if err != nil {
		return c.sendWSError(types.NewWSError(
			types.WSErrorInvalidRequest, fmt.Sprintf("failed to build response pipeline: %v", err), http.StatusInternalServerError,
		))
	}

	if err := t.Initialize(); err != nil {
		return c.sendWSError(types.NewWSError(
			types.WSErrorInvalidRequest, fmt.Sprintf("transformer init failed: %v", err), http.StatusInternalServerError,
		))
	}
	defer t.Close()
	defer wsWriter.Flush()

	for ev, err := range sse.Read(resp.Body, nil) {
		if err != nil {
			if err == io.EOF {
				break
			}
			select {
			case <-c.done:
				return nil
			default:
			}
			logging.ErrorMsg("[ws:%s] SSE read error: %v", c.ID, err)
			break
		}

		if ev.Data != "" {
			upstreamCW.RecordChunk(ev.Type, []byte(ev.Data))
		}

		if transformErr := t.Transform(&ev); transformErr != nil {
			logging.ErrorMsg("[ws:%s] transform error: %v", c.ID, transformErr)
			break
		}
	}

	c.finalizeCapture(resp, upstreamCW, downstreamCW, turnInput)

	return nil
}

func (c *Connection) recordUpstreamRequest(headers http.Header, body []byte) {
	if c.captureRecorder == nil {
		return
	}
	c.captureRecorder.RecordUpstreamRequest(headers, body)
}

func (c *Connection) finalizeCapture(
	resp *http.Response,
	upstreamCW, downstreamCW capture.CaptureWriter,
	turnInput []types.InputItem,
) {
	if c.captureRecorder == nil {
		return
	}

	downstreamRec := c.captureRecorder.RecordDownstreamResponse(resp.Header)
	for _, chunk := range downstreamCW.Chunks() {
		downstreamRec.RecordChunkPreservingTiming(chunk)
	}

	if upstreamRec := c.captureRecorder.GetUpstreamResponseRecorder(); upstreamRec != nil {
		for _, chunk := range upstreamCW.Chunks() {
			upstreamRec.RecordChunkPreservingTiming(chunk)
		}
	}

	if store := capture.GetDefaultMemoryStore(); store != nil {
		store.Store(c.captureRecorder)
	}

	// Track response ID for cleanup and accumulate conversation history.
	// The response pipeline stores the conversation in the DefaultStore.
	// We read it back to accumulate in-memory history for within-connection
	// multi-turn continuity.
	c.mu.Lock()
	responseID := c.previousResponseID
	c.mu.Unlock()
	if responseID != "" {
		c.responseIDs = append(c.responseIDs, responseID)
		c.accumulateConversationHistory(responseID, turnInput)
	}

	dsChunks := downstreamCW.Chunks()
	usChunks := upstreamCW.Chunks()
	dsUsage := capture.ExtractTokenUsageFromChunks(dsChunks)
	usUsage := capture.ExtractTokenUsageFromChunks(usChunks)
	dsReason := capture.ExtractFinishReasonFromChunks(dsChunks)
	usReason := capture.ExtractFinishReasonFromChunks(usChunks)
	logging.InfoMsg("|📤 ⬆️ %d ⬇️ %d 📖 %d 💾 %d r=%s|  |📥 ⬆️ %d ⬇️ %d 📖 %d 💾 %d r=%s| [%s]",
		usUsage.InputTokens, usUsage.OutputTokens,
		usUsage.CacheReadTokens, usUsage.CacheCreationTokens, usReason,
		dsUsage.InputTokens, dsUsage.OutputTokens,
		dsUsage.CacheReadTokens, dsUsage.CacheCreationTokens, dsReason,
		c.ID,
	)
}

// accumulateConversationHistory reads the stored conversation from the
// DefaultStore and accumulates its input and output items into c.history.
// This enables within-connection multi-turn without store chain walks.
func (c *Connection) accumulateConversationHistory(responseID string, turnInput []types.InputItem) {
	conv := conversation.GetFromDefault(responseID)
	if conv == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Append this turn's input items to history
	c.history = append(c.history, turnInput...)

	// Convert output items to input format and append.
	// Function_call items are stored separately and will be recombined
	// by the request pipeline's groupProcessor (convert/responses_to_chat.go).
	for _, out := range conv.Output {
		item := outputToHistoryInput(out)
		if item != nil {
			c.history = append(c.history, *item)
		}
	}
}

// outputToHistoryInput converts a single OutputItem to an InputItem suitable
// for prepending to the next turn's request input.
// Note: function_call items are stored as separate items — the request
// pipeline's groupProcessor recombines them with the next assistant message.
func outputToHistoryInput(out types.OutputItem) *types.InputItem {
	switch out.Type {
	case "message":
		return &types.InputItem{
			Type:    "message",
			Role:    "assistant",
			Content: out.Content,
		}
	case "function_call":
		return &types.InputItem{
			Type:      "function_call",
			CallID:    out.CallID,
			Name:      out.Name,
			Arguments: out.Arguments,
		}
	case "file_search_call", "web_search_call", "computer_use_call":
		return &types.InputItem{
			Type:      "function_call",
			CallID:    out.CallID,
			Name:      out.Name,
			Arguments: out.Arguments,
		}
	case "reasoning":
		// Reasoning items are internal model artifacts — skip for history.
		return nil
	default:
		return nil
	}
}

func parseInputItemsFromBody(body []byte) []types.InputItem {
	// Extract just the "input" field to avoid a full ResponsesRequest unmarshal
	// followed by a wasteful interface{} → JSON → []InputItem round-trip.
	var partial struct {
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(body, &partial); err != nil || partial.Input == nil {
		return nil
	}
	var items []types.InputItem
	if err := json.Unmarshal(partial.Input, &items); err != nil {
		return nil
	}
	return items
}

func inputItemsToInterface(items []types.InputItem) interface{} {
	if len(items) == 0 {
		return nil
	}
	result := make([]interface{}, len(items))
	for i, item := range items {
		result[i] = item
	}
	return result
}

func (c *Connection) sendWSError(wsErr types.WSErrorResponse) error {
	if c.clientConn == nil {
		return nil
	}
	msg, _ := json.Marshal(wsErr)
	c.clientConn.SetWriteDeadline(time.Now().Add(WriteDeadline))
	c.clientConn.WriteMessage(websocket.TextMessage, msg)
	return nil
}

func (c *Connection) Close() {
	c.mu.Lock()
	if !c.active {
		c.mu.Unlock()
		return
	}
	c.active = false
	close(c.done)
	c.mu.Unlock()

	if c.clientConn != nil {
		_ = c.clientConn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(WriteDeadline),
		)
		c.clientConn.Close()
	}

	// Clean up non-persisted conversations from the store.
	// Conversations with store:true (Persisted) survive the connection.
	for _, id := range c.responseIDs {
		conv := conversation.GetFromDefault(id)
		if conv != nil && !conv.Persisted {
			conversation.DeleteFromDefault(id)
		}
	}
}

func generateConnectionID() string {
	return "ws_" + uuid.New().String()
}

func ExtractBearerToken(auth string) string {
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return auth
}

// wsMessageWriter receives SSE wire format from the transform pipeline,
// extracts the JSON payload, sends it as raw JSON to the WS client,
// and records the event in the capture writer.
type wsMessageWriter struct {
	conn          *Connection
	captureWriter capture.CaptureWriter
	buf           strings.Builder
}

func (w *wsMessageWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	w.buf.Write(p)

	s := w.buf.String()
	for {
		idx := strings.Index(s, "\n\n")
		if idx == -1 {
			break
		}
		event := s[:idx+2]
		s = s[idx+2:]

		// Parse SSE wire format: extract all data lines (SSE spec allows multi-line)
		var dataLines []string
		for _, line := range strings.Split(event, "\n") {
			if strings.HasPrefix(line, "data: ") {
				dataLines = append(dataLines, line[6:])
			}
		}
		dataStr := strings.Join(dataLines, "\n")

		// Send raw JSON to WebSocket client (OpenAI WebSocket spec)
		if dataStr != "" {
			if w.conn.clientConn != nil {
				w.conn.clientConn.SetWriteDeadline(time.Now().Add(WriteDeadline))
				if writeErr := w.conn.clientConn.WriteMessage(websocket.TextMessage, []byte(dataStr)); writeErr != nil {
					return n, writeErr
				}
			}
		}

		// Capture downstream chunk (parsed SSE event)
		w.captureDownstreamEvent(event)

		// Cache response ID from completion events
		w.conn.extractResponseID(event)
	}

	w.buf.Reset()
	w.buf.WriteString(s)
	return n, nil
}

func (w *wsMessageWriter) captureDownstreamEvent(event string) {
	if w.captureWriter == nil {
		return
	}
	var eventType, dataStr string
	for _, line := range strings.Split(event, "\n") {
		if strings.HasPrefix(line, "event: ") {
			eventType = line[7:]
		} else if strings.HasPrefix(line, "data: ") {
			dataStr = line[6:]
		}
	}
	if dataStr != "" {
		w.captureWriter.RecordChunk(eventType, []byte(dataStr))
	}
}

func (w *wsMessageWriter) Flush() {
	if w.buf.Len() > 0 {
		remaining := w.buf.String()
		w.buf.Reset()
		// Parse and flush any remaining SSE data
		for _, line := range strings.Split(remaining, "\n") {
			if strings.HasPrefix(line, "data: ") {
				dataStr := line[6:]
				if dataStr != "" {
					if w.conn.clientConn != nil {
						w.conn.clientConn.SetWriteDeadline(time.Now().Add(WriteDeadline))
						w.conn.clientConn.WriteMessage(websocket.TextMessage, []byte(dataStr))
					}
					// Always capture, even when clientConn is nil
					w.captureDownstreamEvent("data: " + dataStr + "\n\n")
				}
			}
		}
	}
}

func (c *Connection) extractResponseID(sseEvent string) {
	if !strings.Contains(sseEvent, "response.completed") {
		return
	}
	for _, line := range strings.Split(sseEvent, "\n") {
		if strings.HasPrefix(line, "data: ") {
			dataStr := line[6:]
			// Try nested format first: {"type":"response.completed","response":{"id":"..."}}
			var wrapper struct {
				Response struct {
					ID string `json:"id"`
				} `json:"response"`
			}
			if err := json.Unmarshal([]byte(dataStr), &wrapper); err == nil && wrapper.Response.ID != "" {
				c.mu.Lock()
				c.previousResponseID = wrapper.Response.ID
				c.mu.Unlock()
				logging.DebugMsg("[ws:%s] cached previous_response_id: %s", c.ID, wrapper.Response.ID)
				return
			}
			// Fall back to flat format: {"id":"..."}
			var resp types.ResponsesResponse
			if err := json.Unmarshal([]byte(dataStr), &resp); err == nil && resp.ID != "" {
				c.mu.Lock()
				c.previousResponseID = resp.ID
				c.mu.Unlock()
				logging.DebugMsg("[ws:%s] cached previous_response_id: %s", c.ID, resp.ID)
			}
			return
		}
	}
}
