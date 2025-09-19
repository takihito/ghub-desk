package mcp

import (
	"testing"

	"ghub-desk/config"
)

func TestAllowedTools_Default(t *testing.T) {
	cfg := &config.Config{}
	tools := AllowedTools(cfg)

	mustContain(t, tools, "view.users")
	mustContain(t, tools, "view.detail-users")
	mustNotContain(t, tools, "pull.users")
	mustNotContain(t, tools, "push.add")
}

func TestAllowedTools_PullOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.MCP.AllowPull = true
	tools := AllowedTools(cfg)

	mustContain(t, tools, "pull.users")
	mustContain(t, tools, "pull.token-permission")
	mustNotContain(t, tools, "push.remove")
}

func TestAllowedTools_WriteOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.MCP.AllowWrite = true
	tools := AllowedTools(cfg)

	mustContain(t, tools, "push.add")
	mustContain(t, tools, "push.remove")
	mustNotContain(t, tools, "pull.teams")
}

func mustContain(t *testing.T, list []string, v string) {
	t.Helper()
	for _, s := range list {
		if s == v {
			return
		}
	}
	t.Fatalf("expected %q in list, got %v", v, list)
}

func mustNotContain(t *testing.T, list []string, v string) {
	t.Helper()
	for _, s := range list {
		if s == v {
			t.Fatalf("expected %q not in list, got %v", v, list)
		}
	}
}
