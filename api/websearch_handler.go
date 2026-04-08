package api

import (
	"fmt"
	"net/http"

	"ai-proxy/config"
	"ai-proxy/logging"
	"ai-proxy/websearch"

	"github.com/gin-gonic/gin"
)

// WebSearchHandler handles web search configuration and service reload endpoints.
type WebSearchHandler struct {
	manager *config.ConfigManager
}

// NewWebSearchHandler creates a new WebSearchHandler.
func NewWebSearchHandler(m *config.ConfigManager) *WebSearchHandler {
	return &WebSearchHandler{manager: m}
}

// ReloadService reinitializes the web search service from the current configuration.
// This allows websearch config changes to take effect without a server restart.
//
// POST /ui/api/config/websearch/reload
//
// @note Reads the websearch config from the ConfigManager and reinitializes
// websearch.DefaultService. If disabled, the service is set to nil.
func (h *WebSearchHandler) ReloadService(c *gin.Context) {
	snap := h.manager.Get()
	if snap == nil || snap.Schema == nil {
		c.JSON(http.StatusInternalServerError, apiResponse{
			OK:    false,
			Error: "Configuration not loaded",
		})
		return
	}

	wsCfg := snap.Schema.WebSearch
	service := websearch.InitDefaultService(wsCfg)
	websearch.DefaultService = service

	if service != nil {
		logging.InfoMsg("Web search service reloaded: backend=%s", service.GetBackend())
		c.JSON(http.StatusOK, apiResponse{
			OK:      true,
			Message: fmt.Sprintf("Web search service reloaded: backend=%s", service.GetBackend()),
		})
	} else {
		logging.InfoMsg("Web search service disabled")
		c.JSON(http.StatusOK, apiResponse{
			OK:      true,
			Message: "Web search service disabled",
		})
	}
}
