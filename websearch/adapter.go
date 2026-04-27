// Package websearch provides web search capabilities with multiple backend implementations.
// This file contains an adapter to bridge websearch.Service to transform/websearch.Service.
package websearch

import (
	"context"
	"time"

	"ai-proxy/config"
	"ai-proxy/types"
)

// TransformerAdapter adapts websearch.Service to the transform/websearch.Service interface.
// This allows the websearch.Service to be used with the websearch transformer.
//
// @brief Adapter bridging websearch.Service to transform/websearch.Service interface.
type TransformerAdapter struct {
	service *Service
}

// NewTransformerAdapter creates a new adapter for the given websearch service.
//
// @param service - the websearch service to adapt. May be nil.
// @return *TransformerAdapter - the adapter, or nil if service is nil.
func NewTransformerAdapter(service *Service) *TransformerAdapter {
	if service == nil {
		return nil
	}
	return &TransformerAdapter{service: service}
}

// ExecuteSearch implements the transform/websearch.Service interface.
// It returns individual web_search_result items with title, url, and content fields.
//
// @param ctx - context for cancellation and timeout
// @param query - the search query string
// @param allowedDomains - domains to restrict results to
// @param blockedDomains - domains to exclude from results
// @return slice of WebSearchResult and error
func (a *TransformerAdapter) ExecuteSearch(ctx context.Context, query string, allowedDomains, blockedDomains []string) ([]types.WebSearchResult, error) {
	if a == nil || a.service == nil {
		return nil, nil
	}

	opts := &SearchOptions{
		AllowedDomains: allowedDomains,
		BlockedDomains: blockedDomains,
		MaxResults:     a.service.config.MaxResults,
	}

	results, err := a.service.SearchRaw(ctx, query, opts)
	if err != nil {
		return []types.WebSearchResult{{
			Type:      "web_search_error",
			ErrorCode: "search_error",
			Message:   err.Error(),
		}}, nil
	}

	if len(results.Results) == 0 {
		return []types.WebSearchResult{}, nil
	}

	// Convert each result to WebSearchResult with proper type
	output := make([]types.WebSearchResult, len(results.Results))
	for i, r := range results.Results {
		output[i] = types.WebSearchResult{
			Type:  "web_search_result",
			Title: r.Title,
			URL:   r.URL,
		}
	}
	return output, nil
}

// IsEnabled returns true if the adapter has a valid service.
func (a *TransformerAdapter) IsEnabled() bool {
	return a != nil && a.service != nil
}

// InitDefaultService initializes the global web search service from the configuration.
// This follows the same pattern as summarizer.InitDefaultService.
//
// @param cfg - the web search configuration from config.json
// @return *Service - initialized service, or nil if disabled/invalid
func InitDefaultService(cfg types.WebSearchConfig) *Service {
	if !cfg.Enabled {
		return nil
	}

	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	maxResults := cfg.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}

	return NewService(Config{
		Enabled:        cfg.Enabled,
		DefaultBackend: cfg.Provider,
		MaxResults:     maxResults,
		Timeout:        timeout,
		ExaAPIKey:      config.ExpandEnvVars(cfg.ExaAPIKey),
		BraveAPIKey:    config.ExpandEnvVars(cfg.BraveAPIKey),
	})
}

// DefaultService is the global web search service instance.
var DefaultService *Service

// GetDefaultAdapter returns an adapter for the default web search service.
// Returns nil if web search is not enabled.
func GetDefaultAdapter() *TransformerAdapter {
	return NewTransformerAdapter(DefaultService)
}
