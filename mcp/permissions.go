package mcp

import "ghub-desk/config"

// AllowedTools returns the list of tool names that should be exposed
// by the MCP server based on configuration permissions.
//
// Policy:
// - view.* is always allowed
// - pull.* is only allowed if AllowPull is true
// - push.* is only allowed if AllowWrite is true
func AllowedTools(cfg *config.Config) []string {
	var tools []string

	// view tools (always on)
	tools = append(tools,
		"view.users",
		"view.detail-users",
		"view.teams",
		"view.repos",
		"view.team-user",
		"view.outside-users",
		"view.token-permission",
	)

	if cfg != nil && cfg.MCP.AllowPull {
		tools = append(tools,
			"pull.users",
			"pull.teams",
			"pull.repositories",
			"pull.team-user",
			"pull.outside-users",
			"pull.token-permission",
		)
	}

	if cfg != nil && cfg.MCP.AllowWrite {
		tools = append(tools,
			"push.remove",
			"push.add",
		)
	}

	return tools
}
