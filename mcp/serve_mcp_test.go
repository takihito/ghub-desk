package mcp

import "testing"

func TestResolvePushAddInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        PushAddIn
		expTarget string
		expValue  string
		expPerm   string
		expectErr bool
	}{
		{
			name:      "team user",
			in:        PushAddIn{TeamUser: " team-one/user-one "},
			expTarget: "team-user",
			expValue:  "team-one/user-one",
		},
		{
			name:      "outside collaborator no permission",
			in:        PushAddIn{OutsideUser: " repo-one/user-two "},
			expTarget: "outside-user",
			expValue:  "repo-one/user-two",
		},
		{
			name:      "outside collaborator alias permission",
			in:        PushAddIn{OutsideUser: "repo-two/user-three", Permission: "read"},
			expTarget: "outside-user",
			expValue:  "repo-two/user-three",
			expPerm:   "pull",
		},
		{
			name:      "team user with permission should error",
			in:        PushAddIn{TeamUser: "team-one/user-one", Permission: "push"},
			expectErr: true,
		},
		{
			name:      "outside collaborator invalid permission",
			in:        PushAddIn{OutsideUser: "repo-one/user", Permission: "owner"},
			expectErr: true,
		},
		{
			name:      "both targets specified",
			in:        PushAddIn{TeamUser: "team-one/user-one", OutsideUser: "repo/user"},
			expectErr: true,
		},
		{
			name:      "empty",
			in:        PushAddIn{TeamUser: ""},
			expectErr: true,
		},
		{
			name:      "invalid format",
			in:        PushAddIn{TeamUser: "invalid"},
			expectErr: true,
		},
		{
			name:      "invalid user",
			in:        PushAddIn{TeamUser: "team-one/user!"},
			expectErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			target, value, perm, err := resolvePushAddInput(tc.in)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if target != tc.expTarget {
				t.Fatalf("unexpected target: got %q, want %q", target, tc.expTarget)
			}
			if value != tc.expValue {
				t.Fatalf("unexpected value: got %q, want %q", value, tc.expValue)
			}
			if perm != tc.expPerm {
				t.Fatalf("unexpected permission: got %q, want %q", perm, tc.expPerm)
			}
		})
	}
}

func TestResolvePushRemoveInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        PushRemoveIn
		expTarget string
		expValue  string
		expectErr bool
	}{
		{
			name:      "team removal",
			in:        PushRemoveIn{Team: "dev-team"},
			expTarget: "team",
			expValue:  "dev-team",
		},
		{
			name:      "outside collaborator removal",
			in:        PushRemoveIn{OutsideUser: "repo-one/user-one"},
			expTarget: "outside-user",
			expValue:  "repo-one/user-one",
		},
		{
			name:      "repository collaborator removal",
			in:        PushRemoveIn{ReposUser: " repo-two/user-two "},
			expTarget: "repos-user",
			expValue:  "repo-two/user-two",
		},
		{
			name:      "no selection",
			in:        PushRemoveIn{},
			expectErr: true,
		},
		{
			name:      "multiple selections",
			in:        PushRemoveIn{Team: "t", User: "u"},
			expectErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			target, value, err := resolvePushRemoveInput(tc.in)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if target != tc.expTarget {
				t.Fatalf("unexpected target: got %q, want %q", target, tc.expTarget)
			}
			if value != tc.expValue {
				t.Fatalf("unexpected value: got %q, want %q", value, tc.expValue)
			}
		})
	}
}
