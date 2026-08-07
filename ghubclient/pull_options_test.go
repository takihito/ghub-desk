package ghubclient

import (
	"bytes"
	"maps"
	"os"
	"testing"
)

func TestPullOptionsOutputDefaultsToStdout(t *testing.T) {
	opts := PullOptions{}
	if got := opts.output(); got != os.Stdout {
		t.Fatalf("expected output() to default to os.Stdout when Output is nil, got %v", got)
	}
}

func TestPullOptionsOutputUsesInjectedWriter(t *testing.T) {
	var buf bytes.Buffer
	opts := PullOptions{Output: &buf}
	if got := opts.output(); got != &buf {
		t.Fatalf("expected output() to return the injected writer, got %v", got)
	}
}

func TestPullOptionsForEndpointResumeMatch(t *testing.T) {
	opts := PullOptions{
		Resume: ResumeState{
			Endpoint: "users",
			Metadata: map[string]string{"team": "alpha"},
			LastPage: 3,
			Count:    120,
		},
	}

	meta := map[string]string{"team": "alpha"}
	got := opts.ForEndpoint("users", meta)

	if got.StartPage != 4 {
		t.Fatalf("StartPage mismatch: expected 4, got %d", got.StartPage)
	}
	if got.InitialCount != 120 {
		t.Fatalf("InitialCount mismatch: expected 120, got %d", got.InitialCount)
	}
	if got.Resume.Endpoint != "" {
		t.Fatalf("Resume should be cleared after matching endpoint, got %q", got.Resume.Endpoint)
	}
}

func TestPullOptionsForEndpointResumeMismatch(t *testing.T) {
	opts := PullOptions{
		Resume: ResumeState{
			Endpoint: "users",
			Metadata: map[string]string{"team": "alpha"},
			LastPage: 2,
			Count:    50,
		},
	}

	meta := map[string]string{"team": "beta"}
	got := opts.ForEndpoint("users", meta)

	if got.StartPage != 1 {
		t.Fatalf("StartPage should reset to 1 on mismatch, got %d", got.StartPage)
	}
	if got.InitialCount != 0 {
		t.Fatalf("InitialCount should reset to 0 on mismatch, got %d", got.InitialCount)
	}
}

func TestMetadataEqual(t *testing.T) {
	testCases := []struct {
		name string
		a    map[string]string
		b    map[string]string
		want bool
	}{
		{"both nil", nil, nil, true},
		{"empty maps", map[string]string{}, map[string]string{}, true},
		{"same entries", map[string]string{"k": "v"}, map[string]string{"k": "v"}, true},
		{"different values", map[string]string{"k": "v"}, map[string]string{"k": "x"}, false},
		{"different keys", map[string]string{"k": "v"}, map[string]string{"x": "v"}, false},
		{"different length", map[string]string{"k": "v"}, map[string]string{"k": "v", "x": "y"}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := maps.Equal(tc.a, tc.b); got != tc.want {
				t.Fatalf("maps.Equal(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
