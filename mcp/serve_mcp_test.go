//go:build mcp_sdk

package mcp

import "testing"

func TestResolvePushAddInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        PushAddIn
		expTarget string
		expValue  string
		expectErr bool
	}{
		{
			name:      "valid",
			in:        PushAddIn{TeamUser: " team-one/user-one "},
			expTarget: "team-user",
			expValue:  "team-one/user-one",
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
			target, value, err := resolvePushAddInput(tc.in)
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
