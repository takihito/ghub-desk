package main

import (
	"testing"

	"ghub-desk/config"
)

func TestGetConfig_Basic(t *testing.T) {
	t.Run("basic config loading", func(t *testing.T) {
		t.Setenv("GHUB_DESK_ORGANIZATION", "test-org")
		t.Setenv("GHUB_DESK_GITHUB_TOKEN", "test-token")

		// Pass empty string to test default path logic
		_, err := config.GetConfig("")
		if err != nil {
			t.Fatalf("config.GetConfig(\"​\") failed: %v", err)
		}
	})
}