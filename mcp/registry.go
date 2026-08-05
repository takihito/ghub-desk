package mcp

import (
	appcfg "ghub-desk/config"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolTier controls whether a tool is registered/exposed based on MCP config permissions.
type toolTier int

const (
	// tierCore tools are always registered: read-only views plus audit log lookup.
	tierCore toolTier = iota
	// tierPull tools call the GitHub API (non-destructive) and require mcp.allow_pull.
	tierPull
	// tierWrite tools mutate GitHub state and require mcp.allow_write.
	tierWrite
)

// allowed reports whether tools at this tier should be exposed for the given configuration.
func (t toolTier) allowed(cfg *appcfg.Config) bool {
	switch t {
	case tierPull:
		return cfg != nil && cfg.MCP.AllowPull
	case tierWrite:
		return cfg != nil && cfg.MCP.AllowWrite
	default:
		return true
	}
}

// toolRegisterFunc registers a single MCP tool named name against srv.
type toolRegisterFunc func(srv *sdk.Server, name string, cfg *appcfg.Config)

// toolDef pairs a tool's name and access tier with the function that registers it.
// The name is the single source of truth: it flows into register (which passes it on to
// sdk.Tool.Name), so the name AllowedTools reports and the name a client actually sees can
// never drift apart the way the old hand-maintained AllowedTools list did.
type toolDef struct {
	name     string
	tier     toolTier
	register toolRegisterFunc
}

// toolRegistry lists every MCP tool this server can expose, in registration order.
// Assembled from the tier-grouped slices declared alongside each tool's handlers
// (coreViewToolDefs and auditLogsToolDef in tools_view.go/auditlogs.go, pullToolDefs in
// tools_pull.go, writeToolDefs in tools_push.go).
var toolRegistry = buildToolRegistry()

func buildToolRegistry() []toolDef {
	all := make([]toolDef, 0, len(coreViewToolDefs)+1+len(pullToolDefs)+len(writeToolDefs))
	all = append(all, coreViewToolDefs...)
	all = append(all, auditLogsToolDef)
	all = append(all, pullToolDefs...)
	all = append(all, writeToolDefs...)
	return all
}

// registerAllowedTools registers every tool in toolRegistry whose tier is permitted by cfg.
func registerAllowedTools(srv *sdk.Server, cfg *appcfg.Config) {
	for _, t := range toolRegistry {
		if !t.tier.allowed(cfg) {
			continue
		}
		t.register(srv, t.name, cfg)
	}
}
