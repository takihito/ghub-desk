package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfig(t *testing.T) {
	t.Run("loads from environment variables", func(t *testing.T) {
		// Ensure no GitHub App envs interfere
		t.Setenv("GHUB_DESK_APP_ID", "")
		t.Setenv("GHUB_DESK_INSTALLATION_ID", "")
		t.Setenv("GHUB_DESK_PRIVATE_KEY", "")
		t.Setenv("GHUB_DESK_ORGANIZATION", "env-org")
		t.Setenv("GHUB_DESK_GITHUB_TOKEN", "env-token")

		// Use a temp file to avoid reading user's real config
		tempFile := filepath.Join(t.TempDir(), "cfg.yaml")
		if err := os.WriteFile(tempFile, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		cfg, err := GetConfig(tempFile)
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

	t.Run("loads from custom path yaml", func(t *testing.T) {
		// Ensure no GitHub App envs interfere
		t.Setenv("GHUB_DESK_APP_ID", "")
		t.Setenv("GHUB_DESK_INSTALLATION_ID", "")
		t.Setenv("GHUB_DESK_PRIVATE_KEY", "")
		tempDir := t.TempDir()
		customPath := filepath.Join(tempDir, "custom_config.yaml")
		t.Setenv("MY_ORG", "custom-path-org")
		t.Setenv("MY_TOKEN", "custom-path-token")

		yamlContent := `
organization: ${MY_ORG}
github_token: $MY_TOKEN
`
		if err := os.WriteFile(customPath, []byte(yamlContent), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := GetConfig(customPath)
		if err != nil {
			t.Fatalf("GetConfig() with custom path error = %v", err)
		}

		if cfg.Organization != "custom-path-org" {
			t.Errorf("Organization = %v, want %v", cfg.Organization, "custom-path-org")
		}
		if cfg.GitHubToken != "custom-path-token" {
			t.Errorf("GitHubToken = %v, want %v", cfg.GitHubToken, "custom-path-token")
		}
	})

	t.Run("error on ambiguous config", func(t *testing.T) {
		t.Setenv("GHUB_DESK_ORGANIZATION", "test-org")
		t.Setenv("GHUB_DESK_GITHUB_TOKEN", "pat-token")
		t.Setenv("GHUB_DESK_APP_ID", "123")
		t.Setenv("GHUB_DESK_INSTALLATION_ID", "456")
		t.Setenv("GHUB_DESK_PRIVATE_KEY", "a-key")

		_, err := GetConfig("")
		if err == nil {
			t.Fatal("expected error for ambiguous config, got nil")
		}
		want := "ambiguous authentication: both github_token and github_app are configured. Please choose only one"
		if err.Error() != want {
			t.Errorf("error = %q, want %q", err.Error(), want)
		}
	})
}
