package downstream

import (
	"fmt"
	"io"
	"net/http"

	"ai-proxy/config"
	"ai-proxy/downstream/protocols"
	"ai-proxy/logging"
	"ai-proxy/types"
	"ai-proxy/upstream"

	"github.com/gin-gonic/gin"
	"github.com/tmaxmax/go-sse"
)

func readBody(c *gin.Context) ([]byte, error) {
	return io.ReadAll(c.Request.Body)
}

func handleUpstreamError(c *gin.Context, resp *http.Response) {
	body, _ := io.ReadAll(resp.Body)
	msg := fmt.Sprintf("Upstream error: %s", string(body))
	logging.ErrorMsg("%s", msg)
	c.JSON(http.StatusBadGateway, gin.H{
		"error": gin.H{
			"message": msg,
			"type":    "upstream_error",
			"code":    fmt.Sprintf("status_%d", resp.StatusCode),
		},
	})
}

func StreamHandler(cfg *config.Config, adapter protocols.ProtocolAdapter) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := readBody(c)
		if err != nil {
			adapter.SendError(c, http.StatusBadRequest, "Failed to read request body")
			return
		}

		logging.RecordDownstreamRequest(c, body)

		if !adapter.IsStreamingRequest(body) {
			adapter.SendError(c, http.StatusBadRequest, "Non-streaming requests not supported")
			return
		}

		transformedBody, err := adapter.TransformRequest(body)
		if err != nil {
			adapter.SendError(c, http.StatusBadRequest, "Failed to transform request")
			return
		}

		proxyAndStream(c, cfg, adapter, transformedBody)
	}
}

func proxyAndStream(c *gin.Context, cfg *config.Config, adapter protocols.ProtocolAdapter, body []byte) {
	client := upstream.NewClient(adapter.UpstreamURL(cfg), adapter.UpstreamAPIKey(cfg))
	defer client.Close()

	req, err := client.BuildRequest(c.Request.Context(), body)
	if err != nil {
		adapter.SendError(c, http.StatusInternalServerError, "Failed to create upstream request")
		return
	}

	client.SetHeaders(req)
	adapter.ForwardHeaders(c.Request.Header, req.Header)

	for _, h := range []string{"Connection", "Keep-Alive", "Upgrade", "TE"} {
		if v := c.Request.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}

	resp, err := client.Do(c.Request.Context(), req)
	if err != nil {
		adapter.SendError(c, http.StatusBadGateway, "Upstream request failed")
		return
	}

	if resp.StatusCode != http.StatusOK {
		handleUpstreamError(c, resp)
		return
	}

	streamResponseWithAdapter(c, resp.Body, adapter)
}

func streamResponseWithAdapter(c *gin.Context, body io.Reader, adapter protocols.ProtocolAdapter) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	var cc *logging.CaptureContext
	if cc = logging.GetCaptureContext(c.Request.Context()); cc != nil {
		startTime := cc.StartTime

		downstreamCapture := logging.NewCaptureWriter(startTime)
		upstreamCapture := logging.NewCaptureWriter(startTime)

		recorder := NewResponseRecorder(c.Writer, downstreamCapture)
		transformer := adapter.CreateTransformer(recorder, types.StreamChunk{})
		defer transformer.Close()

		c.Stream(func(w io.Writer) bool {
			for ev, err := range sse.Read(body, nil) {
				if err != nil {
					logging.ErrorMsg("SSE read error: %v", err)
					return false
				}
				if ev.Data != "" {
					upstreamCapture.RecordChunk(ev.Type, []byte(ev.Data))
				}
				transformer.Transform(&ev)
			}
			return false
		})

		transformer.Flush()

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
		transformer := adapter.CreateTransformer(c.Writer, types.StreamChunk{})
		defer transformer.Close()

		c.Stream(func(w io.Writer) bool {
			for ev, err := range sse.Read(body, nil) {
				if err != nil {
					logging.ErrorMsg("SSE read error: %v", err)
					return false
				}
				transformer.Transform(&ev)
			}
			return false
		})

		transformer.Flush()
	}
}
