package ghubclient

import (
	"errors"
	"testing"
)

func TestPullAllForEachDedupesAndSkipsBlank(t *testing.T) {
	var visited []string
	err := pullAllForEach(
		[]string{"alpha", "", "beta", "alpha", "  "},
		PullOptions{},
		"repos-users", "repo", "repo_index", "repository", "repository name",
		nil,
		func(_ int, name string, _ PullOptions) error {
			visited = append(visited, name)
			return nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := []string{"alpha", "beta"}; !equalStrings(visited, got) {
		t.Fatalf("expected deduped visit order %v, got %v", got, visited)
	}
}

func TestPullAllForEachResumesFromMidpoint(t *testing.T) {
	var visited []string
	opts := PullOptions{
		Resume: ResumeState{
			Endpoint: "repos-users",
			Metadata: map[string]string{"repo": "beta"},
		},
	}
	err := pullAllForEach(
		[]string{"alpha", "beta", "gamma"},
		opts,
		"repos-users", "repo", "repo_index", "repository", "repository name",
		nil,
		func(_ int, name string, itemOpts PullOptions) error {
			visited = append(visited, name)
			if name == "beta" && itemOpts.Resume.Endpoint != "repos-users" {
				t.Fatalf("expected resume state to still be set on the resumed item")
			}
			return nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := []string{"beta", "gamma"}; !equalStrings(visited, got) {
		t.Fatalf("expected resume to skip to beta, got %v", got)
	}
}

func TestPullAllForEachClearsResumeAfterTargetItem(t *testing.T) {
	var resumeAtGamma ResumeState
	opts := PullOptions{
		Resume: ResumeState{
			Endpoint: "repos-users",
			Metadata: map[string]string{"repo": "beta"},
		},
	}
	err := pullAllForEach(
		[]string{"alpha", "beta", "gamma"},
		opts,
		"repos-users", "repo", "repo_index", "repository", "repository name",
		nil,
		func(_ int, name string, itemOpts PullOptions) error {
			if name == "gamma" {
				resumeAtGamma = itemOpts.Resume
			}
			return nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resumeAtGamma.Endpoint != "" {
		t.Fatalf("expected resume state to be cleared once the resumed item (beta) succeeded, got %+v", resumeAtGamma)
	}
}

func TestPullAllForEachAbortsOnErrorWithNilOnError(t *testing.T) {
	var visited []string
	boom := errors.New("boom")
	err := pullAllForEach(
		[]string{"alpha", "beta", "gamma"},
		PullOptions{},
		"repos-users", "repo", "repo_index", "repository", "repository name",
		nil,
		func(_ int, name string, _ PullOptions) error {
			visited = append(visited, name)
			if name == "beta" {
				return boom
			}
			return nil
		},
		nil,
	)
	if !errors.Is(err, boom) {
		t.Fatalf("expected the underlying error to propagate, got %v", err)
	}
	if got := []string{"alpha", "beta"}; !equalStrings(visited, got) {
		t.Fatalf("expected loop to stop right after the failing item, got %v", visited)
	}
}

func TestPullAllForEachContinuesWhenOnErrorSwallows(t *testing.T) {
	var visited []string
	var warned []string
	err := pullAllForEach(
		[]string{"alpha", "beta", "gamma"},
		PullOptions{},
		"team-user", "team", "team_index", "team", "team slug",
		nil,
		func(_ int, name string, _ PullOptions) error {
			visited = append(visited, name)
			if name == "beta" {
				return errors.New("transient failure")
			}
			return nil
		},
		func(name string, err error) error {
			warned = append(warned, name)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := []string{"alpha", "beta", "gamma"}; !equalStrings(visited, got) {
		t.Fatalf("expected all items to be visited despite the failure, got %v", visited)
	}
	if got := []string{"beta"}; !equalStrings(warned, got) {
		t.Fatalf("expected onError to be called for beta only, got %v", warned)
	}
}

func TestPullAllForEachOnErrorCanEscalate(t *testing.T) {
	fatal := errors.New("fatal")
	err := pullAllForEach(
		[]string{"alpha", "beta", "gamma"},
		PullOptions{},
		"team-user", "team", "team_index", "team", "team slug",
		nil,
		func(_ int, name string, _ PullOptions) error {
			if name == "beta" {
				return errors.New("transient failure")
			}
			return nil
		},
		func(name string, err error) error {
			return fatal
		},
	)
	if !errors.Is(err, fatal) {
		t.Fatalf("expected onError's escalated error to propagate, got %v", err)
	}
}

func TestPullAllForEachOnReadyReceivesDedupedList(t *testing.T) {
	var readyWith []string
	err := pullAllForEach(
		[]string{"alpha", "alpha", "beta"},
		PullOptions{},
		"repos-users", "repo", "repo_index", "repository", "repository name",
		func(unique []string) {
			readyWith = append([]string(nil), unique...)
		},
		func(_ int, _ string, _ PullOptions) error { return nil },
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := []string{"alpha", "beta"}; !equalStrings(readyWith, got) {
		t.Fatalf("expected onReady to receive the deduped list %v, got %v", got, readyWith)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
