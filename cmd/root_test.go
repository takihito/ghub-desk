package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVersionInfo tests the version information setting
func TestVersionInfo(t *testing.T) {
	testVersion := "v1.0.0"
	testCommit := "abc123"
	testDate := "2025-01-01"

	SetVersionInfo(testVersion, testCommit, testDate)

	if appVersion != testVersion {
		t.Errorf("Expected version %s, got %s", testVersion, appVersion)
	}
	if appCommit != testCommit {
		t.Errorf("Expected commit %s, got %s", testCommit, appCommit)
	}
	if appDate != testDate {
		t.Errorf("Expected date %s, got %s", testDate, appDate)
	}
}

// TestPullCmdGetTarget tests the target parsing logic
func TestPullCmdGetTarget(t *testing.T) {
	tests := []struct {
		name        string
		cmd         PullCmd
		expected    string
		expectError bool
	}{
		{
			name:        "users target",
			cmd:         PullCmd{CommonTargetOptions: CommonTargetOptions{Users: true}},
			expected:    "users",
			expectError: false,
		},
		{
			name:        "team-user target",
			cmd:         PullCmd{CommonTargetOptions: CommonTargetOptions{TeamUser: "engineering"}},
			expected:    "team-user",
			expectError: false,
		},
		{
			name:        "repos-users target",
			cmd:         PullCmd{CommonTargetOptions: CommonTargetOptions{RepoUsers: "repo-name"}},
			expected:    "repos-users",
			expectError: false,
		},
		{
			name:        "all-repos-users target",
			cmd:         PullCmd{CommonTargetOptions: CommonTargetOptions{AllReposUsers: true}},
			expected:    "all-repos-users",
			expectError: false,
		},
		{
			name:        "user-repos target",
			cmd:         PullCmd{CommonTargetOptions: CommonTargetOptions{UserRepos: "octocat"}},
			expected:    "user-repos",
			expectError: false,
		},
		{
			name:        "user target",
			cmd:         PullCmd{CommonTargetOptions: CommonTargetOptions{User: "octocat"}},
			expected:    "user",
			expectError: false,
		},
		{
			name:        "user-teams target",
			cmd:         PullCmd{CommonTargetOptions: CommonTargetOptions{UserTeams: "octocat"}},
			expected:    "user-teams",
			expectError: false,
		},
		{
			name:        "team-repos target",
			cmd:         PullCmd{CommonTargetOptions: CommonTargetOptions{TeamRepos: "team-slug"}},
			expected:    "team-repos",
			expectError: false,
		},
		{
			name:        "all-repos-teams target",
			cmd:         PullCmd{CommonTargetOptions: CommonTargetOptions{AllReposTeams: true}},
			expected:    "all-repos-teams",
			expectError: false,
		},
		{
			name:        "all-teams-users target",
			cmd:         PullCmd{CommonTargetOptions: CommonTargetOptions{AllTeamsUsers: true}},
			expected:    "all-teams-users",
			expectError: false,
		},
		{
			name:        "no target",
			cmd:         PullCmd{},
			expected:    "",
			expectError: true,
		},
		{
			name: "multiple targets",
			cmd: PullCmd{
				CommonTargetOptions: CommonTargetOptions{Users: true, Teams: true},
			},
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.cmd.CommonTargetOptions.GetTarget()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestParseTeamUsersPath(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{"valid", "team-slug/users", "team-slug", false},
		{"missing suffix", "team-slug", "", true},
		{"wrong suffix", "team/userss", "", true},
		{"empty slug", "/users", "", true},
		{"extra part", "team/slug/users", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := parseTeamUsersPath(tt.input)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error for input %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if actual != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}

func TestExecuteCreatesLogFileWithCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "cli.log")

	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"ghub-desk", "--log-path", logPath, "version"}

	writer, cleanup, err := Execute()
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if cleanup == nil {
		t.Fatalf("expected cleanup function, got nil")
	}

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected log file permissions 0o600, got %v", info.Mode().Perm())
	}

	file, ok := writer.(*os.File)
	if !ok {
		t.Fatalf("expected writer to be *os.File, got %T", writer)
	}

	cleanup()
	if _, err := file.Write([]byte("x")); err == nil {
		t.Fatalf("expected write to closed log file to fail")
	}
}

func TestExecuteLogFileOpenError(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := tmpDir // OpenFile should fail on directory

	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"ghub-desk", "--log-path", logPath, "version"}

	writer, cleanup, err := Execute()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to open log file") {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleanup != nil {
		t.Fatalf("cleanup should be nil when log open fails")
	}
	if writer != os.Stderr {
		t.Fatalf("expected logWriter to be stderr on failure")
	}
}

// Note: Execute is exercised with the version command to avoid Kong exits;
// more complex parser behaviors remain outside these tests.
