package mcp

import "ghub-desk/config"

// AllowedTools returns the list of tool names that should be exposed
// by the MCP server based on configuration permissions.
//
// Policy:
// - view.* は常に許可
// - pull.* は AllowPull が true の場合のみ許可
// - push.* は AllowWrite が true の場合のみ許可
func AllowedTools(cfg *config.Config) []string {
	var tools []string

	// view tools (always on)
	tools = append(tools,
		"view.users",
		"view.detail-users",
		"view.teams",
		"view.repos",
		"view.teams-users",
		"view.outside-users",
		"view.token-permission",
	)

	if cfg != nil && cfg.MCP.AllowPull {
		tools = append(tools,
			"pull.users",
			"pull.teams",
			"pull.repositories",
			"pull.teams-users",
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
