package api

import (
	"ai-proxy/config"
	"ai-proxy/types"
	"ai-proxy/websearch"
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

// setupWebSearchRouter creates a test router with the websearch reload route.
func setupWebSearchRouter() (*gin.Engine, *config.ConfigManager) {
	schema := &config.Schema{
		Providers: []config.Provider{{Name: "test"}},
		Models:    map[string]config.ModelConfig{},
		WebSearch: types.WebSearchConfig{
			Enabled:    true,
			Provider:   "ddg",
			MaxResults: 5,
			Timeout:    10,
		},
	}
	mgr := config.NewManager(schema, "")
	handler := NewWebSearchHandler(mgr)

	r := gin.New()
	api := r.Group("/ui/api")
	api.POST("/config/websearch/reload", handler.ReloadService)
	return r, mgr
}

func TestWebSearchHandler_ReloadService_Enabled(t *testing.T) {
	r, _ := setupWebSearchRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ui/api/config/websearch/reload", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["ok"] != true {
		t.Errorf("ok = %v, want true", resp["ok"])
	}
	msg, _ := resp["message"].(string)
	if !strings.Contains(msg, "ddg") {
		t.Errorf("message = %q, want to contain 'ddg'", msg)
	}

	if websearch.DefaultService == nil {
		t.Error("DefaultService should not be nil after reload with enabled=true")
	}
}

func TestWebSearchHandler_ReloadService_Disabled(t *testing.T) {
	schema := &config.Schema{
		Providers: []config.Provider{{Name: "test"}},
		Models:    map[string]config.ModelConfig{},
		WebSearch: types.WebSearchConfig{
			Enabled:  false,
			Provider: "ddg",
		},
	}
	mgr := config.NewManager(schema, "")
	handler := NewWebSearchHandler(mgr)

	r := gin.New()
	r.POST("/ui/api/config/websearch/reload", handler.ReloadService)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ui/api/config/websearch/reload", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["ok"] != true {
		t.Errorf("ok = %v, want true", resp["ok"])
	}

	if websearch.DefaultService != nil {
		t.Error("DefaultService should be nil after reload with enabled=false")
	}
}

func TestWebSearchHandler_ReloadService_NoConfig(t *testing.T) {
	// Create a manager with nil-ish state by using a schema but then
	// verifying the endpoint works with a valid manager
	schema := &config.Schema{
		Providers: []config.Provider{},
		Models:    map[string]config.ModelConfig{},
	}
	mgr := config.NewManager(schema, "")
	handler := NewWebSearchHandler(mgr)

	r := gin.New()
	r.POST("/ui/api/config/websearch/reload", handler.ReloadService)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ui/api/config/websearch/reload", nil)
	r.ServeHTTP(w, req)

	// Should succeed — disabled by default
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWebSearchHandler_ReloadService_Exa(t *testing.T) {
	schema := &config.Schema{
		Providers: []config.Provider{{Name: "test"}},
		Models:    map[string]config.ModelConfig{},
		WebSearch: types.WebSearchConfig{
			Enabled:    true,
			Provider:   "exa",
			ExaAPIKey:  "test-exa-key",
			MaxResults: 5,
			Timeout:    10,
		},
	}
	mgr := config.NewManager(schema, "")
	handler := NewWebSearchHandler(mgr)

	r := gin.New()
	r.POST("/ui/api/config/websearch/reload", handler.ReloadService)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ui/api/config/websearch/reload", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	if websearch.DefaultService == nil {
		t.Fatal("DefaultService should not be nil with exa provider")
	}
	if websearch.DefaultService.GetBackend() != "exa" {
		t.Errorf("backend = %q, want %q", websearch.DefaultService.GetBackend(), "exa")
	}
}

func TestWebSearchHandler_ReloadService_Brave(t *testing.T) {
	schema := &config.Schema{
		Providers: []config.Provider{{Name: "test"}},
		Models:    map[string]config.ModelConfig{},
		WebSearch: types.WebSearchConfig{
			Enabled:      true,
			Provider:     "brave",
			BraveAPIKey:  "test-brave-key",
			MaxResults:   5,
			Timeout:      10,
		},
	}
	mgr := config.NewManager(schema, "")
	handler := NewWebSearchHandler(mgr)

	r := gin.New()
	r.POST("/ui/api/config/websearch/reload", handler.ReloadService)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ui/api/config/websearch/reload", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	if websearch.DefaultService == nil {
		t.Fatal("DefaultService should not be nil with brave provider")
	}
	if websearch.DefaultService.GetBackend() != "brave" {
		t.Errorf("backend = %q, want %q", websearch.DefaultService.GetBackend(), "brave")
	}
}
