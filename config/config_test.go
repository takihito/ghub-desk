package config

import (
	"os"
	"testing"
)

func TestGetConfig(t *testing.T) {
	// Save original values
	origOrg := os.Getenv(EnvOrg)
	origToken := os.Getenv(EnvGithubToken)
	defer func() {
		os.Setenv(EnvOrg, origOrg)
		os.Setenv(EnvGithubToken, origToken)
	}()

	// Test with valid environment variables
	os.Setenv(EnvOrg, "test-org")
	os.Setenv(EnvGithubToken, "test-token")

	cfg, err := GetConfig()
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
	origOrg := os.Getenv(EnvOrg)
	origToken := os.Getenv(EnvGithubToken)
	defer func() {
		os.Setenv(EnvOrg, origOrg)
		os.Setenv(EnvGithubToken, origToken)
	}()

	// Test with missing organization
	os.Unsetenv(EnvOrg)
	os.Setenv(EnvGithubToken, "test-token")

	_, err := GetConfig()
	if err == nil {
		t.Error("Expected error for missing organization, got nil")
	}

	expectedError := "environment variable GHUB_DESK_ORGANIZATION is required"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestGetConfigMissingToken(t *testing.T) {
	// Save original values
	origOrg := os.Getenv(EnvOrg)
	origToken := os.Getenv(EnvGithubToken)
	defer func() {
		os.Setenv(EnvOrg, origOrg)
		os.Setenv(EnvGithubToken, origToken)
	}()

	// Test with missing token
	os.Setenv(EnvOrg, "test-org")
	os.Unsetenv(EnvGithubToken)

	_, err := GetConfig()
	if err == nil {
		t.Error("Expected error for missing token, got nil")
	}

	expectedError := "environment variable GHUB_DESK_GITHUB_TOKEN is required"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestGetConfigBothMissing(t *testing.T) {
	// Save original values
	origOrg := os.Getenv(EnvOrg)
	origToken := os.Getenv(EnvGithubToken)
	defer func() {
		os.Setenv(EnvOrg, origOrg)
		os.Setenv(EnvGithubToken, origToken)
	}()

	// Test with both missing (organization is checked first)
	os.Unsetenv(EnvOrg)
	os.Unsetenv(EnvGithubToken)

	_, err := GetConfig()
	if err == nil {
		t.Error("Expected error for missing both values, got nil")
	}

	expectedError := "environment variable GHUB_DESK_ORGANIZATION is required"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestConstants(t *testing.T) {
	if EnvOrg != "GHUB_DESK_ORGANIZATION" {
		t.Errorf("Expected EnvOrg to be 'GHUB_DESK_ORGANIZATION', got '%s'", EnvOrg)
	}
	if EnvGithubToken != "GHUB_DESK_GITHUB_TOKEN" {
		t.Errorf("Expected EnvGithubToken to be 'GHUB_DESK_GITHUB_TOKEN', got '%s'", EnvGithubToken)
	}
}
