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
			name:        "teams-users target",
			cmd:         PullCmd{CommonTargetOptions: CommonTargetOptions{TeamsUsers: "engineering"}},
			expected:    "teams-users",
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
		{
			name: "all-teams-users target",
			cmd: PullCmd{
				AllTeamsUsers: true,
			},
			expected:    "all-teams-users",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
            // Pass extra target via named TargetFlag for clarity
            result, err := tt.cmd.CommonTargetOptions.GetTarget(TargetFlag{Enabled: tt.cmd.AllTeamsUsers, Name: "all-teams-users"})

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

// Note: Testing the full Execute() function with Kong is complex due to
// Kong's parser behavior and process exit handling. The above tests cover
// the core logic without triggering Kong's parser.
