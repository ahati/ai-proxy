// Package config provides configuration loading from environment variables and command-line flags.
// Configuration values are loaded with precedence: flags > environment variables > defaults.
package config

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
)

// ErrConfigFileRequired is returned when no config file path is provided.
var ErrConfigFileRequired = errors.New("config file required: use --config-file or CONFIG_FILE environment variable")

// CLIFlags holds the parsed command-line flags.
type CLIFlags struct {
	ConfigFile            string
	SSELogDir             string
	Port                  string
	ConversationStoreSize int
	ConversationStoreTTL  string
}

// ParseFlags parses CLI flags and returns the parsed flags.
// Priority for config file: --config-file flag > CONFIG_FILE env var > XDG discovery > error
// Priority for other flags: flag > environment variable > default
//
// @return CLIFlags - the parsed flags
// @return error - ErrConfigFileRequired if no config file is found in any location
// @post flag.Parse() has been called, consuming command-line arguments
func ParseFlags() (CLIFlags, error) {
	configFilePath := flag.String("config-file", "", "Path to configuration file")
	sseLogDir := flag.String("sse-log-dir", "", "Directory for SSE request/response logging")
	port := flag.String("port", "", "Server port (default: 8080)")
	conversationStoreSize := flag.Int("conversation-store-size", 0, "Max conversations in memory (default: 1000)")
	conversationStoreTTL := flag.String("conversation-store-ttl", "", "Conversation TTL duration (default: 24h)")

	flag.Parse()

	flags := CLIFlags{
		SSELogDir:             *sseLogDir,
		Port:                  *port,
		ConversationStoreSize: *conversationStoreSize,
		ConversationStoreTTL:  *conversationStoreTTL,
	}

	// Priority 1: explicit --config-file flag
	if *configFilePath != "" {
		flags.ConfigFile = *configFilePath
		return flags, nil
	}

	// Priority 2: CONFIG_FILE environment variable
	if envPath, ok := os.LookupEnv("CONFIG_FILE"); ok && envPath != "" {
		flags.ConfigFile = envPath
		return flags, nil
	}

	// Priority 3: XDG discovery
	discoveredPath := discoverConfigPath()
	if discoveredPath != "" {
		flags.ConfigFile = discoveredPath
		return flags, nil
	}

	return flags, ErrConfigFileRequired
}

// discoverConfigPath searches for config.json in XDG standard locations.
// Returns the first found path, or empty string if none found.
//
// Search order:
//  1. $XDG_CONFIG_HOME/ai-proxy/config.json (if XDG_CONFIG_HOME is set)
//  2. $HOME/.config/ai-proxy/config.json (if XDG_CONFIG_HOME is not set)
//  3. Each path in $XDG_CONFIG_DIRS/ai-proxy/config.json (default: /etc/xdg)
//
// @return string - path to config file, or empty string if not found
func discoverConfigPath() string {
	const configSubdir = "ai-proxy"
	const configFilename = "config.json"

	// Check XDG_CONFIG_HOME (or HOME/.config fallback)
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			xdgConfigHome = filepath.Join(homeDir, ".config")
		}
	}

	if xdgConfigHome != "" {
		configPath := filepath.Join(xdgConfigHome, configSubdir, configFilename)
		if fileExists(configPath) {
			return configPath
		}
	}

	// Check XDG_CONFIG_DIRS (colon-separated, default /etc/xdg)
	xdgConfigDirs := os.Getenv("XDG_CONFIG_DIRS")
	if xdgConfigDirs == "" {
		xdgConfigDirs = "/etc/xdg"
	}

	for _, dir := range strings.Split(xdgConfigDirs, ":") {
		if dir == "" {
			continue
		}
		configPath := filepath.Join(dir, configSubdir, configFilename)
		if fileExists(configPath) {
			return configPath
		}
	}

	return ""
}

// fileExists returns true if the path exists and is a regular file.
//
// @param path - the path to check
// @return bool - true if file exists and is readable, false otherwise
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
