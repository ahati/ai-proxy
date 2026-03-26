// Package websearch provides web search capabilities with multiple backend implementations.
package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"ai-proxy/logging"
)

// Config holds the configuration for the web search service.
type Config struct {
	Enabled        bool
	DefaultBackend string
	MaxResults     int
	Timeout        time.Duration
	ExaAPIKey      string
	BraveAPIKey    string
}

// WebSearchInput represents the input for a web search operation.
type WebSearchInput struct {
	Query          string   `json:"query"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	BlockedDomains []string `json:"blocked_domains,omitempty"`
	MaxResults     int      `json:"max_results,omitempty"`
}

// WebSearchToolResult represents the result of a web search tool execution.
type WebSearchToolResult struct {
	ToolUseID string          `json:"tool_use_id"`
	Type      string          `json:"type"`
	Content   []ResultContent `json:"content"`
	IsError   bool            `json:"is_error,omitempty"`
}

// ResultContent represents a single content item in the result.
type ResultContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SearchOptions contains options for customizing search behavior.
type SearchOptions struct {
	AllowedDomains []string
	BlockedDomains []string
	MaxResults     int
}

// SearchResult represents a single search result.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Content string `json:"content"`
}

// SearchResults contains a collection of search results.
type SearchResults struct {
	Results []SearchResult `json:"results"`
}

// Backend defines the interface for web search backends.
type Backend interface {
	// Name returns the name of the backend.
	Name() string
	// Search performs a web search with the given query and options.
	Search(ctx context.Context, query string, opts *SearchOptions) (*SearchResults, error)
}

// Service coordinates web search execution across multiple backends.
type Service struct {
	backend     Backend
	config      Config
	searchCount atomic.Int64
	mu          sync.RWMutex
}

// NewService creates a new web search service with the given configuration.
//
// @param cfg - the configuration for the service
// @return pointer to the new Service instance
func NewService(cfg Config) *Service {
	if cfg.MaxResults <= 0 {
		cfg.MaxResults = 5
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	var backend Backend
	switch cfg.DefaultBackend {
	case "exa":
		backend = NewExaBackend(cfg.ExaAPIKey, cfg.Timeout)
	case "brave":
		backend = NewBraveBackend(cfg.BraveAPIKey, cfg.Timeout)
	case "ddg":
		backend = NewDDGBackend(cfg.Timeout)
	default:
		// Default to DuckDuckGo as fallback (no API key required)
		backend = NewDDGBackend(cfg.Timeout)
	}

	return &Service{
		backend: backend,
		config:  cfg,
	}
}

// ExecuteSearch performs a web search and returns formatted results.
//
// @param ctx - context for cancellation and timeout
// @param toolUseID - unique identifier for the tool use
// @param input - the search input parameters
// @return WebSearchToolResult containing the search results
func (s *Service) ExecuteSearch(ctx context.Context, toolUseID string, input *WebSearchInput) *WebSearchToolResult {
	s.searchCount.Add(1)

	if input == nil || input.Query == "" {
		return &WebSearchToolResult{
			ToolUseID: toolUseID,
			Type:      "tool_result",
			Content: []ResultContent{
				{Type: "text", Text: "Error: empty search query"},
			},
			IsError: true,
		}
	}

	opts := &SearchOptions{
		AllowedDomains: input.AllowedDomains,
		BlockedDomains: input.BlockedDomains,
		MaxResults:     input.MaxResults,
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = s.config.MaxResults
	}

	results, err := s.backend.Search(ctx, input.Query, opts)
	if err != nil {
		logging.ErrorMsg("websearch: search failed for query=%s backend=%s: %v", input.Query, s.backend.Name(), err)
		return &WebSearchToolResult{
			ToolUseID: toolUseID,
			Type:      "tool_result",
			Content: []ResultContent{
				{Type: "text", Text: fmt.Sprintf("Search error: %v", err)},
			},
			IsError: true,
		}
	}

	if len(results.Results) == 0 {
		return &WebSearchToolResult{
			ToolUseID: toolUseID,
			Type:      "tool_result",
			Content: []ResultContent{
				{Type: "text", Text: "No results found for the query."},
			},
		}
	}

	text := s.formatResults(results)
	return &WebSearchToolResult{
		ToolUseID: toolUseID,
		Type:      "tool_result",
		Content: []ResultContent{
			{Type: "text", Text: text},
		},
	}
}

// GetUsage returns the total number of searches performed.
//
// @return the count of searches performed
func (s *Service) GetUsage() int64 {
	return s.searchCount.Load()
}

// formatResults converts search results to a readable text format.
func (s *Service) formatResults(results *SearchResults) string {
	var text string
	for i, r := range results.Results {
		if i > 0 {
			text += "\n\n"
		}
		text += fmt.Sprintf("## %s\n\n%s\n\nURL: %s", r.Title, r.Content, r.URL)
	}
	return text
}

// SetBackend allows changing the backend at runtime.
//
// @param backend - the new backend to use
func (s *Service) SetBackend(backend Backend) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backend = backend
}

// GetBackend returns the current backend name.
//
// @return the name of the current backend
func (s *Service) GetBackend() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backend.Name()
}

// MarshalJSON implements json.Marshaler for WebSearchToolResult.
func (r *WebSearchToolResult) MarshalJSON() ([]byte, error) {
	type Alias WebSearchToolResult
	return json.Marshal((*Alias)(r))
}

// SearchRaw performs a web search and returns raw search results.
// This is used by the transformer adapter to get individual results with title/url/content.
//
// @param ctx - context for cancellation
// @param query - the search query
// @param opts - search options (may be nil)
// @return SearchResults and error
func (s *Service) SearchRaw(ctx context.Context, query string, opts *SearchOptions) (*SearchResults, error) {
	s.searchCount.Add(1)

	if s.backend == nil {
		return nil, fmt.Errorf("no backend configured")
	}
	if opts == nil {
		opts = &SearchOptions{MaxResults: s.config.MaxResults}
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = s.config.MaxResults
	}
	return s.backend.Search(ctx, query, opts)
}
