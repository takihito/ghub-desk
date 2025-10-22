package github

import (
	"strings"
	"testing"
)

func TestPrepareResumeMatchesName(t *testing.T) {
	state := ResumeState{
		Endpoint: "repos-teams",
		Metadata: map[string]string{"repo": "beta", "repo_index": "1"},
	}

	updated, idx, msg, name := prepareResume([]string{"alpha", "beta", "gamma"}, state, "repos-teams", "repo", "repo_index", "repository", "repository name")

	if msg != "" {
		t.Fatalf("unexpected log message: %s", msg)
	}
	if idx != 1 {
		t.Fatalf("expected index 1, got %d", idx)
	}
	if name != "beta" {
		t.Fatalf("expected repo name 'beta', got %q", name)
	}
	if updated.Endpoint != state.Endpoint {
		t.Fatalf("resume endpoint changed: %v", updated.Endpoint)
	}
}

func TestPrepareResumeMissingName(t *testing.T) {
	state := ResumeState{
		Endpoint: "repos-teams",
		Metadata: map[string]string{"repo_index": "2"},
	}

	updated, idx, msg, name := prepareResume([]string{"alpha", "beta", "gamma"}, state, "repos-teams", "repo", "repo_index", "repository", "repository name")

	if updated.Endpoint != "" {
		t.Fatalf("expected resume to be cleared, got endpoint %q", updated.Endpoint)
	}
	if idx != -1 {
		t.Fatalf("expected index -1, got %d", idx)
	}
	if name != "" {
		t.Fatalf("expected empty name, got %q", name)
	}
	if msg == "" || !strings.Contains(msg, "missing") {
		t.Fatalf("expected missing-name message, got %q", msg)
	}
}

func TestPrepareResumeNameNotFound(t *testing.T) {
	state := ResumeState{
		Endpoint: "repos-teams",
		Metadata: map[string]string{"repo": "delta"},
	}

	updated, idx, msg, name := prepareResume([]string{"alpha", "beta", "gamma"}, state, "repos-teams", "repo", "repo_index", "repository", "repository name")

	if updated.Endpoint != "" {
		t.Fatalf("expected resume to be cleared, got endpoint %q", updated.Endpoint)
	}
	if idx != -1 {
		t.Fatalf("expected index -1, got %d", idx)
	}
	if name != "" {
		t.Fatalf("expected empty name when target missing, got %q", name)
	}
	if msg == "" || !strings.Contains(msg, "delta") || !strings.Contains(msg, "not found") {
		t.Fatalf("expected not-found message, got %q", msg)
	}
}
