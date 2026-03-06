package downstream

import (
	"io"

	"ai-proxy/logging"

	"github.com/tmaxmax/go-sse"
)

type SSECaptureReader struct {
	reader  io.Reader
	capture logging.CaptureWriter
}

func NewSSECaptureReader(reader io.Reader, capture logging.CaptureWriter) *SSECaptureReader {
	return &SSECaptureReader{
		reader:  reader,
		capture: capture,
	}
}

func (scr *SSECaptureReader) ForEach(fn func(sse.Event) bool) error {
	for ev, err := range sse.Read(scr.reader, nil) {
		if err != nil {
			return err
		}
		if ev.Data != "" {
			scr.capture.RecordChunk(ev.Type, []byte(ev.Data))
		}
		if !fn(ev) {
			break
		}
	}
	return nil
}
