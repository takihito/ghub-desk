package ghubclient

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-github/v84/github"
	_ "modernc.org/sqlite"
)

// newOrgTestClient returns a go-github client backed by a test server that serves
// GET /orgs/acme with the given JSON body.
func newOrgTestClient(t *testing.T, orgJSON string) *github.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orgs/acme" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, orgJSON)
	}))
	t.Cleanup(server.Close)

	client := github.NewClient(server.Client())
	baseURL, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	client.BaseURL = baseURL
	return client
}

func TestPullOrgPlanStoresSnapshot(t *testing.T) {
	client := newOrgTestClient(t, `{
		"login": "acme",
		"plan": {"name": "enterprise", "seats": 100, "filled_seats": 87, "private_repos": 500, "collaborators": 20}
	}`)

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ghub_org_plans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		login TEXT, plan_name TEXT, seats INTEGER, filled_seats INTEGER,
		private_repos INTEGER, collaborators INTEGER, created_at TEXT, updated_at TEXT
	)`); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	var out bytes.Buffer
	err = PullOrgPlan(context.Background(), client, db, "acme", PullOptions{Store: true, Output: &out})
	if err != nil {
		t.Fatalf("PullOrgPlan() error = %v", err)
	}

	var login, planName string
	var seats, filledSeats int
	err = db.QueryRow(`SELECT login, plan_name, seats, filled_seats FROM ghub_org_plans`).
		Scan(&login, &planName, &seats, &filledSeats)
	if err != nil {
		t.Fatalf("Failed to query stored plan: %v", err)
	}
	if login != "acme" || planName != "enterprise" || seats != 100 || filledSeats != 87 {
		t.Fatalf("unexpected stored values: login=%s plan=%s seats=%d filled=%d", login, planName, seats, filledSeats)
	}

	if got := out.String(); !strings.Contains(got, "Seats: 100 (filled: 87)") {
		t.Fatalf("expected seat summary in output, got %q", got)
	}
}

func TestPullOrgPlanErrorsWhenPlanUnavailable(t *testing.T) {
	client := newOrgTestClient(t, `{"login": "acme"}`)

	var out bytes.Buffer
	err := PullOrgPlan(context.Background(), client, nil, "acme", PullOptions{Store: false, Output: &out})
	if err == nil {
		t.Fatal("expected error when plan is unavailable, got nil")
	}
	if !strings.Contains(err.Error(), "read:org") {
		t.Fatalf("expected error to mention required token access, got: %v", err)
	}
}
