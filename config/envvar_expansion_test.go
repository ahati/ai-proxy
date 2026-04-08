package config

import (
	"os"
	"testing"
)

func TestProviderGetAPIKey_EnvVarExpansion(t *testing.T) {
	tests := []struct {
		name        string
		apiKey      string
		envAPIKey   string
		envVarValue string
		envVarName  string
		want        string
	}{
		{
			name:        "apiKey_with_${VAR}_expansion",
			apiKey:      "${TEST_API_KEY}",
			envAPIKey:   "",
			envVarName:  "TEST_API_KEY",
			envVarValue: "resolved-key-value",
			want:        "resolved-key-value",
		},
		{
			name:        "apiKey_with_${VAR}_when_env_not_set",
			apiKey:      "${MISSING_KEY}",
			envAPIKey:   "",
			envVarName:  "MISSING_KEY",
			envVarValue: "",
			want:        "",
		},
		{
			name:        "apiKey_without_${}_returned_as_is",
			apiKey:      "plain-key-value",
			envAPIKey:   "",
			want:        "plain-key-value",
		},
		{
			name:        "apiKey_with_nested_${VAR}",
			apiKey:      "prefix-${TEST_KEY}-suffix",
			envVarName:  "TEST_KEY",
			envVarValue: "middle",
			want:        "prefix-middle-suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable if specified
			if tt.envVarName != "" && tt.envVarValue != "" {
				os.Setenv(tt.envVarName, tt.envVarValue)
				defer os.Unsetenv(tt.envVarName)
			}

			provider := Provider{
				Name:      "test-provider",
				APIKey:    tt.apiKey,
				EnvAPIKey: tt.envAPIKey,
			}

			got := provider.GetAPIKey()
			if got != tt.want {
				t.Errorf("GetAPIKey() = %q, want %q", got, tt.want)
			}
		})
	}
}