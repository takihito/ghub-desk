package main

import (
    "os"
    "testing"

    "ghub-desk/config"
)

func TestGetConfig_Basic(t *testing.T) {
    t.Run("basic config loading", func(t *testing.T) {
        // Ensure no GitHub App envs interfere
        t.Setenv("GHUB_DESK_APP_ID", "")
        t.Setenv("GHUB_DESK_INSTALLATION_ID", "")
        t.Setenv("GHUB_DESK_PRIVATE_KEY", "")
        t.Setenv("GHUB_DESK_ORGANIZATION", "test-org")
        t.Setenv("GHUB_DESK_GITHUB_TOKEN", "test-token")

        // Use a temp file to avoid reading user's real config
        tmp := t.TempDir() + "/cfg.yaml"
        if err := os.WriteFile(tmp, []byte(""), 0644); err != nil {
            t.Fatalf("write temp cfg: %v", err)
        }
        _, err := config.GetConfig(tmp)
        if err != nil {
            t.Fatalf("config.GetConfig(\"​\") failed: %v", err)
        }
	})
}
