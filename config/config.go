package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	AppName = "ghub-desk"
)

// Config holds the application configuration
type Config struct {
	Organization string    `yaml:"organization"`
	GitHubToken  string    `yaml:"github_token"`
	GitHubApp    GitHubApp `yaml:"github_app"`
}

// GitHubApp holds GitHub App specific configuration
type GitHubApp struct {
	AppID          int64  `yaml:"app_id"`
	InstallationID int64  `yaml:"installation_id"`
	PrivateKey     string `yaml:"private_key"`
}

// GetConfig loads configuration from file and environment variables
func GetConfig(customPath string) (*Config, error) {
	cfg := &Config{}

	// 1. Load from YAML file
	configPath, err := resolveConfigPath(customPath)
	if err != nil {
		return nil, err
	}

	if configPath != "" {
		file, err := os.ReadFile(configPath)
		if err == nil { // File exists and is readable
			// Expand env vars before unmarshalling
			expandedFile := os.ExpandEnv(string(file))
			if err := yaml.Unmarshal([]byte(expandedFile), cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
			}
		} else if !os.IsNotExist(err) {
			// File exists but is not readable for some reason
			return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
		}
	}

	// 2. Override with environment variables
	if org := os.Getenv("GHUB_DESK_ORGANIZATION"); org != "" {
		cfg.Organization = org
	}
	if token := os.Getenv("GHUB_DESK_GITHUB_TOKEN"); token != "" {
		cfg.GitHubToken = token
	}
	if appID := os.Getenv("GHUB_DESK_APP_ID"); appID != "" {
		_, err := fmt.Sscanf(appID, "%d", &cfg.GitHubApp.AppID)
		if err != nil {
			return nil, fmt.Errorf("invalid GHUB_DESK_APP_ID: %w", err)
		}
	}
	if instID := os.Getenv("GHUB_DESK_INSTALLATION_ID"); instID != "" {
		_, err := fmt.Sscanf(instID, "%d", &cfg.GitHubApp.InstallationID)
		if err != nil {
			return nil, fmt.Errorf("invalid GHUB_DESK_INSTALLATION_ID: %w", err)
		}
	}
	if key := os.Getenv("GHUB_DESK_PRIVATE_KEY"); key != "" {
		cfg.GitHubApp.PrivateKey = key
	}

	// 3. Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, err
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

	return nil
}

func resolveConfigPath(customPath string) (string, error) {
	if customPath != "" {
		return customPath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}
	return filepath.Join(home, ".config", AppName, "config.yaml"), nil
}
