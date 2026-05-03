package websocket

import (
	"ai-proxy/capture"
	"ai-proxy/config"
	"ai-proxy/router"
	"ai-proxy/types"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testHTTPServer(t *testing.T, events []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for _, ev := range events {
			fmt.Fprintf(w, "data: %s\n\n", ev)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
}

func testSchema(upstreamURL string) *config.Schema {
	return &config.Schema{
		Providers: []config.Provider{
			{Name: "test", Endpoints: map[string]string{"openai": upstreamURL}, APIKey: "test-key"},
		},
		Models: map[string]config.ModelConfig{
			"m": {Provider: "test", Model: "m", Type: "openai"},
		},
	}
}

func newTestConn(r router.Router) *Connection {
	return &Connection{
		ID:         "test",
		router:     r,
		authHeader: "Bearer test-key",
		config:     &config.Config{},
		active:     true,
		done:       make(chan struct{}),
		mu:         sync.Mutex{},
	}
}

func TestHandleClientMessage_FullPath_SimpleResponse(t *testing.T) {
	srv := testHTTPServer(t, []string{
		`{"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`{"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":null}]}`,
		`{"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
	})
	defer srv.Close()

	r, _ := router.NewRouter(testSchema(srv.URL))
	rec := capture.NewRecorder("r1", "GET", "/", "127.0.0.1:1")
	c := newTestConn(r)
	c.captureRecorder = rec

	msg := []byte(`{"type":"response.create","model":"m","input":[{"type":"message","role":"user","content":"hi"}],"store":false}`)
	err := c.handleClientMessage(msg)
	assert.Nil(t, err)
	assert.NotNil(t, rec.Data().UpstreamRequest)
}

func TestHandleClientMessage_FullPath_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad"}`))
	}))
	defer srv.Close()

	r, _ := router.NewRouter(testSchema(srv.URL))
	c := newTestConn(r)

	msg := []byte(`{"type":"response.create","model":"m","input":[],"store":false}`)
	err := c.handleClientMessage(msg)
	assert.Nil(t, err)
}

func TestHandleClientMessage_FullPath_WithPreviousID(t *testing.T) {
	srv := testHTTPServer(t, []string{
		`{"id":"c2","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`{"id":"c2","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"OK"},"finish_reason":null}]}`,
		`{"id":"c2","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`,
	})
	defer srv.Close()

	r, _ := router.NewRouter(testSchema(srv.URL))
	c := newTestConn(r)
	c.previousResponseID = "prev_123"

	msg := []byte(`{"type":"response.create","model":"m","input":[],"store":false}`)
	err := c.handleClientMessage(msg)
	assert.Nil(t, err)
}

func TestHandleClientMessage_FullPath_WithTools(t *testing.T) {
	srv := testHTTPServer(t, []string{
		`{"id":"c3","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`{"id":"c3","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"OK"},"finish_reason":null}]}`,
		`{"id":"c3","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`,
	})
	defer srv.Close()

	r, _ := router.NewRouter(testSchema(srv.URL))
	c := newTestConn(r)

	msg := []byte(`{"type":"response.create","model":"m","tools":[{"type":"function","name":"search","description":"Search"}],"input":[],"store":false}`)
	err := c.handleClientMessage(msg)
	assert.Nil(t, err)
}

func TestConnection_BuildResponsesBody_FullFields(t *testing.T) {
	c := &Connection{ID: "t"}
	route := &router.ResolvedRoute{Model: "g", SamplingParams: &config.SamplingParams{
		Temperature: float64Ptr(0.3), TopP: float64Ptr(0.8),
	}}

	store := boolPtr(false)
	req := &types.WSRequest{
		Model:               "m",
		Instructions:         "Be helpful",
		MaxOutputTokens:      100,
		Store:                store,
		PreviousResponseID:   "prev",
		Temperature:          float64Ptr(0.5),
		TopP:                 float64Ptr(0.9),
		EncryptedReasoning:   "enc",
		Metadata:             map[string]interface{}{"user": "test"},
		Input:                []types.InputItem{},
	}

	body := c.buildResponsesBody(req, route)
	require.NotNil(t, body)

	var p types.ResponsesRequest
	err := json.Unmarshal(body, &p)
	require.NoError(t, err)

	assert.Equal(t, "g", p.Model)
	assert.Equal(t, "Be helpful", p.Instructions)
	assert.Equal(t, 100, p.MaxOutputTokens)
	assert.False(t, *p.Store)
	assert.Equal(t, "prev", p.PreviousResponseID)
	assert.Equal(t, "enc", p.EncryptedReasoning)
}
