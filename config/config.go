package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	AppName = "ghub-desk"
)

// Debug enables verbose logs within the config package.
var Debug bool

// Config holds the application configuration
type Config struct {
	Organization string    `yaml:"organization"`
	GitHubToken  string    `yaml:"github_token"`
	GitHubApp    GitHubApp `yaml:"github_app"`
	MCP          MCPConfig `yaml:"mcp"`
	DatabasePath string    `yaml:"database_path"`
}

// GitHubApp holds GitHub App specific configuration
type GitHubApp struct {
	AppID          int64  `yaml:"app_id"`
	InstallationID int64  `yaml:"installation_id"`
	PrivateKey     string `yaml:"private_key"`
}

// MCPConfig controls MCP server permissions
type MCPConfig struct {
	AllowPull  bool `yaml:"allow_pull"`
	AllowWrite bool `yaml:"allow_write"`
}

// GetConfig loads configuration from file and environment variables
func GetConfig(customPath string) (*Config, error) {
	cfg, err := LoadConfigNoValidate(customPath)
	if err != nil {
		return nil, err
	}
	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadConfigNoValidate loads configuration from file and environment variables without validation.
// - If a custom path was provided and the file is missing or invalid, an error is returned.
// - If no custom path was provided and the default file is missing, it is ignored.
func LoadConfigNoValidate(customPath string) (*Config, error) {
	cfg := &Config{}

	// 1. Load from YAML file
	isCustom := customPath != ""
	configPath, err := ResolveConfigPath(customPath)
	if err != nil {
		return nil, err
	}

	file, rerr := os.ReadFile(configPath)
	switch {
	case rerr == nil:
		expandedFile := os.ExpandEnv(string(file))
		if err := yaml.Unmarshal([]byte(expandedFile), cfg); err != nil {
			if isCustom {
				return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
			}
		}
	case os.IsNotExist(rerr):
		if isCustom {
			return nil, fmt.Errorf("--config file not found: %s", configPath)
		}
	default:
		if isCustom {
			return nil, fmt.Errorf("failed to read config file %s: %w", configPath, rerr)
		}
	}

	// 2. Overlay with environment variables
	if org := os.Getenv("GHUB_DESK_ORGANIZATION"); org != "" {
		cfg.Organization = org
	}
	if token := os.Getenv("GHUB_DESK_GITHUB_TOKEN"); token != "" {
		cfg.GitHubToken = token
	}
	if dbp := os.Getenv("GHUB_DESK_DB_PATH"); dbp != "" {
		cfg.DatabasePath = dbp
	}
	if appID := os.Getenv("GHUB_DESK_APP_ID"); appID != "" {
		v, err := strconv.ParseInt(appID, 10, 64)
		if err == nil { // best-effort for non-validating load
			cfg.GitHubApp.AppID = v
		}
	}
	if instID := os.Getenv("GHUB_DESK_INSTALLATION_ID"); instID != "" {
		v, err := strconv.ParseInt(instID, 10, 64)
		if err == nil { // best-effort
			cfg.GitHubApp.InstallationID = v
		}
	}
	if key := os.Getenv("GHUB_DESK_PRIVATE_KEY"); key != "" {
		cfg.GitHubApp.PrivateKey = key
	}

	return cfg, nil
}

func validateConfig(cfg *Config) error {
	patConfigured := cfg.GitHubToken != ""
	appConfigured := cfg.GitHubApp.AppID != 0 && cfg.GitHubApp.InstallationID != 0 && cfg.GitHubApp.PrivateKey != ""

	if cfg.Organization == "" {
		return fmt.Errorf("organization is not set. Please set GHUB_DESK_ORGANIZATION or add to config file")
	}

	if patConfigured && appConfigured {
		return fmt.Errorf("ambiguous authentication: both github_token and github_app are configured. Please choose only one")
	}

	if !patConfigured && !appConfigured {
		return fmt.Errorf("authentication not configured: please configure either github_token or github_app")
	}

	// Validate database path from file/env to avoid traversal patterns
	if cfg.DatabasePath != "" {
		cleaned := filepath.Clean(cfg.DatabasePath)
		// basic sanity: disallow NUL and invalid basenames
		if strings.ContainsRune(cleaned, '\x00') || filepath.Base(cleaned) == "." || filepath.Base(cleaned) == ".." {
			return fmt.Errorf("invalid database_path: contains invalid characters or basename")
		}

		if filepath.IsAbs(cleaned) {
			cfg.DatabasePath = cleaned
		} else {
			// For relative paths, ensure it stays within the current working directory.
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("could not get working directory: %w", err)
			}
			abs := filepath.Clean(filepath.Join(wd, cleaned))
			rel, err := filepath.Rel(wd, abs)
			if err != nil {
				return fmt.Errorf("invalid database_path: %w", err)
			}
			if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				return fmt.Errorf("invalid database_path: must reside within current working directory")
			}
			cfg.DatabasePath = abs
		}
	}

	return nil
}

// ResolveConfigPath returns the config file path given a custom path or the default location.
func ResolveConfigPath(customPath string) (string, error) {
	if customPath != "" {
		return customPath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}
	return filepath.Join(home, ".config", AppName, "config.yaml"), nil
}
