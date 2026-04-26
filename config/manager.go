package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"ai-proxy/logging"
)

// Snapshot holds a consistent configuration state.
// The Schema and metadata are always paired — readers never see a torn state.
//
// @invariant Schema != nil after initialization
type Snapshot struct {
	// Schema is the current configuration schema.
	Schema *Schema
	// LoadedAt is the time when this snapshot was created.
	LoadedAt time.Time
	// Persisted indicates whether the in-memory config matches the disk file.
	// True after initial load or explicit save; false after API updates.
	Persisted bool
}

// ConfigManager holds the live configuration snapshot and supports atomic swaps.
// Read path: Get() returns the current snapshot via atomic pointer — zero contention, no locks.
// Write path: UpdateSchema/ReloadFromDisk serialize via writeMu.
//
// Thread safety:
//   - Multiple goroutines can call Get() concurrently without synchronization.
//   - Write operations are serialized by writeMu to ensure consistency.
//   - Readers never see a partially-applied configuration.
//
// @invariant snapshot always contains a non-nil Schema
type ConfigManager struct {
	snapshot   atomic.Pointer[Snapshot]
	configFile string
	writeMu    sync.Mutex
	startTime  time.Time
}

// NewManager creates a new ConfigManager with the given initial schema.
//
// @param schema - Initial configuration schema. Must not be nil.
// @param configFile - Path to the JSON config file on disk. May be empty if no file backing.
// @return *ConfigManager - A new manager with the initial snapshot loaded.
//
// @pre schema != nil
// @post Get() returns a snapshot with the given schema
func NewManager(schema *Schema, configFile string) *ConfigManager {
	m := &ConfigManager{
		configFile: configFile,
		startTime:  time.Now(),
	}
	m.snapshot.Store(&Snapshot{
		Schema:    schema,
		LoadedAt:  time.Now(),
		Persisted: true,
	})
	return m
}

// Get returns the current configuration snapshot.
// This is safe to call from any goroutine without synchronization.
//
// @return *Snapshot - The current configuration snapshot. Never nil.
func (m *ConfigManager) Get() *Snapshot {
	return m.snapshot.Load()
}

// UpdateSchema validates and applies a new configuration schema atomically.
// The old configuration continues to be served until the new one is validated and stored.
// After a successful update, Persisted is set to false (changes are in-memory only).
//
// @param schema - The new configuration schema to apply. Must not be nil.
// @return error - Validation error, or nil on success.
//
// @pre schema != nil
// @post Get() returns a snapshot with the new schema
// @post Persisted is false after successful update
func (m *ConfigManager) UpdateSchema(schema *Schema) error {
	if schema == nil {
		logging.ErrorMsg("config manager: UpdateSchema called with nil schema")
		return fmt.Errorf("schema cannot be nil")
	}

	m.writeMu.Lock()
	defer m.writeMu.Unlock()

	if err := Validate(schema); err != nil {
		logging.ErrorMsg("config manager: schema validation failed: %v", err)
		return fmt.Errorf("validation failed: %w", err)
	}

	m.snapshot.Store(&Snapshot{
		Schema:    schema,
		LoadedAt:  time.Now(),
		Persisted: false,
	})
	return nil
}

// ReloadFromDisk re-reads the configuration file, validates it, and atomically
// applies the new configuration. If the file cannot be read, parsed, or validated,
// the current configuration remains unchanged.
//
// @return *Schema - The newly loaded schema, or nil on error.
// @return error - File read, parse, or validation error, or nil on success.
//
// @post On success, Get() returns a snapshot with the reloaded schema
// @post On error, the current configuration is unchanged
// @post Persisted is true after successful reload
func (m *ConfigManager) ReloadFromDisk() (*Schema, error) {
	if m.configFile == "" {
		logging.ErrorMsg("config manager: ReloadFromDisk called with no config file path")
		return nil, fmt.Errorf("no config file path configured")
	}

	m.writeMu.Lock()
	defer m.writeMu.Unlock()

	loader := NewLoader()
	schema, err := loader.Load(m.configFile)
	if err != nil {
		logging.ErrorMsg("config manager: failed to reload config from '%s': %v", m.configFile, err)
		return nil, fmt.Errorf("failed to reload config: %w", err)
	}

	m.snapshot.Store(&Snapshot{
		Schema:    schema,
		LoadedAt:  time.Now(),
		Persisted: true,
	})
	return schema, nil
}

// SaveToDisk persists the current in-memory configuration to the config file.
// Creates a backup of the existing file before overwriting.
//
// @return error - File operation error, or nil on success.
//
// @post Current schema is written to the config file
// @post A backup file is created with .bak suffix
// @post Persisted is true after successful save
func (m *ConfigManager) SaveToDisk() error {
	if m.configFile == "" {
		logging.ErrorMsg("config manager: SaveToDisk called with no config file path")
		return fmt.Errorf("no config file path configured")
	}

	m.writeMu.Lock()
	defer m.writeMu.Unlock()

	snap := m.snapshot.Load()
	if snap == nil || snap.Schema == nil {
		logging.ErrorMsg("config manager: SaveToDisk called with no configuration")
		return fmt.Errorf("no configuration to save")
	}

	// Create backup of existing file
	if _, err := os.Stat(m.configFile); err == nil {
		backupPath := m.configFile + ".bak"
		data, err := os.ReadFile(m.configFile)
		if err == nil {
			_ = os.WriteFile(backupPath, data, 0600)
		}
	}

	// Marshal schema to pretty-printed JSON
	data, err := json.MarshalIndent(snap.Schema, "", "  ")
	if err != nil {
		logging.ErrorMsg("config manager: failed to marshal config for saving: %v", err)
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.configFile, data, 0644); err != nil {
		logging.ErrorMsg("config manager: failed to write config file '%s': %v", m.configFile, err)
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Update persisted flag
	m.snapshot.Store(&Snapshot{
		Schema:    snap.Schema,
		LoadedAt:  snap.LoadedAt,
		Persisted: true,
	})

	return nil
}

// ConfigFilePath returns the path to the configuration file on disk.
//
// @return string - The config file path, or empty string if not configured.
func (m *ConfigManager) ConfigFilePath() string {
	return m.configFile
}

// StartTime returns the time when the manager was created (server start time).
//
// @return time.Time - The server start time.
func (m *ConfigManager) StartTime() time.Time {
	return m.startTime
}
