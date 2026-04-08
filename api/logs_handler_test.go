package api

import (
	"ai-proxy/capture"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupLogsRouter creates a test router with logs routes.
func setupLogsRouter() (*gin.Engine, *LogsHandler) {
	r := gin.New()
	handler := NewLogsHandler()
	api := r.Group("/ui/api")
	{
		api.GET("/logs", handler.List)
		api.GET("/logs/:id", handler.Get)
		api.POST("/logs/flush", handler.Flush)
		api.DELETE("/logs", handler.Clear)
		api.GET("/logs/config", handler.GetConfig)
		api.PUT("/logs/config", handler.UpdateConfig)
	}
	return r, handler
}

func TestLogsHandler_List_Disabled(t *testing.T) {
	orig := capture.GetDefaultMemoryStore()
	capture.InitMemoryStore(false, 0)
	defer func() {
		capture.InitMemoryStore(orig != nil, func() int {
			if orig != nil {
				return orig.Capacity()
			}
			return 0
		}())
	}()

	r, _ := setupLogsRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ui/api/logs", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestLogsHandler_List_Empty(t *testing.T) {
	orig := capture.GetDefaultMemoryStore()
	capture.InitMemoryStore(true, 100)
	defer func() {
		capture.InitMemoryStore(orig != nil, func() int {
			if orig != nil {
				return orig.Capacity()
			}
			return 0
		}())
	}()

	r, _ := setupLogsRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ui/api/logs?page=1&per_page=10", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["total"].(float64) != 0 {
		t.Errorf("total = %v, want 0", resp["total"])
	}
}

func TestLogsHandler_List_WithData(t *testing.T) {
	orig := capture.GetDefaultMemoryStore()
	capture.InitMemoryStore(true, 100)
	defer func() {
		capture.InitMemoryStore(orig != nil, func() int {
			if orig != nil {
				return orig.Capacity()
			}
			return 0
		}())
	}()

	// Store some entries
	r1 := capture.NewRecorder("handler-test-1", "POST", "/v1/chat/completions", "127.0.0.1")
	capture.GetDefaultMemoryStore().Store(r1)
	r2 := capture.NewRecorder("handler-test-2", "GET", "/v1/models", "127.0.0.1")
	capture.GetDefaultMemoryStore().Store(r2)

	r, _ := setupLogsRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ui/api/logs?page=1&per_page=10", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Logs  []capture.LogEntry `json:"logs"`
		Total int                `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Total != 2 {
		t.Errorf("total = %d, want 2", resp.Total)
	}
	if len(resp.Logs) != 2 {
		t.Errorf("len(logs) = %d, want 2", len(resp.Logs))
	}
	// Newest first
	if len(resp.Logs) > 0 && resp.Logs[0].RequestID != "handler-test-2" {
		t.Errorf("first entry = %q, want %q", resp.Logs[0].RequestID, "handler-test-2")
	}
}

func TestLogsHandler_Get_Found(t *testing.T) {
	orig := capture.GetDefaultMemoryStore()
	capture.InitMemoryStore(true, 100)
	defer func() {
		capture.InitMemoryStore(orig != nil, func() int {
			if orig != nil {
				return orig.Capacity()
			}
			return 0
		}())
	}()

	r := capture.NewRecorder("get-test-id", "POST", "/v1/chat", "127.0.0.1")
	r.RecordDownstreamRequest(nil, json.RawMessage(`{"model":"gpt-4"}`))
	capture.GetDefaultMemoryStore().Store(r)

	router, _ := setupLogsRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ui/api/logs/get-test-id", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var entry capture.LogEntry
	json.Unmarshal(w.Body.Bytes(), &entry)
	if entry.RequestID != "get-test-id" {
		t.Errorf("RequestID = %q, want %q", entry.RequestID, "get-test-id")
	}
}

func TestLogsHandler_Get_NotFound(t *testing.T) {
	orig := capture.GetDefaultMemoryStore()
	capture.InitMemoryStore(true, 100)
	defer func() {
		capture.InitMemoryStore(orig != nil, func() int {
			if orig != nil {
				return orig.Capacity()
			}
			return 0
		}())
	}()

	router, _ := setupLogsRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ui/api/logs/nonexistent", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestLogsHandler_Clear(t *testing.T) {
	orig := capture.GetDefaultMemoryStore()
	capture.InitMemoryStore(true, 100)
	defer func() {
		capture.InitMemoryStore(orig != nil, func() int {
			if orig != nil {
				return orig.Capacity()
			}
			return 0
		}())
	}()

	capture.GetDefaultMemoryStore().Store(capture.NewRecorder("1", "GET", "/test", "localhost"))
	capture.GetDefaultMemoryStore().Store(capture.NewRecorder("2", "POST", "/test", "localhost"))

	router, _ := setupLogsRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/ui/api/logs", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	if capture.GetDefaultMemoryStore().Count() != 0 {
		t.Errorf("Count() after clear = %d, want 0", capture.GetDefaultMemoryStore().Count())
	}
}

func TestLogsHandler_GetConfig_Enabled(t *testing.T) {
	orig := capture.GetDefaultMemoryStore()
	capture.InitMemoryStore(true, 200)
	defer func() {
		capture.InitMemoryStore(orig != nil, func() int {
			if orig != nil {
				return orig.Capacity()
			}
			return 0
		}())
	}()

	router, _ := setupLogsRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ui/api/logs/config", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Enabled  bool `json:"enabled"`
		Capacity int  `json:"capacity"`
		Count    int  `json:"count"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Enabled {
		t.Error("enabled should be true")
	}
	if resp.Capacity != 200 {
		t.Errorf("capacity = %d, want 200", resp.Capacity)
	}
}

func TestLogsHandler_GetConfig_Disabled(t *testing.T) {
	orig := capture.GetDefaultMemoryStore()
	capture.InitMemoryStore(false, 0)
	defer func() {
		capture.InitMemoryStore(orig != nil, func() int {
			if orig != nil {
				return orig.Capacity()
			}
			return 0
		}())
	}()

	router, _ := setupLogsRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ui/api/logs/config", nil)
	router.ServeHTTP(w, req)

	var resp struct {
		Enabled bool `json:"enabled"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Enabled {
		t.Error("enabled should be false when store is nil")
	}
}

func TestLogsHandler_UpdateConfig_Enable(t *testing.T) {
	orig := capture.GetDefaultMemoryStore()
	capture.InitMemoryStore(false, 0)
	defer func() {
		capture.InitMemoryStore(orig != nil, func() int {
			if orig != nil {
				return orig.Capacity()
			}
			return 0
		}())
	}()

	router, _ := setupLogsRouter()

	// Enable
	body := `{"enabled": true, "capacity": 500}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/ui/api/logs/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	if capture.GetDefaultMemoryStore() == nil {
		t.Fatal("DefaultMemoryStore should not be nil after enabling")
	}
	if capture.GetDefaultMemoryStore().Capacity() != 500 {
		t.Errorf("Capacity = %d, want 500", capture.GetDefaultMemoryStore().Capacity())
	}
}

func TestLogsHandler_UpdateConfig_Disable(t *testing.T) {
	orig := capture.GetDefaultMemoryStore()
	capture.InitMemoryStore(true, 100)
	defer func() {
		capture.InitMemoryStore(orig != nil, func() int {
			if orig != nil {
				return orig.Capacity()
			}
			return 0
		}())
	}()

	router, _ := setupLogsRouter()

	body := `{"enabled": false}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/ui/api/logs/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	if capture.GetDefaultMemoryStore() != nil {
		t.Error("DefaultMemoryStore should be nil after disabling")
	}
}

func TestLogsHandler_Flush_MissingDirectory(t *testing.T) {
	orig := capture.GetDefaultMemoryStore()
	capture.InitMemoryStore(true, 100)
	defer func() {
		capture.InitMemoryStore(orig != nil, func() int {
			if orig != nil {
				return orig.Capacity()
			}
			return 0
		}())
	}()

	router, _ := setupLogsRouter()

	body := `{"ids": ["test"]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ui/api/logs/flush", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestIsValidFlushPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "valid absolute", path: "/tmp/logs", want: true},
		{name: "valid home dir", path: "/home/user/logs", want: true},
		{name: "valid var log", path: "/var/log/ai-proxy", want: true},
		{name: "empty", path: "", want: false},
		{name: "relative", path: "tmp/logs", want: false},
		{name: "dot relative", path: "./logs", want: false},
		{name: "etc", path: "/etc", want: false},
		{name: "etc subpath", path: "/etc/ai-proxy", want: false},
		{name: "proc", path: "/proc/self", want: false},
		{name: "sys", path: "/sys/kernel", want: false},
		{name: "dev", path: "/dev/null", want: false},
		{name: "boot", path: "/boot/grub", want: false},
		{name: "root", path: "/root", want: false},
		{name: "bin", path: "/bin/bash", want: false},
		{name: "sbin", path: "/sbin/iptables", want: false},
		{name: "usr", path: "/usr/local", want: false},
		{name: "lib", path: "/lib/x86_64", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidFlushPath(tt.path); got != tt.want {
				t.Errorf("isValidFlushPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestLogsHandler_Flush_InvalidPath(t *testing.T) {
	orig := capture.GetDefaultMemoryStore()
	capture.InitMemoryStore(true, 100)
	defer func() {
		capture.InitMemoryStore(orig != nil, func() int {
			if orig != nil {
				return orig.Capacity()
			}
			return 0
		}())
	}()

	router, _ := setupLogsRouter()

	body := `{"directory": "/etc/evil", "ids": ["test"]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ui/api/logs/flush", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestLogsHandler_Flush_RelativePath(t *testing.T) {
	orig := capture.GetDefaultMemoryStore()
	capture.InitMemoryStore(true, 100)
	defer func() {
		capture.InitMemoryStore(orig != nil, func() int {
			if orig != nil {
				return orig.Capacity()
			}
			return 0
		}())
	}()

	router, _ := setupLogsRouter()

	body := `{"directory": "relative/path"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ui/api/logs/flush", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
