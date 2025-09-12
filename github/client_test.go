package github

import (
	"testing"

	"ghub-desk/config"
)

func TestInitClient(t *testing.T) {
	t.Run("with PAT", func(t *testing.T) {
		cfg := &config.Config{
			GitHubToken: "test-pat-token",
		}
		client, err := InitClient(cfg)
		if err != nil {
			t.Fatalf("InitClient() with PAT error = %v", err)
		}
		if client == nil {
			t.Error("InitClient() with PAT returned nil client")
		}
	})

	t.Run("with empty config", func(t *testing.T) {
		cfg := &config.Config{}
		_, err := InitClient(cfg)
		if err == nil {
			t.Fatal("InitClient() with empty config should have returned an error")
		}
	})

	// Note: Testing GitHub App auth would require a valid private key and is more complex.
	// We rely on the config validation to ensure the app config is present.
}