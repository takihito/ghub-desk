package github

import (
	"context"
	"net/http"
	"testing"

	"ghub-desk/config"
	apigithub "github.com/google/go-github/v55/github"
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

	expectedError := "チーム/ユーザー形式が正しくありません。{team_slug}/{user_name} の形式で指定してください"
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
	err := ExecutePushAdd(context.Background(), client, org, "invalid-target", "resource-name", "")
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
	err := ExecutePushAdd(context.Background(), client, org, "team-user", "invalid-format", "")
	if err == nil {
		t.Error("Expected error for invalid team-user format, got nil")
	}

	expectedError := "チーム/ユーザー形式が正しくありません。{team_slug}/{user_name} の形式で指定してください"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}

	// Test with invalid team-user format (too many parts)
	err = ExecutePushAdd(context.Background(), client, org, "team-user", "team/user/extra", "")
	if err == nil {
		t.Error("Expected error for invalid team-user format with extra parts, got nil")
	}

	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestExecutePushRemove_InvalidOutsideUserFormat(t *testing.T) {
	cfg := &config.Config{GitHubToken: "test-token"}
	client, _ := InitClient(cfg)
	org := "test-org"

	err := ExecutePushRemove(context.Background(), client, org, "outside-user", "invalid-format")
	if err == nil {
		t.Fatal("Expected error for invalid outside-user format, got nil")
	}
	expected := "リポジトリ/ユーザー形式が正しくありません。{repository}/{user_name} の形式で指定してください"
	if err.Error() != expected {
		t.Fatalf("Expected error '%s', got '%s'", expected, err.Error())
	}

	err = ExecutePushRemove(context.Background(), client, org, "outside-user", "repo/user/extra")
	if err == nil {
		t.Fatal("Expected error for invalid outside-user format with extra parts, got nil")
	}
	if err.Error() != expected {
		t.Fatalf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestExecutePushAdd_InvalidOutsideUserFormat(t *testing.T) {
	cfg := &config.Config{GitHubToken: "test-token"}
	client, _ := InitClient(cfg)
	org := "test-org"

	err := ExecutePushAdd(context.Background(), client, org, "outside-user", "invalid-format", "")
	if err == nil {
		t.Fatal("Expected error for invalid outside-user format, got nil")
	}
	expected := "リポジトリ/ユーザー形式が正しくありません。{repository}/{user_name} の形式で指定してください"
	if err.Error() != expected {
		t.Fatalf("Expected error '%s', got '%s'", expected, err.Error())
	}

	err = ExecutePushAdd(context.Background(), client, org, "outside-user", "repo/user/extra", "")
	if err == nil {
		t.Fatal("Expected error for invalid outside-user format with extra parts, got nil")
	}
	if err.Error() != expected {
		t.Fatalf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestExecutePushRemove_InvalidReposUserFormat(t *testing.T) {
	cfg := &config.Config{GitHubToken: "test-token"}
	client, _ := InitClient(cfg)
	org := "test-org"

	err := ExecutePushRemove(context.Background(), client, org, "repos-user", "invalid-format")
	if err == nil {
		t.Fatal("Expected error for invalid repos-user format, got nil")
	}
	expected := "リポジトリ/ユーザー形式が正しくありません。{repository}/{user_name} の形式で指定してください"
	if err.Error() != expected {
		t.Fatalf("Expected error '%s', got '%s'", expected, err.Error())
	}

	err = ExecutePushRemove(context.Background(), client, org, "repos-user", "repo/user/extra")
	if err == nil {
		t.Fatal("Expected error for invalid repos-user format with extra parts, got nil")
	}
	if err.Error() != expected {
		t.Fatalf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

// Note: We cannot easily test the actual API calls without mocking the GitHub client
// or using integration tests with a real GitHub API (which would require valid tokens
// and could have side effects). The tests above focus on input validation and error handling.

func TestFormatScopePermission_Undef(t *testing.T) {
	got := FormatScopePermission(nil)
	want := "ResponseHeaderScopePermission:undef"
	if got != want {
		t.Errorf("FormatScopePermission(nil) = %q, want %q", got, want)
	}
}

func TestFormatScopePermission_WithHeaders(t *testing.T) {
	// Build a minimal github.Response with desired headers
	httpResp := &http.Response{Header: http.Header{}}
	httpResp.Header.Set("X-Accepted-OAuth-Scopes", "repo,user")
	httpResp.Header.Set("X-Accepted-GitHub-Permissions", "admin:org,write:org")

	resp := &apigithub.Response{Response: httpResp}

	got := FormatScopePermission(resp)
	want := "X-Accepted-OAuth-Scopes:repo,user, X-Accepted-GitHub-Permissions:admin:org,write:org"
	if got != want {
		t.Errorf("FormatScopePermission(headers) = %q, want %q", got, want)
	}
}
