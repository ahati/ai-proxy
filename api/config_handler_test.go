package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"ai-proxy/config"
	"ai-proxy/types"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestConfigManager creates a ConfigManager with a standard test schema.
func newTestConfigManager() *config.ConfigManager {
	schema := &config.Schema{
		Providers: []config.Provider{
			{
				Name:      "test-provider",
				Endpoints: map[string]string{"openai": "https://api.example.com/v1"},
				APIKey:    "sk-test-key-12345678",
				EnvAPIKey: "TEST_API_KEY",
			},
		},
		Models: map[string]config.ModelConfig{
			"gpt-4": {
				Provider: "test-provider",
				Model:    "gpt-4-turbo",
				Type:     "openai",
			},
		},
		Fallback: config.FallbackConfig{
			Enabled:  true,
			Provider: "test-provider",
			Model:    "{model}",
		},
		WebSearch: types.WebSearchConfig{
			Enabled:    true,
			Provider:   "exa",
			ExaAPIKey:  "exa-key-1234567890",
			MaxResults: 10,
		},
	}
	return config.NewManager(schema, "")
}

// setupTestRouter creates a Gin router with config handler routes.
func setupTestRouter(manager *config.ConfigManager) *gin.Engine {
	r := gin.New()
	handler := NewConfigHandler(manager)
	uiAPI := r.Group("/ui/api")
	{
		uiAPI.GET("/config", handler.GetConfig)
		uiAPI.PUT("/config", handler.UpdateConfig)
		uiAPI.PATCH("/config/models", handler.PatchModels)
		uiAPI.DELETE("/config/models/:name", handler.DeleteModel)
		uiAPI.PATCH("/config/providers", handler.PatchProviders)
		uiAPI.DELETE("/config/providers/:name", handler.DeleteProvider)
		uiAPI.POST("/config/reload", handler.ReloadConfig)
		uiAPI.POST("/config/save", handler.SaveConfig)
		uiAPI.GET("/status", handler.GetStatus)
	}
	return r
}

func TestGetConfig(t *testing.T) {
	manager := newTestConfigManager()
	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/api/config", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp configResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Schema == nil {
		t.Error("expected schema in response")
	}
	if len(resp.Schema.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(resp.Schema.Providers))
	}
	if len(resp.Schema.Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(resp.Schema.Models))
	}

	// Verify API key is masked
	key := resp.Schema.Providers[0].APIKey
	if key == "sk-test-key-12345678" {
		t.Error("API key should be masked")
	}
	if key == "" {
		t.Error("API key mask should not be empty for non-empty keys")
	}

	// Verify websearch API key is masked
	if resp.Schema.WebSearch.ExaAPIKey == "exa-key-1234567890" {
		t.Error("WebSearch ExaAPIKey should be masked")
	}
}

func TestGetConfig_Masking(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{"empty key", "", ""},
		{"short key", "abc", "***masked***"},
		{"8 char key", "12345678", "***masked***"},
		{"long key", "sk-test-key-12345678", "sk-t...5678"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskAPIKey(tt.apiKey)
			if result != tt.expected {
				t.Errorf("maskAPIKey(%q) = %q, want %q", tt.apiKey, result, tt.expected)
			}
		})
	}
}

func TestUpdateConfig(t *testing.T) {
	manager := newTestConfigManager()
	router := setupTestRouter(manager)

	newSchema := config.Schema{
		Providers: []config.Provider{
			{
				Name:      "new-provider",
				Endpoints: map[string]string{"openai": "https://new.example.com/v1"},
				APIKey:    "new-key",
			},
		},
		Models: map[string]config.ModelConfig{
			"claude-3": {
				Provider: "new-provider",
				Model:    "claude-3-opus",
				Type:     "openai",
			},
		},
	}

	body, _ := json.Marshal(newSchema)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/ui/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify the config was actually updated
	snap := manager.Get()
	if len(snap.Schema.Providers) != 1 || snap.Schema.Providers[0].Name != "new-provider" {
		t.Error("config was not updated")
	}
	if _, ok := snap.Schema.Models["claude-3"]; !ok {
		t.Error("model 'claude-3' not found in updated config")
	}

	// Verify persisted flag is false after update
	if snap.Persisted {
		t.Error("config should be marked as not persisted after update")
	}
}

func TestUpdateConfig_InvalidJSON(t *testing.T) {
	manager := newTestConfigManager()
	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/ui/api/config", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestUpdateConfig_InvalidSchema(t *testing.T) {
	manager := newTestConfigManager()
	router := setupTestRouter(manager)

	// Schema with no providers (invalid)
	newSchema := config.Schema{
		Providers: []config.Provider{},
	}

	body, _ := json.Marshal(newSchema)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/ui/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Verify original config is unchanged
	snap := manager.Get()
	if len(snap.Schema.Providers) != 1 {
		t.Error("original config should be unchanged after failed update")
	}
}

func TestPatchModels(t *testing.T) {
	manager := newTestConfigManager()
	router := setupTestRouter(manager)

	models := map[string]config.ModelConfig{
		"gpt-4o": {
			Provider: "test-provider",
			Model:    "gpt-4o",
			Type:     "openai",
		},
	}

	body, _ := json.Marshal(models)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/ui/api/config/models", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	snap := manager.Get()
	if _, ok := snap.Schema.Models["gpt-4o"]; !ok {
		t.Error("model 'gpt-4o' should be added")
	}
	if _, ok := snap.Schema.Models["gpt-4"]; !ok {
		t.Error("original model 'gpt-4' should still exist")
	}
	if len(snap.Schema.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(snap.Schema.Models))
	}
}

func TestDeleteModel(t *testing.T) {
	manager := newTestConfigManager()
	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/ui/api/config/models/gpt-4", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	snap := manager.Get()
	if _, ok := snap.Schema.Models["gpt-4"]; ok {
		t.Error("model 'gpt-4' should be removed")
	}
}

func TestDeleteModel_NotFound(t *testing.T) {
	manager := newTestConfigManager()
	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/ui/api/config/models/nonexistent", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestPatchProviders(t *testing.T) {
	manager := newTestConfigManager()
	router := setupTestRouter(manager)

	providers := []config.Provider{
		{
			Name:      "test-provider",
			Endpoints: map[string]string{"openai": "https://updated.example.com/v1"},
			APIKey:    "updated-key",
		},
		{
			Name:      "new-provider",
			Endpoints: map[string]string{"anthropic": "https://new.example.com/v1"},
			APIKey:    "new-key",
		},
	}

	body, _ := json.Marshal(providers)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/ui/api/config/providers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	snap := manager.Get()
	if len(snap.Schema.Providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(snap.Schema.Providers))
	}
}

func TestDeleteProvider(t *testing.T) {
	manager := newTestConfigManager()
	router := setupTestRouter(manager)

	// First remove the model that references the provider
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/ui/api/config/models/gpt-4", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("failed to delete model: %d", w.Code)
	}

	// Update config: add a second provider, remove fallback, remove models
	newSchema := config.Schema{
		Providers: []config.Provider{
			{
				Name:      "test-provider",
				Endpoints: map[string]string{"openai": "https://api.example.com/v1"},
				APIKey:    "test-key",
			},
			{
				Name:      "other-provider",
				Endpoints: map[string]string{"openai": "https://other.example.com/v1"},
				APIKey:    "other-key",
			},
		},
		Models:   map[string]config.ModelConfig{},
		Fallback: config.FallbackConfig{Enabled: false},
	}
	body, _ := json.Marshal(newSchema)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/ui/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("failed to update config: %d %s", w.Code, w.Body.String())
	}

	// Now delete the provider
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/ui/api/config/providers/test-provider", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	snap := manager.Get()
	if len(snap.Schema.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(snap.Schema.Providers))
	}
	if snap.Schema.Providers[0].Name != "other-provider" {
		t.Errorf("expected remaining provider 'other-provider', got '%s'", snap.Schema.Providers[0].Name)
	}
}

func TestDeleteProvider_ReferencedByModel(t *testing.T) {
	manager := newTestConfigManager()
	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/ui/api/config/providers/test-provider", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}
}

func TestDeleteProvider_NotFound(t *testing.T) {
	manager := newTestConfigManager()
	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/ui/api/config/providers/nonexistent", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestSaveAndReload(t *testing.T) {
	// Create a temp config file with initial content
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Write initial file so that backup can be created on first save
	initialContent := `{"providers": [{"name": "initial"}]}`
	if err := os.WriteFile(configPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to create initial config: %v", err)
	}

	initialSchema := &config.Schema{
		Providers: []config.Provider{
			{
				Name:      "test-provider",
				Endpoints: map[string]string{"openai": "https://api.example.com/v1"},
				APIKey:    "test-key",
			},
		},
		Models: map[string]config.ModelConfig{},
	}

	manager := config.NewManager(initialSchema, configPath)
	router := setupTestRouter(manager)

	// Save config to disk
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ui/api/config/save", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file should exist after save")
	}

	// Verify backup was created
	if _, err := os.Stat(configPath + ".bak"); os.IsNotExist(err) {
		t.Error("backup file should exist after save")
	}

	// Update config in memory
	newSchema := config.Schema{
		Providers: []config.Provider{
			{
				Name:      "reloaded-provider",
				Endpoints: map[string]string{"openai": "https://reloaded.example.com/v1"},
				APIKey:    "reloaded-key",
			},
		},
		Models: map[string]config.ModelConfig{},
	}
	body, _ := json.Marshal(newSchema)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/ui/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// Verify in-memory config changed
	snap := manager.Get()
	if snap.Schema.Providers[0].Name != "reloaded-provider" {
		t.Error("in-memory config should be updated")
	}

	// Reload from disk — should revert to original
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/ui/api/config/reload", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify config reverted to disk version
	snap = manager.Get()
	if snap.Schema.Providers[0].Name != "test-provider" {
		t.Errorf("expected 'test-provider' after reload, got '%s'", snap.Schema.Providers[0].Name)
	}
}

func TestGetStatus(t *testing.T) {
	manager := newTestConfigManager()
	router := setupTestRouter(manager)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/api/status", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp statusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", resp.Status)
	}
	if resp.Providers != 1 {
		t.Errorf("expected 1 provider, got %d", resp.Providers)
	}
	if resp.Models != 1 {
		t.Errorf("expected 1 model, got %d", resp.Models)
	}
	if resp.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
}

func TestDeepCopySchema(t *testing.T) {
	original := &config.Schema{
		Providers: []config.Provider{
			{Name: "test", Endpoints: map[string]string{"openai": "https://example.com"}, APIKey: "key"},
		},
		Models: map[string]config.ModelConfig{
			"gpt-4": {Provider: "test", Model: "gpt-4"},
		},
	}

	copy := deepCopySchema(original)

	// Modify copy and verify original is unchanged
	copy.Providers[0].Name = "modified"
	copy.Models["new-model"] = config.ModelConfig{Provider: "test"}

	if original.Providers[0].Name == "modified" {
		t.Error("original schema should not be modified when copy is changed")
	}
	if _, ok := original.Models["new-model"]; ok {
		t.Error("original schema should not have new model added to copy")
	}
}
