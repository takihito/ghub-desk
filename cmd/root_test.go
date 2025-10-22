package cmd

import (
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

// Note: Testing the full Execute() function with Kong is complex due to
// Kong's parser behavior and process exit handling. The above tests cover
// the core logic without triggering Kong's parser.
