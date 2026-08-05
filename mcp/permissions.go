package mcp

import "ghub-desk/config"

// AllowedTools returns the list of tool names that would be exposed by the MCP server for
// the given configuration. Derived directly from toolRegistry (see registry.go) so this list
// can never drift from the tools Serve actually registers.
func AllowedTools(cfg *config.Config) []string {
	var names []string
	for _, t := range toolRegistry {
		if t.tier.allowed(cfg) {
			names = append(names, t.name)
		}
	}
	return names
}
