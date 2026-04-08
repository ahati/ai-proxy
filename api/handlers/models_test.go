package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ai-proxy/config"

	"github.com/gin-gonic/gin"
)

func TestModelsHandler_Handle_NoConfig(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)

	h := &ModelsHandler{manager: nil}
	h.Handle(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestModelsHandler_Handle_Success(t *testing.T) {
	cfg := &config.Config{
		AppConfig: &config.Schema{
			Providers: []config.Provider{
				{
					Name:      "openai",
					Endpoints: map[string]string{"openai": "https://api.example.com/v1/chat/completions"},
					APIKey:    "test-api-key",
				},
			},
			Models: map[string]config.ModelConfig{
				"gpt-4":   {Provider: "openai", Model: "gpt-4-turbo"},
				"gpt-3.5": {Provider: "openai", Model: "gpt-3.5-turbo"},
			},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)

	mgr := config.NewManager(cfg.AppConfig, "")
	h := &ModelsHandler{manager: mgr}
	h.Handle(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response ModelsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Object != "list" {
		t.Errorf("expected object 'list', got %v", response.Object)
	}

	if len(response.Data) != 2 {
		t.Errorf("expected 2 models, got %d", len(response.Data))
	}

	modelIDs := make(map[string]bool)
	for _, m := range response.Data {
		modelIDs[m.ID] = true
		if m.Object != "model" {
			t.Errorf("expected object 'model', got %v", m.Object)
		}
		if m.OwnedBy != "openai" {
			t.Errorf("expected owned_by 'openai', got %v", m.OwnedBy)
		}
	}

	if !modelIDs["gpt-4"] {
		t.Error("expected model 'gpt-4' in response")
	}
	if !modelIDs["gpt-3.5"] {
		t.Error("expected model 'gpt-3.5' in response")
	}
}

func TestModelsHandler_Handle_EmptyModels(t *testing.T) {
	cfg := &config.Config{
		AppConfig: &config.Schema{
			Providers: []config.Provider{
				{
					Name:      "openai",
					Endpoints: map[string]string{"openai": "https://api.example.com/v1/chat/completions"},
					APIKey:    "test-api-key",
				},
			},
			Models: map[string]config.ModelConfig{},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)

	mgr := config.NewManager(cfg.AppConfig, "")
	h := &ModelsHandler{manager: mgr}
	h.Handle(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response ModelsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response.Data) != 0 {
		t.Errorf("expected 0 models, got %d", len(response.Data))
	}
}
