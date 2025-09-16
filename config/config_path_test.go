package config

import (
    "path/filepath"
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
    want := filepath.Join(td, ".config", AppName, "config.yaml")
    if got != want {
        t.Errorf("got %q, want %q", got, want)
    }
}
