package logging

import (
	"github.com/gin-gonic/gin"
)

func CaptureMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cc := NewCaptureContext(c.Request)

		ctx := WithCaptureContext(c.Request.Context(), cc)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		go func() {
			storage := GetStorage()
			if storage == nil {
				return
			}
			if err := storage.Write(cc.Recorder); err != nil {
				ErrorMsg("Failed to write capture: %v", err)
			}
		}()
	}
}

var globalStorage *Storage

func InitStorage(baseDir string) {
	if baseDir != "" {
		globalStorage = NewStorage(baseDir)
	}
}

func GetStorage() *Storage {
	return globalStorage
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
