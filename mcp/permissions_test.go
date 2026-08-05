package mcp

import (
	"strings"
	"testing"

	"ghub-desk/config"
)

// TestToolRegistryTiersMatchNamingConvention guards against the drift this registry was
// built to prevent: every pull_*/push_* tool must carry the matching tier, and every other
// tool (view_*, auditlogs, health) must be tierCore. Names must also be unique.
func TestToolRegistryTiersMatchNamingConvention(t *testing.T) {
	seen := make(map[string]bool, len(toolRegistry))
	for _, def := range toolRegistry {
		if seen[def.name] {
			t.Fatalf("duplicate tool name in toolRegistry: %q", def.name)
		}
		seen[def.name] = true

		switch {
		case strings.HasPrefix(def.name, "pull_"):
			if def.tier != tierPull {
				t.Errorf("tool %q should be tierPull, got %v", def.name, def.tier)
			}
		case strings.HasPrefix(def.name, "push_"):
			if def.tier != tierWrite {
				t.Errorf("tool %q should be tierWrite, got %v", def.name, def.tier)
			}
		default:
			if def.tier != tierCore {
				t.Errorf("tool %q should be tierCore, got %v", def.name, def.tier)
			}
		}
	}
}

// TestAllowedTools_FullyPermitted confirms AllowedTools reports every registered tool once
// pull and write are both enabled, i.e. it stays in sync with toolRegistry by construction.
func TestAllowedTools_FullyPermitted(t *testing.T) {
	cfg := &config.Config{}
	cfg.MCP.AllowPull = true
	cfg.MCP.AllowWrite = true

	got := AllowedTools(cfg)
	if len(got) != len(toolRegistry) {
		t.Fatalf("expected %d tools when fully permitted, got %d: %v", len(toolRegistry), len(got), got)
	}
	for _, def := range toolRegistry {
		mustContain(t, got, def.name)
	}
}

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
