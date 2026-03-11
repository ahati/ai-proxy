package upstream

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"ai-proxy/logging"
)

type Client interface {
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
	Close()
}

type HTTPClient struct {
	URL    string
	APIKey string
	Client *http.Client
}

func NewClient(url, apiKey string) *HTTPClient {
	return &HTTPClient{
		URL:    url,
		APIKey: apiKey,
		Client: &http.Client{Timeout: 0},
	}
}

func (c *HTTPClient) BuildRequest(ctx context.Context, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.URL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if cc := logging.GetCaptureContext(ctx); cc != nil {
		cc.Recorder.UpstreamRequest = &logging.HTTPRequestCapture{
			At:      cc.StartTime,
			Body:    body,
			RawBody: body,
		}
	}

	return req, nil
}

func (c *HTTPClient) SetHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	if cc := logging.GetCaptureContext(req.Context()); cc != nil && cc.Recorder.UpstreamRequest != nil {
		cc.Recorder.UpstreamRequest.Headers = logging.SanitizeHeaders(req.Header)
	}
}

func (c *HTTPClient) GetAPIKey(clientAuth string) string {
	if strings.HasPrefix(clientAuth, "Bearer ") {
		return strings.TrimPrefix(clientAuth, "Bearer ")
	}
	return c.APIKey
}

func (c *HTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	logging.InfoMsg("Sending request to upstream: %s", c.URL)
	resp, err := c.Client.Do(req.WithContext(ctx))
	if err != nil {
		logging.ErrorMsg("Upstream request failed: %v", err)
		return nil, fmt.Errorf("upstream request: %w", err)
	}

	if cc := logging.GetCaptureContext(ctx); cc != nil {
		cc.Recorder.UpstreamResponse = &logging.SSEResponseCapture{
			StatusCode: resp.StatusCode,
			Headers:    logging.SanitizeHeaders(resp.Header),
			Chunks:     []logging.SSEChunk{},
		}
	}

	return resp, nil
}

func (c *HTTPClient) Close() {
	c.Client.CloseIdleConnections()
}
