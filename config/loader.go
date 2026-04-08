// Package config provides configuration loading from JSON files.
// This file implements the configuration loader that reads JSON config files
// with validation and environment variable resolution.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Loader handles loading and validating configuration from JSON files.
type Loader struct{}

// NewLoader creates a new configuration loader instance.
//
// @return *Loader - a new Loader instance
func NewLoader() *Loader {
	return &Loader{}
}

// Load reads and parses a JSON configuration file, validates it,
// and resolves any environment variables referenced in the configuration.
//
// @param path - path to the JSON configuration file
// @return *Schema - the parsed, validated, and resolved configuration schema
// @return error - file read error, JSON parse error, validation error, or nil on success
// @post Returns a fully populated and validated Schema on success
func (l *Loader) Load(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var schema Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	if err := Validate(&schema); err != nil {
		return nil, err
	}

	l.resolveEnvVars(&schema)

	return &schema, nil
}

// resolveEnvVars resolves environment variables in the configuration.
// For each provider, if EnvAPIKey is set and APIKey is empty, the APIKey
// field is populated from the environment variable value.
// Also expands ${VAR} patterns in string fields like API keys.
//
// @param s - the schema to resolve environment variables in
func (l *Loader) resolveEnvVars(s *Schema) {
	for i := range s.Providers {
		p := &s.Providers[i]
		// Don't expand ${VAR} patterns in apiKey - preserve raw syntax for display/editing
		// Expansion happens dynamically in Provider.GetAPIKey() at runtime
		// If apiKey is empty and envApiKey is set, get from env
		if p.APIKey == "" && p.EnvAPIKey != "" {
			p.APIKey = os.Getenv(p.EnvAPIKey)
		}
	}

	// Expand environment variables in websearch config
	s.WebSearch.ExaAPIKey = expandEnvVars(s.WebSearch.ExaAPIKey)
	s.WebSearch.BraveAPIKey = expandEnvVars(s.WebSearch.BraveAPIKey)
}
