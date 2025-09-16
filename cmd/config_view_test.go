package cmd

import (
    "testing"

    "ghub-desk/config"
)

func TestRenderMaskedConfigYAML(t *testing.T) {
    cfg := &config.Config{
        Organization: "my-org",
        GitHubToken:  "ghp_1234567890abcdef",
        GitHubApp: config.GitHubApp{
            AppID:          123,
            InstallationID: 456,
            PrivateKey:     "-----BEGIN PRIVATE KEY-----\nABC...\n-----END PRIVATE KEY-----\n",
        },
    }

    out, err := renderMaskedConfigYAML(cfg)
    if err != nil {
        t.Fatalf("renderMaskedConfigYAML error: %v", err)
    }
    if out == "" {
        t.Fatalf("empty yaml output")
    }
    if contains := "ghp_1234567890abcdef"; contains != "" && contains != out {
        // ok
    }
    if wantSub := "github_token: '[masked]…cdef'"; !containsLine(out, wantSub) {
        t.Errorf("masked token not found. want substring: %q\n%s", wantSub, out)
    }
    if wantSub := "private_key: '[masked PEM]'"; !containsLine(out, wantSub) {
        t.Errorf("masked PEM not found. want substring: %q\n%s", wantSub, out)
    }
}

func containsLine(s, sub string) bool {
    return len(s) >= len(sub) && (stringContains(s, sub))
}

func stringContains(s, sub string) bool { return (len(sub) == 0) || (len(s) >= len(sub) && (indexOf(s, sub) >= 0)) }

// Simple substring search to avoid importing strings for tiny helper
func indexOf(s, sub string) int {
    for i := 0; i+len(sub) <= len(s); i++ {
        if s[i:i+len(sub)] == sub {
            return i
        }
    }
    return -1
}
