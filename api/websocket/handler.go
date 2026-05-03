package websocket

import (
	"ai-proxy/capture"
	"ai-proxy/config"
	"ai-proxy/logging"
	"ai-proxy/router"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Handler handles WebSocket connections for the Responses API.
// It upgrades HTTP connections to WebSocket and manages the connection lifecycle.
//
// @brief WebSocket handler for /v1/responses endpoint.
//
// @note Supports OpenAI Responses API WebSocket mode for long-running,
//       tool-call-heavy workflows with up to 40% faster execution.
type Handler struct {
	config      *config.Config
	manager     *config.ConfigManager
	router      router.Router
	upgrader    websocket.Upgrader
	connections *ConnectionManager
}

// NewHandler creates a new WebSocket handler.
func NewHandler(cfg *config.Config, manager *config.ConfigManager) *Handler {
	var r router.Router
	if manager != nil {
		snap := manager.Get()
		if snap != nil && snap.Schema != nil {
			r, _ = router.NewRouter(snap.Schema)
		}
	}

	return &Handler{
		config:  cfg,
		manager: manager,
		router:  r,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			HandshakeTimeout: 10 * time.Second,
		},
		connections: NewConnectionManager(),
	}
}

// Handle handles a WebSocket connection request.
func (h *Handler) Handle(c *gin.Context) {
	clientConn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logging.ErrorMsg("websocket upgrade failed: %v", err)
		return
	}

	model := c.Query("model")
	authHeader := h.extractAuthHeader(c)

	// Extract the capture recorder from the middleware's context so that
	// each turn's request/response data is logged to the in-memory store.
	var recorder *capture.Recorder
	if cc := capture.GetCaptureContext(c.Request.Context()); cc != nil {
		recorder = cc.Recorder
	}

	conn := h.connections.NewConnection(clientConn, h.config, h.manager, h.router, authHeader, model, recorder)

	logging.InfoMsg("websocket connection established: %s", conn.ID)

	conn.Run()

	h.connections.Remove(conn.ID)
	logging.InfoMsg("websocket connection closed: %s", conn.ID)
}

func (h *Handler) extractAuthHeader(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	return auth
}

// IsWebSocketRequest checks if a request is a WebSocket upgrade request.
func IsWebSocketRequest(c *gin.Context) bool {
	upgrade := c.GetHeader("Upgrade")
	connection := c.GetHeader("Connection")
	return strings.EqualFold(upgrade, "websocket") &&
		strings.Contains(strings.ToLower(connection), "upgrade")
}
