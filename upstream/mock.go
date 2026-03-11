package upstream

import (
	"context"
	"net/http"
)

type MockClient struct {
	Response *http.Response
	Error    error
	Requests []*http.Request
}

func (m *MockClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	m.Requests = append(m.Requests, req)
	return m.Response, m.Error
}

func (m *MockClient) Close() {}
