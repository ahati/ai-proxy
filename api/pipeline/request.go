// Package pipeline provides declarative builders for request and response
// transformation pipelines.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"ai-proxy/capture"
	"ai-proxy/convert"
)

// RequestConfig holds all parameters needed to build a request transformation
// pipeline. It mirrors the response pipeline's transform.Config but for the
// request (one-shot JSON mutation) side.
//
// @brief Declarative configuration for request transformation.
type RequestConfig struct {
	// DownstreamFormat is the incoming protocol ("openai", "anthropic", "responses").
	DownstreamFormat string
	// UpstreamFormat is the target protocol from route.OutputProtocol.
	UpstreamFormat string
	// ResolvedModel is the actual model name on the upstream provider.
	ResolvedModel string
	// IsPassthrough skips core conversion when true.
	IsPassthrough bool
	// ReasoningSplit injects reasoning_split into supported request formats.
	ReasoningSplit bool
	// WebSearchEnabled converts server web search tools to function tools.
	WebSearchEnabled bool
	// Store enables conversation storage (used by Responses API ZDR mode).
	Store bool
}

// RequestTransform is a one-shot request body transformation function.
type RequestTransform func(ctx context.Context, body []byte) ([]byte, error)

// BuildRequestPipeline constructs a request transformation function based on
// the provided configuration. The returned function performs pre-processing,
// optional core conversion, and post-processing in sequence.
//
// @brief Declarative request pipeline builder.
//
// @note The pipeline follows this general stage order:
//  1. Pre-processing: model update, web search tool conversion
//  2. Core conversion: format-specific transformation (skipped if passthrough)
//  3. Post-processing: web search normalization (for Anthropic downstream)
//
// @param cfg Request pipeline configuration.
// @return RequestTransform function ready to transform request bodies.
// @return Error if config is invalid.
//
// @pre cfg.DownstreamFormat and cfg.UpstreamFormat must be valid protocols.
func BuildRequestPipeline(cfg RequestConfig) (RequestTransform, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("request pipeline config: %w", err)
	}

	// Start with model update (always runs)
	steps := []RequestTransform{stepUpdateModel(cfg.ResolvedModel)}

	// Web search pre-processing: convert server tools to function tools
	// before passthrough check. This is required for Messages handler because
	// the proxy's web search transformer intercepts function tool calls at the
	// SSE stream level, so the upstream request must use function tools even in
	// passthrough mode. Non-Anthropic providers don't understand server-side
	// web_search_20250305 tools.
	if cfg.WebSearchEnabled {
		steps = append(steps, stepConvertServerWebSearch())
	}

	// Core conversion or passthrough
	if cfg.IsPassthrough {
		// Even in passthrough, Anthropic downstream needs web search normalization
		if cfg.DownstreamFormat == "anthropic" {
			steps = append(steps, stepNormalizeWebSearchResults())
		}
		// Otherwise just pass through (model already updated)
	} else {
		coreStep, err := buildCoreConversion(cfg)
		if err != nil {
			return nil, err
		}
		steps = append(steps, coreStep)
	}

	return chainSteps(steps), nil
}

// Validate checks that the configuration has valid format combinations.
//
// @return Error if formats are missing or unsupported.
func (c RequestConfig) Validate() error {
	if c.DownstreamFormat == "" {
		return fmt.Errorf("downstream format is required")
	}
	if c.UpstreamFormat == "" {
		return fmt.Errorf("upstream format is required")
	}
	if c.ResolvedModel == "" {
		return fmt.Errorf("resolved model is required")
	}

	validFormats := map[string]bool{"openai": true, "anthropic": true, "responses": true}
	if !validFormats[c.DownstreamFormat] {
		return fmt.Errorf("unsupported downstream format: %q", c.DownstreamFormat)
	}
	if !validFormats[c.UpstreamFormat] {
		return fmt.Errorf("unsupported upstream format: %q", c.UpstreamFormat)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Core conversion builders (3×3 Downstream × Upstream matrix)
// ─────────────────────────────────────────────────────────────────────────────

// buildCoreConversion selects the appropriate conversion step based on the
// Downstream→Upstream format pair.
func buildCoreConversion(cfg RequestConfig) (RequestTransform, error) {
	switch cfg.DownstreamFormat {
	case "openai":
		return buildOpenAIConversion(cfg)
	case "anthropic":
		return buildAnthropicConversion(cfg)
	case "responses":
		return buildResponsesConversion(cfg)
	default:
		return nil, fmt.Errorf("unsupported downstream format: %q", cfg.DownstreamFormat)
	}
}

// buildOpenAIConversion builds conversion steps when downstream is OpenAI Chat.
func buildOpenAIConversion(cfg RequestConfig) (RequestTransform, error) {
	switch cfg.UpstreamFormat {
	case "openai":
		return stepOpenAIToOpenAI(cfg.ReasoningSplit), nil
	case "anthropic":
		return stepOpenAIToAnthropic(), nil
	case "responses":
		return stepOpenAIToResponses(), nil
	default:
		// Unknown upstream format: passthrough allows unsupported/future formats
		// to work without conversion, only updating the model field
		return stepPassthrough(), nil
	}
}

// buildAnthropicConversion builds conversion steps when downstream is Anthropic Messages.
func buildAnthropicConversion(cfg RequestConfig) (RequestTransform, error) {
	switch cfg.UpstreamFormat {
	case "openai":
		return stepAnthropicToOpenAI(cfg.ReasoningSplit), nil
	case "anthropic":
		return stepNormalizeWebSearchResults(), nil
	case "responses":
		return stepAnthropicToResponses(), nil
	default:
		// Unknown upstream format: passthrough allows unsupported/future formats
		// to work without conversion, only updating the model field
		return stepPassthrough(), nil
	}
}

// buildResponsesConversion builds conversion steps when downstream is Responses API.
func buildResponsesConversion(cfg RequestConfig) (RequestTransform, error) {
	switch cfg.UpstreamFormat {
	case "openai":
		return stepResponsesToOpenAI(cfg), nil
	case "anthropic":
		return stepResponsesToAnthropic(cfg.Store), nil
	default:
		// Unknown upstream format: passthrough allows unsupported/future formats
		// to work without conversion, only updating the model field
		return stepPassthrough(), nil
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Step functions
// ─────────────────────────────────────────────────────────────────────────────

// stepUpdateModel replaces the model field in the request body with the
// resolved upstream model name.
func stepUpdateModel(model string) RequestTransform {
	return func(_ context.Context, body []byte) ([]byte, error) {
		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			return body, nil // passthrough on parse failure
		}
		req["model"] = model
		return json.Marshal(req)
	}
}

// stepConvertServerWebSearch converts web_search_20250305 server tools to
// regular function tools for providers that don't understand server-side
// web search.
func stepConvertServerWebSearch() RequestTransform {
	return func(_ context.Context, body []byte) ([]byte, error) {
		return convert.ConvertServerWebSearchToFunctionTool(body), nil
	}
}

// stepNormalizeWebSearchResults normalizes web_search_tool_result blocks to
// tool_result blocks for upstream compatibility.
func stepNormalizeWebSearchResults() RequestTransform {
	return func(_ context.Context, body []byte) ([]byte, error) {
		return convert.NormalizeWebSearchToolResultsInMessages(body), nil
	}
}

// stepPassthrough returns the body unchanged.
func stepPassthrough() RequestTransform {
	return func(_ context.Context, body []byte) ([]byte, error) {
		return body, nil
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// OpenAI downstream conversion steps
// ─────────────────────────────────────────────────────────────────────────────

// stepOpenAIToOpenAI adds stream_options.include_usage and optionally injects
// reasoning_split for same-format OpenAI requests.
func stepOpenAIToOpenAI(reasoningSplit bool) RequestTransform {
	return func(_ context.Context, body []byte) ([]byte, error) {
		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			return body, nil
		}

		// Add stream_options.include_usage for usage statistics in streaming
		if stream, ok := req["stream"].(bool); !ok || stream {
			req["stream"] = true
			req["stream_options"] = map[string]interface{}{
				"include_usage": true,
			}
		}

		// Inject reasoning_split if configured
		if reasoningSplit {
			req["reasoning_split"] = true
		}

		return json.Marshal(req)
	}
}

// stepOpenAIToAnthropic converts OpenAI Chat Completions to Anthropic Messages.
func stepOpenAIToAnthropic() RequestTransform {
	return func(_ context.Context, body []byte) ([]byte, error) {
		converter := convert.NewChatToAnthropicConverter()
		return converter.Convert(body)
	}
}

// stepOpenAIToResponses converts OpenAI Chat Completions to Responses API format.
func stepOpenAIToResponses() RequestTransform {
	return func(_ context.Context, body []byte) ([]byte, error) {
		return convert.TransformChatToResponses(body)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Anthropic downstream conversion steps
// ─────────────────────────────────────────────────────────────────────────────

// stepAnthropicToOpenAI converts Anthropic Messages to OpenAI Chat Completions.
func stepAnthropicToOpenAI(reasoningSplit bool) RequestTransform {
	return func(_ context.Context, body []byte) ([]byte, error) {
		transformed, err := convert.TransformAnthropicToChat(body)
		if err != nil {
			return nil, err
		}

		// Inject reasoning_split if configured
		if reasoningSplit {
			var req map[string]interface{}
			if err := json.Unmarshal(transformed, &req); err == nil {
				req["reasoning_split"] = true
				transformed, _ = json.Marshal(req)
			}
		}
		return transformed, nil
	}
}

// stepAnthropicToResponses converts Anthropic Messages to Responses API format.
func stepAnthropicToResponses() RequestTransform {
	return func(_ context.Context, body []byte) ([]byte, error) {
		return convert.TransformAnthropicToResponses(body)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Responses downstream conversion steps
// ─────────────────────────────────────────────────────────────────────────────

// stepResponsesToOpenAI converts Responses API to OpenAI Chat Completions.
// Handles cache hit detection and capture context.
// Note: Cache hit detection happens after conversion completes because the
// ResponsesToChatConverter tracks hits internally during the previous_id lookup.
func stepResponsesToOpenAI(cfg RequestConfig) RequestTransform {
	return func(ctx context.Context, body []byte) ([]byte, error) {
		converter := convert.NewResponsesToChatConverter()
		converter.SetReasoningSplit(cfg.ReasoningSplit)
		converter.SetStore(cfg.Store)
		result, err := converter.Convert(body)
		if err == nil && converter.CacheHit() {
			capture.SetCacheHit(ctx)
		}
		return result, err
	}
}

// stepResponsesToAnthropic converts Responses API to Anthropic Messages.
// Handles cache hit detection via context.
// Note: Cache hit detection is delegated to TransformResponsesToAnthropicWithOptions
// which performs the previous_id lookup and sets capture.SetCacheHit internally.
func stepResponsesToAnthropic(store bool) RequestTransform {
	return func(ctx context.Context, body []byte) ([]byte, error) {
		return convert.TransformResponsesToAnthropicWithOptions(body, ctx, store)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Composition
// ─────────────────────────────────────────────────────────────────────────────

// chainSteps composes multiple RequestTransform steps into a single function.
// Each step receives the output of the previous step. If any step fails, the
// chain stops and returns the error.
func chainSteps(steps []RequestTransform) RequestTransform {
	return func(ctx context.Context, body []byte) ([]byte, error) {
		var err error
		for _, step := range steps {
			body, err = step(ctx, body)
			if err != nil {
				return nil, err
			}
		}
		return body, nil
	}
}
