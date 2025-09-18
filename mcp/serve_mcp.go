//go:build mcp_sdk

package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	appcfg "ghub-desk/config"
	"ghub-desk/store"

	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Serve starts the MCP server using the go-sdk over stdio.
// Tools provided in phase 1:
// - health: simple readiness check
// - view.users: return users from local SQLite DB
func Serve(ctx context.Context, cfg *appcfg.Config) error {
	impl := &sdk.Implementation{
		Name:    "ghub-desk",
		Title:   "ghub-desk MCP",
		Version: "dev",
	}
	srv := sdk.NewServer(impl, &sdk.ServerOptions{HasTools: true})

	// health tool (no input)
	sdk.AddTool[struct{}, HealthOut](srv, &sdk.Tool{
		Name:        "health",
		Title:       "Health Check",
		Description: "Returns server health status.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, HealthOut, error) {
		_ = ctx
		_ = req
		return nil, HealthOut{Status: "ok", Time: time.Now().UTC().Format(time.RFC3339)}, nil
	})

	// view.users tool (no input for now)
	sdk.AddTool[struct{}, ViewUsersOut](srv, &sdk.Tool{
		Name:        "view.users",
		Title:       "View Users",
		Description: "List users from local database.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewUsersOut, error) {
		_ = ctx
		_ = req
		users, err := listUsers()
		if err != nil {
			// return as tool error (not protocol error)
			return &sdk.CallToolResult{}, ViewUsersOut{}, fmt.Errorf("failed to list users: %w", err)
		}
		return nil, ViewUsersOut{Users: users}, nil
	})

	// Respect config permissions if needed in the future for additional tools.
	// For phase 1, only non-destructive tools are registered.

	// Run server over stdio transport
	return srv.Run(ctx, &sdk.StdioTransport{})
}

type HealthOut struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

type ViewUsersOut struct {
	Users []User `json:"users"`
}

type User struct {
	ID       int64  `json:"id"`
	Login    string `json:"login"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Company  string `json:"company,omitempty"`
	Location string `json:"location,omitempty"`
}

func listUsers() ([]User, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, login, name, email, company, location FROM ghub_users ORDER BY login LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []User
	for rows.Next() {
		var (
			id                                    int64
			login, name, email, company, location sql.NullString
		)
		if err := rows.Scan(&id, &login, &name, &email, &company, &location); err != nil {
			return nil, err
		}
		res = append(res, User{
			ID:       id,
			Login:    login.String,
			Name:     name.String,
			Email:    email.String,
			Company:  company.String,
			Location: location.String,
		})
	}
	return res, nil
}
