package mcp

import (
	"context"
	"fmt"
	"io"
	"os"

	appcfg "ghub-desk/config"
	"ghub-desk/store"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Serve starts the MCP server using the go-sdk over stdio, registering every tool in
// toolRegistry (see registry.go) whose tier is permitted by cfg's MCP permissions.
func Serve(ctx context.Context, cfg *appcfg.Config, debug bool, debugWriter io.Writer) error {
	// Apply DB path from config if provided
	if cfg != nil && cfg.DatabasePath != "" {
		store.SetDBPath(cfg.DatabasePath)
	}
	// Ensure configuration is provided before accessing permissions or auth
	if cfg == nil {
		return fmt.Errorf("configuration is required to start MCP server")
	}
	impl := &sdk.Implementation{
		Name:    "ghub-desk",
		Title:   "ghub-desk MCP",
		Version: "dev",
	}
	srv := sdk.NewServer(impl, &sdk.ServerOptions{HasTools: true, HasResources: true})
	registerDocsResources(srv)

	registerAllowedTools(srv, cfg)

	// Run server over stdio transport
	var transport sdk.Transport = &sdk.StdioTransport{}
	if debug {
		writer := debugWriter
		if writer == nil {
			writer = os.Stderr
		}
		transport = &sdk.LoggingTransport{
			Transport: &sdk.StdioTransport{},
			Writer:    writer,
		}
	}

	return srv.Run(ctx, transport)
}
