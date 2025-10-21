package github

import (
	"testing"
)

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
			if got := metadataEqual(tc.a, tc.b); got != tc.want {
				t.Fatalf("metadataEqual(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
