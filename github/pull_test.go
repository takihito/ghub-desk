package github

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestHandlePullTarget_UnknownTarget(t *testing.T) {
	ctx := context.Background()
	client := InitClient("test-token")

	// Create in-memory database for testing
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	org := "test-org"
	token := "test-token"
	storeData := true

	// Test with unknown target
	err = HandlePullTarget(ctx, client, db, org, "unknown-target", token, storeData, DefaultSleep)
	if err == nil {
		t.Error("Expected error for unknown target, got nil")
	}

	expectedError := "unknown target: unknown-target"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestHandlePullTarget_ValidTargets(t *testing.T) {
	ctx := context.Background()
	client := InitClient("test-token")

	// Note: These tests will fail with actual API calls due to invalid token/org
	// but they test the target parsing logic

	validTargets := []string{
		"users",
		"detail-users",
		"teams",
		"repos",
		"all-teams-users",
		"token-permission",
		"outside-users",
		"test-team/users",
	}

	for _, target := range validTargets {
		t.Run("target_"+target, func(t *testing.T) {
			// Create fresh in-memory database for each test
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatalf("Failed to open test database: %v", err)
			}
			defer db.Close()

			org := "test-org"
			token := "test-token"
			storeData := false // Set to false to avoid actual storage

			// This will likely fail due to invalid credentials, but should not fail on target parsing
			err = HandlePullTarget(ctx, client, db, org, target, token, storeData, DefaultSleep)

			// We expect these to fail with API errors, not parsing errors
			if err != nil && err.Error() == "unknown target: "+target {
				t.Errorf("Target '%s' should be recognized, but got parsing error", target)
			}

			// If it's an API error (not parsing error), that's expected with fake credentials
			// The important thing is that the target was recognized
		})
	}
}

// Note: Testing the actual API calls (PullUsers, PullTeams, etc.) would require:
// - Valid GitHub API credentials
// - Actual organization data
// - Network connectivity
// - Potentially creating/modifying real GitHub resources
//
// These are better suited for integration tests rather than unit tests.
// The functions could be refactored to accept interfaces for easier mocking.
