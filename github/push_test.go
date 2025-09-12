package github

import (
	"context"
	"testing"

	"ghub-desk/config"
)

func TestExecutePushRemove_InvalidTarget(t *testing.T) {
	cfg := &config.Config{GitHubToken: "test-token"}
	client, _ := InitClient(cfg)
	org := "test-org"

	// Test with invalid target
	err := ExecutePushRemove(context.Background(), client, org, "invalid-target", "resource-name")
	if err == nil {
		t.Error("Expected error for invalid target, got nil")
	}

	expectedError := "サポートされていない削除対象: invalid-target"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestExecutePushRemove_InvalidTeamUserFormat(t *testing.T) {
	cfg := &config.Config{GitHubToken: "test-token"}
	client, _ := InitClient(cfg)
	org := "test-org"

	// Test with invalid team-user format (no slash)
	err := ExecutePushRemove(context.Background(), client, org, "team-user", "invalid-format")
	if err == nil {
		t.Error("Expected error for invalid team-user format, got nil")
	}

	expectedError := "チーム/ユーザー形式が正しくありません。{team_name}/{user_name} の形式で指定してください"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}

	// Test with invalid team-user format (too many parts)
	err = ExecutePushRemove(context.Background(), client, org, "team-user", "team/user/extra")
	if err == nil {
		t.Error("Expected error for invalid team-user format with extra parts, got nil")
	}

	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestExecutePushAdd_InvalidTarget(t *testing.T) {
	cfg := &config.Config{GitHubToken: "test-token"}
	client, _ := InitClient(cfg)
	org := "test-org"

	// Test with invalid target
	err := ExecutePushAdd(context.Background(), client, org, "invalid-target", "resource-name")
	if err == nil {
		t.Error("Expected error for invalid target, got nil")
	}

	expectedError := "サポートされていない追加対象: invalid-target"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestExecutePushAdd_InvalidTeamUserFormat(t *testing.T) {
	cfg := &config.Config{GitHubToken: "test-token"}
	client, _ := InitClient(cfg)
	org := "test-org"

	// Test with invalid team-user format (no slash)
	err := ExecutePushAdd(context.Background(), client, org, "team-user", "invalid-format")
	if err == nil {
		t.Error("Expected error for invalid team-user format, got nil")
	}

	expectedError := "チーム/ユーザー形式が正しくありません。{team_name}/{user_name} の形式で指定してください"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}

	// Test with invalid team-user format (too many parts)
	err = ExecutePushAdd(context.Background(), client, org, "team-user", "team/user/extra")
	if err == nil {
		t.Error("Expected error for invalid team-user format with extra parts, got nil")
	}

	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// Note: We cannot easily test the actual API calls without mocking the GitHub client
// or using integration tests with a real GitHub API (which would require valid tokens
// and could have side effects). The tests above focus on input validation and error handling.