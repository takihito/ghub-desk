package cmd

import (
	"context"
	"fmt"

	"ghub-desk/mcp"
)

// McpCmd starts the MCP server
type McpCmd struct{}

func (m *McpCmd) Run(cli *CLI) error {
	// Load config to get MCP permissions and auth
	cfg, err := cli.Config()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	cli.debugf("DEBUG: Starting MCP server (allow_pull=%v, allow_write=%v)\n", cfg.MCP.AllowPull, cfg.MCP.AllowWrite)
	cli.debugf("DEBUG: Exposing tools: %v\n", mcp.AllowedTools(cfg))
	ctx := context.Background()
	return mcp.Serve(ctx, cfg, cli.Debug, cli.debugWriter)
}
