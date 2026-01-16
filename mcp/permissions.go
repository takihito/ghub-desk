package mcp

import "ghub-desk/config"

// AllowedTools returns the list of tool names that should be exposed
// by the MCP server based on configuration permissions.
//
// Policy:
// - view_* and auditlogs are always allowed
// - pull_* is only allowed if AllowPull is true
// - push_* is only allowed if AllowWrite is true
func AllowedTools(cfg *config.Config) []string {
	var tools []string

	// view tools (always on)
	tools = append(tools,
		"auditlogs",
		"view_users",
		"view_detail-users",
		"view_teams",
		"view_repos",
		"view_team-user",
		"view_outside-users",
		"view_token-permission",
	)

	if cfg != nil && cfg.MCP.AllowPull {
		tools = append(tools,
			"pull_users",
			"pull_teams",
			"pull_repositories",
			"pull_team-user",
			"pull_outside-users",
			"pull_token-permission",
		)
	}

	if cfg != nil && cfg.MCP.AllowWrite {
		tools = append(tools,
			"push_remove",
			"push_add",
		)
	}

	return tools
}
