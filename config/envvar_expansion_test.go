package config

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"

	"ai-proxy/logging"
)

func captureInfoLog(f func()) string {
	var buf bytes.Buffer
	old := logging.Info
	logging.Info = log.New(&buf, "[INFO] ", log.LstdFlags)
	defer func() { logging.Info = old }()
	f()
	return buf.String()
}

func TestExpandEnvVars_LogsMissingVar(t *testing.T) {
	os.Unsetenv("DEFINITELY_MISSING_VAR_XYZ_001")

	output := captureInfoLog(func() {
		result := ExpandEnvVars("${DEFINITELY_MISSING_VAR_XYZ_001}")
		if result != "" {
			t.Errorf("expected empty string for missing var, got %q", result)
		}
	})

	if !strings.Contains(output, "DEFINITELY_MISSING_VAR_XYZ_001") {
		t.Errorf("expected warning about missing var, got: %s", output)
	}
	if !strings.Contains(output, "Warning") {
		t.Errorf("expected 'Warning' in log output, got: %s", output)
	}
}

func TestExpandEnvVars_NoLogWhenVarSet(t *testing.T) {
	os.Setenv("TEST_EXPAND_VAR_SET", "hello")
	defer os.Unsetenv("TEST_EXPAND_VAR_SET")

	output := captureInfoLog(func() {
		result := ExpandEnvVars("${TEST_EXPAND_VAR_SET}")
		if result != "hello" {
			t.Errorf("expected 'hello', got %q", result)
		}
	})

	if strings.Contains(output, "Warning") {
		t.Errorf("expected no warning when var is set, got: %s", output)
	}
}

func TestExpandEnvVars_MultipleVars_LogsOnlyMissing(t *testing.T) {
	os.Setenv("TEST_EXPAND_PRESENT", "value")
	os.Unsetenv("TEST_EXPAND_ABSENT_002")
	defer os.Unsetenv("TEST_EXPAND_PRESENT")

	output := captureInfoLog(func() {
		result := ExpandEnvVars("${TEST_EXPAND_PRESENT}-${TEST_EXPAND_ABSENT_002}")
		if result != "value-" {
			t.Errorf("expected 'value-', got %q", result)
		}
	})

	if !strings.Contains(output, "TEST_EXPAND_ABSENT_002") {
		t.Errorf("expected warning about TEST_EXPAND_ABSENT_002, got: %s", output)
	}
	if strings.Contains(output, "TEST_EXPAND_PRESENT") {
		t.Errorf("should not warn about set variable, got: %s", output)
	}
}

func TestExpandEnvVars_EmptyInput_NoLog(t *testing.T) {
	output := captureInfoLog(func() {
		result := ExpandEnvVars("")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	if strings.Contains(output, "Warning") {
		t.Errorf("expected no warning for empty input, got: %s", output)
	}
}

func TestExpandEnvVars_PlainString_NoLog(t *testing.T) {
	output := captureInfoLog(func() {
		result := ExpandEnvVars("plain-key-no-vars")
		if result != "plain-key-no-vars" {
			t.Errorf("expected 'plain-key-no-vars', got %q", result)
		}
	})

	if strings.Contains(output, "Warning") {
		t.Errorf("expected no warning for plain string, got: %s", output)
	}
}

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
			name:      "apiKey_without_${}_returned_as_is",
			apiKey:    "plain-key-value",
			envAPIKey: "",
			want:      "plain-key-value",
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

func TestProviderGetAPIKey_LogsWarningWhenEnvApiKeyMissing(t *testing.T) {
	os.Unsetenv("MISSING_ENV_KEY_003")

	provider := Provider{
		Name:      "test-provider",
		EnvAPIKey: "MISSING_ENV_KEY_003",
	}

	output := captureInfoLog(func() {
		got := provider.GetAPIKey()
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	if !strings.Contains(output, "MISSING_ENV_KEY_003") {
		t.Errorf("expected warning about missing envApiKey, got: %s", output)
	}
	if !strings.Contains(output, "test-provider") {
		t.Errorf("expected provider name in warning, got: %s", output)
	}
}

func TestProviderGetAPIKey_NoWarningWhenEnvApiKeySet(t *testing.T) {
	os.Setenv("PRESENT_ENV_KEY_004", "secret")
	defer os.Unsetenv("PRESENT_ENV_KEY_004")

	provider := Provider{
		Name:      "test-provider",
		EnvAPIKey: "PRESENT_ENV_KEY_004",
	}

	output := captureInfoLog(func() {
		got := provider.GetAPIKey()
		if got != "secret" {
			t.Errorf("expected 'secret', got %q", got)
		}
	})

	if strings.Contains(output, "Warning") {
		t.Errorf("expected no warning when env var is set, got: %s", output)
	}
}

func TestLoaderResolveEnvVars_LogsWarningWhenMissing(t *testing.T) {
	os.Unsetenv("MISSING_LOADER_KEY_005")

	schema := &Schema{
		Providers: []Provider{
			{
				Name:      "provider-missing-key",
				EnvAPIKey: "MISSING_LOADER_KEY_005",
			},
		},
	}

	output := captureInfoLog(func() {
		loader := NewLoader()
		loader.resolveEnvVars(schema)
	})

	if schema.Providers[0].APIKey != "" {
		t.Errorf("expected empty APIKey, got %q", schema.Providers[0].APIKey)
	}
	if !strings.Contains(output, "MISSING_LOADER_KEY_005") {
		t.Errorf("expected warning about missing loader key, got: %s", output)
	}
	if !strings.Contains(output, "provider-missing-key") {
		t.Errorf("expected provider name in warning, got: %s", output)
	}
}

func TestLoaderResolveEnvVars_NoWarningWhenSet(t *testing.T) {
	os.Setenv("PRESENT_LOADER_KEY_006", "resolved")
	defer os.Unsetenv("PRESENT_LOADER_KEY_006")

	schema := &Schema{
		Providers: []Provider{
			{
				Name:      "provider-with-key",
				EnvAPIKey: "PRESENT_LOADER_KEY_006",
			},
		},
	}

	output := captureInfoLog(func() {
		loader := NewLoader()
		loader.resolveEnvVars(schema)
	})

	if schema.Providers[0].APIKey != "resolved" {
		t.Errorf("expected 'resolved', got %q", schema.Providers[0].APIKey)
	}
	if strings.Contains(output, "Warning") {
		t.Errorf("expected no warning when env var is set, got: %s", output)
	}
}
