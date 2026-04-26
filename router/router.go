// Package router provides model resolution with fallback support.
// It implements routing logic to map model names to providers.
package router

import (
	"fmt"
	"strings"

	"ai-proxy/config"
)

// Router defines the interface for model resolution.
type Router interface {
	// Resolve resolves a model name to a route with provider information.
	Resolve(modelName string) (*ResolvedRoute, error)
	// ResolveWithProtocol resolves a model name with incoming protocol context.
	// Used for "auto" type routing to enable passthrough optimization.
	ResolveWithProtocol(modelName, incomingProtocol string) (*ResolvedRoute, error)
	// GetProvider retrieves a provider by name.
	GetProvider(name string) (config.Provider, bool)
	// ListModels returns all configured model names.
	ListModels() []string
}

// ResolvedRoute contains the resolved routing information for a model.
type ResolvedRoute struct {
	// Provider is the upstream provider configuration.
	Provider config.Provider
	// Model is the actual model identifier on the upstream provider.
	Model string
	// OutputProtocol specifies the protocol to use: "openai", "anthropic", or "auto".
	OutputProtocol string
	// KimiToolCallTransform enables tool call transformation for this route.
	KimiToolCallTransform bool
	// GLM5ToolCallTransform enables GLM-5 XML tool call extraction for this route.
	GLM5ToolCallTransform bool
	// ReasoningSplit enables separate reasoning output for this route.
	ReasoningSplit bool
	// IsPassthrough indicates when no protocol transformation is needed.
	// True when incoming protocol matches output protocol (passthrough mode).
	// SamplingParams contains optional sampling parameters to inject.
	SamplingParams *config.SamplingParams
	IsPassthrough  bool
}

// router implements the Router interface.
type router struct {
	schema *config.Schema
	// providersMap is a lookup map from provider name to Provider.
	providersMap map[string]config.Provider
}

// NewRouter creates a new Router from the given schema.
// Returns an error if the schema is nil.
func NewRouter(s *config.Schema) (Router, error) {
	if s == nil {
		return nil, fmt.Errorf("schema cannot be nil")
	}

	// Build provider lookup map
	providersMap := make(map[string]config.Provider)
	for _, p := range s.Providers {
		providersMap[p.Name] = p
	}

	return &router{
		schema:       s,
		providersMap: providersMap,
	}, nil
}

// Resolve resolves a model name to a route with provider information.
// It first checks for an exact model match in the schema, following recursive
// model references (when Model field matches another key in Models) and merging
// properties along the chain. Alias properties override base properties.
// If not found and fallback is enabled, it uses the fallback configuration.
// Returns an error if the model is unknown and no fallback is available.
//
// @pre modelName must not be empty
// @post returned OutputProtocol is determined by merged modelConfig.Type, fallback.Type, or provider's default
// @post IsPassthrough is false when returned (use ResolveWithProtocol for passthrough detection)
func (r *router) Resolve(modelName string) (*ResolvedRoute, error) {
	// Check for exact model match with recursive resolution
	if modelConfig, ok := r.schema.Models[modelName]; ok {
		merged, upstreamModel, err := r.resolveModelChain(modelName, modelConfig)
		if err != nil {
			return nil, err
		}

		provider, ok := r.providersMap[merged.Provider]
		if !ok {
			return nil, fmt.Errorf("provider '%s' not found for model '%s'", merged.Provider, modelName)
		}

		// Determine output protocol
		outputProtocol := provider.GetDefaultProtocol() // default to provider's default
		if merged.Type != "" {
			outputProtocol = merged.Type
		}

		return &ResolvedRoute{
			Provider:              provider,
			Model:                 upstreamModel,
			OutputProtocol:        outputProtocol,
			KimiToolCallTransform: derefBool(merged.KimiToolCallTransform),
			GLM5ToolCallTransform: derefBool(merged.GLM5ToolCallTransform),
			ReasoningSplit:        derefBool(merged.ReasoningSplit),
			SamplingParams:        merged.SamplingParams,
			IsPassthrough:         false,
		}, nil
	}

	// Model not found, check fallback
	if r.schema.Fallback.Enabled {
		provider, ok := r.providersMap[r.schema.Fallback.Provider]
		if !ok {
			return nil, fmt.Errorf("provider '%s' not found for fallback", r.schema.Fallback.Provider)
		}

		// Replace {model} placeholder with the requested model name
		model := r.schema.Fallback.Model
		model = strings.ReplaceAll(model, "{model}", modelName)

		// Determine output protocol for fallback
		outputProtocol := provider.GetDefaultProtocol() // default to provider's default
		if r.schema.Fallback.Type != "" {
			outputProtocol = r.schema.Fallback.Type
		}

		return &ResolvedRoute{
			Provider:              provider,
			Model:                 model,
			OutputProtocol:        outputProtocol,
			KimiToolCallTransform: r.schema.Fallback.KimiToolCallTransform,
			GLM5ToolCallTransform: r.schema.Fallback.GLM5ToolCallTransform,
			ReasoningSplit:        r.schema.Fallback.ReasoningSplit,
			SamplingParams:        r.schema.Fallback.SamplingParams,
			IsPassthrough:         false,
		}, nil
	}

	// No match and no fallback
	return nil, fmt.Errorf("unknown model: '%s'", modelName)
}

// resolveModelChain follows model references recursively and returns the merged
// configuration and the final upstream model identifier.
//
// If the Model field of a config matches another key in schema.Models, the
// chain continues. Properties are merged with alias values overriding base values.
// Cycle detection prevents infinite recursion.
//
// Self-reference (Model == current key) is treated as a leaf, not a cycle,
// because it simply means the upstream model ID matches the alias name.
func (r *router) resolveModelChain(modelName string, initialConfig config.ModelConfig) (config.ModelConfig, string, error) {
	chain := []config.ModelConfig{initialConfig}
	visited := map[string]bool{modelName: true}

	currentName := modelName
	current := initialConfig
	for {
		nextName := current.Model
		if nextName == "" {
			// Leaf: empty model field, use empty string as upstream model
			break
		}

		// Self-reference: the upstream model ID happens to match this config's key.
		// Treat as a leaf, not a cycle.
		if nextName == currentName {
			break
		}

		nextConfig, ok := r.schema.Models[nextName]
		if !ok {
			// Leaf: model field is not a key in Models map
			break
		}

		if visited[nextName] {
			return config.ModelConfig{}, "", fmt.Errorf(
				"model resolution cycle detected: '%s' references '%s' which was already visited",
				modelName, nextName,
			)
		}

		visited[nextName] = true
		chain = append(chain, nextConfig)
		currentName = nextName
		current = nextConfig
	}

	merged := mergeModelConfigs(chain)
	return merged, current.Model, nil
}

// mergeModelConfigs merges a chain of model configs, with later configs (bases)
// providing defaults and earlier configs (aliases) overriding them.
// String fields: non-empty overrides. Bool fields: non-nil overrides.
func mergeModelConfigs(chain []config.ModelConfig) config.ModelConfig {
	if len(chain) == 0 {
		return config.ModelConfig{}
	}

	// Start with the deepest base (last in chain)
	merged := chain[len(chain)-1]

	// Walk backwards from second-to-last to first, overlaying alias properties
	for i := len(chain) - 2; i >= 0; i-- {
		alias := chain[i]
		if alias.Provider != "" {
			merged.Provider = alias.Provider
		}
		if alias.Type != "" {
			merged.Type = alias.Type
		}
		if alias.KimiToolCallTransform != nil {
			merged.KimiToolCallTransform = alias.KimiToolCallTransform
		}
		if alias.GLM5ToolCallTransform != nil {
			merged.GLM5ToolCallTransform = alias.GLM5ToolCallTransform
		}
		if alias.ReasoningSplit != nil {
			merged.ReasoningSplit = alias.ReasoningSplit
		}
		if alias.SamplingParams != nil {
			merged.SamplingParams = alias.SamplingParams
		}
	}

	return merged
}

// derefBool returns the value of a *bool, defaulting to false if nil.
func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// ResolveWithProtocol resolves a model name with incoming protocol context.
// This is used for "auto" type routing to enable passthrough optimization.
// When the model is configured with type "auto", it checks if the provider
// supports the incoming protocol and sets IsPassthrough accordingly.
//
// @pre modelName must not be empty
// @pre incomingProtocol should be "openai", "anthropic", or "responses"
// @post If OutputProtocol is "auto", it will be resolved to a concrete protocol
// @post IsPassthrough will be true when incoming protocol matches output protocol
func (r *router) ResolveWithProtocol(modelName, incomingProtocol string) (*ResolvedRoute, error) {
	// First get base route
	route, err := r.Resolve(modelName)
	if err != nil {
		return nil, err
	}

	// If not auto type, return as-is
	if route.OutputProtocol != "auto" {
		return route, nil
	}

	// Handle auto type - check if provider supports incoming protocol
	if route.Provider.HasProtocol(incomingProtocol) {
		route.OutputProtocol = incomingProtocol
		route.IsPassthrough = true
	} else {
		// Use provider's default protocol
		route.OutputProtocol = route.Provider.GetDefaultProtocol()
		route.IsPassthrough = false
	}

	return route, nil
}

// GetProvider retrieves a provider by name.
// Returns the provider and true if found, or an empty provider and false if not.
func (r *router) GetProvider(name string) (config.Provider, bool) {
	provider, ok := r.providersMap[name]
	return provider, ok
}

// ListModels returns all configured model names.
func (r *router) ListModels() []string {
	models := make([]string, 0, len(r.schema.Models))
	for name := range r.schema.Models {
		models = append(models, name)
	}
	return models
}
