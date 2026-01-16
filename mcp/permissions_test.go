package mcp

import (
	"testing"

	"ghub-desk/config"
)

func TestAllowedTools_Default(t *testing.T) {
	cfg := &config.Config{}
	tools := AllowedTools(cfg)

	mustContain(t, tools, "view_users")
	mustContain(t, tools, "view_detail-users")
	mustContain(t, tools, "auditlogs")
	mustNotContain(t, tools, "pull_users")
	mustNotContain(t, tools, "push_add")
}

func TestAllowedTools_PullOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.MCP.AllowPull = true
	tools := AllowedTools(cfg)

	mustContain(t, tools, "pull_users")
	mustContain(t, tools, "pull_token-permission")
	mustContain(t, tools, "auditlogs")
	mustNotContain(t, tools, "push_remove")
}

func TestAllowedTools_WriteOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.MCP.AllowWrite = true
	tools := AllowedTools(cfg)

	mustContain(t, tools, "push_add")
	mustContain(t, tools, "push_remove")
	mustContain(t, tools, "auditlogs")
	mustNotContain(t, tools, "pull_teams")
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
