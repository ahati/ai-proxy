package mcp

import (
	"testing"

	"ai-proxy/websearch"

	"github.com/stretchr/testify/assert"
)

func TestNewServer_NilService(t *testing.T) {
	s := NewServer(nil)
	assert.Nil(t, s)
}

func TestNewServer_WithService(t *testing.T) {
	svc := websearch.NewService(websearch.Config{
		Enabled:        true,
		DefaultBackend: "ddg",
		MaxResults:     5,
	})
	s := NewServer(svc)
	assert.NotNil(t, s)
	assert.True(t, s.Enabled())
}

func TestServer_Enabled(t *testing.T) {
	svc := websearch.NewService(websearch.Config{
		Enabled:        true,
		DefaultBackend: "ddg",
		MaxResults:     5,
	})
	s := NewServer(svc)
	assert.True(t, s.Enabled())

	s2 := &Server{}
	assert.False(t, s2.Enabled())
}

func TestServer_Handler(t *testing.T) {
	svc := websearch.NewService(websearch.Config{
		Enabled:        true,
		DefaultBackend: "ddg",
		MaxResults:     5,
	})
	s := NewServer(svc)
	h := s.Handler()
	assert.NotNil(t, h)
}

func TestFormatResultContent(t *testing.T) {
	tests := []struct {
		name   string
		result *websearch.WebSearchToolResult
		want   string
	}{
		{
			name:   "empty content",
			result: &websearch.WebSearchToolResult{Content: nil},
			want:   "No results found.",
		},
		{
			name: "single result",
			result: &websearch.WebSearchToolResult{
				Content: []websearch.ResultContent{
					{Type: "text", Text: "Hello world"},
				},
			},
			want: "Hello world",
		},
		{
			name: "multiple results",
			result: &websearch.WebSearchToolResult{
				Content: []websearch.ResultContent{
					{Type: "text", Text: "First"},
					{Type: "text", Text: "Second"},
				},
			},
			want: "First\nSecond",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatResultContent(tt.result)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractQueryFromURI(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "simple query",
			uri:  "websearch://search/golang",
			want: "golang",
		},
		{
			name: "url-encoded query",
			uri:  "websearch://search/hello%20world",
			want: "hello world",
		},
		{
			name: "multi-segment path",
			uri:  "websearch://search/golang%20MCP%20protocol",
			want: "golang MCP protocol",
		},
		{
			name: "empty path after search",
			uri:  "websearch://search/",
			want: "",
		},
		{
			name: "no path",
			uri:  "websearch://search",
			want: "",
		},
		{
			name: "invalid URI",
			uri:  "://invalid",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractQueryFromURI(tt.uri)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFixWebsearchURISpaces(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "no websearch URI",
			body: `{"method":"ping"}`,
			want: `{"method":"ping"}`,
		},
		{
			name: "already encoded",
			body: `{"uri":"websearch://search/hello%20world"}`,
			want: `{"uri":"websearch://search/hello%20world"}`,
		},
		{
			name: "unencoded spaces",
			body: `{"uri":"websearch://search/Munich weather today"}`,
			want: `{"uri":"websearch://search/Munich%20weather%20today"}`,
		},
		{
			name: "mixed encoded and unencoded",
			body: `{"uri":"websearch://search/hello%20world foo bar"}`,
			want: `{"uri":"websearch://search/hello%20world%20foo%20bar"}`,
		},
		{
			name: "no spaces in query",
			body: `{"uri":"websearch://search/golang"}`,
			want: `{"uri":"websearch://search/golang"}`,
		},
		{
			name: "full JSON-RPC request",
			body: `{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"websearch://search/Munich weather today 2026-04-27"}}`,
			want: `{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"websearch://search/Munich%20weather%20today%202026-04-27"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixWebsearchURISpaces([]byte(tt.body))
			assert.Equal(t, tt.want, string(got))
		})
	}
}
