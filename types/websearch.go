package types

import "encoding/json"

// WebSearchConfig holds configuration for the web search service.
type WebSearchConfig struct {
	Enabled     bool   `json:"enabled"`
	Provider    string `json:"provider"` // "exa", "brave", or "ddg"
	ExaAPIKey   string `json:"exa_api_key"`
	BraveAPIKey string `json:"brave_api_key"`
	MaxResults  int    `json:"max_results"` // default: 10
	Timeout     int    `json:"timeout"`     // seconds, default: 30
}

// WebSearchInput represents the input parameters for a web search query.
type WebSearchInput struct {
	Query          string        `json:"query"`
	AllowedDomains []string      `json:"allowed_domains,omitempty"`
	BlockedDomains []string      `json:"blocked_domains,omitempty"`
	UserLocation   *UserLocation `json:"user_location,omitempty"`
}

// UserLocation provides geographic context for search results.
type UserLocation struct {
	Type     string `json:"type"` // "approximate"
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Country  string `json:"country,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

// WebSearchToolResult represents the result from executing a web search.
type WebSearchToolResult struct {
	Type      string            `json:"type"` // "web_search_tool_result"
	ToolUseID string            `json:"tool_use_id"`
	Content   []WebSearchResult `json:"content"`
}

// WebSearchResult represents a single search result.
type WebSearchResult struct {
	Type      string `json:"type"` // "web_search_result" or "web_search_error"
	Title     string `json:"title,omitempty"`
	URL       string `json:"url,omitempty"`
	ErrorCode string `json:"error_code,omitempty"` // For error type
	Message   string `json:"message,omitempty"`    // For error type
}

// WebSearchTool defines the web search tool for API requests.
type WebSearchTool struct {
	Type           string        `json:"type"` // "web_search_20250305"
	Name           string        `json:"name"` // "web_search"
	MaxUses        int           `json:"max_uses,omitempty"`
	AllowedDomains []string      `json:"allowed_domains,omitempty"`
	BlockedDomains []string      `json:"blocked_domains,omitempty"`
	UserLocation   *UserLocation `json:"user_location,omitempty"`
}

// ServerToolUseBlock represents a server-side tool use content block.
type ServerToolUseBlock struct {
	Type  string          `json:"type"` // "server_tool_use"
	ID    string          `json:"id"`
	Name  string          `json:"name"` // "web_search"
	Input json.RawMessage `json:"input"`
}
