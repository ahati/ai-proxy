package pipeline

import (
	"fmt"
	"io"

	"ai-proxy/convert"
	"ai-proxy/transform"
	"ai-proxy/transform/toolcall"
	wstransform "ai-proxy/transform/websearch"
	"ai-proxy/websearch"
)

// BuildPipeline constructs a complete transformer chain from a Config.
// It selects and orders stages based on UpstreamFormat + DownstreamFormat
// and applies feature flags (tool call extraction, web search, etc.).
//
// @brief Declarative pipeline builder for transformer chains.
//
// @note The pipeline follows this general stage order:
//
//  1. Input parser (parse upstream SSE into PipelineEvents)
//  2. Tool call extraction (extract from reasoning if enabled)
//  3. Format conversion (OpenAI ↔ Anthropic ↔ Responses)
//  4. Web search interception (if enabled)
//  5. Output writer (write final SSE to io.Writer)
//
// @param w The final output writer (typically HTTP response writer).
// @param cfg Pipeline configuration from route resolution.
//
// @return transform.SSETransformer A transformer ready for SSE processing.
// @return error If config is invalid.
//
// @pre w must be non-nil.
// @pre cfg must pass Validate().
func BuildPipeline(w io.Writer, cfg transform.Config) (transform.SSETransformer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("pipeline config: %w", err)
	}

	// Passthrough: same format, no features
	if cfg.IsPassthrough() {
		return transform.NewPassthroughTransformer(w), nil
	}

	// Build based on upstream format
	switch cfg.UpstreamFormat {
	case "openai":
		return buildOpenAIInputStage(w, cfg)
	case "anthropic":
		return buildAnthropicInputStage(w, cfg)
	case "responses":
		return buildResponsesInputStage(w, cfg)
	default:
		return nil, fmt.Errorf("unsupported upstream format: %q", cfg.UpstreamFormat)
	}
}

// buildOpenAIInputStage creates a transformer for OpenAI upstream format.
func buildOpenAIInputStage(w io.Writer, cfg transform.Config) (transform.SSETransformer, error) {
	switch cfg.DownstreamFormat {
	case "openai":
		return buildOpenAIToOpenAI(w, cfg)
	case "anthropic":
		return buildOpenAIToAnthropic(w, cfg)
	case "responses":
		return buildOpenAIToResponses(w, cfg)
	default:
		return nil, fmt.Errorf("unsupported downstream format: %q", cfg.DownstreamFormat)
	}
}

// buildAnthropicInputStage creates a transformer for Anthropic upstream format.
func buildAnthropicInputStage(w io.Writer, cfg transform.Config) (transform.SSETransformer, error) {
	switch cfg.DownstreamFormat {
	case "openai":
		return buildAnthropicToOpenAI(w, cfg)
	case "anthropic":
		return buildAnthropicToAnthropic(w, cfg)
	case "responses":
		return buildAnthropicToResponses(w, cfg)
	default:
		return nil, fmt.Errorf("unsupported downstream format: %q", cfg.DownstreamFormat)
	}
}

// buildResponsesInputStage creates a transformer for Responses upstream format.
func buildResponsesInputStage(w io.Writer, cfg transform.Config) (transform.SSETransformer, error) {
	switch cfg.DownstreamFormat {
	case "openai":
		return buildResponsesToOpenAI(w, cfg)
	case "anthropic":
		return buildResponsesToAnthropic(w, cfg)
	case "responses":
		return buildResponsesToResponses(w, cfg)
	default:
		return nil, fmt.Errorf("unsupported downstream format: %q", cfg.DownstreamFormat)
	}
}

// buildOpenAIToOpenAI builds a transformer for OpenAI→OpenAI with optional tool call extraction.
//
// @brief OpenAI chunks → tool call extraction → OpenAI chunks.
func buildOpenAIToOpenAI(w io.Writer, cfg transform.Config) (transform.SSETransformer, error) {
	if cfg.NeedsToolCallExtraction() {
		t := toolcall.NewOpenAITransformer(w)
		t.SetKimiToolCallTransform(cfg.KimiToolCallTransform)
		t.SetGLM5ToolCallTransform(cfg.GLM5ToolCallTransform)
		return t, nil
	}
	return transform.NewPassthroughTransformer(w), nil
}

// buildOpenAIToAnthropic builds a transformer for OpenAI→Anthropic conversion.
//
// @brief OpenAI chunks → tool call extraction → Anthropic SSE.
//
// Chain: OpenAITransformer (tool extraction) → ChatToAnthropicTransformer → [web search] → output
func buildOpenAIToAnthropic(w io.Writer, cfg transform.Config) (transform.SSETransformer, error) {
	// Build the chain from end to start:
	// w ← [web search] ← ChatToAnthropicTransformer ← OpenAITransformer

	// Final output stage
	var outputStage transform.Stage = transform.NewSSEWriterStage(w)

	// Wrap with web search if enabled (web search intercepts Anthropic-format events)
	if cfg.NeedsWebSearch() {
		adapter := websearch.GetDefaultAdapter()
		if adapter != nil && adapter.IsEnabled() {
			outputStage = wstransform.NewTransformer(transform.SSETransformerFromStage(outputStage), adapter)
		}
	}

	// ChatToAnthropicTransformer sends PipelineEvent directly to outputStage (no SSE round-trip)
	chatToAnthropic := convert.NewChatToAnthropicTransformer(nil) // nil writer since we use outputStage
	chatToAnthropic.SetOutputStage(outputStage)

	// OpenAITransformer feeds OpenAI chunks to ChatToAnthropic via receiver pattern
	t := toolcall.NewOpenAITransformerWithReceiver(chatToAnthropic)
	t.SetKimiToolCallTransform(cfg.KimiToolCallTransform)
	t.SetGLM5ToolCallTransform(cfg.GLM5ToolCallTransform)

	return t, nil
}

// buildOpenAIToResponses builds a transformer for OpenAI→Responses conversion.
//
// @brief OpenAI chunks → tool call extraction → Responses SSE.
//
// Chain: OpenAITransformer (tool extraction) → ChatToResponsesTransformer → [web search] → output
func buildOpenAIToResponses(w io.Writer, cfg transform.Config) (transform.SSETransformer, error) {
	// ChatToResponsesTransformer writes Responses SSE directly to w
	chatToResponses := convert.NewChatToResponsesTransformer(w)
	chatToResponses.SetInputItems(cfg.InputItems)
	chatToResponses.SetStore(cfg.Store)
	chatToResponses.SetPreviousResponseID(cfg.PreviousResponseID)
	chatToResponses.SetReasoningSummaryMode(cfg.ReasoningSummaryMode)
	chatToResponses.SetEncryptedReasoning(cfg.EncryptedReasoning)

	// OpenAITransformer feeds OpenAI chunks to ChatToResponses via receiver pattern
	t := toolcall.NewOpenAITransformerWithReceiver(chatToResponses)
	t.SetKimiToolCallTransform(cfg.KimiToolCallTransform)
	t.SetGLM5ToolCallTransform(cfg.GLM5ToolCallTransform)

	// Wrap with web search if enabled (intercepts input OpenAI chunks)
	if cfg.NeedsWebSearch() {
		adapter := websearch.GetDefaultAdapter()
		if adapter != nil && adapter.IsEnabled() {
			return wstransform.NewTransformer(t, adapter), nil
		}
	}

	return t, nil
}

// buildAnthropicToOpenAI builds a transformer for Anthropic→OpenAI conversion.
//
// @brief Anthropic SSE → tool call extraction → OpenAI chunks.
//
// Chain: AnthropicTransformer (tool extraction) → AnthropicToChatStreamingConverter → output
func buildAnthropicToOpenAI(w io.Writer, cfg transform.Config) (transform.SSETransformer, error) {
	// AnthropicToChatStreamingConverter writes OpenAI chunks directly to w
	chatConverter := convert.NewAnthropicToChatStreamingConverter(w)

	// AnthropicTransformer feeds Anthropic events to converter via receiver pattern
	t := toolcall.NewAnthropicTransformerWithReceiver(chatConverter)
	t.SetKimiToolCallTransform(cfg.KimiToolCallTransform)
	t.SetGLM5ToolCallTransform(cfg.GLM5ToolCallTransform)

	return t, nil
}

// buildAnthropicToAnthropic builds a transformer for Anthropic→Anthropic with optional tool call extraction.
//
// @brief Anthropic SSE → tool call extraction → Anthropic SSE.
func buildAnthropicToAnthropic(w io.Writer, cfg transform.Config) (transform.SSETransformer, error) {
	var baseTransformer transform.SSETransformer

	if cfg.NeedsToolCallExtraction() {
		t := toolcall.NewAnthropicTransformer(w)
		t.SetKimiToolCallTransform(cfg.KimiToolCallTransform)
		t.SetGLM5ToolCallTransform(cfg.GLM5ToolCallTransform)
		baseTransformer = t
	} else {
		baseTransformer = transform.NewPassthroughTransformer(w)
	}

	// Wrap with web search if enabled
	if cfg.NeedsWebSearch() {
		adapter := websearch.GetDefaultAdapter()
		if adapter != nil && adapter.IsEnabled() {
			baseTransformer = wstransform.NewTransformer(baseTransformer, adapter)
		}
	}

	return baseTransformer, nil
}

// buildAnthropicToResponses builds a transformer for Anthropic→Responses conversion.
//
// @brief Anthropic SSE → tool call extraction → Responses SSE.
//
// Chain: AnthropicTransformer (tool extraction) → AnthropicToResponsesStreamingConverter → [web search] → output
func buildAnthropicToResponses(w io.Writer, cfg transform.Config) (transform.SSETransformer, error) {
	// AnthropicToResponsesStreamingConverter writes Responses SSE directly to w
	responsesConverter := convert.NewResponsesTransformer(w)
	responsesConverter.SetInputItems(cfg.InputItems)
	responsesConverter.SetStore(cfg.Store)
	responsesConverter.SetPreviousResponseID(cfg.PreviousResponseID)
	responsesConverter.SetReasoningSummaryMode(cfg.ReasoningSummaryMode)
	responsesConverter.SetEncryptedReasoning(cfg.EncryptedReasoning)

	// AnthropicTransformer feeds Anthropic events to converter via receiver pattern
	t := toolcall.NewAnthropicTransformerWithReceiver(responsesConverter)
	t.SetKimiToolCallTransform(cfg.KimiToolCallTransform)
	t.SetGLM5ToolCallTransform(cfg.GLM5ToolCallTransform)

	// Wrap with web search if enabled
	if cfg.NeedsWebSearch() {
		adapter := websearch.GetDefaultAdapter()
		if adapter != nil && adapter.IsEnabled() {
			return wstransform.NewTransformer(t, adapter), nil
		}
	}

	return t, nil
}

// buildResponsesToOpenAI builds a transformer for Responses→OpenAI conversion.
//
// @brief Responses SSE → conversion → OpenAI Chat chunks.
//
// Chain: ResponsesToChatStreamingConverter → output
func buildResponsesToOpenAI(w io.Writer, cfg transform.Config) (transform.SSETransformer, error) {
	// ResponsesToChatStreamingConverter writes OpenAI chunks directly to w
	converter := convert.NewResponsesToChatStreamingConverter(w)

	// Wrap Stage as SSETransformer
	return transform.SSETransformerFromStage(converter), nil
}

// buildResponsesToAnthropic builds a transformer for Responses→Anthropic conversion.
//
// @brief Responses SSE → conversion → Anthropic SSE.
//
// Chain: ResponsesToAnthropicStreamingConverter → [web search] → output
func buildResponsesToAnthropic(w io.Writer, cfg transform.Config) (transform.SSETransformer, error) {
	// Final output stage
	var outputStage transform.Stage = transform.NewSSEWriterStage(w)

	// Wrap with web search if enabled
	if cfg.NeedsWebSearch() {
		adapter := websearch.GetDefaultAdapter()
		if adapter != nil && adapter.IsEnabled() {
			outputStage = wstransform.NewTransformer(transform.SSETransformerFromStage(outputStage), adapter)
		}
	}

	// ResponsesToAnthropicStreamingConverter sends Anthropic events directly to outputStage
	converter := convert.NewResponsesToAnthropicStreamingConverter(nil)
	converter.SetOutputStage(outputStage)

	// Wrap Stage as SSETransformer
	return transform.SSETransformerFromStage(converter), nil
}

// buildResponsesToResponses builds a transformer for Responses→Responses with optional passthrough.
//
// @brief Responses SSE → passthrough → Responses SSE.
func buildResponsesToResponses(w io.Writer, cfg transform.Config) (transform.SSETransformer, error) {
	return transform.NewPassthroughTransformer(w), nil
}
