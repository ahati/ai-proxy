package logging

import (
	"io"
	"sync"
)

type capturingReadCloser struct {
	r    io.ReadCloser
	mu   sync.Mutex
	data []byte
	resp *SSEResponseCapture
}

func WrapResponseBody(r io.ReadCloser, resp *SSEResponseCapture) io.ReadCloser {
	return &capturingReadCloser{
		r:    r,
		resp: resp,
		data: []byte{},
	}
}

func (c *capturingReadCloser) Read(p []byte) (n int, err error) {
	n, err = c.r.Read(p)
	if n > 0 {
		c.mu.Lock()
		c.data = append(c.data, p[:n]...)
		c.mu.Unlock()
	}
	return n, err
}

func (c *capturingReadCloser) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.resp != nil && len(c.data) > 0 {
		c.resp.RawBody = c.data
	}

	return c.r.Close()
}
