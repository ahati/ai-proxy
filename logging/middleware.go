package logging

import (
	"github.com/gin-gonic/gin"
)

func CaptureMiddleware(storage *Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		cc := NewCaptureContext(c.Request)

		ctx := WithCaptureContext(c.Request.Context(), cc)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		if storage != nil {
			go func() {
				if err := storage.Write(cc.Recorder); err != nil {
					ErrorMsg("Failed to write capture: %v", err)
				}
			}()
		}
	}
}

func RecordDownstreamRequest(c *gin.Context, body []byte) {
	if cc := GetCaptureContext(c.Request.Context()); cc != nil {
		cc.Recorder.DownstreamRequest = &HTTPRequestCapture{
			At:      cc.StartTime,
			Headers: SanitizeHeaders(c.Request.Header),
			Body:    body,
			RawBody: body,
		}
	}
}
