package config

import "fmt"

// isValidProtocol checks if the given string is a valid protocol name.
// Valid protocols are: "openai", "anthropic", "responses".
func isValidProtocol(p string) bool {
	return p == "openai" || p == "anthropic" || p == "responses"
}

// Validate checks that the configuration schema is valid according to the rules:
//   - At least one provider required
//   - Each provider must have: name, endpoints (at least one)
//   - Endpoints must use valid protocol names
//   - If multiple endpoints, default must be specified and valid
//   - At least one API key source (apiKey or envApiKey) per provider
//   - Model mappings must reference existing providers
//   - If fallback.enabled, provider must exist
//
// @param s - the schema to validate
// @return error - a descriptive error if validation fails, nil otherwise
func Validate(s *Schema) error {
	// Validate at least one provider
	if len(s.Providers) == 0 {
		return fmt.Errorf("at least one provider required")
	}

	// Build a set of valid provider names for reference validation
	providerNames := make(map[string]bool, len(s.Providers))

	// Validate each provider
	for i, p := range s.Providers {
		providerLabel := p.Name
		if providerLabel == "" {
			providerLabel = fmt.Sprintf("index %d", i)
		}

		// Validate name is required
		if p.Name == "" {
			return fmt.Errorf("provider '%s': name is required", providerLabel)
		}

		// Validate endpoints is required and has at least one entry
		if len(p.Endpoints) == 0 {
			return fmt.Errorf("provider '%s': endpoints is required with at least one protocol endpoint", p.Name)
		}

		// Validate endpoints format
		for protocol := range p.Endpoints {
			if !isValidProtocol(protocol) {
				return fmt.Errorf("provider '%s': invalid protocol '%s' in endpoints (must be openai, anthropic, or responses)", p.Name, protocol)
			}
		}

		// Multi-protocol providers must have a default if more than one endpoint
		if len(p.Endpoints) > 1 {
			if p.Default == "" {
				return fmt.Errorf("provider '%s': 'default' field is required when multiple endpoints are configured", p.Name)
			}
			if !isValidProtocol(p.Default) {
				return fmt.Errorf("provider '%s': default protocol '%s' is invalid (must be openai, anthropic, or responses)", p.Name, p.Default)
			}
			if _, exists := p.Endpoints[p.Default]; !exists {
				return fmt.Errorf("provider '%s': default protocol '%s' not found in endpoints", p.Name, p.Default)
			}
		} else if p.Default != "" {
			// Single-endpoint providers: validate Default if explicitly set
			if !isValidProtocol(p.Default) {
				return fmt.Errorf("provider '%s': default protocol '%s' is invalid (must be openai, anthropic, or responses)", p.Name, p.Default)
			}
			if _, exists := p.Endpoints[p.Default]; !exists {
				return fmt.Errorf("provider '%s': default protocol '%s' not found in endpoints", p.Name, p.Default)
			}
		}

		// Validate at least one API key source
		if p.APIKey == "" && p.EnvAPIKey == "" {
			return fmt.Errorf("provider '%s': at least one of apiKey or envApiKey is required", p.Name)
		}

		providerNames[p.Name] = true
	}

	// Validate model mappings reference existing providers
	for name, mc := range s.Models {
		if !providerNames[mc.Provider] {
			return fmt.Errorf("model '%s' references unknown provider '%s'", name, mc.Provider)
		}

		// Validate type field if present
		if mc.Type != "" && mc.Type != "openai" && mc.Type != "anthropic" && mc.Type != "auto" {
			return fmt.Errorf("model '%s': type must be 'openai', 'anthropic', or 'auto'", name)
		}
	}

	// Validate fallback configuration
	if s.Fallback.Enabled {
		if !providerNames[s.Fallback.Provider] {
			return fmt.Errorf("fallback references unknown provider '%s'", s.Fallback.Provider)
		}

		// Validate fallback type field if present
		if s.Fallback.Type != "" && s.Fallback.Type != "openai" && s.Fallback.Type != "anthropic" && s.Fallback.Type != "auto" {
			return fmt.Errorf("fallback: type must be 'openai', 'anthropic', or 'auto'")
		}
	}

	return nil
}
