package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveConfigPath_Custom(t *testing.T) {
	got, err := ResolveConfigPath("/tmp/custom.yaml")
	if err != nil {
		t.Fatalf("ResolveConfigPath(custom) error = %v", err)
	}
	if got != "/tmp/custom.yaml" {
		t.Errorf("got %q, want %q", got, "/tmp/custom.yaml")
	}
}

func TestResolveConfigPath_Default(t *testing.T) {
	td := t.TempDir()
	t.Setenv("HOME", td)

	got, err := ResolveConfigPath("")
	if err != nil {
		t.Fatalf("ResolveConfigPath(\"\") error = %v", err)
	}
	want := filepath.Join(td, "."+AppName, "config.yaml")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLegacyConfigDirWarning(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(home string)
		wantWarning bool
	}{
		{
			name:        "no legacy config — no warning",
			setup:       func(home string) {},
			wantWarning: false,
		},
		{
			name: "legacy config exists, new config exists — no warning",
			setup: func(home string) {
				writeFile(t, filepath.Join(home, ".config", AppName, "config.yaml"))
				writeFile(t, filepath.Join(home, "."+AppName, "config.yaml"))
			},
			wantWarning: false,
		},
		{
			name: "legacy config exists, new config absent — warning",
			setup: func(home string) {
				writeFile(t, filepath.Join(home, ".config", AppName, "config.yaml"))
			},
			wantWarning: true,
		},
		{
			name: "new dir exists (session.json) but new config absent — warning",
			setup: func(home string) {
				writeFile(t, filepath.Join(home, ".config", AppName, "config.yaml"))
				writeFile(t, filepath.Join(home, "."+AppName, "session.json"))
			},
			wantWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := t.TempDir()
			t.Setenv("HOME", td)
			tt.setup(td)

			got := LegacyConfigDirWarning()
			if tt.wantWarning && got == "" {
				t.Error("expected a warning but got empty string")
			}
			if !tt.wantWarning && got != "" {
				t.Errorf("expected no warning but got %q", got)
			}
			if tt.wantWarning {
				if !strings.Contains(got, "warning:") {
					t.Errorf("warning missing 'warning:' prefix: %q", got)
				}
				if !strings.Contains(got, "cp ") {
					t.Errorf("warning missing migration command: %q", got)
				}
			}
		})
	}
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
