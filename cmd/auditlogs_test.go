package cmd

import (
	"strings"
	"testing"
	"time"

	"ghub-desk/auditlog"
)

func TestBuildAuditLogCreatedClause(t *testing.T) {
	now := time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		raw       string
		want      string
		expectErr bool
	}{
		{
			name: "default is 30 days ago",
			raw:  "",
			want: "created:>=2025-12-15",
		},
		{
			name: "single date",
			raw:  "2025-01-02",
			want: "created:2025-01-02",
		},
		{
			name: "comparison date",
			raw:  ">=2025-01-02",
			want: "created:>=2025-01-02",
		},
		{
			name: "comparison date less-than",
			raw:  "<=2025-01-02",
			want: "created:<=2025-01-02",
		},
		{
			name: "range date",
			raw:  "2025-01-01..2025-01-31",
			want: "created:2025-01-01..2025-01-31",
		},
		{
			name: "created prefix allowed",
			raw:  "created:2025-01-02",
			want: "created:2025-01-02",
		},
		{
			name:      "invalid date",
			raw:       "2025-13-01",
			expectErr: true,
		},
		{
			name:      "invalid format",
			raw:       ">=2025-01",
			expectErr: true,
		},
		{
			name:      "range reversed",
			raw:       "2025-02-01..2025-01-01",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := auditlog.BuildCreatedClause(tt.raw, now)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error for raw=%q", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestBuildAuditLogPhrase(t *testing.T) {
	now := time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC)

	got, err := auditlog.BuildPhrase("acme", "octocat", "", "", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "actor:octocat") {
		t.Fatalf("expected actor filter, got %q", got)
	}
	if !strings.Contains(got, "created:>=2025-12-15") {
		t.Fatalf("expected default created filter, got %q", got)
	}

	got, err = auditlog.BuildPhrase("acme", "octocat", "repo-one", "2025-01-02", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "actor:octocat repo:acme/repo-one created:2025-01-02" {
		t.Fatalf("unexpected phrase: %q", got)
	}

	if _, err := auditlog.BuildPhrase("acme", "", "", "", now); err == nil {
		t.Fatalf("expected error for empty user")
	}
}
