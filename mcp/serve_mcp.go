//go:build mcp_sdk

package mcp

import (
	"context"

	"ghub-desk/config"
	// NOTE: Skeleton import path; actual server wiring to be implemented.
	_ "github.com/modelcontextprotocol/go-sdk"
)

// Serve starts the MCP server using the go-sdk (skeleton).
func Serve(ctx context.Context, cfg *config.Config) error {
	_ = ctx
	_ = cfg
	// TODO: Initialize go-sdk MCP server (stdio transport),
	// register tools based on cfg.MCP permissions, and serve.
	return nil
}
