// Package config provides configuration structs for the multi-provider multi-protocol proxy.
// This file defines the JSON schema for provider and model configuration.
package config

import (
	"os"
	"regexp"
	"strings"

	"ai-proxy/types"
)

// envVarRegex matches ${VAR_NAME} or $VAR_NAME patterns
var envVarRegex = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)\}|\$([a-zA-Z_][a-zA-Z0-9_]*)`)

// Provider defines an upstream API provider configuration.
// A provider represents an external API service that can handle requests.
type Provider struct {
	// Name is the unique identifier for this provider.
	Name string `json:"name"`
	// Endpoints maps protocol names to their specific endpoint URLs.
	// Required: at least one endpoint must be specified.
	// Protocols: "openai", "anthropic", "responses"
	// Example: {"openai": "https://api.provider.com/v1/chat/completions"}
	Endpoints map[string]string `json:"endpoints"`
	// Default specifies the default protocol for multi-protocol providers.
	// Required when Endpoints has more than one entry.
	// Must be one of: "openai", "anthropic", "responses".
	Default string `json:"default,omitempty"`
	// APIKey is the direct API key for authentication (optional).
	// If not set, EnvAPIKey is used to fetch from environment.
	APIKey string `json:"apiKey,omitempty"`
	// EnvAPIKey is the environment variable name containing the API key.
	// Used when APIKey is not directly set.
	EnvAPIKey string `json:"envApiKey,omitempty"`
}

// GetAPIKey returns the API key for this provider.
// If APIKey is set directly (may contain ${VAR} syntax), it is expanded and returned.
// Otherwise, the value is fetched from the environment variable specified by EnvAPIKey.
//
// @return string - the resolved API key, or empty string if not configured
func (p *Provider) GetAPIKey() string {
	if p.APIKey != "" {
		// Expand ${VAR} syntax if present, otherwise return as-is
		return expandEnvVars(p.APIKey)
	}
	return os.Getenv(p.EnvAPIKey)
}

// expandEnvVars expands ${VAR_NAME} patterns in a string with the
// corresponding environment variable values. If the environment
// variable is not set, the pattern is left unchanged.
//
// @param s - the string to expand
// @return the expanded string
func expandEnvVars(s string) string {
	if s == "" {
		return s
	}
	return envVarRegex.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name from ${VAR} or $VAR format
		varName := ""
		if strings.HasPrefix(match, "${") && strings.HasSuffix(match, "}") {
			varName = match[2 : len(match)-1]
		} else if strings.HasPrefix(match, "$") {
			varName = match[1:]
		}
		if varName == "" {
			return match
		}
		if value := os.Getenv(varName); value != "" {
			return value
		}
		// If env var not set, return empty string (or could return original match)
		return ""
	})
}

// GetEndpoint returns the endpoint URL for the specified protocol.
//
// @param protocol - the protocol name ("openai", "anthropic", "responses")
// @return the endpoint URL, or empty string if not found
func (p *Provider) GetEndpoint(protocol string) string {
	return p.Endpoints[protocol]
}

// SupportedProtocols returns the list of protocols this provider supports.
//
// @return slice of supported protocol names
func (p *Provider) SupportedProtocols() []string {
	protocols := make([]string, 0, len(p.Endpoints))
	for protocol := range p.Endpoints {
		protocols = append(protocols, protocol)
	}
	return protocols
}

// HasProtocol checks if the provider supports the given protocol.
//
// @param protocol - the protocol to check
// @return true if the protocol is supported
func (p *Provider) HasProtocol(protocol string) bool {
	_, exists := p.Endpoints[protocol]
	return exists
}

// GetDefaultProtocol returns the default protocol for this provider.
// Returns the configured Default field, or the only protocol if single-endpoint.
//
// @return the default protocol name, or empty string if none configured
func (p *Provider) GetDefaultProtocol() string {
	if p.Default != "" {
		return p.Default
	}
	// Single protocol: return the only one
	if len(p.Endpoints) == 1 {
		for protocol := range p.Endpoints {
			return protocol
		}
	}
	return ""
}

// Bool returns a pointer to a bool value.
// Used for pointer bool fields in ModelConfig to distinguish omitted from explicit false.
func Bool(b bool) *bool {
	return &b
}

// ModelConfig defines how a model alias maps to a specific provider and model.
// This allows routing requests to the appropriate upstream provider.
//
// Recursive resolution: if the Model field matches another key in the Models map,
// properties are inherited from that config and merged (alias overrides base).
// Use Bool() helper for boolean fields to distinguish "not set" (nil, inherit)
// from "explicitly false" (*false, override).
type ModelConfig struct {
	// Provider is the name of the provider to use for this model.
	Provider string `json:"provider"`
	// Model is the actual model identifier to use on the upstream provider.
	Model string `json:"model"`
	// Type specifies the output protocol: "openai", "anthropic", or "auto".
	// "auto" means use the incoming request's protocol for passthrough.
	// Empty defaults to provider's default protocol.
	Type string `json:"type,omitempty"`
	// KimiToolCallTransform enables tool call transformation for this model.
	// When true, tool calls are transformed between OpenAI and Anthropic formats.
	KimiToolCallTransform *bool `json:"kimi_tool_call_transform,omitempty"`
	// GLM5ToolCallTransform enables GLM-5 style XML tool call extraction.
	// When true, extracts tool calls from <tool_call> tags in reasoning_content.
	GLM5ToolCallTransform *bool `json:"glm5_tool_call_transform,omitempty"`
	// ReasoningSplit enables separate reasoning output for providers that support it.
	// When true, adds "reasoning_split": true to the ChatCompletionRequest.
	// Supported by MiniMax M2.7 to return reasoning in reasoning_details field
	// instead of embedded aisaI tags in content.
	ReasoningSplit *bool `json:"reasoning_split,omitempty"`
	// SamplingParams defines optional sampling parameters to merge into requests.
	// Only parameters not already set by the client are overridden when Override=false.
	SamplingParams *SamplingParams `json:"sampling_params,omitempty"`
}

// SamplingParams defines optional sampling parameters that can be configured
// at the model level and merged into requests.
// This allows enforcing consistent behavior across all requests to a model,
// or providing defaults when clients don't specify parameters.
type SamplingParams struct {
	// Override controls whether config values override client values.
	// When nil or true, config values always replace client values.
	// When false, config values only apply if client didn't set them.
	// Default: true (config overrides client).
	Override *bool `json:"override,omitempty"`

	// Temperature controls randomness in output generation.
	// Range: 0.0 to 2.0. Higher values produce more random output.
	Temperature *float64 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling.
	// Range: 0.0 to 1.0. Alternative to temperature.
	TopP *float64 `json:"top_p,omitempty"`

	// TopK limits sampling to the K most likely tokens.
	// Not supported by all providers.
	TopK *int `json:"top_k,omitempty"`

	// PresencePenalty penalizes new tokens based on presence in text so far.
	// Range: -2.0 to 2.0.
	PresencePenalty *float64 `json:"presence_penalty,omitempty"`

	// FrequencyPenalty penalizes new tokens based on frequency in text so far.
	// Range: -2.0 to 2.0.
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
}

// ShouldOverride returns whether config values should override client values.
// Default is true when Override is nil (omitted from config).
//
// @return true if config values should override client values
func (s *SamplingParams) ShouldOverride() bool {
	if s == nil || s.Override == nil {
		return true // Default: config overrides client
	}
	return *s.Override
}

// HasParams returns true if any sampling parameter is set.
//
// @return true if at least one parameter has a non-nil value
func (s *SamplingParams) HasParams() bool {
	if s == nil {
		return false
	}
	return s.Temperature != nil || s.TopP != nil || s.TopK != nil ||
		s.PresencePenalty != nil || s.FrequencyPenalty != nil
}

// FallbackConfig defines the fallback behavior when a request fails.
// Fallback allows routing failed requests to an alternative provider.
type FallbackConfig struct {
	// Enabled determines whether fallback is active.
	Enabled bool `json:"enabled"`
	// Provider is the name of the fallback provider.
	Provider string `json:"provider"`
	// Model is the model to use for fallback requests.
	Model string `json:"model"`
	// Type specifies the output protocol for fallback: "openai", "anthropic", or "auto".
	Type string `json:"type,omitempty"`
	// KimiToolCallTransform enables tool call transformation for fallback requests.
	KimiToolCallTransform bool `json:"kimi_tool_call_transform"`
	// GLM5ToolCallTransform enables GLM-5 style XML tool call extraction for fallback.
	GLM5ToolCallTransform bool `json:"glm5_tool_call_transform"`
	// ReasoningSplit enables separate reasoning output for fallback requests.
	ReasoningSplit bool `json:"reasoning_split,omitempty"`
	// SamplingParams defines optional sampling parameters to merge into fallback requests.
	SamplingParams *SamplingParams `json:"sampling_params,omitempty"`
}

// SummarizerConfig defines the configuration for the reasoning summarizer.
// The summarizer uses a small fast model to generate concise summaries of
// the model's internal reasoning process.
type SummarizerConfig struct {
	// Enabled determines whether summarization is active.
	Enabled bool `json:"enabled"`
	// Mode specifies the summarizer mode: "http" (API calls) or "local" (llama.cpp).
	// Default is "http".
	Mode string `json:"mode,omitempty"`
	// Provider is the name of the provider to use for HTTP summarization.
	Provider string `json:"provider"`
	// Model is the model to use for summarization (e.g., "gpt-4o-mini", "claude-3-haiku").
	Model string `json:"model"`
	// Prompt is an optional custom prompt for summarization.
	// If empty, a default prompt is used.
	Prompt string `json:"prompt,omitempty"`
	// Local contains configuration for local llama.cpp summarization.
	Local LocalSummarizerConfig `json:"local,omitempty"`
}

// LocalSummarizerConfig defines configuration for local llama.cpp summarization.
type LocalSummarizerConfig struct {
	// ModelPath is the path to the GGUF model file.
	ModelPath string `json:"model_path"`
	// ContextSize is the context window size for the model.
	ContextSize int `json:"context_size,omitempty"`
	// Threads is the number of CPU threads to use (0 = auto).
	Threads int `json:"threads,omitempty"`
	// GPULayers is the number of layers to offload to GPU (0 = CPU-only).
	GPULayers int `json:"gpu_layers,omitempty"`
	// MaxSummaryTokens is the maximum number of tokens to generate.
	MaxSummaryTokens int `json:"max_summary_tokens,omitempty"`
	// MaxReasoningChars limits input reasoning text length (0 = unlimited).
	MaxReasoningChars int `json:"max_reasoning_chars,omitempty"`
}

// ResponsesConfig defines configuration specific to the Responses API.
// These settings control behavior when handling Responses API requests.
type ResponsesConfig struct {
	// MaxContextTokens limits the conversation history token count.
	// If set, older turns are truncated to stay within this limit.
	// Default: 0 (no limit)
	MaxContextTokens int `json:"max_context_tokens"`
}

// MemoryLogsConfig defines the configuration for in-memory request logging.
// When enabled, the last N captured request/response logs are stored in memory
// and accessible via the /ui/api/logs API and Logs UI page.
type MemoryLogsConfig struct {
	// Enabled determines whether in-memory logging is active.
	// When nil (omitted from JSON), the feature defaults to enabled.
	// Set to false to disable at startup; can be toggled at runtime via UI.
	Enabled *bool `json:"enabled,omitempty"`
	// Capacity is the maximum number of log entries to retain.
	// Oldest entries are discarded when the limit is reached.
	// Default: 2000. Minimum: 10.
	Capacity int `json:"capacity,omitempty"`
}

// IsEnabled returns whether in-memory logging is enabled.
// Returns true (enabled) when Enabled is nil (omitted from config).
//
// @return true if in-memory logging should be active
func (c MemoryLogsConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// GetCapacity returns the configured capacity or the default of 2000.
//
// @return the effective capacity for the in-memory log store
func (c MemoryLogsConfig) GetCapacity() int {
	if c.Capacity <= 0 {
		return 2000
	}
	return c.Capacity
}

// MCPWebSearchConfig defines MCP endpoint settings for the web search tool.
type MCPWebSearchConfig struct {
	// Enabled determines whether the MCP web_search tool endpoint is active.
	Enabled bool `json:"enabled"`
}

// MCPConfig defines configuration for MCP (Model Context Protocol) server endpoints.
type MCPConfig struct {
	// WebSearch defines the MCP endpoint for web search tool exposure.
	WebSearch MCPWebSearchConfig `json:"websearch"`
}

// Schema is the root configuration structure for the multi-provider proxy.
// It contains all provider definitions, model mappings, and fallback settings.
type Schema struct {
	// Providers is a list of available upstream providers.
	Providers []Provider `json:"providers"`
	// Models maps model aliases to their provider and model configuration.
	Models map[string]ModelConfig `json:"models"`
	// Fallback defines the fallback behavior for failed requests.
	Fallback FallbackConfig `json:"fallback"`
	// Summarizer defines the summarizer configuration.
	Summarizer SummarizerConfig `json:"summarizer"`
	// Responses defines Responses API specific configuration.
	Responses ResponsesConfig `json:"responses"`
	// WebSearch defines the web search service configuration.
	WebSearch types.WebSearchConfig `json:"websearch"`
	// MemoryLogs defines the in-memory request logging configuration.
	MemoryLogs MemoryLogsConfig `json:"memoryLogs"`
	// RecursiveModelResolution enables recursive model chain resolution.
	// When true, Model field can reference another model key, with properties
	// merged along the chain (alias overrides base). When false, only single-level
	// resolution is performed. Default: false.
	RecursiveModelResolution bool `json:"recursive_model_resolution,omitempty"`
	// MCP defines the MCP server endpoint configuration.
	MCP MCPConfig `json:"mcp"`
}
