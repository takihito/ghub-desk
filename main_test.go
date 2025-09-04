package main

import (
	"os"
	"testing"

	"github.com/google/go-github/v55/github"
)

func TestGetConfig(t *testing.T) {
	// Save original values
	origOrg := os.Getenv(envOrg)
	origToken := os.Getenv(envGithubToken)
	defer func() {
		os.Setenv(envOrg, origOrg)
		os.Setenv(envGithubToken, origToken)
	}()

	// Test with valid environment variables
	os.Setenv(envOrg, "test-org")
	os.Setenv(envGithubToken, "test-token")

	config, err := getConfig()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if config.Organization != "test-org" {
		t.Errorf("Expected organization 'test-org', got '%s'", config.Organization)
	}
	if config.GitHubToken != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", config.GitHubToken)
	}
}

func TestGetConfigMissingOrg(t *testing.T) {
	// Save original values
	origOrg := os.Getenv(envOrg)
	origToken := os.Getenv(envGithubToken)
	defer func() {
		os.Setenv(envOrg, origOrg)
		os.Setenv(envGithubToken, origToken)
	}()

	// Test with missing organization
	os.Unsetenv(envOrg)
	os.Setenv(envGithubToken, "test-token")

	_, err := getConfig()
	if err == nil {
		t.Error("Expected error for missing organization, got nil")
	}
}

func TestGetConfigMissingToken(t *testing.T) {
	// Save original values
	origOrg := os.Getenv(envOrg)
	origToken := os.Getenv(envGithubToken)
	defer func() {
		os.Setenv(envOrg, origOrg)
		os.Setenv(envGithubToken, origToken)
	}()

	// Test with missing token
	os.Setenv(envOrg, "test-org")
	os.Unsetenv(envGithubToken)

	_, err := getConfig()
	if err == nil {
		t.Error("Expected error for missing token, got nil")
	}
}

func TestUsage(t *testing.T) {
	// usage() prints help, just check it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("usage panicked: %v", r)
		}
	}()
	usage()
}

func TestFormatTime(t *testing.T) {
	// Test with zero time
	var zeroTime github.Timestamp
	result := formatTime(zeroTime)
	if result != "" {
		t.Errorf("Expected empty string for zero time, got '%s'", result)
	}
}
