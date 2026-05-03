package websocket

import (
	"ai-proxy/capture"
	"ai-proxy/config"
	"ai-proxy/router"
	"ai-proxy/types"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"github.com/gin-gonic/gin"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ConnectionManager tests ---

func TestConnectionManager_NewConnectionManager(t *testing.T) {
	cm := NewConnectionManager()
	require.NotNil(t, cm)
	assert.NotNil(t, cm.connections)
	assert.Equal(t, 0, cm.Count())
}

func TestConnectionManager_Count(t *testing.T) {
	cm := NewConnectionManager()
	assert.Equal(t, 0, cm.Count())
	cm.NewConnection(nil, nil, nil, nil, "", "", nil)
	assert.Equal(t, 1, cm.Count())
	cm.NewConnection(nil, nil, nil, nil, "", "", nil)
	assert.Equal(t, 2, cm.Count())
}

func TestConnectionManager_Remove(t *testing.T) {
	cm := NewConnectionManager()
	conn := cm.NewConnection(nil, nil, nil, nil, "", "", nil)
	assert.Equal(t, 1, cm.Count())
	cm.Remove(conn.ID)
	assert.Equal(t, 0, cm.Count())
	cm.Remove("nonexistent")
	assert.Equal(t, 0, cm.Count())
}

func TestConnectionManager_NewConnection(t *testing.T) {
	cm := NewConnectionManager()
	cfg := &config.Config{Port: "8080"}
	schema := &config.Schema{
		Providers: []config.Provider{{Name: "p1", Endpoints: map[string]string{"openai": "http://x.com"}}},
		Models:    map[string]config.ModelConfig{"m1": {Provider: "p1", Model: "m1", Type: "openai"}},
	}
	manager := config.NewManager(schema, "")
	r, _ := router.NewRouter(schema)
	recorder := capture.NewRecorder("r1", "GET", "/", "127.0.0.1:1")

	conn := cm.NewConnection(nil, cfg, manager, r, "Bearer sk-test", "m1", recorder)
	assert.NotNil(t, conn)
	assert.True(t, strings.HasPrefix(conn.ID, "ws_"))
	assert.True(t, conn.active)
	assert.Equal(t, "Bearer sk-test", conn.authHeader)
	assert.Equal(t, "m1", conn.defaultModel)
	assert.NotNil(t, conn.captureRecorder)
}

// --- generateConnectionID ---

func TestGenerateConnectionID(t *testing.T) {
	id1 := generateConnectionID()
	id2 := generateConnectionID()
	assert.NotEqual(t, id1, id2)
	assert.True(t, strings.HasPrefix(id1, "ws_"))
}

// --- ExtractBearerToken ---

func TestExtractBearerToken(t *testing.T) {
	tests := []struct{ name, auth, want string }{
		{"bearer token", "Bearer sk-abc123", "sk-abc123"},
		{"no prefix", "sk-abc123", "sk-abc123"},
		{"empty", "", ""},
		{"bearer only", "Bearer ", ""},
		{"lowercase", "bearer sk-abc", "bearer sk-abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ExtractBearerToken(tt.auth))
		})
	}
}

// --- inputItemsToInterface ---

func TestInputItemsToInterface(t *testing.T) {
	assert.Nil(t, inputItemsToInterface(nil))
	assert.Nil(t, inputItemsToInterface([]types.InputItem{}))
	items := []types.InputItem{{Type: "message", Role: "user", Content: "hello"}}
	result := inputItemsToInterface(items)
	require.NotNil(t, result)
	arr := result.([]interface{})
	assert.Equal(t, 1, len(arr))
}

// --- parseInputItemsFromBody ---

func TestParseInputItemsFromBody(t *testing.T) {
	body := []byte(`{"model":"m","input":[{"type":"message","role":"user","content":"hi"}]}`)
	items := parseInputItemsFromBody(body)
	require.NotNil(t, items)
	assert.Equal(t, 1, len(items))
	assert.Equal(t, "message", items[0].Type)
	assert.Equal(t, "user", items[0].Role)
	assert.Equal(t, "hi", items[0].Content)

	// Missing input field
	assert.Nil(t, parseInputItemsFromBody([]byte(`{}`)))
	// Invalid JSON
	assert.Nil(t, parseInputItemsFromBody([]byte(`{invalid}`)))
	// Input field is not an array (e.g. string)
	assert.Nil(t, parseInputItemsFromBody([]byte(`{"input":"not-an-array"}`)))
	// Null input field
	assert.Nil(t, parseInputItemsFromBody([]byte(`{"input":null}`)))
	// Empty array
	items = parseInputItemsFromBody([]byte(`{"input":[]}`))
	assert.NotNil(t, items)
	assert.Equal(t, 0, len(items))
}

// --- sendWSError ---

func TestConnection_SendWSError_NilConn(t *testing.T) {
	c := &Connection{ID: "test", active: true, done: make(chan struct{})}
	err := c.sendWSError(types.NewWSError("code", "msg", 400))
	assert.Nil(t, err)
}

// --- extractResponseID ---

func TestConnection_ExtractResponseID(t *testing.T) {
	c := &Connection{ID: "test", active: true, done: make(chan struct{}), mu: sync.Mutex{}}
	// Non-completion
	c.extractResponseID("event: delta\ndata: {}\n\n")
	assert.Empty(t, c.previousResponseID)
	// Invalid JSON in completion
	c.extractResponseID("event: response.completed\ndata: not-json\n\n")
	assert.Empty(t, c.previousResponseID)
	// Valid
	c.extractResponseID("event: response.completed\ndata: {\"id\":\"r1\"}\n\n")
	assert.Equal(t, "r1", c.previousResponseID)
	// Empty ID
	c.extractResponseID("event: response.completed\ndata: {\"id\":\"\"}\n\n")
	assert.Equal(t, "r1", c.previousResponseID)
}

// --- Close ---

func TestConnection_Close(t *testing.T) {
	c := &Connection{ID: "test", active: true, done: make(chan struct{})}
	c.Close()
	assert.False(t, c.active)
	c.Close() // double close
	assert.False(t, c.active)
}

// --- wsMessageWriter tests ---

func TestWSMessageWriter_Write_SingleEvent(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{})}
	cw := capture.NewCaptureWriter(time.Now())
	w := &wsMessageWriter{conn: c, captureWriter: cw}
	data := []byte("data: {\"a\":1}\n\n")
	n, _ := w.Write(data)
	assert.Equal(t, len(data), n)
	assert.Equal(t, 1, len(cw.Chunks()))
}

func TestWSMessageWriter_Write_PartialThenComplete(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{})}
	cw := capture.NewCaptureWriter(time.Now())
	w := &wsMessageWriter{conn: c, captureWriter: cw}
	w.Write([]byte("data: {\"a\":1}"))
	assert.Equal(t, 0, len(cw.Chunks()))
	w.Write([]byte("\n\n"))
	assert.Equal(t, 1, len(cw.Chunks()))
}

func TestWSMessageWriter_Write_MultipleEvents(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{})}
	cw := capture.NewCaptureWriter(time.Now())
	w := &wsMessageWriter{conn: c, captureWriter: cw}
	data := []byte("data: {}\n\ndata: {}\n\n")
	w.Write(data)
	assert.Equal(t, 2, len(cw.Chunks()))
}

func TestWSMessageWriter_Write_EventsWithEventType(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{})}
	cw := capture.NewCaptureWriter(time.Now())
	w := &wsMessageWriter{conn: c, captureWriter: cw}
	w.Write([]byte("event: response.created\ndata: {\"id\":\"r1\"}\n\n"))
	assert.Equal(t, 1, len(cw.Chunks()))
	assert.Equal(t, "response.created", cw.Chunks()[0].Event)
}

func TestWSMessageWriter_Write_NoNewline(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{})}
	cw := capture.NewCaptureWriter(time.Now())
	w := &wsMessageWriter{conn: c, captureWriter: cw}
	w.Write([]byte("data: {\"a\":1}"))
	assert.Equal(t, 0, len(cw.Chunks()))
	assert.True(t, w.buf.Len() > 0)
}

func TestWSMessageWriter_Flush(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{})}
	cw := capture.NewCaptureWriter(time.Now())
	w := &wsMessageWriter{conn: c, captureWriter: cw}
	w.Write([]byte("data: {\"a\":1}\n\ndata: {\"b\":2}"))
	assert.True(t, w.buf.Len() > 0)
	w.Flush()
}

func TestWSMessageWriter_Flush_Empty(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{})}
	w := &wsMessageWriter{conn: c, captureWriter: nil}
	w.Flush()
}

func TestWSMessageWriter_Flush_NoCapture(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{})}
	w := &wsMessageWriter{conn: c, captureWriter: nil}
	w.Write([]byte("data: {}\n\n"))
	w.Flush()
}

func TestWSMessageWriter_CaptureDownstreamEvent(t *testing.T) {
	cw := capture.NewCaptureWriter(time.Now())
	w := &wsMessageWriter{captureWriter: cw}
	w.captureDownstreamEvent("event: response.created\ndata: {\"id\":\"r1\"}\n\n")
	assert.Equal(t, "response.created", cw.Chunks()[0].Event)
	w.captureDownstreamEvent("data: {\"type\":\"test\"}\n\n")
	assert.Equal(t, 2, len(cw.Chunks()))
	w.captureDownstreamEvent("\n\n")
	assert.Equal(t, 2, len(cw.Chunks()))
	// Nil captureWriter
	w2 := &wsMessageWriter{captureWriter: nil}
	assert.NotPanics(t, func() { w2.captureDownstreamEvent("data: {}\n\n") })
}

// --- buildResponsesBody ---

func TestConnection_BuildResponsesBody(t *testing.T) {
	c := &Connection{ID: "test"}
	route := &router.ResolvedRoute{
		Model: "glm-5",
		SamplingParams: &config.SamplingParams{
			Temperature: float64Ptr(0.5),
			TopP:        float64Ptr(0.9),
		},
	}

	body := c.buildResponsesBody(&types.WSRequest{
		Model: "m1", Store: boolPtr(true), Temperature: float64Ptr(0.7),
		Input: []types.InputItem{{Type: "message", Role: "user", Content: "hi"}},
	}, route)
	var p1 types.ResponsesRequest
	json.Unmarshal(body, &p1)
	assert.Equal(t, "glm-5", p1.Model)
	assert.True(t, *p1.Stream)
	assert.True(t, *p1.Store)
	assert.Equal(t, 0.7, p1.Temperature)

	// With tools
	body = c.buildResponsesBody(&types.WSRequest{
		Model: "m1", Tools: []types.ResponsesTool{{Type: "function", Name: "f"}},
	}, route)
	var p2 types.ResponsesRequest
	json.Unmarshal(body, &p2)
	assert.Equal(t, 1, len(p2.Tools))

	// Sampling params fallback
	body = c.buildResponsesBody(&types.WSRequest{Model: "m1"}, route)
	var p3 types.ResponsesRequest
	json.Unmarshal(body, &p3)
	assert.Equal(t, 0.5, p3.Temperature)

	// No sampling params
	body = c.buildResponsesBody(&types.WSRequest{Model: "m1"}, &router.ResolvedRoute{Model: "g"})
	var p4 types.ResponsesRequest
	json.Unmarshal(body, &p4)
	assert.Equal(t, 0.0, p4.Temperature)
}

// --- recordDownstreamRequest ---

func TestConnection_RecordDownstreamRequest(t *testing.T) {
	rec := capture.NewRecorder("r1", "GET", "/", "127.0.0.1:1")
	c := &Connection{ID: "t", authHeader: "Bearer sk", captureRecorder: rec}
	msg := []byte(`{"t":"r"}`)
	c.recordDownstreamRequest(msg, "https://api.example.com")
	d := rec.Data()
	assert.Equal(t, "https://api.example.com", d.UpstreamURL)
	assert.Equal(t, json.RawMessage(msg), d.DownstreamRequest.Body)

	// Nil recorder
	c2 := &Connection{authHeader: "x", captureRecorder: nil}
	assert.NotPanics(t, func() { c2.recordDownstreamRequest(msg, "url") })
}

// --- recordUpstreamRequest ---

func TestConnection_RecordUpstreamRequest(t *testing.T) {
	rec := capture.NewRecorder("r1", "GET", "/", "127.0.0.1:1")
	c := &Connection{ID: "t", captureRecorder: rec}
	h := http.Header{"Content-Type": {"application/json"}}
	body := []byte(`{"m":"g"}`)
	c.recordUpstreamRequest(h, body)
	assert.Equal(t, json.RawMessage(body), rec.Data().UpstreamRequest.Body)

	c2 := &Connection{captureRecorder: nil}
	assert.NotPanics(t, func() { c2.recordUpstreamRequest(h, body) })
}

// --- handleWarmup ---

func TestConnection_HandleWarmup(t *testing.T) {
	c := &Connection{ID: "test", active: true, done: make(chan struct{}), mu: sync.Mutex{}}
	err := c.handleWarmup(&types.WSRequest{Model: "glm-5"})
	assert.Nil(t, err)
	assert.True(t, strings.HasPrefix(c.previousResponseID, "resp_warmup_"))
}

// --- sendWSEvent ---

func TestConnection_SendWSEvent(t *testing.T) {
	c := &Connection{ID: "test", active: true, done: make(chan struct{})}
	assert.NotPanics(t, func() {
		c.sendWSEvent(map[string]string{"type": "test"})
		c.sendWSEvent(types.WSEvent{Type: types.WSEventResponseCreated, ResponseID: "r"})
	})
}

// --- handleClientMessage - error paths ---

func testRouter() router.Router {
	r, _ := router.NewRouter(&config.Schema{
		Providers: []config.Provider{{Name: "p", Endpoints: map[string]string{"openai": "http://x.com"}}},
		Models:    map[string]config.ModelConfig{"m": {Provider: "p", Model: "m", Type: "openai"}},
	})
	return r
}

func TestConnection_HandleClientMessage_InvalidJSON(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{}), router: testRouter()}
	err := c.handleClientMessage([]byte(`not json`))
	assert.Nil(t, err) // error sent to WS, method returns nil
}

func TestConnection_HandleClientMessage_WrongType(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{}), router: testRouter()}
	err := c.handleClientMessage([]byte(`{"type":"wrong","model":"m"}`))
	assert.Nil(t, err)
}

func TestConnection_HandleClientMessage_MissingModel(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{}), router: testRouter()}
	err := c.handleClientMessage([]byte(`{"type":"response.create"}`))
	assert.Nil(t, err)
}

func TestConnection_HandleClientMessage_NoRouter(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{})}
	err := c.handleClientMessage([]byte(`{"type":"response.create","model":"m"}`))
	assert.Nil(t, err)
}

func TestConnection_HandleClientMessage_Warmup(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{}), mu: sync.Mutex{}, router: testRouter()}
	err := c.handleClientMessage([]byte(`{"type":"response.create","model":"m","generate":false}`))
	assert.Nil(t, err)
	assert.True(t, strings.HasPrefix(c.previousResponseID, "resp_warmup_"))
}

func TestConnection_HandleClientMessage_ModelResolutionFail(t *testing.T) {
	c := &Connection{ID: "t", active: true, done: make(chan struct{}), router: testRouter()}
	err := c.handleClientMessage([]byte(`{"type":"response.create","model":"nonexistent"}`))
	assert.Nil(t, err)
}

// --- transformRequest ---

func TestConnection_TransformRequest(t *testing.T) {
	c := &Connection{ID: "t"}
	route := &router.ResolvedRoute{
		Model:          "glm-5",
		OutputProtocol: "openai",
		IsPassthrough:  false,
	}
	body := []byte(`{"model":"glm-5","input":[{"type":"message","role":"user","content":"hi"}]}`)
	result, err := c.transformRequest(body, route, true)
	require.NoError(t, err)
	// Should be transformed to OpenAI format
	assert.Contains(t, string(result), "messages")
	assert.Contains(t, string(result), "glm-5")
}

func TestConnection_TransformRequest_Anthropic(t *testing.T) {
	c := &Connection{ID: "t"}
	route := &router.ResolvedRoute{
		Model:          "glm-5",
		OutputProtocol: "anthropic",
		IsPassthrough:  false,
	}
	body := []byte(`{"model":"glm-5","input":[{"type":"message","role":"user","content":"hi"}]}`)
	result, err := c.transformRequest(body, route, true)
	require.NoError(t, err)
	assert.Contains(t, string(result), "glm-5")
}

func TestConnection_TransformRequest_Passthrough(t *testing.T) {
	c := &Connection{ID: "t"}
	route := &router.ResolvedRoute{
		Model:          "glm-5",
		OutputProtocol: "responses",
		IsPassthrough:  true,
	}
	body := []byte(`{"model":"glm-5","input":[]}`)
	result, err := c.transformRequest(body, route, true)
	require.NoError(t, err)
	assert.Contains(t, string(result), "glm-5")
}

// --- finalizeCapture ---

func TestConnection_FinalizeCapture(t *testing.T) {
	rec := capture.NewRecorder("r1", "GET", "/", "127.0.0.1:1")
	c := &Connection{ID: "t", captureRecorder: rec}

	// Must initialize upstream response before adding chunks
	rec.RecordUpstreamResponse(200, http.Header{})

	upCW := capture.NewCaptureWriter(time.Now())
	downCW := capture.NewCaptureWriter(time.Now())
	upCW.RecordChunk("message_start", []byte(`{}`))
	downCW.RecordChunk("response.created", []byte(`{"response_id":"r1"}`))

	resp := &http.Response{StatusCode: 200, Header: http.Header{"X-Test": {"1"}}}
	c.finalizeCapture(resp, upCW, downCW, nil)

	d := rec.Data()
	assert.NotNil(t, d.DownstreamResponse)
}

func TestConnection_FinalizeCapture_NilRecorder(t *testing.T) {
	c := &Connection{ID: "t", captureRecorder: nil}
	upCW := capture.NewCaptureWriter(time.Now())
	downCW := capture.NewCaptureWriter(time.Now())
	assert.NotPanics(t, func() {
		c.finalizeCapture(&http.Response{}, upCW, downCW, nil)
	})
}








func TestHandler_ExtractAuthHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"with bearer", "Bearer sk-test", "Bearer sk-test"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				c.Request.Header.Set("Authorization", tt.header)
			}
			assert.Equal(t, tt.want, h.extractAuthHeader(c))
		})
	}
}

func TestHandler_Handle_NonWebSocket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Port: "8080"}
	h := NewHandler(cfg, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	h.Handle(c)
}


func TestConnection_SendWSError_WithClientConn(t *testing.T) {
	// Set up a paired WebSocket connection
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
		// Read the error message
		_, _, _ = conn.ReadMessage()
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer client.Close()

	c := &Connection{ID: "t", clientConn: client, active: true, done: make(chan struct{})}
	err = c.sendWSError(types.NewWSError("code", "msg", 400))
	assert.Nil(t, err)
}

func TestConnection_SendWSEvent_WithClientConn(t *testing.T) {
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
		_, _, _ = conn.ReadMessage()
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer client.Close()

	c := &Connection{ID: "t", clientConn: client, active: true, done: make(chan struct{})}
	c.sendWSEvent(types.WSEvent{Type: types.WSEventResponseCreated, ResponseID: "r"})
}

func TestConnection_Close_WithClientConn(t *testing.T) {
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	c := &Connection{ID: "t", clientConn: client, active: true, done: make(chan struct{})}
	c.Close()
	// Double close
	c.Close()
}

// --- Helpers ---

func float64Ptr(f float64) *float64 { return &f }
func boolPtr(b bool) *bool          { return &b }

// --- WS connection readFromClient test ---

func TestConnection_ReadFromClient_Close(t *testing.T) {
	c := &Connection{
		ID: "test", active: true, done: make(chan struct{}),
		clientConn: nil, mu: sync.Mutex{},
	}
	// Close immediately
	close(c.done)
	c.readFromClient()
	// Should exit without panic
}
