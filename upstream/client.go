package upstream

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"ai-proxy/logging"
)

type Client struct {
	URL    string
	APIKey string
	Client *http.Client
}

func NewClient(url, apiKey string) *Client {
	return &Client{
		URL:    url,
		APIKey: apiKey,
		Client: &http.Client{Timeout: 0},
	}
}

func (c *Client) BuildRequest(ctx context.Context, body []byte) (*http.Request, error) {
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

func (c *Client) SetHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	if cc := logging.GetCaptureContext(req.Context()); cc != nil && cc.Recorder.UpstreamRequest != nil {
		cc.Recorder.UpstreamRequest.Headers = logging.SanitizeHeaders(req.Header)
	}
}

func (c *Client) GetAPIKey(clientAuth string) string {
	if strings.HasPrefix(clientAuth, "Bearer ") {
		return strings.TrimPrefix(clientAuth, "Bearer ")
	}
	return c.APIKey
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	logging.InfoMsg("Sending request to upstream: %s", c.URL)
	resp, err := c.Client.Do(req)
	if err != nil {
		logging.ErrorMsg("Upstream request failed: %v", err)
		return nil, fmt.Errorf("upstream request: %w", err)
	}

	if cc := logging.GetCaptureContext(req.Context()); cc != nil {
		cc.Recorder.UpstreamResponse = &logging.SSEResponseCapture{
			StatusCode: resp.StatusCode,
			Headers:    logging.SanitizeHeaders(resp.Header),
			Chunks:     []logging.SSEChunk{},
		}
	}

	return resp, nil
}

func (c *Client) Close() {
	c.Client.CloseIdleConnections()
}
