package websocket

import (
	"ai-proxy/config"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestIsWebSocketRequest(t *testing.T) {
	tests := []struct {
		name       string
		upgrade    string
		connection string
		expected   bool
	}{
		{
			name:       "valid websocket request",
			upgrade:    "websocket",
			connection: "Upgrade",
			expected:   true,
		},
		{
			name:       "valid websocket request with keep-alive",
			upgrade:    "websocket",
			connection: "keep-alive, Upgrade",
			expected:   true,
		},
		{
			name:       "case insensitive upgrade",
			upgrade:    "WebSocket",
			connection: "upgrade",
			expected:   true,
		},
		{
			name:       "case insensitive connection",
			upgrade:    "websocket",
			connection: "UPGRADE",
			expected:   true,
		},
		{
			name:       "missing upgrade header",
			upgrade:    "",
			connection: "Upgrade",
			expected:   false,
		},
		{
			name:       "missing connection header",
			upgrade:    "websocket",
			connection: "",
			expected:   false,
		},
		{
			name:       "wrong upgrade value",
			upgrade:    "h2c",
			connection: "Upgrade",
			expected:   false,
		},
		{
			name:       "connection without upgrade",
			upgrade:    "websocket",
			connection: "keep-alive",
			expected:   false,
		},
		{
			name:       "both headers empty",
			upgrade:    "",
			connection: "",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req := httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
			if tt.upgrade != "" {
				req.Header.Set("Upgrade", tt.upgrade)
			}
			if tt.connection != "" {
				req.Header.Set("Connection", tt.connection)
			}
			c.Request = req

			result := IsWebSocketRequest(c)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Port: "8080",
	}

	handler := NewHandler(cfg, nil)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.upgrader)
	assert.NotNil(t, handler.connections)
	assert.Equal(t, cfg, handler.config)
}

func TestNewHandlerWithManager(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Port: "8080",
	}

	schema := &config.Schema{
		Providers: []config.Provider{
			{
				Name: "test-provider",
				Endpoints: map[string]string{
					"responses": "https://api.test.com/v1/responses",
				},
			},
		},
		Models: map[string]config.ModelConfig{
			"test-model": {
				Provider: "test-provider",
				Model:    "test-model-id",
			},
		},
	}

	manager := config.NewManager(schema, "")
	handler := NewHandler(cfg, manager)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.router)
}

func TestConnectionManager(t *testing.T) {
	cm := NewConnectionManager()

	assert.NotNil(t, cm)
	assert.NotNil(t, cm.connections)
	assert.Equal(t, 0, cm.Count())
}
