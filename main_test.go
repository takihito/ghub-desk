package main

import (
	"testing"

	"ghub-desk/config"
)

func TestGetConfig_Basic(t *testing.T) {
	// This test is now covered in detail in config/config_test.go
	// We can keep a very basic test here to ensure it doesn't panic.
	t.Run("basic config loading", func(t *testing.T) {
		t.Setenv("GHUB_DESK_ORGANIZATION", "test-org")
		t.Setenv("GHUB_DESK_GITHUB_TOKEN", "test-token")

		_, err := config.GetConfig()
		if err != nil {
			t.Fatalf("config.GetConfig() failed: %v", err)
		}
	})
}
