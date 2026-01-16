package auditlog

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	ghapi "github.com/google/go-github/v55/github"
)

func TestBuildCreatedClause(t *testing.T) {
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
			name: "range date",
			raw:  "2025-01-01..2025-01-31",
			want: "created:2025-01-01..2025-01-31",
		},
		{
			name:      "invalid date",
			raw:       "2025-13-01",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCreatedClause(tt.raw, now)
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

func TestBuildPhrase(t *testing.T) {
	now := time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC)

	got, err := BuildPhrase("acme", "octocat", "", "", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "actor:octocat created:>=2025-12-15" {
		t.Fatalf("unexpected phrase: %q", got)
	}

	got, err = BuildPhrase("acme", "octocat", "repo-one", "2025-01-02", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "actor:octocat repo:acme/repo-one created:2025-01-02" {
		t.Fatalf("unexpected phrase: %q", got)
	}

	if _, err := BuildPhrase("", "octocat", "repo-one", "", now); err == nil {
		t.Fatalf("expected error for missing org")
	}
}

func TestRepoFromEntry(t *testing.T) {
	repo := "repo-name"
	repository := "repository-name"

	if got := RepoFromEntry(nil); got != "" {
		t.Fatalf("expected empty repo for nil entry, got %q", got)
	}

	entry := &ghapi.AuditEntry{Repo: &repo, Repository: &repository}
	if got := RepoFromEntry(entry); got != repo {
		t.Fatalf("expected repo %q, got %q", repo, got)
	}

	entry = &ghapi.AuditEntry{Repository: &repository}
	if got := RepoFromEntry(entry); got != repository {
		t.Fatalf("expected repo %q, got %q", repository, got)
	}
}

func TestUserFromEntry(t *testing.T) {
	user := "target-user"
	target := "target-login"

	if got := UserFromEntry(nil); got != "" {
		t.Fatalf("expected empty user for nil entry, got %q", got)
	}

	entry := &ghapi.AuditEntry{User: &user, TargetLogin: &target}
	if got := UserFromEntry(entry); got != user {
		t.Fatalf("expected user %q, got %q", user, got)
	}

	entry = &ghapi.AuditEntry{TargetLogin: &target}
	if got := UserFromEntry(entry); got != target {
		t.Fatalf("expected user %q, got %q", target, got)
	}
}

func TestFetchEntries(t *testing.T) {
	var calls int32

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orgs/acme/audit-log" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		after := r.URL.Query().Get("after")
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		switch after {
		case "":
			w.Header().Set("Link", fmt.Sprintf(`<%s/orgs/acme/audit-log?after=cursor-1>; rel="next"`, server.URL))
			fmt.Fprint(w, `[{"action":"repo.create"},{"action":"repo.delete"}]`)
		case "cursor-1":
			fmt.Fprint(w, `[{"action":"team.add_member"}]`)
		default:
			http.Error(w, "bad cursor", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	client := ghapi.NewClient(server.Client())
	baseURL, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatalf("failed to parse base URL: %v", err)
	}
	client.BaseURL = baseURL

	opts := &ghapi.GetAuditLogOptions{
		Phrase: ghapi.String("actor:octocat"),
		ListCursorOptions: ghapi.ListCursorOptions{
			PerPage: 100,
		},
	}
	entries, err := FetchEntries(context.Background(), client, "acme", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}
