package mcp

import (
	"context"
	"fmt"

	"ghub-desk/config"
)

// Serve starts a minimal stub MCP server.
// This stub is used in default builds to keep tests green.
// A full implementation using the go-sdk is provided behind a build tag.
func Serve(ctx context.Context, cfg *config.Config) error {
	_ = ctx
	fmt.Println("MCP server stub: start (use -tags mcp_sdk for full server)")
	// No-op stub. Full server lives in serve_mcp.go (build tag: mcp_sdk).
	return nil
}
