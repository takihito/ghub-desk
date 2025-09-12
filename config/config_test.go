package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfig(t *testing.T) {
	t.Run("loads from environment variables", func(t *testing.T) {
		t.Setenv("GHUB_DESK_ORGANIZATION", "env-org")
		t.Setenv("GHUB_DESK_GITHUB_TOKEN", "env-token")

		cfg, err := GetConfig()
		if err != nil {
			t.Fatalf("GetConfig() error = %v", err)
		}

		if cfg.Organization != "env-org" {
			t.Errorf("Organization = %v, want %v", cfg.Organization, "env-org")
		}
		if cfg.GitHubToken != "env-token" {
			t.Errorf("GitHubToken = %v, want %v", cfg.GitHubToken, "env-token")
		}
	})

	t.Run("loads from yaml and expands env", func(t *testing.T) {
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".config", AppName)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", tempDir)
		t.Setenv("MY_ORG", "yaml-org")
		t.Setenv("MY_TOKEN", "yaml-token")

		yamlContent := `
organization: ${MY_ORG}
github_token: $MY_TOKEN
`
		if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := GetConfig()
		if err != nil {
			t.Fatalf("GetConfig() error = %v", err)
		}

		if cfg.Organization != "yaml-org" {
			t.Errorf("Organization = %v, want %v", cfg.Organization, "yaml-org")
		}
		if cfg.GitHubToken != "yaml-token" {
			t.Errorf("GitHubToken = %v, want %v", cfg.GitHubToken, "yaml-token")
		}
	})

	t.Run("env overrides yaml", func(t *testing.T) {
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".config", AppName)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", tempDir)
		t.Setenv("GHUB_DESK_ORGANIZATION", "env-org-override")

		yamlContent := `organization: "yaml-org"
github_token: "yaml-token"`
		if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := GetConfig()
		if err != nil {
			t.Fatalf("GetConfig() error = %v", err)
		}

		if cfg.Organization != "env-org-override" {
			t.Errorf("Organization = %v, want %v", cfg.Organization, "env-org-override")
		}
		// This should still be from yaml as it's not overridden by env
		if cfg.GitHubToken != "yaml-token" {
			t.Errorf("GitHubToken = %v, want %v", cfg.GitHubToken, "yaml-token")
		}
	})

	t.Run("error on ambiguous config", func(t *testing.T) {
		t.Setenv("GHUB_DESK_ORGANIZATION", "test-org")
		t.Setenv("GHUB_DESK_GITHUB_TOKEN", "pat-token")
		t.Setenv("GHUB_DESK_APP_ID", "123")
		t.Setenv("GHUB_DESK_INSTALLATION_ID", "456")
		t.Setenv("GHUB_DESK_PRIVATE_KEY", "a-key")

		_, err := GetConfig()
		if err == nil {
			t.Fatal("expected error for ambiguous config, got nil")
		}
		want := "ambiguous authentication: both github_token and github_app are configured. Please choose only one"
		if err.Error() != want {
			t.Errorf("error = %q, want %q", err.Error(), want)
		}
	})

	t.Run("error on missing auth", func(t *testing.T) {
		// Unset all relevant env vars
		t.Setenv("GHUB_DESK_ORGANIZATION", "test-org")
		t.Setenv("GHUB_DESK_GITHUB_TOKEN", "")
		t.Setenv("GHUB_DESK_APP_ID", "")
		t.Setenv("GHUB_DESK_INSTALLATION_ID", "")
		t.Setenv("GHUB_DESK_PRIVATE_KEY", "")

		_, err := GetConfig()
		if err == nil {
			t.Fatal("expected error for missing auth, got nil")
		}
		want := "authentication not configured: please configure either github_token or github_app"
		if err.Error() != want {
			t.Errorf("error = %q, want %q", err.Error(), want)
		}
	})
}