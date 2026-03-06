package downstream

import (
	"io"
	"net/http"

	"ai-proxy/config"
	"ai-proxy/logging"
	"ai-proxy/upstream"

	"github.com/gin-gonic/gin"
	"github.com/tmaxmax/go-sse"
)

func handler(c *gin.Context, cfg *config.Config) {
	body, err := readBody(c)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Failed to read request body", "")
		return
	}

	logging.RecordDownstreamRequest(c, body)

	proxyAndRespond(c, cfg, body)
}

func resolveAPIKey(c *gin.Context, cfg *config.Config) string {
	return cfg.OpenAIUpstreamAPIKey
}

func proxyAndRespond(c *gin.Context, cfg *config.Config, body []byte) {
	client := upstream.NewClient(cfg.OpenAIUpstreamURL, cfg.OpenAIUpstreamAPIKey)
	defer client.Close()

	req, err := client.BuildRequest(c.Request.Context(), body)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to create upstream request", "")
		return
	}

	client.SetHeaders(req)

	for k, v := range c.Request.Header {
		if len(k) > 1 && k[:2] == "X-" || k == "Extra" {
			req.Header[k] = v
		}
	}

	for _, h := range []string{"Connection", "Keep-Alive", "Upgrade", "TE"} {
		if v := c.Request.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		sendError(c, http.StatusBadGateway, "Upstream request failed", "")
		return
	}

	if resp.StatusCode != http.StatusOK {
		handleUpstreamError(c, resp)
		return
	}

	var cc *logging.CaptureContext
	if cc = logging.GetCaptureContext(c.Request.Context()); cc != nil {
		startTime := cc.StartTime

		downstreamCapture := logging.NewCaptureWriter(startTime)
		upstreamCapture := logging.NewCaptureWriter(startTime)

		recorder := NewResponseRecorder(c.Writer, downstreamCapture)
		toolCallTransformer := NewToolCallTransformer(recorder)

		c.Stream(func(w io.Writer) bool {
			for ev, err := range sse.Read(resp.Body, nil) {
				if err != nil {
					return false
				}
				if ev.Data != "" {
					upstreamCapture.RecordChunk(ev.Type, []byte(ev.Data))
				}
				toolCallTransformer.Transform(&ev)
			}
			return false
		})

		toolCallTransformer.Close()

		cc.Recorder.DownstreamResponse = &logging.SSEResponseCapture{
			Chunks: downstreamCapture.Chunks(),
		}
		if cc.Recorder.UpstreamResponse != nil {
			cc.Recorder.UpstreamResponse.Chunks = upstreamCapture.Chunks()
		}
		if !cc.IDExtracted {
			for _, chunk := range downstreamCapture.Chunks() {
				if id := logging.ExtractRequestIDFromSSEChunk(chunk.Data); id != "" {
					cc.SetRequestID(id)
					break
				}
			}
		}
	} else {
		toolCallTransformer := NewToolCallTransformer(c.Writer)
		streamResponse(c, resp.Body, toolCallTransformer)
	}
}

func readBody(c *gin.Context) ([]byte, error) {
	return io.ReadAll(c.Request.Body)
}
