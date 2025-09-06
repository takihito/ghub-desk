package config

import (
	"fmt"
	"os"
)

const (
	// Environment variable names
	EnvOrg         = "GHUB_DESK_ORGANIZATION"
	EnvGithubToken = "GHUB_DESK_GITHUB_TOKEN"
)

// Config holds the application configuration loaded from environment variables
type Config struct {
	Organization string
	GitHubToken  string
}

// GetConfig loads and validates configuration from environment variables
func GetConfig() (*Config, error) {
	org := os.Getenv(EnvOrg)
	if org == "" {
		return nil, fmt.Errorf("environment variable %s is required", EnvOrg)
	}

	token := os.Getenv(EnvGithubToken)
	if token == "" {
		return nil, fmt.Errorf("environment variable %s is required", EnvGithubToken)
	}

	return &Config{
		Organization: org,
		GitHubToken:  token,
	}, nil
}
