package handlers

import (
	"net/http"

	"ai-proxy/config"

	"github.com/gin-gonic/gin"
)

// ModelsHandler handles requests to list available models.
// It returns the models configured in the proxy configuration.
//
// This handler:
//   - Accepts GET requests to retrieve available model aliases
//   - Returns a list of models configured in the proxy
//
// @note This endpoint returns locally configured models, not upstream models.
type ModelsHandler struct {
	cfg *config.Config
}

// NewModelsHandler creates a Gin handler for the /v1/models endpoint.
//
// @param cfg - Application configuration. Must not be nil.
// @return Gin handler function that processes models list requests.
func NewModelsHandler(cfg *config.Config) gin.HandlerFunc {
	h := &ModelsHandler{cfg: cfg}
	return h.Handle
}

// Model represents a model in the OpenAI models API response.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelsResponse represents the response from the models endpoint.
type ModelsResponse struct {
	Data   []Model `json:"data"`
	Object string  `json:"object"`
}

// Handle processes the models list request by returning configured models.
//
// @param c - Gin context for the HTTP request.
func (h *ModelsHandler) Handle(c *gin.Context) {
	schema := h.cfg.GetSchema()
	if schema == nil {
		sendOpenAIError(c, http.StatusInternalServerError, "Configuration not loaded")
		return
	}

	models := make([]Model, 0, len(schema.Models))
	for id, mc := range schema.Models {
		models = append(models, Model{
			ID:      id,
			Object:  "model",
			Created: 1700000000,
			OwnedBy: mc.Provider,
		})
	}

	response := ModelsResponse{
		Data:   models,
		Object: "list",
	}

	c.JSON(http.StatusOK, response)
}
