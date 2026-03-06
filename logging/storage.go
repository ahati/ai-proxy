package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const filePerms = 0644
const dirPerms = 0755

type Storage struct {
	baseDir string
}

func NewStorage(baseDir string) *Storage {
	return &Storage{baseDir: baseDir}
}

func (s *Storage) Write(recorder *RequestRecorder) error {
	dir := s.logDir()
	if err := os.MkdirAll(dir, dirPerms); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	filename := s.filename(recorder.RequestID)
	fullpath := filepath.Join(dir, filename)

	if _, err := os.Stat(fullpath); err == nil {
		filename = s.filenameWithTimestamp(recorder.RequestID)
		fullpath = filepath.Join(dir, filename)
	}

	data := s.serialize(recorder)

	file, err := os.OpenFile(fullpath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, filePerms)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("encode log data: %w", err)
	}

	return nil
}

func (s *Storage) logDir() string {
	return filepath.Join(s.baseDir, time.Now().Format("2006-01-02"))
}

func (s *Storage) filename(requestID string) string {
	timestamp := time.Now().Format("20060102-150405")
	safeID := sanitizeFilename(requestID)
	return fmt.Sprintf("%s_%s.json", timestamp, safeID)
}

func (s *Storage) filenameWithTimestamp(requestID string) string {
	timestamp := time.Now().Format("20060102-150405")
	safeID := sanitizeFilename(requestID)
	return fmt.Sprintf("%s_%s_1.json", timestamp, safeID)
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(name)
}

type logData struct {
	RequestID          string              `json:"request_id"`
	StartedAt          time.Time           `json:"started_at"`
	DurationMS         int64               `json:"duration_ms,omitempty"`
	Method             string              `json:"method"`
	Path               string              `json:"path"`
	ClientIP           string              `json:"client_ip,omitempty"`
	DownstreamRequest  *HTTPRequestCapture `json:"downstream_request,omitempty"`
	UpstreamRequest    *HTTPRequestCapture `json:"upstream_request,omitempty"`
	UpstreamResponse   *SSEResponseCapture `json:"upstream_response,omitempty"`
	DownstreamResponse *SSEResponseCapture `json:"downstream_response,omitempty"`
}

func (s *Storage) serialize(r *RequestRecorder) logData {
	return logData{
		RequestID:          r.RequestID,
		StartedAt:          r.StartedAt,
		DurationMS:         time.Since(r.StartedAt).Milliseconds(),
		Method:             r.Method,
		Path:               r.Path,
		ClientIP:           r.ClientIP,
		DownstreamRequest:  r.DownstreamRequest,
		UpstreamRequest:    r.UpstreamRequest,
		UpstreamResponse:   r.UpstreamResponse,
		DownstreamResponse: r.DownstreamResponse,
	}
}
