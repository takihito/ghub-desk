package cmd

import (
    "strings"
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
    // Ensure raw token is not present (must be masked)
    if strings.Contains(out, "ghp_1234567890abcdef") {
        t.Errorf("raw github_token should be masked, but found in output:\n%s", out)
    }
    if wantSub := "github_token: '[masked]…cdef'"; !strings.Contains(out, wantSub) {
        t.Errorf("masked token not found. want substring: %q\n%s", wantSub, out)
    }
    if wantSub := "private_key: '[masked PEM]'"; !strings.Contains(out, wantSub) {
        t.Errorf("masked PEM not found. want substring: %q\n%s", wantSub, out)
    }
}
