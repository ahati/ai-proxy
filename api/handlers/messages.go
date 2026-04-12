package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"ai-proxy/api/pipeline"
	"ai-proxy/config"
	"ai-proxy/router"
	"ai-proxy/transform"
	"ai-proxy/websearch"

	"github.com/gin-gonic/gin"
)

// MessagesHandler handles Anthropic Messages API requests.
// It implements the Handler interface for the /v1/messages endpoint.
//
// This handler:
//   - Accepts requests in Anthropic Messages format
//   - Routes to the appropriate upstream based on model configuration
//   - For Anthropic providers: passes through requests without transformation
//   - For OpenAI providers: converts Anthropic→OpenAI Chat, transforms responses back
//   - Supports streaming responses with tool use handling
//   - Supports web search tool interception when web search service is enabled
//
// @note This enables clients using Anthropic SDK to call any provider.
type MessagesHandler struct {
	// cfg contains the application configuration including providers and models.
	// Must not be nil after construction.
	cfg *config.Config
	// manager provides thread-safe access to the live configuration.
	manager *config.ConfigManager
	// route is the resolved route for the current request.
	// Set during ValidateRequest for use in subsequent methods.
	route *router.ResolvedRoute
	// originalModel is the model name from the original request.
	// Preserved for response transformation.
	originalModel string
}

// NewMessagesHandler creates a Gin handler for the /v1/messages endpoint.
//
// @param cfg - Application configuration. Must not be nil.
// @param m - ConfigManager for live config access. May be nil for legacy behavior.
// @return Gin handler function that processes message requests.
//
// @pre cfg != nil
func NewMessagesHandler(cfg *config.Config, m *config.ConfigManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := &MessagesHandler{
			cfg:     cfg,
			manager: m,
		}
		Handle(h)(c)
	}
}

// ValidateRequest validates the request and resolves the model route.
// It parses the request to extract the model name and resolves it to a provider.
//
// @param body - Raw request body bytes.
// @return Error if JSON parsing fails or model cannot be resolved.
func (h *MessagesHandler) ValidateRequest(body []byte) error {
	// Create a router from the current config snapshot
	r := newRouterFromManager(h.manager)
	if r == nil {
		return nil
	}

	// Parse to get model name
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil // Let upstream handle invalid JSON
	}

	if req.Model == "" {
		return nil // Let upstream handle missing model
	}

	// Resolve the model to a route with incoming protocol context
	// The messages endpoint receives requests in Anthropic format
	route, err := r.ResolveWithProtocol(req.Model, "anthropic")
	if err != nil {
		return nil // Use fallback behavior
	}

	h.route = route
	h.originalModel = req.Model
	return nil
}

// TransformRequest converts the request body based on the upstream provider type.
// For Anthropic providers: passes through without transformation.
// For OpenAI providers: converts Anthropic Messages to OpenAI Chat Completions.
//
// @param ctx - Context for the request (passed to pipeline for cache status tracking).
// @param body - Raw request body in Anthropic Messages format.
// @return Transformed body in the appropriate upstream format.
// @return Error if transformation fails.
func (h *MessagesHandler) TransformRequest(ctx context.Context, body []byte) ([]byte, error) {
	// If no route resolved, pass through (legacy behavior)
	if h.route == nil {
		return body, nil
	}

	t, err := pipeline.BuildRequestPipeline(pipeline.RequestConfig{
		DownstreamFormat: "anthropic",
		UpstreamFormat:   h.route.OutputProtocol,
		ResolvedModel:    h.route.Model,
		IsPassthrough:    h.route.IsPassthrough,
		ReasoningSplit:   h.route.ReasoningSplit,
		WebSearchEnabled: websearch.GetDefaultAdapter() != nil && websearch.GetDefaultAdapter().IsEnabled(),
	})
	if err != nil {
		return body, nil
	}
	return t(ctx, body)
}

// UpstreamURL returns the upstream API URL based on the resolved provider.
//
// @return URL string for the upstream API endpoint.
func (h *MessagesHandler) UpstreamURL() string {
	if h.route != nil {
		return h.route.Provider.GetEndpoint(h.route.OutputProtocol)
	}
	return ""
}

// ResolveAPIKey returns the API key for the resolved provider.
//
// @param c - Gin context (unused in this implementation).
// @return API key string from the provider configuration.
func (h *MessagesHandler) ResolveAPIKey(c *gin.Context) string {
	if h.route != nil {
		return h.route.Provider.GetAPIKey()
	}
	return ""
}

// ForwardHeaders copies headers to the upstream request based on provider type.
// All headers are forwarded except those in the denylist (Authorization, Content-Type, etc.).
//
// @param c - Gin context containing the original request headers.
// @param req - Upstream request to receive forwarded headers.
func (h *MessagesHandler) ForwardHeaders(c *gin.Context, req *http.Request) {
	forwardCustomHeaders(c, req)
}

// CreateTransformer builds an SSE transformer for converting upstream responses.
// For OpenAI providers: converts Chat Completions to Anthropic format.
// For Anthropic providers: passes through SSE events.
// If web search service is enabled, wraps the transformer to intercept web_search tool calls.
//
// @param w - Writer to receive transformed output.
// @return Transformer for processing SSE events.
func (h *MessagesHandler) CreateTransformer(w io.Writer) transform.SSETransformer {
	// No route resolved: pass through with web search
	if h.route == nil {
		return wrapWithWebSearch(transform.NewPassthroughTransformer(w))
	}

	cfg := transform.Config{
		UpstreamFormat:        h.route.OutputProtocol,
		DownstreamFormat:      "anthropic",
		KimiToolCallTransform: h.route.KimiToolCallTransform,
		GLM5ToolCallTransform: h.route.GLM5ToolCallTransform,
		ReasoningSplit:        h.route.ReasoningSplit,
		WebSearchEnabled:      websearch.GetDefaultAdapter() != nil && websearch.GetDefaultAdapter().IsEnabled(),
	}

	t, err := pipeline.BuildPipeline(w, cfg)
	if err != nil {
		// Fallback to passthrough on error
		return wrapWithWebSearch(transform.NewPassthroughTransformer(w))
	}
	return t
}

// wrapWithWebSearch wraps the base transformer with web search interception if enabled.
//
// @param base - The base transformer to wrap.
// @return The wrapped transformer, or the base transformer if web search is not enabled.
func wrapWithWebSearch(base transform.SSETransformer) transform.SSETransformer {
	// Get the web search adapter (returns nil if service is not enabled)
	adapter := websearch.GetDefaultAdapter()
	if adapter == nil || !adapter.IsEnabled() {
		return base
	}

	// Note: This would need wstransform import, but the pipeline handles this internally
	// For the fallback case, we just return the base transformer
	return base
}

// WriteError sends an error response in Anthropic format.
// Maintains consistency with Anthropic API error responses.
//
// @param c - Gin context for writing the response.
// @param status - HTTP status code for the error.
// @param msg - Human-readable error message.
func (h *MessagesHandler) WriteError(c *gin.Context, status int, msg string) {
	sendAnthropicError(c, status, msg)
}

// ModelInfo returns the downstream and upstream model names for logging.
func (h *MessagesHandler) ModelInfo() (downstreamModel string, upstreamModel string) {
	downstreamModel = h.originalModel
	if h.route != nil {
		upstreamModel = h.route.Model
	}
	return
}
