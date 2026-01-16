package mcp

import (
	"testing"
	"time"

	ghapi "github.com/google/go-github/v55/github"
)

func TestFormatAuditLogTimestamp(t *testing.T) {
	ts := ghapi.Timestamp{Time: time.Date(2026, 1, 14, 12, 34, 56, 789000000, time.UTC)}
	got := formatAuditLogTimestamp(ts)
	want := "2026-01-14T12:34:56.789Z"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}

	if got := formatAuditLogTimestamp(ghapi.Timestamp{}); got != "" {
		t.Fatalf("expected empty string for zero timestamp, got %q", got)
	}
}

func TestNormalizeAuditLogEntries(t *testing.T) {
	action := "repo.create"
	actor := "octocat"
	actorIP := "1.2.3.4"
	user := "target-user"
	repo := "repo-name"
	org := "acme"
	docID := "doc-1"
	event := "repo"
	opType := "create"
	permission := "admin"
	team := "platform"
	message := "created repo"
	userAgent := "curl/8.0"
	createdAt := ghapi.Timestamp{Time: time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)}
	timestamp := ghapi.Timestamp{Time: time.Date(2026, 1, 14, 1, 2, 3, 4000000, time.UTC)}

	entry := &ghapi.AuditEntry{
		Action:        &action,
		Actor:         &actor,
		ActorIP:       &actorIP,
		User:          &user,
		Repo:          &repo,
		Org:           &org,
		CreatedAt:     &createdAt,
		Timestamp:     &timestamp,
		DocumentID:    &docID,
		Event:         &event,
		OperationType: &opType,
		Permission:    &permission,
		Team:          &team,
		Message:       &message,
		UserAgent:     &userAgent,
	}

	out := normalizeAuditLogEntries([]*ghapi.AuditEntry{entry, nil})
	if len(out) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(out))
	}
	got := out[0]
	if got.Action != action {
		t.Fatalf("expected action %q, got %q", action, got.Action)
	}
	if got.Actor != actor {
		t.Fatalf("expected actor %q, got %q", actor, got.Actor)
	}
	if got.ActorIP != actorIP {
		t.Fatalf("expected actor_ip %q, got %q", actorIP, got.ActorIP)
	}
	if got.User != user {
		t.Fatalf("expected user %q, got %q", user, got.User)
	}
	if got.Repo != repo {
		t.Fatalf("expected repo %q, got %q", repo, got.Repo)
	}
	if got.Org != org {
		t.Fatalf("expected org %q, got %q", org, got.Org)
	}
	if got.CreatedAt != "2026-01-14T00:00:00Z" {
		t.Fatalf("unexpected created_at: %q", got.CreatedAt)
	}
	if got.Timestamp != "2026-01-14T01:02:03.004Z" {
		t.Fatalf("unexpected timestamp: %q", got.Timestamp)
	}
	if got.DocumentID != docID {
		t.Fatalf("expected document_id %q, got %q", docID, got.DocumentID)
	}
	if got.Event != event {
		t.Fatalf("expected event %q, got %q", event, got.Event)
	}
	if got.OperationType != opType {
		t.Fatalf("expected operation_type %q, got %q", opType, got.OperationType)
	}
	if got.Permission != permission {
		t.Fatalf("expected permission %q, got %q", permission, got.Permission)
	}
	if got.Team != team {
		t.Fatalf("expected team %q, got %q", team, got.Team)
	}
	if got.Message != message {
		t.Fatalf("expected message %q, got %q", message, got.Message)
	}
	if got.UserAgent != userAgent {
		t.Fatalf("expected user_agent %q, got %q", userAgent, got.UserAgent)
	}
}
