package config

import (
	"flag"
	"os"
	"testing"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		envValue       string
		setEnv         bool
		expectedPath   string
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:           "flag provided",
			args:           []string{"test", "--config-file=/path/to/config.yaml"},
			envValue:       "",
			setEnv:         false,
			expectedPath:   "/path/to/config.yaml",
			expectError:    false,
			expectedErrMsg: "",
		},
		{
			name:           "flag with equals sign",
			args:           []string{"test", "--config-file=/etc/app/config.json"},
			envValue:       "",
			setEnv:         false,
			expectedPath:   "/etc/app/config.json",
			expectError:    false,
			expectedErrMsg: "",
		},
		{
			name:           "flag with separate value",
			args:           []string{"test", "--config-file", "/opt/config.toml"},
			envValue:       "",
			setEnv:         false,
			expectedPath:   "/opt/config.toml",
			expectError:    false,
			expectedErrMsg: "",
		},
		{
			name:           "env var fallback when no flag",
			args:           []string{"test"},
			envValue:       "/env/config.yaml",
			setEnv:         true,
			expectedPath:   "/env/config.yaml",
			expectError:    false,
			expectedErrMsg: "",
		},
		{
			name:           "flag overrides env var",
			args:           []string{"test", "--config-file=/flag/config.yaml"},
			envValue:       "/env/config.yaml",
			setEnv:         true,
			expectedPath:   "/flag/config.yaml",
			expectError:    false,
			expectedErrMsg: "",
		},
		{
			name:           "neither flag nor env var provided",
			args:           []string{"test"},
			envValue:       "",
			setEnv:         false,
			expectedPath:   "",
			expectError:    true,
			expectedErrMsg: "config file required: use --config-file or CONFIG_FILE environment variable",
		},
		{
			name:           "env var set to empty string",
			args:           []string{"test"},
			envValue:       "",
			setEnv:         true,
			expectedPath:   "",
			expectError:    true,
			expectedErrMsg: "config file required: use --config-file or CONFIG_FILE environment variable",
		},
		{
			name:           "flag with relative path",
			args:           []string{"test", "--config-file=./local-config.yaml"},
			envValue:       "",
			setEnv:         false,
			expectedPath:   "./local-config.yaml",
			expectError:    false,
			expectedErrMsg: "",
		},
		{
			name:           "flag with spaces in path",
			args:           []string{"test", "--config-file=/path with spaces/config.yaml"},
			envValue:       "",
			setEnv:         false,
			expectedPath:   "/path with spaces/config.yaml",
			expectError:    false,
			expectedErrMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)

			originalArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = originalArgs }()

			if tt.setEnv {
				os.Setenv("CONFIG_FILE", tt.envValue)
			} else {
				os.Unsetenv("CONFIG_FILE")
			}
			defer os.Unsetenv("CONFIG_FILE")

			// Also clear XDG vars to ensure test isolation
			os.Unsetenv("XDG_CONFIG_HOME")
			os.Unsetenv("XDG_CONFIG_DIRS")

			flags, err := ParseFlags()

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseFlags() expected error, got nil")
				}
				if err != nil && err.Error() != tt.expectedErrMsg {
					t.Errorf("ParseFlags() error = %q, want %q", err.Error(), tt.expectedErrMsg)
				}
				if flags.ConfigFile != "" {
					t.Errorf("ParseFlags() path = %q, want empty string on error", flags.ConfigFile)
				}
			} else {
				if err != nil {
					t.Errorf("ParseFlags() unexpected error: %v", err)
				}
				if flags.ConfigFile != tt.expectedPath {
					t.Errorf("ParseFlags() path = %q, want %q", flags.ConfigFile, tt.expectedPath)
				}
			}
		})
	}
}

func TestParseFlags_ErrConfigFileRequired(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"test"}
	os.Unsetenv("CONFIG_FILE")
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_DIRS")

	err := ErrConfigFileRequired

	if err == nil {
		t.Error("ErrConfigFileRequired should not be nil")
	}

	expectedMsg := "config file required: use --config-file or CONFIG_FILE environment variable"
	if err.Error() != expectedMsg {
		t.Errorf("ErrConfigFileRequired.Error() = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestParseFlags_MultipleFlags(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"test", "--config-file=/config.yaml", "--port=8080"}
	os.Unsetenv("CONFIG_FILE")
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_DIRS")

	flags, err := ParseFlags()

	if err != nil {
		t.Errorf("ParseFlags() unexpected error: %v", err)
	}
	if flags.ConfigFile != "/config.yaml" {
		t.Errorf("ParseFlags() path = %q, want /config.yaml", flags.ConfigFile)
	}
	if flags.Port != "8080" {
		t.Errorf("ParseFlags() port = %q, want 8080", flags.Port)
	}
}

func TestParseFlags_FlagPrecedenceOverEnv(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"test", "--config-file=/flag-config.yaml"}
	os.Setenv("CONFIG_FILE", "/env-config.yaml")
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_DIRS")
	defer os.Unsetenv("CONFIG_FILE")

	flags, err := ParseFlags()

	if err != nil {
		t.Errorf("ParseFlags() unexpected error: %v", err)
	}
	if flags.ConfigFile != "/flag-config.yaml" {
		t.Errorf("ParseFlags() path = %q, want /flag-config.yaml (flag should override env)", flags.ConfigFile)
	}
}

func TestParseFlags_EnvWithSpecialChars(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"test"}
	os.Setenv("CONFIG_FILE", "/path/with-special_chars.123/config.yaml")
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_DIRS")
	defer os.Unsetenv("CONFIG_FILE")

	flags, err := ParseFlags()

	if err != nil {
		t.Errorf("ParseFlags() unexpected error: %v", err)
	}
	if flags.ConfigFile != "/path/with-special_chars.123/config.yaml" {
		t.Errorf("ParseFlags() path = %q, want /path/with-special_chars.123/config.yaml", flags.ConfigFile)
	}
}

func TestDiscoverConfigPath_XDGConfigHome(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configDir := tmpDir + "/ai-proxy"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	configPath := configDir + "/config.json"
	if err := os.WriteFile(configPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	os.Unsetenv("XDG_CONFIG_DIRS")
	defer os.Unsetenv("XDG_CONFIG_HOME")

	result := discoverConfigPath()
	if result != configPath {
		t.Errorf("discoverConfigPath() = %q, want %q", result, configPath)
	}
}

func TestDiscoverConfigPath_HomeFallback(t *testing.T) {
	// Create temp config file in ~/.config/ai-proxy/
	tmpDir := t.TempDir()
	homeConfigDir := tmpDir + "/.config/ai-proxy"
	if err := os.MkdirAll(homeConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	configPath := homeConfigDir + "/config.json"
	if err := os.WriteFile(configPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Set HOME to temp dir and unset XDG_CONFIG_HOME
	os.Setenv("HOME", tmpDir)
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_CONFIG_DIRS")
	defer os.Unsetenv("HOME")

	result := discoverConfigPath()
	if result != configPath {
		t.Errorf("discoverConfigPath() = %q, want %q", result, configPath)
	}
}

func TestDiscoverConfigPath_XDGConfigDirs(t *testing.T) {
	// Create temp config file in XDG_CONFIG_DIRS path
	tmpDir := t.TempDir()
	configDir := tmpDir + "/ai-proxy"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	configPath := configDir + "/config.json"
	if err := os.WriteFile(configPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	os.Unsetenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_DIRS", tmpDir)
	defer os.Unsetenv("XDG_CONFIG_DIRS")

	result := discoverConfigPath()
	if result != configPath {
		t.Errorf("discoverConfigPath() = %q, want %q", result, configPath)
	}
}

func TestDiscoverConfigPath_XDGConfigDirs_FirstWins(t *testing.T) {
	// Create two config files in different XDG_CONFIG_DIRS paths
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	configDir1 := tmpDir1 + "/ai-proxy"
	if err := os.MkdirAll(configDir1, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	configPath1 := configDir1 + "/config.json"
	if err := os.WriteFile(configPath1, []byte(`{"first":true}`), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	configDir2 := tmpDir2 + "/ai-proxy"
	if err := os.MkdirAll(configDir2, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	configPath2 := configDir2 + "/config.json"
	if err := os.WriteFile(configPath2, []byte(`{"second":true}`), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	os.Unsetenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_DIRS", tmpDir1+":"+tmpDir2)
	defer os.Unsetenv("XDG_CONFIG_DIRS")

	result := discoverConfigPath()
	if result != configPath1 {
		t.Errorf("discoverConfigPath() = %q, want %q (first should win)", result, configPath1)
	}
}

func TestDiscoverConfigPath_NotFound(t *testing.T) {
	// Use non-existent paths
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_DIRS", "/nonexistent1:/nonexistent2")
	defer os.Unsetenv("XDG_CONFIG_DIRS")

	result := discoverConfigPath()
	if result != "" {
		t.Errorf("discoverConfigPath() = %q, want empty string", result)
	}
}

func TestDiscoverConfigPath_Priority(t *testing.T) {
	// Test that XDG_CONFIG_HOME takes priority over XDG_CONFIG_DIRS
	tmpHome := t.TempDir()
	tmpDirs := t.TempDir()

	// Create config in XDG_CONFIG_HOME
	homeConfigDir := tmpHome + "/ai-proxy"
	if err := os.MkdirAll(homeConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	homeConfigPath := homeConfigDir + "/config.json"
	if err := os.WriteFile(homeConfigPath, []byte(`{"source":"home"}`), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create config in XDG_CONFIG_DIRS
	dirsConfigDir := tmpDirs + "/ai-proxy"
	if err := os.MkdirAll(dirsConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	dirsConfigPath := dirsConfigDir + "/config.json"
	if err := os.WriteFile(dirsConfigPath, []byte(`{"source":"dirs"}`), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	os.Setenv("XDG_CONFIG_HOME", tmpHome)
	os.Setenv("XDG_CONFIG_DIRS", tmpDirs)
	defer os.Unsetenv("XDG_CONFIG_HOME")
	defer os.Unsetenv("XDG_CONFIG_DIRS")

	result := discoverConfigPath()
	if result != homeConfigPath {
		t.Errorf("discoverConfigPath() = %q, want %q (XDG_CONFIG_HOME should win)", result, homeConfigPath)
	}
}

func TestFileExists(t *testing.T) {
	t.Run("file exists", func(t *testing.T) {
		tmpFile := t.TempDir() + "/testfile"
		if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		if !fileExists(tmpFile) {
			t.Error("fileExists() = false, want true")
		}
	})

	t.Run("file does not exist", func(t *testing.T) {
		if fileExists("/nonexistent/file/path") {
			t.Error("fileExists() = true, want false")
		}
	})

	t.Run("directory exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		if fileExists(tmpDir) {
			t.Error("fileExists() = true for directory, want false")
		}
	})
}

func TestParseFlags_XDGDiscovery(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configDir := tmpDir + "/ai-proxy"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	configPath := configDir + "/config.json"
	if err := os.WriteFile(configPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Reset flag set
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"test"}
	os.Unsetenv("CONFIG_FILE")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	os.Unsetenv("XDG_CONFIG_DIRS")
	defer os.Unsetenv("XDG_CONFIG_HOME")

	flags, err := ParseFlags()

	if err != nil {
		t.Errorf("ParseFlags() unexpected error: %v", err)
	}
	if flags.ConfigFile != configPath {
		t.Errorf("ParseFlags() configFile = %q, want %q", flags.ConfigFile, configPath)
	}
}

func TestParseFlags_EnvOverridesXDG(t *testing.T) {
	// Create temp config file for XDG
	tmpDir := t.TempDir()
	configDir := tmpDir + "/ai-proxy"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	configPath := configDir + "/config.json"
	if err := os.WriteFile(configPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Reset flag set
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"test"}
	os.Setenv("CONFIG_FILE", "/env/config.yaml")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Unsetenv("CONFIG_FILE")
	defer os.Unsetenv("XDG_CONFIG_HOME")

	flags, err := ParseFlags()

	if err != nil {
		t.Errorf("ParseFlags() unexpected error: %v", err)
	}
	// CONFIG_FILE env should take priority over XDG discovery
	if flags.ConfigFile != "/env/config.yaml" {
		t.Errorf("ParseFlags() configFile = %q, want /env/config.yaml (env should override XDG)", flags.ConfigFile)
	}
}
