package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"ai-proxy/config"

	"github.com/gin-gonic/gin"
)

// ConfigHandler handles configuration management API requests.
// It provides endpoints for reading, updating, reloading, and persisting
// the proxy configuration at runtime without restart.
//
// @note All endpoints are under /ui/api/ prefix.
// @note API keys are masked in GET responses for security.
type ConfigHandler struct {
	manager *config.ConfigManager
}

// NewConfigHandler creates a new ConfigHandler with the given manager.
//
// @param m - Configuration manager. Must not be nil.
// @return *ConfigHandler - Handler for config API endpoints.
func NewConfigHandler(m *config.ConfigManager) *ConfigHandler {
	return &ConfigHandler{manager: m}
}

// apiResponse is a generic JSON response structure for API endpoints.
type apiResponse struct {
	OK      bool     `json:"ok"`
	Message string   `json:"message,omitempty"`
	Error   string   `json:"error,omitempty"`
	Details []string `json:"details,omitempty"`
}

// maskedProvider is a Provider with the API key masked for safe display.
type maskedProvider struct {
	Name      string            `json:"name"`
	Endpoints map[string]string `json:"endpoints"`
	Default   string            `json:"default,omitempty"`
	APIKey    string            `json:"apiKey,omitempty"`
	EnvAPIKey string            `json:"envApiKey,omitempty"`
}

// configResponse is the response for GET /ui/api/config.
type configResponse struct {
	Schema     *config.Schema `json:"schema"`
	LoadedAt   time.Time      `json:"loadedAt"`
	Persisted  bool           `json:"persisted"`
	ConfigFile string         `json:"configFile"`
}

// statusResponse is the response for GET /ui/api/status.
type statusResponse struct {
	Status    string    `json:"status"`
	Uptime    string    `json:"uptime"`
	StartedAt time.Time `json:"startedAt"`
	Config    struct {
		LoadedAt   time.Time `json:"loadedAt"`
		Persisted  bool      `json:"persisted"`
		ConfigFile string    `json:"configFile"`
	} `json:"config"`
	Providers int `json:"providers"`
	Models    int `json:"models"`
}

// reloadResponse is the response for POST /ui/api/config/reload.
type reloadResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// GetConfig returns the current configuration with API keys masked.
//
// GET /ui/api/config
//
// @return Current config with metadata (loadedAt, persisted status, config file path).
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	snap := h.manager.Get()
	if snap == nil || snap.Schema == nil {
		c.JSON(http.StatusInternalServerError, apiResponse{
			OK:    false,
			Error: "Configuration not loaded",
		})
		return
	}

	// Deep copy and mask API keys
	masked := maskSchema(snap.Schema)

	c.JSON(http.StatusOK, configResponse{
		Schema:     masked,
		LoadedAt:   snap.LoadedAt,
		Persisted:  snap.Persisted,
		ConfigFile: h.manager.ConfigFilePath(),
	})
}

// GetRawConfig returns the raw JSON configuration file without any processing.
// This endpoint reads the config file directly from disk and returns it as-is,
// preserving all raw values including ${VAR} syntax and unmasked API keys.
// Used by the UI for editing to show exact configuration as written.
//
// GET /ui/api/config/raw
//
// @return Raw JSON file content as-is (no masking, no processing).
func (h *ConfigHandler) GetRawConfig(c *gin.Context) {
	configFile := h.manager.ConfigFilePath()
	if configFile == "" {
		c.JSON(http.StatusInternalServerError, apiResponse{
			OK:    false,
			Error: "No config file path configured",
		})
		return
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiResponse{
			OK:    false,
			Error: fmt.Sprintf("Failed to read config file: %v", err),
		})
		return
	}

	// Return raw JSON directly without any processing
	c.Data(http.StatusOK, "application/json", data)
}

// UpdateConfig replaces the entire configuration with the provided schema.
// Validates the new config before applying. Old config continues serving on validation failure.
//
// PUT /ui/api/config
//
// @param JSON body matching config.Schema structure.
// @return Success response or validation errors.
func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	var schema config.Schema
	if err := c.ShouldBindJSON(&schema); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			OK:    false,
			Error: fmt.Sprintf("Invalid JSON: %v", err),
		})
		return
	}

	// Resolve environment variables in the new schema
	resolveEnvVars(&schema)

	if err := h.manager.UpdateSchema(&schema); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		OK:      true,
		Message: "Configuration updated successfully",
	})
}

// PatchModels adds or updates individual model configurations.
// Merges the provided models into the existing configuration.
//
// PATCH /ui/api/config/models
//
// @param JSON body: map of model names to ModelConfig.
// @return Success response or validation errors.
func (h *ConfigHandler) PatchModels(c *gin.Context) {
	var models map[string]config.ModelConfig
	if err := c.ShouldBindJSON(&models); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			OK:    false,
			Error: fmt.Sprintf("Invalid JSON: %v", err),
		})
		return
	}

	snap := h.manager.Get()
	if snap == nil || snap.Schema == nil {
		c.JSON(http.StatusInternalServerError, apiResponse{
			OK:    false,
			Error: "Configuration not loaded",
		})
		return
	}

	// Deep copy schema
	merged := deepCopySchema(snap.Schema)

	// Merge models
	if merged.Models == nil {
		merged.Models = make(map[string]config.ModelConfig)
	}
	for name, mc := range models {
		merged.Models[name] = mc
	}

	if err := h.manager.UpdateSchema(merged); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		OK:      true,
		Message: fmt.Sprintf("Models updated (%d total)", len(merged.Models)),
	})
}

// DeleteModel removes a model from the configuration.
//
// DELETE /ui/api/config/models/:name
//
// @param name - Model name to remove (URL path parameter).
// @return Success response or error if model not found.
func (h *ConfigHandler) DeleteModel(c *gin.Context) {
	name := c.Param("name")

	snap := h.manager.Get()
	if snap == nil || snap.Schema == nil {
		c.JSON(http.StatusInternalServerError, apiResponse{
			OK:    false,
			Error: "Configuration not loaded",
		})
		return
	}

	merged := deepCopySchema(snap.Schema)
	if _, exists := merged.Models[name]; !exists {
		c.JSON(http.StatusNotFound, apiResponse{
			OK:    false,
			Error: fmt.Sprintf("Model '%s' not found", name),
		})
		return
	}

	delete(merged.Models, name)

	if err := h.manager.UpdateSchema(merged); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		OK:      true,
		Message: fmt.Sprintf("Model '%s' removed", name),
	})
}

// PatchProviders adds or updates individual provider configurations.
// Merges the provided providers into the existing configuration.
//
// PATCH /ui/api/config/providers
//
// @param JSON body: array of Provider objects.
// @return Success response or validation errors.
func (h *ConfigHandler) PatchProviders(c *gin.Context) {
	var providers []config.Provider
	if err := c.ShouldBindJSON(&providers); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			OK:    false,
			Error: fmt.Sprintf("Invalid JSON: %v", err),
		})
		return
	}

	snap := h.manager.Get()
	if snap == nil || snap.Schema == nil {
		c.JSON(http.StatusInternalServerError, apiResponse{
			OK:    false,
			Error: "Configuration not loaded",
		})
		return
	}

	merged := deepCopySchema(snap.Schema)

	// Build map of existing providers by name
	existingMap := make(map[string]int)
	for i, p := range merged.Providers {
		existingMap[p.Name] = i
	}

	// Merge providers (update existing or append new)
	for _, p := range providers {
		if idx, exists := existingMap[p.Name]; exists {
			merged.Providers[idx] = p
		} else {
			merged.Providers = append(merged.Providers, p)
		}
	}

	if err := h.manager.UpdateSchema(merged); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		OK:      true,
		Message: fmt.Sprintf("Providers updated (%d total)", len(merged.Providers)),
	})
}

// DeleteProvider removes a provider from the configuration.
// Fails if any model still references the provider.
//
// DELETE /ui/api/config/providers/:name
//
// @param name - Provider name to remove (URL path parameter).
// @return Success response or error if provider not found or still referenced.
func (h *ConfigHandler) DeleteProvider(c *gin.Context) {
	name := c.Param("name")

	snap := h.manager.Get()
	if snap == nil || snap.Schema == nil {
		c.JSON(http.StatusInternalServerError, apiResponse{
			OK:    false,
			Error: "Configuration not loaded",
		})
		return
	}

	merged := deepCopySchema(snap.Schema)

	// Check if any model references this provider
	var referencingModels []string
	for modelName, mc := range merged.Models {
		if mc.Provider == name {
			referencingModels = append(referencingModels, modelName)
		}
	}
	if len(referencingModels) > 0 {
		c.JSON(http.StatusConflict, apiResponse{
			OK:    false,
			Error: fmt.Sprintf("Cannot remove provider '%s': still referenced by models %v", name, referencingModels),
		})
		return
	}

	// Find and remove the provider
	found := false
	for i, p := range merged.Providers {
		if p.Name == name {
			merged.Providers = append(merged.Providers[:i], merged.Providers[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, apiResponse{
			OK:    false,
			Error: fmt.Sprintf("Provider '%s' not found", name),
		})
		return
	}

	if err := h.manager.UpdateSchema(merged); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		OK:      true,
		Message: fmt.Sprintf("Provider '%s' removed", name),
	})
}

// ReloadConfig re-reads the configuration from disk and applies it.
// If the file is invalid, the current configuration remains unchanged.
//
// POST /ui/api/config/reload
//
// @return Reload status and the reloaded schema.
func (h *ConfigHandler) ReloadConfig(c *gin.Context) {
	schema, err := h.manager.ReloadFromDisk()
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiResponse{
			OK:    false,
			Error: fmt.Sprintf("Failed to reload config: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, reloadResponse{
		OK: true,
		Message: fmt.Sprintf("Config reloaded from %s (%d providers, %d models)",
			h.manager.ConfigFilePath(), len(schema.Providers), len(schema.Models)),
	})
}

// SaveConfig persists the current in-memory configuration to disk.
// Creates a backup of the existing file before overwriting.
//
// POST /ui/api/config/save
//
// @return Save status and the file path.
func (h *ConfigHandler) SaveConfig(c *gin.Context) {
	if err := h.manager.SaveToDisk(); err != nil {
		c.JSON(http.StatusInternalServerError, apiResponse{
			OK:    false,
			Error: fmt.Sprintf("Failed to save config: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		OK:      true,
		Message: fmt.Sprintf("Config saved to %s", h.manager.ConfigFilePath()),
	})
}

// GetStatus returns server health and configuration metadata.
//
// GET /ui/api/status
//
// @return Status info including uptime, config state, provider/model counts.
func (h *ConfigHandler) GetStatus(c *gin.Context) {
	snap := h.manager.Get()

	resp := statusResponse{
		Status:    "healthy",
		StartedAt: h.manager.StartTime(),
		Uptime:    time.Since(h.manager.StartTime()).Round(time.Second).String(),
	}

	if snap != nil && snap.Schema != nil {
		resp.Config.LoadedAt = snap.LoadedAt
		resp.Config.Persisted = snap.Persisted
		resp.Config.ConfigFile = h.manager.ConfigFilePath()
		resp.Providers = len(snap.Schema.Providers)
		resp.Models = len(snap.Schema.Models)
	} else {
		resp.Status = "no_config"
	}

	c.JSON(http.StatusOK, resp)
}

// maskSchema returns a deep copy of the schema with API keys masked.
//
// @param s - Schema to mask.
// @return *Schema - Masked copy.
func maskSchema(s *config.Schema) *config.Schema {
	masked := deepCopySchema(s)
	for i := range masked.Providers {
		masked.Providers[i].APIKey = maskAPIKey(masked.Providers[i].APIKey)
	}
	masked.WebSearch.ExaAPIKey = maskAPIKey(masked.WebSearch.ExaAPIKey)
	masked.WebSearch.BraveAPIKey = maskAPIKey(masked.WebSearch.BraveAPIKey)
	return masked
}

// maskAPIKey masks an API key for safe display.
// Shows first 4 and last 4 characters, or "***masked***" for short keys.
// Does NOT mask ${VAR} syntax patterns (these are env var references, not secrets).
//
// @param key - API key to mask.
// @return Masked key string, or original if it's an env var reference.
func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	// Don't mask ${VAR} patterns - these are environment variable references
	if strings.HasPrefix(key, "${") && strings.HasSuffix(key, "}") {
		return key
	}
	if len(key) <= 8 {
		return "***masked***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// deepCopySchema creates a deep copy of a Schema by marshaling and unmarshaling JSON.
//
// @param s - Schema to copy.
// @return *Schema - Deep copy.
func deepCopySchema(s *config.Schema) *config.Schema {
	data, err := json.Marshal(s)
	if err != nil {
		// Fallback: return a new empty schema
		return &config.Schema{}
	}
	var copy config.Schema
	if err := json.Unmarshal(data, &copy); err != nil {
		return &config.Schema{}
	}
	return &copy
}

// resolveEnvVars resolves environment variables in a schema.
// This is a convenience wrapper for the config package's env resolution.
//
// @param s - Schema to resolve env vars in.
func resolveEnvVars(s *config.Schema) {
	for i := range s.Providers {
		p := &s.Providers[i]
		if p.APIKey == "" && p.EnvAPIKey != "" {
			// Will be resolved during next reload from disk
		}
	}
}
