// Package mcp provides an MCP (Model Context Protocol) server endpoint
// that exposes web search as both a tool and a resource template.
package mcp

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"ai-proxy/websearch"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// webSearchInput defines the input schema for the web_search MCP tool.
type webSearchInput struct {
	Query string `json:"query" jsonschema:"required,The search query"`
}

// Server wraps an MCP server that exposes web search as a tool and resource.
type Server struct {
	server  *mcpsdk.Server
	service *websearch.Service
}

// NewServer creates an MCP server with the web_search tool and resource template registered.
// Returns nil if service is nil.
func NewServer(ws *websearch.Service) *Server {
	if ws == nil {
		return nil
	}

	s := &Server{
		service: ws,
	}

	s.server = mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "ai-proxy",
		Version: "1.0.0",
	}, nil)

	// Register as a tool (for clients that support MCP tools)
	mcpsdk.AddTool(s.server, &mcpsdk.Tool{
		Name:        "web_search",
		Description: "Search the web using configured backend (Exa, Brave, or DuckDuckGo)",
	}, s.handleWebSearch)

	// Register as a resource template (for clients that only support MCP resources, e.g. Codex)
	s.server.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		Name:        "web_search",
		URITemplate: "websearch://search/{query}",
		Description: "Search the web. Read this resource with a query path to get search results.",
		MIMEType:    "text/plain",
	}, s.handleWebSearchResource)

	return s
}

// handleWebSearch implements the MCP tool handler for web_search.
func (s *Server) handleWebSearch(ctx context.Context, req *mcpsdk.CallToolRequest, input webSearchInput) (*mcpsdk.CallToolResult, any, error) {
	result := s.service.ExecuteSearch(ctx, "", &websearch.WebSearchInput{
		Query: input.Query,
	})

	text := formatResultContent(result)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: text},
		},
		IsError: result.IsError,
	}, nil, nil
}

// handleWebSearchResource handles resource reads for websearch://search/{query}.
func (s *Server) handleWebSearchResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	// Extract query from URI: websearch://search/{query}
	uri := req.Params.URI
	query := extractQueryFromURI(uri)

	if query == "" {
		return &mcpsdk.ReadResourceResult{
			Contents: []*mcpsdk.ResourceContents{
				{
					URI:      uri,
					MIMEType: "text/plain",
					Text:     "Error: no search query provided in URI path",
				},
			},
		}, nil
	}

	result := s.service.ExecuteSearch(ctx, "", &websearch.WebSearchInput{
		Query: query,
	})

	text := formatResultContent(result)

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{
			{
				URI:      uri,
				MIMEType: "text/plain",
				Text:     text,
			},
		},
	}, nil
}

// extractQueryFromURI parses the query from a websearch://search/{query} URI.
func extractQueryFromURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	// Path is "/search/{query}" — strip the leading "/" from TrimPrefix result
	path := strings.TrimPrefix(u.Path, "/search/")
	// TrimPrefix returns the original if prefix not found, so check for leading /
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		// Fallback: check for ?query=... query parameter
		return u.Query().Get("query")
	}
	// URL-decode the path component
	decoded, err := url.PathUnescape(path)
	if err != nil {
		return path
	}
	return decoded
}

// formatResultContent extracts text from the service result content items.
func formatResultContent(r *websearch.WebSearchToolResult) string {
	if len(r.Content) == 0 {
		return "No results found."
	}
	var b strings.Builder
	for i, c := range r.Content {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(c.Text)
	}
	return b.String()
}

// Handler returns an http.Handler for the Streamable HTTP transport.
// Mount this on the Gin router at /mcp.
func (s *Server) Handler() http.Handler {
	getServer := func(r *http.Request) *mcpsdk.Server {
		return s.server
	}
	inner := mcpsdk.NewStreamableHTTPHandler(getServer, &mcpsdk.StreamableHTTPOptions{
		Stateless: true,
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read body", http.StatusBadRequest)
				return
			}
			r.Body.Close()
			body = fixWebsearchURISpaces(body)
			r.Body = io.NopCloser(strings.NewReader(string(body)))
			r.ContentLength = int64(len(body))
		}
		inner.ServeHTTP(w, r)
	})
}

// fixWebsearchURISpaces percent-encodes bare spaces in websearch:// URIs
// within JSON-RPC request bodies. MCP clients like Codex may send unencoded
// spaces in resource URIs, which the go-sdk's URI template matcher rejects.
func fixWebsearchURISpaces(body []byte) []byte {
	s := string(body)
	if !strings.Contains(s, "websearch://") {
		return body
	}
	var result strings.Builder
	result.Grow(len(s))
	for {
		idx := strings.Index(s, "websearch://search/")
		if idx < 0 {
			result.WriteString(s)
			break
		}
		result.WriteString(s[:idx])
		prefix := "websearch://search/"
		result.WriteString(prefix)
		s = s[idx+len(prefix):]
		// Collect URI path until the closing JSON quote
		var path strings.Builder
		for len(s) > 0 && s[0] != '"' {
			if s[0] == '\\' && len(s) > 1 {
				path.WriteByte(s[0])
				path.WriteByte(s[1])
				s = s[2:]
			} else if s[0] == ' ' {
				path.WriteString("%20")
				s = s[1:]
			} else {
				path.WriteByte(s[0])
				s = s[1:]
			}
		}
		result.WriteString(path.String())
	}
	return []byte(result.String())
}

// Enabled returns true if the MCP server is ready to serve requests.
func (s *Server) Enabled() bool {
	return s != nil && s.server != nil && s.service != nil
}
