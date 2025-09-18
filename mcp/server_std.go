//go:build !mcp_sdk

package mcp

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"ghub-desk/config"
)

// Serve starts a minimal stub MCP server for default builds.
// It prints the allowed tool list and waits for Ctrl+C.
// The full MCP server using go-sdk is provided behind the build tag: mcp_sdk.
func Serve(ctx context.Context, cfg *config.Config) error {
	tools := AllowedTools(cfg)
	fmt.Println("[MCP] stub server starting...")
	fmt.Printf("[MCP] allowed tools: %v\n", tools)
	fmt.Println("[MCP] build with '-tags mcp_sdk' for full go-sdk server")
	fmt.Println("[MCP] press Ctrl+C to exit")

	// Wait for interrupt
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	fmt.Println("[MCP] shutting down")
	return nil
}
