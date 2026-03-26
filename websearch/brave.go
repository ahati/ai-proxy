package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// BraveBackend implements the Backend interface for Brave Search API.
type BraveBackend struct {
	apiKey  string
	timeout time.Duration
	client  *http.Client
}

// braveResponse represents the response from Brave Search API.
type braveResponse struct {
	Web braveWebResults `json:"web"`
}

// braveWebResults contains web search results.
type braveWebResults struct {
	Results []braveResult `json:"results"`
}

// braveResult represents a single result from Brave Search API.
type braveResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// NewBraveBackend creates a new Brave backend with the given API key and timeout.
//
// @param apiKey - the Brave Search API key
// @param timeout - the HTTP timeout for requests
// @return pointer to the new BraveBackend instance
func NewBraveBackend(apiKey string, timeout time.Duration) *BraveBackend {
	return &BraveBackend{
		apiKey:  apiKey,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the name of the backend.
//
// @return the backend name "brave"
func (b *BraveBackend) Name() string {
	return "brave"
}

// Search performs a web search using the Brave Search API.
//
// @param ctx - context for cancellation
// @param query - the search query
// @param opts - search options
// @return SearchResults and error
func (b *BraveBackend) Search(ctx context.Context, query string, opts *SearchOptions) (*SearchResults, error) {
	if b.apiKey == "" {
		return nil, fmt.Errorf("brave API key not configured")
	}

	count := 5
	if opts != nil && opts.MaxResults > 0 {
		count = opts.MaxResults
	}

	// Build the search URL
	searchURL := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), count)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", b.apiKey)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("brave API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var braveResp braveResponse
	if err := json.NewDecoder(resp.Body).Decode(&braveResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]SearchResult, 0, len(braveResp.Web.Results))
	for _, r := range braveResp.Web.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Description,
			Content: r.Description, // Brave doesn't return full content, use description
		})
	}

	return &SearchResults{Results: results}, nil
}
