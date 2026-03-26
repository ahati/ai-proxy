package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ExaBackend implements the Backend interface for Exa.ai search API.
type ExaBackend struct {
	apiKey  string
	timeout time.Duration
	client  *http.Client
}

// exaRequest represents the request body for Exa API.
type exaRequest struct {
	Query     string            `json:"query"`
	Type      string            `json:"type"`
	NumResults int               `json:"numResults"`
	Contents  exaContentsConfig `json:"contents"`
}

// exaContentsConfig specifies what content to retrieve.
type exaContentsConfig struct {
	Text exaTextConfig `json:"text"`
}

// exaTextConfig specifies text content options.
type exaTextConfig struct {
	MaxCharacters int `json:"maxCharacters"`
}

// exaResponse represents the response from Exa API.
type exaResponse struct {
	Results []exaResult `json:"results"`
}

// exaResult represents a single result from Exa API.
type exaResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Text    string `json:"text"`
	Summary string `json:"summary,omitempty"`
}

// NewExaBackend creates a new Exa backend with the given API key and timeout.
//
// @param apiKey - the Exa API key
// @param timeout - the HTTP timeout for requests
// @return pointer to the new ExaBackend instance
func NewExaBackend(apiKey string, timeout time.Duration) *ExaBackend {
	return &ExaBackend{
		apiKey:  apiKey,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the name of the backend.
//
// @return the backend name "exa"
func (b *ExaBackend) Name() string {
	return "exa"
}

// Search performs a web search using the Exa API.
//
// @param ctx - context for cancellation
// @param query - the search query
// @param opts - search options
// @return SearchResults and error
func (b *ExaBackend) Search(ctx context.Context, query string, opts *SearchOptions) (*SearchResults, error) {
	if b.apiKey == "" {
		return nil, fmt.Errorf("exa API key not configured")
	}

	numResults := 5
	if opts != nil && opts.MaxResults > 0 {
		numResults = opts.MaxResults
	}

	reqBody := exaRequest{
		Query:      query,
		Type:       "auto",
		NumResults: numResults,
		Contents: exaContentsConfig{
			Text: exaTextConfig{
				MaxCharacters: 4000,
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.exa.ai/search", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", b.apiKey)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("exa API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var exaResp exaResponse
	if err := json.NewDecoder(resp.Body).Decode(&exaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]SearchResult, 0, len(exaResp.Results))
	for _, r := range exaResp.Results {
		content := r.Text
		if r.Summary != "" {
			content = r.Summary
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Content: content,
		})
	}

	return &SearchResults{Results: results}, nil
}
