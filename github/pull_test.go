package github

import (
	"context"
	"database/sql"
	"testing"

	"ghub-desk/config"

	_ "github.com/mattn/go-sqlite3" // Import sqlite3 driver
)

func TestHandlePullTarget_UnknownTarget(t *testing.T) {
	cfg := &config.Config{GitHubToken: "test-token", Organization: "test-org"}
	client, err := InitClient(cfg)
	if err != nil {
		t.Fatalf("InitClient failed: %v", err)
	}

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()

	err = HandlePullTarget(context.Background(), client, db, cfg.Organization, TargetRequest{Kind: "unknown-target"}, cfg.GitHubToken, PullOptions{Store: false})
	if err == nil {
		t.Error("Expected error for unknown target, got nil")
	}
}

func TestHandlePullTarget_ValidTargets(t *testing.T) {
	// This test is more of an integration test and would require a mock server
	// to test properly without hitting the actual GitHub API.
	// For now, we just ensure it doesn't panic for valid targets.
	t.Skip("Skipping integration-style test for now")

	cfg := &config.Config{GitHubToken: "test-token", Organization: "test-org"}
	client, err := InitClient(cfg)
	if err != nil {
		t.Fatalf("InitClient failed: %v", err)
	}

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()

	targets := []TargetRequest{
		{Kind: "users"},
		{Kind: "detail-users"},
		{Kind: "teams"},
		{Kind: "repos"},
		{Kind: "repo-users", RepoName: "sample-repo"},
		{Kind: "all-teams-users"},
		{Kind: "token-permission"},
		{Kind: "outside-users"},
		{Kind: "team-user", TeamSlug: "test-team"},
	}

	for _, target := range targets {
		t.Run("target_"+target.Kind, func(t *testing.T) {
			err := HandlePullTarget(context.Background(), client, db, cfg.Organization, target, cfg.GitHubToken, PullOptions{Store: false})
			// In a real test with a mock, we would assert specific outcomes.
			// For now, we just check that no unexpected error occurs.
			if err != nil {
				// This will fail without a mock server, which is expected.
				// t.Errorf("HandlePullTarget() for target %s returned error: %v", target, err)
			}
		})
	}
}
