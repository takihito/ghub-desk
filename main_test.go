package main

import (
	"os"
	"testing"

	"ghub-desk/config"
)

func TestGetConfig(t *testing.T) {
	// Save original values
	origOrg := os.Getenv(config.EnvOrg)
	origToken := os.Getenv(config.EnvGithubToken)
	defer func() {
		os.Setenv(config.EnvOrg, origOrg)
		os.Setenv(config.EnvGithubToken, origToken)
	}()

	// Test with valid environment variables
	os.Setenv(config.EnvOrg, "test-org")
	os.Setenv(config.EnvGithubToken, "test-token")

	cfg, err := config.GetConfig()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cfg.Organization != "test-org" {
		t.Errorf("Expected organization 'test-org', got '%s'", cfg.Organization)
	}
	if cfg.GitHubToken != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", cfg.GitHubToken)
	}
}

func TestGetConfigMissingOrg(t *testing.T) {
	// Save original values
	origOrg := os.Getenv(config.EnvOrg)
	origToken := os.Getenv(config.EnvGithubToken)
	defer func() {
		os.Setenv(config.EnvOrg, origOrg)
		os.Setenv(config.EnvGithubToken, origToken)
	}()

	// Test with missing organization
	os.Unsetenv(config.EnvOrg)
	os.Setenv(config.EnvGithubToken, "test-token")

	_, err := config.GetConfig()
	if err == nil {
		t.Error("Expected error for missing organization, got nil")
	}
}

func TestGetConfigMissingToken(t *testing.T) {
	// Save original values
	origOrg := os.Getenv(config.EnvOrg)
	origToken := os.Getenv(config.EnvGithubToken)
	defer func() {
		os.Setenv(config.EnvOrg, origOrg)
		os.Setenv(config.EnvGithubToken, origToken)
	}()

	// Test with missing token
	os.Setenv(config.EnvOrg, "test-org")
	os.Unsetenv(config.EnvGithubToken)

	_, err := config.GetConfig()
	if err == nil {
		t.Error("Expected error for missing token, got nil")
	}
}
