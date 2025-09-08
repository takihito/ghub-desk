package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestUsage(t *testing.T) {
	// This is a simple test to ensure Usage() doesn't panic
	// We can't easily test the output without capturing stdout
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Usage() panicked: %v", r)
		}
	}()

	Usage()
}

func TestExecuteNoArgs(t *testing.T) {
	// Save original args
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	// Test with no arguments
	os.Args = []string{"ghub-desk"}

	err := Execute()
	if err == nil {
		t.Error("Expected error when no command provided, got nil")
	}

	if err.Error() != "command required" {
		t.Errorf("Expected 'command required' error, got '%s'", err.Error())
	}
}

func TestExecuteUnknownCommand(t *testing.T) {
	// Save original args
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	// Test with unknown command
	os.Args = []string{"ghub-desk", "unknown-command"}

	err := Execute()
	if err == nil {
		t.Error("Expected error for unknown command, got nil")
	}

	if err.Error() != "unknown command: unknown-command" {
		t.Errorf("Expected 'unknown command: unknown-command' error, got '%s'", err.Error())
	}
}

func TestExecuteHelpCommands(t *testing.T) {
	// Save original args
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	helpCommands := []string{"help", "-h", "--help"}

	for _, helpCmd := range helpCommands {
		t.Run("help_command_"+helpCmd, func(t *testing.T) {
			os.Args = []string{"ghub-desk", helpCmd}

			err := Execute()
			// Help commands should not return an error
			if err != nil {
				t.Errorf("Help command '%s' returned error: %v", helpCmd, err)
			}
		})
	}
}

// Note: Testing the individual command functions (PullCmd, ViewCmd, PushCmd, InitCmd)
// would require more complex setup including:
// - Mocking database connections
// - Mocking GitHub API clients
// - Setting up environment variables
// - Creating temporary files/databases
//
// These would be better suited for integration tests or would require refactoring
// the functions to accept dependencies as parameters for easier testing.

// Tests for PullCmd argument parsing
func TestPullCmdArgumentParsing(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "no target specified",
			args:        []string{"--store"},
			expectError: true,
			errorMsg:    "pull対象を指定してください",
		},
		{
			name:        "teams-users without team name",
			args:        []string{"--teams-users"},
			expectError: true,
			errorMsg:    "--teams-users にはチーム名を指定してください",
		},
		{
			name:        "teams-users with flag as team name",
			args:        []string{"--teams-users", "--store"},
			expectError: true,
			errorMsg:    "--teams-users にはチーム名を指定してください",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PullCmd(tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("PullCmd() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if tt.expectError && err != nil {
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

// Tests for ViewCmd argument parsing
func TestViewCmdArgumentParsing(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "no target specified",
			args:        []string{},
			expectError: true,
			errorMsg:    "view対象を指定してください",
		},
		{
			name:        "teams-users without team name",
			args:        []string{"--teams-users"},
			expectError: true,
			errorMsg:    "--teams-users にはチーム名を指定してください",
		},
		{
			name:        "teams-users with flag as team name",
			args:        []string{"--teams-users", "--invalid-flag"},
			expectError: true,
			errorMsg:    "--teams-users にはチーム名を指定してください",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ViewCmd(tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("ViewCmd() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if tt.expectError && err != nil {
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

// Tests for PushCmd argument parsing
func TestPushCmdArgumentParsing(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "no remove flag",
			args:        []string{"--team", "test-team"},
			expectError: true,
			errorMsg:    "pushサブコマンドを指定してください",
		},
		{
			name:        "remove flag but no target",
			args:        []string{"--remove"},
			expectError: true,
			errorMsg:    "削除対象を指定してください",
		},
		{
			name:        "remove flag with target but no resource name",
			args:        []string{"--remove", "--team"},
			expectError: true,
			errorMsg:    "削除対象を指定してください",
		},
		{
			name:        "remove flag with target and flag as resource name",
			args:        []string{"--remove", "--team", "--exec"},
			expectError: true,
			errorMsg:    "削除対象を指定してください",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PushCmd(tt.args)
			if (err != nil) != tt.expectError {
				t.Errorf("PushCmd() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if tt.expectError && err != nil {
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

// Tests for command execution with init command
func TestExecuteInitCommand(t *testing.T) {
	// Save original args
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	// Test init command - this will try to create a database file
	// We'll test that it recognizes the command, not the actual database creation
	os.Args = []string{"ghub-desk", "init"}

	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	originalDir, err := os.Getwd()

	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Init command might fail due to database setup, but should not fail on command parsing
	// The important thing is that it's recognized as a valid command
	if err := Execute(); err != nil {
		// If it's a command parsing error, that would be bad
		if strings.Contains(err.Error(), "unknown command") {
			t.Errorf("Init command should be recognized, got: %v", err)
		}
		// Other errors (like database initialization) are expected in test environment
	}
}

// Tests for valid command recognition
func TestExecuteValidCommands(t *testing.T) {
	// Save original args
	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	validCommands := []string{"pull", "view", "push", "init"}

	for _, cmd := range validCommands {
		t.Run("command_"+cmd, func(t *testing.T) {
			os.Args = []string{"ghub-desk", cmd}

			err := Execute()
			// These commands will likely fail due to missing arguments or database/API setup
			// but they should not fail with "unknown command" error
			if err != nil && strings.Contains(err.Error(), "unknown command") {
				t.Errorf("Command '%s' should be recognized as valid", cmd)
			}
		})
	}
}

// Tests for PullCmd with valid options (without actual execution)
func TestPullCmdValidOptions(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		hasStore bool
		target   string
	}{
		{
			name:     "users with store",
			args:     []string{"--store", "--users"},
			hasStore: true,
			target:   "users",
		},
		{
			name:     "detail-users without store",
			args:     []string{"--detail-users"},
			hasStore: false,
			target:   "detail-users",
		},
		{
			name:     "teams with store",
			args:     []string{"--teams", "--store"},
			hasStore: true,
			target:   "teams",
		},
		{
			name:     "repos",
			args:     []string{"--repos"},
			hasStore: false,
			target:   "repos",
		},
		{
			name:     "teams-users with team name",
			args:     []string{"--teams-users", "engineering"},
			hasStore: false,
			target:   "teams-users",
		},
		{
			name:     "all-teams-users",
			args:     []string{"--all-teams-users"},
			hasStore: false,
			target:   "all-teams-users",
		},
		{
			name:     "token-permission",
			args:     []string{"--token-permission"},
			hasStore: false,
			target:   "token-permission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These tests verify argument parsing logic, not actual execution
			// They will fail due to missing configuration, but argument parsing should work
			err := PullCmd(tt.args)

			// Should not fail due to argument parsing issues
			if err != nil {
				if strings.Contains(err.Error(), "pull対象を指定してください") {
					t.Errorf("Target should be parsed correctly for %s", tt.name)
				}
				if strings.Contains(err.Error(), "--teams-users にはチーム名を指定してください") {
					t.Errorf("Team name should be parsed correctly for %s", tt.name)
				}
			}
		})
	}
}

// Tests for ViewCmd with valid options (without actual execution)
func TestViewCmdValidOptions(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		target string
	}{
		{
			name:   "users",
			args:   []string{"--users"},
			target: "users",
		},
		{
			name:   "detail-users",
			args:   []string{"--detail-users"},
			target: "detail-users",
		},
		{
			name:   "teams",
			args:   []string{"--teams"},
			target: "teams",
		},
		{
			name:   "repos",
			args:   []string{"--repos"},
			target: "repos",
		},
		{
			name:   "teams-users with team name",
			args:   []string{"--teams-users", "engineering"},
			target: "teams-users",
		},
		{
			name:   "token-permission",
			args:   []string{"--token-permission"},
			target: "token-permission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These tests verify argument parsing logic
			// They will fail due to database initialization, but argument parsing should work
			err := ViewCmd(tt.args)

			// Should not fail due to argument parsing issues
			if err != nil {
				if strings.Contains(err.Error(), "view対象を指定してください") {
					t.Errorf("Target should be parsed correctly for %s", tt.name)
				}
				if strings.Contains(err.Error(), "--teams-users にはチーム名を指定してください") {
					t.Errorf("Team name should be parsed correctly for %s", tt.name)
				}
			}
		})
	}
}

// Tests for PushCmd with valid options (without actual execution)
func TestPushCmdValidOptions(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		target       string
		resourceName string
		exec         bool
	}{
		{
			name:         "remove team without exec",
			args:         []string{"--remove", "--team", "old-team"},
			target:       "team",
			resourceName: "old-team",
			exec:         false,
		},
		{
			name:         "remove user with exec",
			args:         []string{"--remove", "--user", "old-user", "--exec"},
			target:       "user",
			resourceName: "old-user",
			exec:         true,
		},
		{
			name:         "remove team-user",
			args:         []string{"--remove", "--team-user", "team/user"},
			target:       "team-user",
			resourceName: "team/user",
			exec:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These tests verify argument parsing logic
			// They will fail due to missing configuration, but argument parsing should work
			err := PushCmd(tt.args)

			// Should not fail due to argument parsing issues
			if err != nil {
				if strings.Contains(err.Error(), "pushサブコマンドを指定してください") {
					t.Errorf("Remove flag should be parsed correctly for %s", tt.name)
				}
				if strings.Contains(err.Error(), "削除対象を指定してください") {
					t.Errorf("Target and resource should be parsed correctly for %s", tt.name)
				}
			}
		})
	}
}
