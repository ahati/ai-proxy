package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DDGBackend implements the Backend interface for DuckDuckGo Instant Answer API.
type DDGBackend struct {
	timeout time.Duration
	client  *http.Client
}

// ddgResponse represents the response from DuckDuckGo Instant Answer API.
type ddgResponse struct {
	AbstractText   string      `json:"AbstractText"`
	AbstractURL    string      `json:"AbstractURL"`
	AbstractSource string      `json:"AbstractSource"`
	Heading        string      `json:"Heading"`
	RelatedTopics  []ddgTopic  `json:"RelatedTopics"`
	Results        []ddgResult `json:"Results"`
}

// ddgTopic represents a related topic from DuckDuckGo.
type ddgTopic struct {
	Text string `json:"Text"`
	URL  string `json:"FirstURL"`
}

// ddgResult represents a result from DuckDuckGo.
type ddgResult struct {
	Text string `json:"Text"`
	URL  string `json:"FirstURL"`
}

// NewDDGBackend creates a new DuckDuckGo backend with the given timeout.
//
// @param timeout - the HTTP timeout for requests
// @return pointer to the new DDGBackend instance
func NewDDGBackend(timeout time.Duration) *DDGBackend {
	return &DDGBackend{
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the name of the backend.
//
// @return the backend name "ddg"
func (b *DDGBackend) Name() string {
	return "ddg"
}

// Search performs a web search using the DuckDuckGo Instant Answer API.
//
// @param ctx - context for cancellation
// @param query - the search query
// @param opts - search options
// @return SearchResults and error
func (b *DDGBackend) Search(ctx context.Context, query string, opts *SearchOptions) (*SearchResults, error) {
	// DuckDuckGo Instant Answer API is free and requires no API key
	searchURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1",
		url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("duckduckgo API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var ddgResp ddgResponse
	if err := json.NewDecoder(resp.Body).Decode(&ddgResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]SearchResult, 0)

	// Add the main abstract if available
	if ddgResp.AbstractText != "" {
		title := ddgResp.Heading
		if title == "" {
			title = ddgResp.AbstractSource
		}
		results = append(results, SearchResult{
			Title:   title,
			URL:     ddgResp.AbstractURL,
			Content: ddgResp.AbstractText,
		})
	}

	// Add related topics
	maxResults := 5
	if opts != nil && opts.MaxResults > 0 {
		maxResults = opts.MaxResults
	}

	for _, topic := range ddgResp.RelatedTopics {
		if len(results) >= maxResults {
			break
		}
		if topic.Text != "" && topic.URL != "" {
			// Extract title from text (usually at the beginning before " - ")
			title := extractTitle(topic.Text)
			results = append(results, SearchResult{
				Title:   title,
				URL:     topic.URL,
				Snippet: topic.Text,
				Content: topic.Text,
			})
		}
	}

	// Add any additional results
	for _, result := range ddgResp.Results {
		if len(results) >= maxResults {
			break
		}
		if result.Text != "" && result.URL != "" {
			title := extractTitle(result.Text)
			results = append(results, SearchResult{
				Title:   title,
				URL:     result.URL,
				Snippet: result.Text,
				Content: result.Text,
			})
		}
	}

	// If no results found, return an empty result set
	if len(results) == 0 {
		return &SearchResults{Results: []SearchResult{}}, nil
	}

	return &SearchResults{Results: results}, nil
}

// extractTitle extracts a title from DuckDuckGo result text.
// DuckDuckGo results often have format "Title - Description" or just description.
func extractTitle(text string) string {
	// Try to extract title from "Title - Description" format
	if idx := strings.Index(text, " - "); idx > 0 {
		return text[:idx]
	}
	// Try to extract title from "Title: Description" format
	if idx := strings.Index(text, ": "); idx > 0 && idx < 100 {
		return text[:idx]
	}
	// Use first 50 chars as title if no separator found
	if len(text) > 50 {
		return text[:50] + "..."
	}
	return text
}
