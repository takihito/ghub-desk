package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	appcfg "ghub-desk/config"
	gh "ghub-desk/github"
	"ghub-desk/store"
	v "ghub-desk/validate"

	ghapi "github.com/google/go-github/v55/github"
	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultPullInterval = 3 * time.Second
)

func intPtr(i int) *int           { return &i }
func floatPtr(f float64) *float64 { return &f }

func pullOptionProperties(extra map[string]*jsonschema.Schema) map[string]*jsonschema.Schema {
	props := map[string]*jsonschema.Schema{
		"no_store": {
			Type:        "boolean",
			Description: "When true, skip writing fetched data to the local SQLite database.",
		},
		"stdout": {
			Type:        "boolean",
			Description: "When true, stream GitHub API responses to stdout for debugging.",
		},
		"interval_seconds": {
			Type:        "number",
			Description: "Delay between GitHub API requests in seconds (default: 3).",
			Minimum:     floatPtr(0),
		},
	}
	for key, schema := range extra {
		props[key] = schema
	}
	return props
}

// Serve starts the MCP server using the go-sdk over stdio.
// Tools provided in phase 1:
// - health: simple readiness check
// - view_users: return users from local SQLite DB
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

	// health tool (no input)
	sdk.AddTool[struct{}, HealthOut](srv, &sdk.Tool{
		Name:        "health",
		Title:       "Health Check",
		Description: "Returns server health status.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(_ context.Context, _ *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, HealthOut, error) {
		return nil, HealthOut{Status: "ok", Time: time.Now().UTC().Format(time.RFC3339)}, nil
	})

	// view_users tool (no input for now)
	sdk.AddTool[struct{}, ViewUsersOut](srv, &sdk.Tool{
		Name:        "view_users",
		Title:       "View Users",
		Description: "List users from local database. Usage: " + docsToolsURI + "#view_users.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(_ context.Context, _ *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewUsersOut, error) {
		users, err := listUsers()
		if err != nil {
			// return as tool error (not protocol error)
			return &sdk.CallToolResult{}, ViewUsersOut{}, fmt.Errorf("failed to list users: %w", err)
		}
		return nil, ViewUsersOut{Users: users}, nil
	})

	// view_detail-users tool (same output shape as view_users for now)
	sdk.AddTool[struct{}, ViewUsersOut](srv, &sdk.Tool{
		Name:        "view_detail-users",
		Title:       "View Detail Users",
		Description: "List users with details from local database. Usage: " + docsToolsURI + "#view_detail-users.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(_ context.Context, _ *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewUsersOut, error) {
		users, err := listUsers()
		if err != nil {
			return &sdk.CallToolResult{}, ViewUsersOut{}, fmt.Errorf("failed to list users: %w", err)
		}
		return nil, ViewUsersOut{Users: users}, nil
	})

	// view_user {user}
	sdk.AddTool[ViewUserIn, ViewUserOut](srv, &sdk.Tool{
		Name:        "view_user",
		Title:       "View Single User",
		Description: "Show one user profile from local database. Pass {\"user\":\"github-login\"}. Usage: " + docsToolsURI + "#view_user.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"user": {
					Type:        "string",
					Title:       "User Login",
					Description: "GitHub username (1-39 chars, alnum or hyphen).",
					MinLength:   intPtr(v.UserNameMin),
					MaxLength:   intPtr(v.UserNameMax),
					Pattern:     v.UserNamePattern,
				},
			},
			Required: []string{"user"},
		},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in ViewUserIn) (*sdk.CallToolResult, ViewUserOut, error) {
		login := strings.TrimSpace(in.User)
		if login == "" {
			return &sdk.CallToolResult{}, ViewUserOut{}, fmt.Errorf("user is required")
		}
		if err := v.ValidateUserName(login); err != nil {
			return &sdk.CallToolResult{}, ViewUserOut{}, err
		}
		out, err := getUserProfile(login)
		if err != nil {
			return &sdk.CallToolResult{}, ViewUserOut{}, fmt.Errorf("failed to get user: %w", err)
		}
		return nil, out, nil
	})

	// view_user-teams {user}
	sdk.AddTool[ViewUserTeamsIn, ViewUserTeamsOut](srv, &sdk.Tool{
		Name:        "view_user-teams",
		Title:       "View User Teams",
		Description: "List teams a user belongs to from local database. Pass {\"user\":\"github-login\"}. Usage: " + docsToolsURI + "#view_user-teams.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"user": {
					Type:        "string",
					Title:       "User Login",
					Description: "GitHub username (1-39 chars, alnum or hyphen).",
					MinLength:   intPtr(v.UserNameMin),
					MaxLength:   intPtr(v.UserNameMax),
					Pattern:     v.UserNamePattern,
				},
			},
			Required: []string{"user"},
		},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in ViewUserTeamsIn) (*sdk.CallToolResult, ViewUserTeamsOut, error) {
		login := strings.TrimSpace(in.User)
		if login == "" {
			return &sdk.CallToolResult{}, ViewUserTeamsOut{}, fmt.Errorf("user is required")
		}
		if err := v.ValidateUserName(login); err != nil {
			return &sdk.CallToolResult{}, ViewUserTeamsOut{}, err
		}
		out, err := listUserTeams(login)
		if err != nil {
			return &sdk.CallToolResult{}, ViewUserTeamsOut{}, fmt.Errorf("failed to list user teams: %w", err)
		}
		return nil, out, nil
	})

	// view_teams
	sdk.AddTool[struct{}, ViewTeamsOut](srv, &sdk.Tool{
		Name:        "view_teams",
		Title:       "View Teams",
		Description: "List teams from local database. Usage: " + docsToolsURI + "#view_teams.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewTeamsOut, error) {
		teams, err := listTeams()
		if err != nil {
			return &sdk.CallToolResult{}, ViewTeamsOut{}, fmt.Errorf("failed to list teams: %w", err)
		}
		return nil, ViewTeamsOut{Teams: teams}, nil
	})

	// view_repos
	sdk.AddTool[struct{}, ViewReposOut](srv, &sdk.Tool{
		Name:        "view_repos",
		Title:       "View Repositories",
		Description: "List repositories from local database. Usage: " + docsToolsURI + "#view_repos.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewReposOut, error) {
		repos, err := listRepositories()
		if err != nil {
			return &sdk.CallToolResult{}, ViewReposOut{}, fmt.Errorf("failed to list repositories: %w", err)
		}
		return nil, ViewReposOut{Repositories: repos}, nil
	})

	// view_team-user {team}
	sdk.AddTool[ViewTeamUsersIn, ViewTeamUsersOut](srv, &sdk.Tool{
		Name:        "view_team-user",
		Title:       "View Team Users",
		Description: "List users in a specific team from local database. Pass {\"team\":\"team-slug\"} using the lowercase-slug format (alnum + hyphen). Usage: " + docsToolsURI + "#view_team-user.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"team": {
					Type:        "string",
					Title:       "Team Slug",
					Description: "team slug (lowercase alnum + hyphen)",
					MinLength:   intPtr(1),
					MaxLength:   intPtr(100),
					Pattern:     v.TeamSlugPattern,
				},
			},
			Required: []string{"team"},
		},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in ViewTeamUsersIn) (*sdk.CallToolResult, ViewTeamUsersOut, error) {
		if in.Team == "" {
			return &sdk.CallToolResult{}, ViewTeamUsersOut{}, fmt.Errorf("team is required")
		}
		if err := v.ValidateTeamSlug(in.Team); err != nil {
			return &sdk.CallToolResult{}, ViewTeamUsersOut{}, err
		}
		users, err := listTeamUsers(in.Team)
		if err != nil {
			return &sdk.CallToolResult{}, ViewTeamUsersOut{}, fmt.Errorf("failed to list team users: %w", err)
		}
		return nil, ViewTeamUsersOut{Team: in.Team, Users: users}, nil
	})

	// view_repos-users {repository}
	sdk.AddTool[ViewRepoUsersIn, ViewRepoUsersOut](srv, &sdk.Tool{
		Name:        "view_repos-users",
		Title:       "View Repository Collaborators",
		Description: "List direct collaborators for a repository from the local cache. Pass {\"repository\":\"repo-name\"} (1-100 chars, alnum/underscore/hyphen). Usage: " + docsToolsURI + "#view_repos-users.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"repository": {
					Type:        "string",
					Title:       "Repository Name",
					Description: "Repository name (1-100 chars, alnum/underscore/hyphen).",
					MinLength:   intPtr(v.RepoNameMin),
					MaxLength:   intPtr(v.RepoNameMax),
					Pattern:     v.RepoNamePattern,
				},
			},
			Required: []string{"repository"},
		},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in ViewRepoUsersIn) (*sdk.CallToolResult, ViewRepoUsersOut, error) {
		repo := strings.TrimSpace(in.Repository)
		if repo == "" {
			return &sdk.CallToolResult{}, ViewRepoUsersOut{}, fmt.Errorf("repository is required")
		}
		if err := v.ValidateRepoName(repo); err != nil {
			return &sdk.CallToolResult{}, ViewRepoUsersOut{}, err
		}
		out, err := listRepoUsers(repo)
		if err != nil {
			return &sdk.CallToolResult{}, ViewRepoUsersOut{}, fmt.Errorf("failed to list repository users: %w", err)
		}
		return nil, out, nil
	})

	// view_repos-teams {repository}
	sdk.AddTool[ViewRepoTeamsIn, ViewRepoTeamsOut](srv, &sdk.Tool{
		Name:        "view_repos-teams",
		Title:       "View Repository Teams",
		Description: "List teams with access to a repository from the local cache. Pass {\"repository\":\"repo-name\"} (1-100 chars, alnum/underscore/hyphen). Usage: " + docsToolsURI + "#view_repos-teams.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"repository": {
					Type:        "string",
					Title:       "Repository Name",
					Description: "Repository name (1-100 chars, alnum/underscore/hyphen).",
					MinLength:   intPtr(v.RepoNameMin),
					MaxLength:   intPtr(v.RepoNameMax),
					Pattern:     v.RepoNamePattern,
				},
			},
			Required: []string{"repository"},
		},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in ViewRepoTeamsIn) (*sdk.CallToolResult, ViewRepoTeamsOut, error) {
		repo := strings.TrimSpace(in.Repository)
		if repo == "" {
			return &sdk.CallToolResult{}, ViewRepoTeamsOut{}, fmt.Errorf("repository is required")
		}
		if err := v.ValidateRepoName(repo); err != nil {
			return &sdk.CallToolResult{}, ViewRepoTeamsOut{}, err
		}
		out, err := listRepoTeams(repo)
		if err != nil {
			return &sdk.CallToolResult{}, ViewRepoTeamsOut{}, fmt.Errorf("failed to list repository teams: %w", err)
		}
		return nil, out, nil
	})

	// view_repos-teams-users {repository}
	sdk.AddTool[ViewRepoTeamsUsersIn, ViewRepoTeamsUsersOut](srv, &sdk.Tool{
		Name:        "view_repos-teams-users",
		Title:       "View Repository Team Users",
		Description: "List members of teams linked to a repository from the local cache. Pass {\"repository\":\"repo-name\"} (1-100 chars, alnum/underscore/hyphen). Usage: " + docsToolsURI + "#view_repos-teams-users.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"repository": {
					Type:        "string",
					Title:       "Repository Name",
					Description: "Repository name (1-100 chars, alnum/underscore/hyphen).",
					MinLength:   intPtr(v.RepoNameMin),
					MaxLength:   intPtr(v.RepoNameMax),
					Pattern:     v.RepoNamePattern,
				},
			},
			Required: []string{"repository"},
		},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in ViewRepoTeamsUsersIn) (*sdk.CallToolResult, ViewRepoTeamsUsersOut, error) {
		repo := strings.TrimSpace(in.Repository)
		if repo == "" {
			return &sdk.CallToolResult{}, ViewRepoTeamsUsersOut{}, fmt.Errorf("repository is required")
		}
		if err := v.ValidateRepoName(repo); err != nil {
			return &sdk.CallToolResult{}, ViewRepoTeamsUsersOut{}, err
		}
		out, err := listRepoTeamsUsers(repo)
		if err != nil {
			return &sdk.CallToolResult{}, ViewRepoTeamsUsersOut{}, fmt.Errorf("failed to list repository team users: %w", err)
		}
		return nil, out, nil
	})

	// view_team-repos {team}
	sdk.AddTool[ViewTeamReposIn, ViewTeamReposOut](srv, &sdk.Tool{
		Name:        "view_team-repos",
		Title:       "View Team Repositories",
		Description: "List repositories a team can access from local database. Pass {\"team\":\"team-slug\"}. Usage: " + docsToolsURI + "#view_team-repos.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"team": {
					Type:        "string",
					Title:       "Team Slug",
					Description: "Team slug (lowercase alnum + hyphen).",
					MinLength:   intPtr(v.TeamSlugMin),
					MaxLength:   intPtr(v.TeamSlugMax),
					Pattern:     v.TeamSlugPattern,
				},
			},
			Required: []string{"team"},
		},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in ViewTeamReposIn) (*sdk.CallToolResult, ViewTeamReposOut, error) {
		team := strings.TrimSpace(in.Team)
		if team == "" {
			return &sdk.CallToolResult{}, ViewTeamReposOut{}, fmt.Errorf("team is required")
		}
		if err := v.ValidateTeamSlug(team); err != nil {
			return &sdk.CallToolResult{}, ViewTeamReposOut{}, err
		}
		out, err := listTeamRepositories(team)
		if err != nil {
			return &sdk.CallToolResult{}, ViewTeamReposOut{}, fmt.Errorf("failed to list team repositories: %w", err)
		}
		return nil, out, nil
	})

	// view_all-teams-users
	sdk.AddTool[struct{}, ViewAllTeamsUsersOut](srv, &sdk.Tool{
		Name:        "view_all-teams-users",
		Title:       "View All Team Memberships",
		Description: "Enumerate every team membership entry stored in the local database. Usage: " + docsToolsURI + "#view_all-teams-users.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewAllTeamsUsersOut, error) {
		entries, err := listAllTeamsUsers()
		if err != nil {
			return &sdk.CallToolResult{}, ViewAllTeamsUsersOut{}, fmt.Errorf("failed to list team memberships: %w", err)
		}
		return nil, ViewAllTeamsUsersOut{Entries: entries}, nil
	})

	// view_all-repos-users
	sdk.AddTool[struct{}, ViewAllReposUsersOut](srv, &sdk.Tool{
		Name:        "view_all-repos-users",
		Title:       "View All Repository Collaborators",
		Description: "Enumerate collaborators for every repository stored in the local database. Usage: " + docsToolsURI + "#view_all-repos-users.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewAllReposUsersOut, error) {
		entries, err := listAllRepositoriesUsers()
		if err != nil {
			return &sdk.CallToolResult{}, ViewAllReposUsersOut{}, fmt.Errorf("failed to list repository collaborators: %w", err)
		}
		return nil, ViewAllReposUsersOut{Entries: entries}, nil
	})

	// view_all-repos-teams
	sdk.AddTool[struct{}, ViewAllReposTeamsOut](srv, &sdk.Tool{
		Name:        "view_all-repos-teams",
		Title:       "View All Repository Teams",
		Description: "Enumerate team access for every repository stored in the local database. Usage: " + docsToolsURI + "#view_all-repos-teams.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewAllReposTeamsOut, error) {
		entries, err := listAllRepositoriesTeams()
		if err != nil {
			return &sdk.CallToolResult{}, ViewAllReposTeamsOut{}, fmt.Errorf("failed to list repository teams: %w", err)
		}
		return nil, ViewAllReposTeamsOut{Entries: entries}, nil
	})

	// view_user-repos {user}
	sdk.AddTool[ViewUserReposIn, ViewUserReposOut](srv, &sdk.Tool{
		Name:        "view_user-repos",
		Title:       "View User Repository Access",
		Description: "List repositories a user can access and how the access is granted. Pass {\"user\":\"github-login\"} (1-39 chars, alnum or hyphen). Usage: " + docsToolsURI + "#view_user-repos.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"user": {
					Type:        "string",
					Title:       "User Login",
					Description: "GitHub username (1-39 chars, alnum or hyphen).",
					MinLength:   intPtr(v.UserNameMin),
					MaxLength:   intPtr(v.UserNameMax),
					Pattern:     v.UserNamePattern,
				},
			},
			Required: []string{"user"},
		},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in ViewUserReposIn) (*sdk.CallToolResult, ViewUserReposOut, error) {
		login := strings.TrimSpace(in.User)
		if login == "" {
			return &sdk.CallToolResult{}, ViewUserReposOut{}, fmt.Errorf("user is required")
		}
		if err := v.ValidateUserName(login); err != nil {
			return &sdk.CallToolResult{}, ViewUserReposOut{}, err
		}
		out, err := listUserRepositories(login)
		if err != nil {
			return &sdk.CallToolResult{}, ViewUserReposOut{}, fmt.Errorf("failed to list user repositories: %w", err)
		}
		return nil, out, nil
	})

	// view_outside-users
	sdk.AddTool[struct{}, ViewOutsideUsersOut](srv, &sdk.Tool{
		Name:        "view_outside-users",
		Title:       "View Outside Collaborators",
		Description: "List outside collaborators from local database. Usage: " + docsToolsURI + "#view_outside-users.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewOutsideUsersOut, error) {
		users, err := listOutsideUsers()
		if err != nil {
			return &sdk.CallToolResult{}, ViewOutsideUsersOut{}, fmt.Errorf("failed to list outside users: %w", err)
		}
		return nil, ViewOutsideUsersOut{Users: users}, nil
	})

	// view_settings (masked configuration)
	sdk.AddTool[struct{}, ViewSettingsOut](srv, &sdk.Tool{
		Name:        "view_settings",
		Title:       "View Masked Settings",
		Description: "Show application configuration with secrets masked, useful for confirming MCP permissions. Usage: " + docsToolsURI + "#view_settings.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewSettingsOut, error) {
		out := maskConfig(cfg)
		return nil, out, nil
	})

	// view_token-permission
	sdk.AddTool[struct{}, ViewTokenPermissionOut](srv, &sdk.Tool{
		Name:        "view_token-permission",
		Title:       "View Token Permission",
		Description: "Show token permission info from local database. Usage: " + docsToolsURI + "#view_token-permission.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewTokenPermissionOut, error) {
		tp, err := getTokenPermission()
		if err != nil {
			return &sdk.CallToolResult{}, ViewTokenPermissionOut{}, fmt.Errorf("failed to get token permission: %w", err)
		}
		return nil, ViewTokenPermissionOut(tp), nil
	})

	// auditlogs (available by default)
	sdk.AddTool[AuditLogsIn, AuditLogsOut](srv, &sdk.Tool{
		Name:        "auditlogs",
		Title:       "Audit Logs",
		Description: "Fetch organization audit log entries by actor. Usage: " + docsToolsURI + "#auditlogs.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"user": {
					Type:        "string",
					Title:       "User Login",
					Description: "GitHub username (1-39 chars, alnum or hyphen).",
					MinLength:   intPtr(v.UserNameMin),
					MaxLength:   intPtr(v.UserNameMax),
					Pattern:     v.UserNamePattern,
				},
				"repo": {
					Type:        "string",
					Title:       "Repository Name",
					Description: "Optional repository name (1-100 chars, alnum/underscore/hyphen).",
					MinLength:   intPtr(v.RepoNameMin),
					MaxLength:   intPtr(v.RepoNameMax),
					Pattern:     v.RepoNamePattern,
				},
				"created": {
					Type:        "string",
					Title:       "Created Filter",
					Description: "Date filter (YYYY-MM-DD, >=YYYY-MM-DD, <=YYYY-MM-DD, or YYYY-MM-DD..YYYY-MM-DD). Defaults to last 30 days.",
				},
				"per_page": {
					Type:        "integer",
					Title:       "Per Page",
					Description: "Entries per page (max 100). Default is 100.",
					Minimum:     floatPtr(1),
					Maximum:     floatPtr(100),
				},
			},
			Required: []string{"user"},
		},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in AuditLogsIn) (*sdk.CallToolResult, AuditLogsOut, error) {
		perPage := in.PerPage
		if perPage == 0 {
			perPage = 100
		}
		if perPage < 0 {
			return &sdk.CallToolResult{}, AuditLogsOut{}, fmt.Errorf("per_page must be positive")
		}
		if perPage > 100 {
			return &sdk.CallToolResult{}, AuditLogsOut{}, fmt.Errorf("per_page must be 100 or less")
		}

		phrase, err := buildAuditLogPhrase(cfg.Organization, in.User, in.Repo, in.Created, time.Now())
		if err != nil {
			return &sdk.CallToolResult{}, AuditLogsOut{}, err
		}
		client, err := gh.InitClient(cfg)
		if err != nil {
			return &sdk.CallToolResult{}, AuditLogsOut{}, fmt.Errorf("github client init: %w", err)
		}
		opts := &ghapi.GetAuditLogOptions{
			Phrase: ghapi.String(phrase),
			ListCursorOptions: ghapi.ListCursorOptions{
				PerPage: perPage,
			},
		}
		entries, err := fetchAuditLogEntries(ctx, client, cfg.Organization, opts)
		if err != nil {
			return &sdk.CallToolResult{}, AuditLogsOut{}, fmt.Errorf("failed to fetch audit logs: %w", err)
		}
		normalized := normalizeAuditLogEntries(entries)
		return nil, AuditLogsOut{Count: len(normalized), Entries: normalized}, nil
	})

	// pull tools (non-destructive): gated by AllowPull
	if cfg.MCP.AllowPull {
		pullSchema := func(extra map[string]*jsonschema.Schema, required []string) *jsonschema.Schema {
			schema := &jsonschema.Schema{
				Type:       "object",
				Properties: pullOptionProperties(extra),
			}
			if len(required) > 0 {
				schema.Required = required
			}
			return schema
		}

		// pull_users
		sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
			Name:        "pull_users",
			Title:       "Pull Users",
			Description: "Fetch organization members from GitHub; optionally store them in SQLite. Usage: " + docsToolsURI + "#pull_users.",
			InputSchema: pullSchema(nil, nil),
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
			opts := resolvePullOptions(in.NoStore, in.Stdout, in.IntervalSeconds)
			if err := doPull(ctx, cfg, "users", opts, "", ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "users"}, nil
		})

		// pull_detail-users
		sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
			Name:        "pull_detail-users",
			Title:       "Pull Detailed Users",
			Description: "Fetch organization members with profile details; optionally store them in SQLite. Usage: " + docsToolsURI + "#pull_detail-users.",
			InputSchema: pullSchema(nil, nil),
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
			opts := resolvePullOptions(in.NoStore, in.Stdout, in.IntervalSeconds)
			if err := doPull(ctx, cfg, "detail-users", opts, "", ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "detail-users"}, nil
		})

		// pull_teams
		sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
			Name:        "pull_teams",
			Title:       "Pull Teams",
			Description: "Fetch organization teams from GitHub; optionally store them in SQLite. Usage: " + docsToolsURI + "#pull_teams.",
			InputSchema: pullSchema(nil, nil),
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
			opts := resolvePullOptions(in.NoStore, in.Stdout, in.IntervalSeconds)
			if err := doPull(ctx, cfg, "teams", opts, "", ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "teams"}, nil
		})

		// pull_repositories
		sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
			Name:        "pull_repositories",
			Title:       "Pull Repositories",
			Description: "Fetch repositories from GitHub; optionally store them in SQLite. Usage: " + docsToolsURI + "#pull_repositories.",
			InputSchema: pullSchema(nil, nil),
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
			opts := resolvePullOptions(in.NoStore, in.Stdout, in.IntervalSeconds)
			if err := doPull(ctx, cfg, "repos", opts, "", ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "repos"}, nil
		})

		// pull_all-teams-users
		sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
			Name:        "pull_all-teams-users",
			Title:       "Pull All Team Memberships",
			Description: "Fetch every team membership from GitHub; optionally store them in SQLite. Usage: " + docsToolsURI + "#pull_all-teams-users.",
			InputSchema: pullSchema(nil, nil),
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
			opts := resolvePullOptions(in.NoStore, in.Stdout, in.IntervalSeconds)
			if err := doPull(ctx, cfg, "all-teams-users", opts, "", ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "all-teams-users"}, nil
		})

		// pull_all-repos-users
		sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
			Name:        "pull_all-repos-users",
			Title:       "Pull All Repository Collaborators",
			Description: "Fetch collaborators for every repository; optionally store them in SQLite. Usage: " + docsToolsURI + "#pull_all-repos-users.",
			InputSchema: pullSchema(nil, nil),
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
			opts := resolvePullOptions(in.NoStore, in.Stdout, in.IntervalSeconds)
			if err := doPull(ctx, cfg, "all-repos-users", opts, "", ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "all-repos-users"}, nil
		})

		// pull_all-repos-teams
		sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
			Name:        "pull_all-repos-teams",
			Title:       "Pull All Repository Teams",
			Description: "Fetch team access for every repository; optionally store them in SQLite. Usage: " + docsToolsURI + "#pull_all-repos-teams.",
			InputSchema: pullSchema(nil, nil),
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
			opts := resolvePullOptions(in.NoStore, in.Stdout, in.IntervalSeconds)
			if err := doPull(ctx, cfg, "all-repos-teams", opts, "", ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "all-repos-teams"}, nil
		})

		// pull_team-user
		sdk.AddTool[PullTeamUsersIn, PullResult](srv, &sdk.Tool{
			Name:        "pull_team-user",
			Title:       "Pull Team Users",
			Description: "Fetch members for a specific team; optionally store them in SQLite. Provide {\"team\":\"team-slug\"} plus optional pull flags (no_store/stdout/interval_seconds). Usage: " + docsToolsURI + "#pull_team-user.",
			InputSchema: pullSchema(map[string]*jsonschema.Schema{
				"team": {
					Type:        "string",
					Title:       "Team Slug",
					Description: "Team slug (lowercase alnum + hyphen).",
					MinLength:   intPtr(v.TeamSlugMin),
					MaxLength:   intPtr(v.TeamSlugMax),
					Pattern:     v.TeamSlugPattern,
				},
			}, []string{"team"}),
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullTeamUsersIn) (*sdk.CallToolResult, PullResult, error) {
			team := strings.TrimSpace(in.Team)
			if team == "" {
				return &sdk.CallToolResult{}, PullResult{}, fmt.Errorf("team is required")
			}
			if err := v.ValidateTeamSlug(team); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			opts := resolvePullOptions(in.NoStore, in.Stdout, in.IntervalSeconds)
			if err := doPull(ctx, cfg, "team-user", opts, team, ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "team-user", Value: team}, nil
		})

		// pull_repos-users
		sdk.AddTool[PullRepoTargetIn, PullResult](srv, &sdk.Tool{
			Name:        "pull_repos-users",
			Title:       "Pull Repository Collaborators",
			Description: "Fetch direct collaborators for a repository; optionally store them in SQLite. Provide {\"repository\":\"repo-name\"} plus optional pull flags. Usage: " + docsToolsURI + "#pull_repos-users.",
			InputSchema: pullSchema(map[string]*jsonschema.Schema{
				"repository": {
					Type:        "string",
					Title:       "Repository Name",
					Description: "Repository name (1-100 chars, alnum/underscore/hyphen).",
					MinLength:   intPtr(v.RepoNameMin),
					MaxLength:   intPtr(v.RepoNameMax),
					Pattern:     v.RepoNamePattern,
				},
			}, []string{"repository"}),
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullRepoTargetIn) (*sdk.CallToolResult, PullResult, error) {
			repo := strings.TrimSpace(in.Repository)
			if repo == "" {
				return &sdk.CallToolResult{}, PullResult{}, fmt.Errorf("repository is required")
			}
			if err := v.ValidateRepoName(repo); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			opts := resolvePullOptions(in.NoStore, in.Stdout, in.IntervalSeconds)
			if err := doPull(ctx, cfg, "repos-users", opts, "", repo); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "repos-users", Value: repo}, nil
		})

		// pull_repos-teams
		sdk.AddTool[PullRepoTargetIn, PullResult](srv, &sdk.Tool{
			Name:        "pull_repos-teams",
			Title:       "Pull Repository Teams",
			Description: "Fetch team permissions for a repository; optionally store them in SQLite. Provide {\"repository\":\"repo-name\"} plus optional pull flags. Usage: " + docsToolsURI + "#pull_repos-teams.",
			InputSchema: pullSchema(map[string]*jsonschema.Schema{
				"repository": {
					Type:        "string",
					Title:       "Repository Name",
					Description: "Repository name (1-100 chars, alnum/underscore/hyphen).",
					MinLength:   intPtr(v.RepoNameMin),
					MaxLength:   intPtr(v.RepoNameMax),
					Pattern:     v.RepoNamePattern,
				},
			}, []string{"repository"}),
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullRepoTargetIn) (*sdk.CallToolResult, PullResult, error) {
			repo := strings.TrimSpace(in.Repository)
			if repo == "" {
				return &sdk.CallToolResult{}, PullResult{}, fmt.Errorf("repository is required")
			}
			if err := v.ValidateRepoName(repo); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			opts := resolvePullOptions(in.NoStore, in.Stdout, in.IntervalSeconds)
			if err := doPull(ctx, cfg, "repos-teams", opts, "", repo); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "repos-teams", Value: repo}, nil
		})

		// pull_outside-users
		sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
			Name:        "pull_outside-users",
			Title:       "Pull Outside Collaborators",
			Description: "Fetch outside collaborators; optionally store them in SQLite. Usage: " + docsToolsURI + "#pull_outside-users.",
			InputSchema: pullSchema(nil, nil),
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
			opts := resolvePullOptions(in.NoStore, in.Stdout, in.IntervalSeconds)
			if err := doPull(ctx, cfg, "outside-users", opts, "", ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "outside-users"}, nil
		})

		// pull_token-permission
		sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
			Name:        "pull_token-permission",
			Title:       "Pull Token Permission",
			Description: "Fetch GitHub token permission headers; optionally store them in SQLite. Usage: " + docsToolsURI + "#pull_token-permission.",
			InputSchema: pullSchema(nil, nil),
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
			opts := resolvePullOptions(in.NoStore, in.Stdout, in.IntervalSeconds)
			if err := doPull(ctx, cfg, "token-permission", opts, "", ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "token-permission"}, nil
		})
	}

	// Respect config permissions if needed in the future for additional tools.
	// For phase 1, only non-destructive tools are registered.

	if cfg.MCP.AllowWrite {
		sdk.AddTool[PushAddIn, PushResult](srv, &sdk.Tool{
			Name:        "push_add",
			Title:       "Push Add",
			Description: "Add users to teams or invite outside collaborators. Use team_user=\"team-slug/username\" or outside_user=\"repository/username\"; dry-run unless exec=true. Usage: " + docsToolsURI + "#push_add. Safety: " + docsSafetyURI + ".",
			InputSchema: &jsonschema.Schema{
				Type:        "object",
				Description: "Provide exactly one of team_user or outside_user.",
				Properties: map[string]*jsonschema.Schema{
					"team_user": {
						Type:        "string",
						Description: "Team/user pair in the form {team_slug}/{user_name}.",
					},
					"outside_user": {
						Type:        "string",
						Description: "Repository/user pair in the form {repository}/{user_name}.",
					},
					"permission": {
						Type:        "string",
						Description: "Optional permission for outside collaborators (pull|push|admin, aliases: read→pull, write→push).",
					},
					"exec": {
						Type:        "boolean",
						Description: "Execute add when true; otherwise dry run.",
					},
					"no_store": {
						Type:        "boolean",
						Description: "Skip local database update when true.",
					},
				},
			},
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PushAddIn) (*sdk.CallToolResult, PushResult, error) {
			target, value, permission, err := resolvePushAddInput(in)
			if err != nil {
				return &sdk.CallToolResult{}, PushResult{}, err
			}
			if !in.Exec {
				msg := fmt.Sprintf("DRYRUN: Would add %s '%s' to organization %s", target, value, cfg.Organization)
				if permission != "" {
					msg = fmt.Sprintf("DRYRUN: Would add %s '%s' (permission=%s) to organization %s", target, value, permission, cfg.Organization)
				}
				return nil, PushResult{Ok: true, Target: target, Value: value, Executed: false, Message: msg}, nil
			}
			if err := doPushAdd(ctx, cfg, target, value, permission, !in.NoStore); err != nil {
				return &sdk.CallToolResult{}, PushResult{}, err
			}
			msg := fmt.Sprintf("Added %s '%s' to organization %s", target, value, cfg.Organization)
			if permission != "" {
				msg = fmt.Sprintf("Added %s '%s' (permission=%s) to organization %s", target, value, permission, cfg.Organization)
			}
			return nil, PushResult{Ok: true, Target: target, Value: value, Executed: true, Message: msg}, nil
		})

		sdk.AddTool[PushRemoveIn, PushResult](srv, &sdk.Tool{
			Name:        "push_remove",
			Title:       "Push Remove",
			Description: "Remove teams, users, or collaborators. Choose one target (team, user, team_user, outside_user, repos_user); dry-run unless exec=true. Usage: " + docsToolsURI + "#push_remove. Safety: " + docsSafetyURI + ".",
			InputSchema: &jsonschema.Schema{
				Type:        "object",
				Description: "Provide exactly one removal target (team, user, team_user, outside_user, or repos_user).",
				Properties: map[string]*jsonschema.Schema{
					"team": {
						Type:        "string",
						Description: "Team slug to delete from the organization.",
						MinLength:   intPtr(v.TeamSlugMin),
						MaxLength:   intPtr(v.TeamSlugMax),
						Pattern:     v.TeamSlugPattern,
					},
					"user": {
						Type:        "string",
						Description: "Username to remove from the organization.",
						MinLength:   intPtr(v.UserNameMin),
						MaxLength:   intPtr(v.UserNameMax),
						Pattern:     v.UserNamePattern,
					},
					"team_user": {
						Type:        "string",
						Description: "Team/user pair in the form {team_slug}/{user_name}.",
					},
					"outside_user": {
						Type:        "string",
						Description: "Repository/user pair in the form {repository}/{user_name} (outside collaborator).",
					},
					"repos_user": {
						Type:        "string",
						Description: "Repository/user pair in the form {repository}/{user_name} (direct collaborator).",
					},
					"exec": {
						Type:        "boolean",
						Description: "Execute removal when true; otherwise dry run.",
					},
					"no_store": {
						Type:        "boolean",
						Description: "Skip local database update when true.",
					},
				},
			},
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PushRemoveIn) (*sdk.CallToolResult, PushResult, error) {
			target, value, err := resolvePushRemoveInput(in)
			if err != nil {
				return &sdk.CallToolResult{}, PushResult{}, err
			}
			if !in.Exec {
				msg := fmt.Sprintf("DRYRUN: Would remove %s '%s' from organization %s", target, value, cfg.Organization)
				return nil, PushResult{Ok: true, Target: target, Value: value, Executed: false, Message: msg}, nil
			}
			if err := doPushRemove(ctx, cfg, target, value, !in.NoStore); err != nil {
				return &sdk.CallToolResult{}, PushResult{}, err
			}
			msg := fmt.Sprintf("Removed %s '%s' from organization %s", target, value, cfg.Organization)
			return nil, PushResult{Ok: true, Target: target, Value: value, Executed: true, Message: msg}, nil
		})
	}

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

type HealthOut struct {
	Status string `json:"status" jsonschema:"health status (ok)"`
	Time   string `json:"time" jsonschema:"server time in RFC3339"`
}

type ViewUsersOut struct {
	Users []User `json:"users" jsonschema:"list of organization users"`
}

type User struct {
	ID       int64  `json:"id" jsonschema:"GitHub user ID"`
	Login    string `json:"login" jsonschema:"GitHub login"`
	Name     string `json:"name,omitempty" jsonschema:"display name"`
	Email    string `json:"email,omitempty" jsonschema:"email address (may be empty)"`
	Company  string `json:"company,omitempty" jsonschema:"company (may be empty)"`
	Location string `json:"location,omitempty" jsonschema:"location (may be empty)"`
}

type UserProfile struct {
	ID        int64  `json:"id" jsonschema:"GitHub user ID"`
	Login     string `json:"login" jsonschema:"GitHub login"`
	Name      string `json:"name,omitempty" jsonschema:"display name"`
	Email     string `json:"email,omitempty" jsonschema:"email address (may be empty)"`
	Company   string `json:"company,omitempty" jsonschema:"company (may be empty)"`
	Location  string `json:"location,omitempty" jsonschema:"location (may be empty)"`
	CreatedAt string `json:"created_at,omitempty" jsonschema:"record created at (local DB)"`
	UpdatedAt string `json:"updated_at,omitempty" jsonschema:"record updated at (local DB)"`
}

type ViewUserIn struct {
	User string `json:"user" jsonschema:"user login (1-39 chars, alnum or hyphen)"`
}

type ViewUserOut struct {
	Found bool        `json:"found" jsonschema:"true when user record exists"`
	User  UserProfile `json:"user" jsonschema:"user profile"`
}

func listUsers() ([]User, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	entries, err := store.FetchUsers(db)
	if err != nil {
		return nil, err
	}
	res := make([]User, 0, len(entries))
	for _, entry := range entries {
		res = append(res, User{
			ID:       entry.ID,
			Login:    entry.Login,
			Name:     entry.Name,
			Email:    entry.Email,
			Company:  entry.Company,
			Location: entry.Location,
		})
	}
	return res, nil
}

func getUserProfile(login string) (ViewUserOut, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return ViewUserOut{}, err
	}
	defer db.Close()

	rec, found, err := store.FetchUserProfile(db, login)
	if err != nil {
		return ViewUserOut{}, err
	}
	out := ViewUserOut{
		Found: found,
		User: UserProfile{
			ID:        rec.ID,
			Login:     rec.Login,
			Name:      rec.Name,
			Email:     rec.Email,
			Company:   rec.Company,
			Location:  rec.Location,
			CreatedAt: rec.CreatedAt,
			UpdatedAt: rec.UpdatedAt,
		},
	}
	return out, nil
}

type Team struct {
	ID          int64  `json:"id" jsonschema:"team ID"`
	Slug        string `json:"slug" jsonschema:"team slug (lowercase alnum + hyphen)"`
	Name        string `json:"name" jsonschema:"team name"`
	Description string `json:"description,omitempty" jsonschema:"team description"`
	Privacy     string `json:"privacy,omitempty" jsonschema:"team privacy (e.g., closed)"`
}

type ViewTeamsOut struct {
	Teams []Team `json:"teams"`
}

func listTeams() ([]Team, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	entries, err := store.FetchTeams(db)
	if err != nil {
		return nil, err
	}
	var res []Team
	for _, entry := range entries {
		res = append(res, Team{
			ID:          entry.ID,
			Slug:        entry.Slug,
			Name:        entry.Name,
			Description: entry.Description,
			Privacy:     entry.Privacy,
		})
	}
	return res, nil
}

type Repo struct {
	ID          int64  `json:"id" jsonschema:"repository ID"`
	Name        string `json:"name" jsonschema:"repository name"`
	FullName    string `json:"full_name" jsonschema:"full name (org/name)"`
	Description string `json:"description,omitempty" jsonschema:"repository description"`
	Private     bool   `json:"private" jsonschema:"is private"`
	Language    string `json:"language,omitempty" jsonschema:"primary language"`
	Stars       int    `json:"stargazers_count" jsonschema:"stars count"`
}

type ViewReposOut struct {
	Repositories []Repo `json:"repositories"`
}

func listRepositories() ([]Repo, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	entries, err := store.FetchRepositories(db)
	if err != nil {
		return nil, err
	}
	var res []Repo
	for _, entry := range entries {
		res = append(res, Repo{
			ID:          entry.ID,
			Name:        entry.Name,
			FullName:    entry.FullName,
			Description: entry.Description,
			Private:     entry.Private,
			Language:    entry.Language,
			Stars:       entry.Stars,
		})
	}
	return res, nil
}

type TeamUser struct {
	UserID int64  `json:"user_id" jsonschema:"user ID"`
	Login  string `json:"login" jsonschema:"user login"`
	Role   string `json:"role" jsonschema:"team role (e.g., member)"`
}

type ViewTeamUsersIn struct {
	Team string `json:"team" jsonschema:"team slug (lowercase alnum + hyphen)"`
}

type ViewTeamUsersOut struct {
	Team  string     `json:"team"`
	Users []TeamUser `json:"users"`
}

type ViewUserTeamsIn struct {
	User string `json:"user" jsonschema:"user login (1-39 chars, alnum or hyphen)"`
}

type UserTeam struct {
	TeamSlug string `json:"team_slug" jsonschema:"team slug"`
	TeamName string `json:"team_name" jsonschema:"team name"`
	Role     string `json:"role,omitempty" jsonschema:"membership role"`
}

type ViewUserTeamsOut struct {
	User  string     `json:"user"`
	Teams []UserTeam `json:"teams"`
}

func listTeamUsers(teamSlug string) ([]TeamUser, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	entries, err := store.FetchTeamUsers(db, teamSlug)
	if err != nil {
		return nil, err
	}
	var res []TeamUser
	for _, entry := range entries {
		res = append(res, TeamUser{
			UserID: entry.UserID,
			Login:  entry.Login,
			Role:   entry.Role,
		})
	}
	return res, nil
}

func listUserTeams(userLogin string) (ViewUserTeamsOut, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return ViewUserTeamsOut{}, err
	}
	defer db.Close()

	entries, err := store.FetchUserTeams(db, userLogin)
	if err != nil {
		return ViewUserTeamsOut{}, err
	}

	out := ViewUserTeamsOut{User: strings.TrimSpace(userLogin)}
	for _, entry := range entries {
		out.Teams = append(out.Teams, UserTeam{
			TeamSlug: entry.TeamSlug,
			TeamName: entry.TeamName,
			Role:     entry.Role,
		})
	}

	return out, nil
}

type RepoUser struct {
	UserID     int64  `json:"user_id" jsonschema:"user ID"`
	Login      string `json:"login" jsonschema:"user login"`
	Permission string `json:"permission,omitempty" jsonschema:"repository permission (normalized)"`
}

type ViewRepoUsersIn struct {
	Repository string `json:"repository" jsonschema:"repository name"`
}

type ViewRepoUsersOut struct {
	Repository string     `json:"repository"`
	FullName   string     `json:"full_name,omitempty"`
	Users      []RepoUser `json:"users"`
}

func listRepoUsers(repoName string) (ViewRepoUsersOut, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return ViewRepoUsersOut{}, err
	}
	defer db.Close()

	repoDisplay, fullName, entries, err := store.FetchRepoUsers(db, repoName)
	if err != nil {
		return ViewRepoUsersOut{}, err
	}

	out := ViewRepoUsersOut{
		Repository: repoDisplay,
		FullName:   fullName,
	}

	for _, entry := range entries {
		out.Users = append(out.Users, RepoUser{
			UserID:     entry.UserID,
			Login:      entry.Login,
			Permission: normalizePermissionValue(entry.Permission),
		})
	}

	return out, nil
}

type RepoTeam struct {
	ID          int64  `json:"id" jsonschema:"team ID"`
	Slug        string `json:"team_slug" jsonschema:"team slug"`
	Name        string `json:"team_name" jsonschema:"team display name"`
	Permission  string `json:"permission,omitempty" jsonschema:"repository permission"`
	Privacy     string `json:"privacy,omitempty" jsonschema:"team privacy"`
	Description string `json:"description,omitempty" jsonschema:"team description"`
}

type ViewRepoTeamsIn struct {
	Repository string `json:"repository" jsonschema:"repository name"`
}

type ViewRepoTeamsOut struct {
	Repository string     `json:"repository"`
	FullName   string     `json:"full_name,omitempty"`
	Teams      []RepoTeam `json:"teams"`
}

type RepoTeamUser struct {
	TeamSlug       string `json:"team_slug" jsonschema:"team slug"`
	TeamPermission string `json:"team_permission,omitempty" jsonschema:"permission granted to team on repository"`
	UserLogin      string `json:"user_login" jsonschema:"user login"`
	Role           string `json:"role,omitempty" jsonschema:"team membership role"`
	Name           string `json:"name,omitempty" jsonschema:"user display name"`
	Email          string `json:"email,omitempty" jsonschema:"user email"`
	Company        string `json:"company,omitempty" jsonschema:"user company"`
	Location       string `json:"location,omitempty" jsonschema:"user location"`
}

type ViewRepoTeamsUsersIn struct {
	Repository string `json:"repository" jsonschema:"repository name"`
}

type ViewRepoTeamsUsersOut struct {
	Repository string         `json:"repository"`
	FullName   string         `json:"full_name,omitempty"`
	Members    []RepoTeamUser `json:"members"`
}

type ViewTeamReposIn struct {
	Team string `json:"team" jsonschema:"team slug"`
}

type TeamRepository struct {
	RepoName    string `json:"repo_name" jsonschema:"repository name"`
	FullName    string `json:"full_name,omitempty" jsonschema:"repository full name"`
	Permission  string `json:"permission,omitempty" jsonschema:"permission granted to team"`
	Privacy     string `json:"privacy,omitempty" jsonschema:"team privacy"`
	Description string `json:"description,omitempty" jsonschema:"team description"`
}

type ViewTeamReposOut struct {
	Team         string           `json:"team"`
	Repositories []TeamRepository `json:"repositories"`
}

func listRepoTeams(repoName string) (ViewRepoTeamsOut, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return ViewRepoTeamsOut{}, err
	}
	defer db.Close()

	repoDisplay, fullName, entries, err := store.FetchRepoTeams(db, repoName)
	if err != nil {
		return ViewRepoTeamsOut{}, err
	}

	out := ViewRepoTeamsOut{
		Repository: repoDisplay,
		FullName:   fullName,
	}

	for _, entry := range entries {
		out.Teams = append(out.Teams, RepoTeam{
			ID:          entry.ID,
			Slug:        entry.Slug,
			Name:        entry.Name,
			Permission:  normalizePermissionValue(entry.Permission),
			Privacy:     entry.Privacy,
			Description: entry.Description,
		})
	}

	return out, nil
}

func listRepoTeamsUsers(repoName string) (ViewRepoTeamsUsersOut, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return ViewRepoTeamsUsersOut{}, err
	}
	defer db.Close()

	repoDisplay, fullName, entries, err := store.FetchRepoTeamUsers(db, repoName)
	if err != nil {
		return ViewRepoTeamsUsersOut{}, err
	}

	out := ViewRepoTeamsUsersOut{
		Repository: repoDisplay,
		FullName:   fullName,
	}
	for _, e := range entries {
		out.Members = append(out.Members, RepoTeamUser{
			TeamSlug:       e.TeamSlug,
			TeamPermission: normalizePermissionValue(e.TeamPermission),
			UserLogin:      e.UserLogin,
			Role:           e.Role,
			Name:           e.Name,
			Email:          e.Email,
			Company:        e.Company,
			Location:       e.Location,
		})
	}

	return out, nil
}

func listTeamRepositories(teamSlug string) (ViewTeamReposOut, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return ViewTeamReposOut{}, err
	}
	defer db.Close()

	entries, err := store.FetchTeamRepositories(db, teamSlug)
	if err != nil {
		return ViewTeamReposOut{}, err
	}

	out := ViewTeamReposOut{Team: strings.TrimSpace(teamSlug)}
	for _, entry := range entries {
		out.Repositories = append(out.Repositories, TeamRepository{
			RepoName:    entry.RepoName,
			FullName:    entry.FullName,
			Permission:  normalizePermissionValue(entry.Permission),
			Privacy:     entry.Privacy,
			Description: entry.Description,
		})
	}

	return out, nil
}

type AllTeamsUsersEntry struct {
	TeamSlug  string `json:"team_slug"`
	TeamName  string `json:"team_name"`
	UserLogin string `json:"user_login"`
	UserName  string `json:"user_name"`
	Role      string `json:"role"`
}

type ViewAllTeamsUsersOut struct {
	Entries []AllTeamsUsersEntry `json:"entries"`
}

func listAllTeamsUsers() ([]AllTeamsUsersEntry, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	entries, err := store.FetchAllTeamsUsers(db)
	if err != nil {
		return nil, err
	}

	out := make([]AllTeamsUsersEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, AllTeamsUsersEntry{
			TeamSlug:  entry.TeamSlug,
			TeamName:  entry.TeamName,
			UserLogin: entry.UserLogin,
			UserName:  entry.UserName,
			Role:      entry.Role,
		})
	}

	return out, nil
}

type AllReposUsersEntry struct {
	RepoName   string `json:"repo_name"`
	FullName   string `json:"full_name"`
	UserLogin  string `json:"user_login"`
	UserName   string `json:"user_name"`
	Permission string `json:"permission"`
}

type ViewAllReposUsersOut struct {
	Entries []AllReposUsersEntry `json:"entries"`
}

func listAllRepositoriesUsers() ([]AllReposUsersEntry, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	entries, err := store.FetchAllRepositoriesUsers(db)
	if err != nil {
		return nil, err
	}

	out := make([]AllReposUsersEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, AllReposUsersEntry{
			RepoName:   entry.RepoName,
			FullName:   entry.FullName,
			UserLogin:  entry.UserLogin,
			UserName:   entry.UserName,
			Permission: normalizePermissionValue(entry.Permission),
		})
	}

	return out, nil
}

type AllReposTeamsEntry struct {
	RepoName    string `json:"repo_name"`
	FullName    string `json:"full_name"`
	TeamSlug    string `json:"team_slug"`
	TeamName    string `json:"team_name"`
	Permission  string `json:"permission"`
	Privacy     string `json:"privacy"`
	Description string `json:"description"`
}

type ViewAllReposTeamsOut struct {
	Entries []AllReposTeamsEntry `json:"entries"`
}

func listAllRepositoriesTeams() ([]AllReposTeamsEntry, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	entries, err := store.FetchAllRepositoriesTeams(db)
	if err != nil {
		return nil, err
	}

	out := make([]AllReposTeamsEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, AllReposTeamsEntry{
			RepoName:    entry.RepoName,
			FullName:    entry.FullName,
			TeamSlug:    entry.TeamSlug,
			TeamName:    entry.TeamName,
			Permission:  normalizePermissionValue(entry.Permission),
			Privacy:     entry.Privacy,
			Description: entry.Description,
		})
	}

	return out, nil
}

type ViewUserReposIn struct {
	User string `json:"user" jsonschema:"user login"`
}

type UserRepoAccess struct {
	Repository string   `json:"repository"`
	AccessFrom []string `json:"access_from"`
	Permission string   `json:"permission"`
}

type ViewUserReposOut struct {
	User         string           `json:"user"`
	Repositories []UserRepoAccess `json:"repositories"`
}

func listUserRepositories(userLogin string) (ViewUserReposOut, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return ViewUserReposOut{}, err
	}
	defer db.Close()

	entries, err := store.FetchUserRepositories(db, userLogin)
	if err != nil {
		return ViewUserReposOut{}, err
	}

	cleanLogin := strings.TrimSpace(userLogin)
	output := make([]UserRepoAccess, 0, len(entries))
	for _, entry := range entries {
		output = append(output, UserRepoAccess{
			Repository: entry.Repository,
			AccessFrom: append([]string(nil), entry.AccessFrom...),
			Permission: entry.Permission,
		})
	}

	return ViewUserReposOut{User: cleanLogin, Repositories: output}, nil
}

type ViewSettingsOut struct {
	Organization string `json:"organization"`
	GitHubToken  string `json:"github_token"`
	GitHubApp    struct {
		AppID          int64  `json:"app_id"`
		InstallationID int64  `json:"installation_id"`
		PrivateKey     string `json:"private_key"`
	} `json:"github_app"`
	MCP struct {
		AllowPull  bool `json:"allow_pull"`
		AllowWrite bool `json:"allow_write"`
	} `json:"mcp"`
	DatabasePath string `json:"database_path"`
	SessionPath  string `json:"session_path"`
}

func maskConfig(cfg *appcfg.Config) ViewSettingsOut {
	if cfg == nil {
		return ViewSettingsOut{}
	}
	var out ViewSettingsOut
	out.Organization = cfg.Organization
	out.GitHubToken = maskSecret(cfg.GitHubToken)
	out.DatabasePath = cfg.DatabasePath
	out.SessionPath = cfg.SessionPath
	out.GitHubApp.AppID = cfg.GitHubApp.AppID
	out.GitHubApp.InstallationID = cfg.GitHubApp.InstallationID
	if cfg.GitHubApp.PrivateKey != "" {
		out.GitHubApp.PrivateKey = "[masked PEM]"
	}
	out.MCP.AllowPull = cfg.MCP.AllowPull
	out.MCP.AllowWrite = cfg.MCP.AllowWrite
	return out
}

func maskSecret(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) > 8 {
		return "[masked]…" + s[len(s)-4:]
	}
	return "[masked]"
}

type ViewOutsideUsersOut struct {
	Users []User `json:"users" jsonschema:"list of outside collaborators"`
}

func listOutsideUsers() ([]User, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	entries, err := store.FetchOutsideUsers(db)
	if err != nil {
		return nil, err
	}

	var res []User
	for _, entry := range entries {
		res = append(res, User{
			ID:       entry.ID,
			Login:    entry.Login,
			Name:     entry.Name,
			Email:    entry.Email,
			Company:  entry.Company,
			Location: entry.Location,
		})
	}
	return res, nil
}

type ViewTokenPermissionOut struct {
	OAuthScopes               string `json:"oauth_scopes" jsonschema:"X-OAuth-Scopes"`
	AcceptedOAuthScopes       string `json:"accepted_oauth_scopes" jsonschema:"X-Accepted-OAuth-Scopes"`
	AcceptedGitHubPermissions string `json:"accepted_github_permissions" jsonschema:"X-Accepted-GitHub-Permissions"`
	GitHubMediaType           string `json:"github_media_type" jsonschema:"X-GitHub-Media-Type"`
	RateLimit                 int    `json:"rate_limit" jsonschema:"rate limit"`
	RateRemaining             int    `json:"rate_remaining" jsonschema:"rate remaining"`
	RateReset                 int    `json:"rate_reset" jsonschema:"rate reset epoch"`
	CreatedAt                 string `json:"created_at" jsonschema:"record created at"`
	UpdatedAt                 string `json:"updated_at" jsonschema:"record updated at"`
}

func getTokenPermission() (ViewTokenPermissionOut, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return ViewTokenPermissionOut{}, err
	}
	defer db.Close()
	record, found, err := store.FetchTokenPermission(db)
	if err != nil {
		return ViewTokenPermissionOut{}, err
	}
	if !found {
		return ViewTokenPermissionOut{}, fmt.Errorf("no token permission data; run pull_token-permission with store=true first")
	}

	return ViewTokenPermissionOut{
		OAuthScopes:               record.OAuthScopes,
		AcceptedOAuthScopes:       record.AcceptedOAuthScopes,
		AcceptedGitHubPermissions: record.AcceptedGitHubPermissions,
		GitHubMediaType:           record.GitHubMediaType,
		RateLimit:                 record.RateLimit,
		RateRemaining:             record.RateRemaining,
		RateReset:                 record.RateReset,
		CreatedAt:                 record.CreatedAt,
		UpdatedAt:                 record.UpdatedAt,
	}, nil
}

type AuditLogsIn struct {
	User    string `json:"user"`
	Repo    string `json:"repo,omitempty"`
	Created string `json:"created,omitempty"`
	PerPage int    `json:"per_page,omitempty"`
}

type AuditLogsOut struct {
	Count   int             `json:"count"`
	Entries []AuditLogEntry `json:"entries"`
}

// Pull inputs/outputs
type PullCommonIn struct {
	NoStore         bool    `json:"no_store,omitempty"`
	Stdout          bool    `json:"stdout,omitempty"`
	IntervalSeconds float64 `json:"interval_seconds,omitempty"`
}

type PullTeamUsersIn struct {
	PullCommonIn
	Team string `json:"team"`
}

type PullRepoTargetIn struct {
	PullCommonIn
	Repository string `json:"repository"`
}

type PullResult struct {
	Ok     bool   `json:"ok"`
	Target string `json:"target"`
	Value  string `json:"value,omitempty"`
}

type PushAddIn struct {
	TeamUser    string `json:"team_user,omitempty"`
	OutsideUser string `json:"outside_user,omitempty"`
	Permission  string `json:"permission,omitempty"`
	Exec        bool   `json:"exec,omitempty"`
	NoStore     bool   `json:"no_store,omitempty"`
}

type PushRemoveIn struct {
	Team        string `json:"team,omitempty"`
	User        string `json:"user,omitempty"`
	TeamUser    string `json:"team_user,omitempty"`
	OutsideUser string `json:"outside_user,omitempty"`
	ReposUser   string `json:"repos_user,omitempty"`
	Exec        bool   `json:"exec,omitempty"`
	NoStore     bool   `json:"no_store,omitempty"`
}

type PushResult struct {
	Ok       bool   `json:"ok"`
	Target   string `json:"target,omitempty"`
	Value    string `json:"value,omitempty"`
	Executed bool   `json:"executed"`
	Message  string `json:"message,omitempty"`
}

// resolvePullOptions converts MCP inputs to GitHub pull options.
// Default is to save, disable only when `no_store` is specified. The interval is specified in seconds, with a default of 3 seconds.
func resolvePullOptions(noStore, stdout bool, intervalSeconds float64) gh.PullOptions {
	interval := defaultPullInterval
	if intervalSeconds > 0 {
		ms := math.Round(intervalSeconds * 1000)
		interval = time.Duration(ms) * time.Millisecond
	}
	return gh.PullOptions{
		Store:    !noStore,
		Stdout:   stdout,
		Interval: interval,
	}
}

func resolvePushAddInput(in PushAddIn) (string, string, string, error) {
	teamUser := strings.TrimSpace(in.TeamUser)
	outsideUser := strings.TrimSpace(in.OutsideUser)

	switch {
	case teamUser == "" && outsideUser == "":
		return "", "", "", fmt.Errorf("target required: specify team_user or outside_user")
	case teamUser != "" && outsideUser != "":
		return "", "", "", fmt.Errorf("target conflicted: specify only one of team_user or outside_user")
	}

	if teamUser != "" {
		if strings.TrimSpace(in.Permission) != "" {
			return "", "", "", fmt.Errorf("The permission flag can only be used with outside_user")
		}
		teamSlug, userName, err := v.ParseTeamUserPair(teamUser)
		if err != nil {
			return "", "", "", err
		}
		return "team-user", fmt.Sprintf("%s/%s", teamSlug, userName), "", nil
	}

	perm, err := normalizeOutsidePermission(in.Permission)
	if err != nil {
		return "", "", "", err
	}
	repoName, userLogin, err := v.ParseRepoUserPair(outsideUser)
	if err != nil {
		return "", "", "", err
	}
	return "outside-user", fmt.Sprintf("%s/%s", repoName, userLogin), perm, nil
}

func doPull(ctx context.Context, cfg *appcfg.Config, target string, opts gh.PullOptions, teamSlug, repoName string) error {
	client, err := gh.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client init: %w", err)
	}
	var db *sql.DB
	if opts.Store ||
		target == "all-teams-users" ||
		target == "all-repos-users" ||
		target == "all-repos-teams" {
		db, err = store.InitDatabase()
		if err != nil {
			return fmt.Errorf("db init: %w", err)
		}
		defer db.Close()
	}
	req := gh.TargetRequest{Kind: target}
	if teamSlug != "" {
		req.TeamSlug = teamSlug
	}
	if repoName != "" {
		req.RepoName = repoName
	}
	if opts.Interval <= 0 {
		opts.Interval = defaultPullInterval
	}
	return gh.HandlePullTarget(ctx, client, db, cfg.Organization, req, cfg.GitHubToken, opts)
}

func doPushAdd(ctx context.Context, cfg *appcfg.Config, target, value, permission string, storeResult bool) error {
	client, err := gh.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client init: %w", err)
	}
	if err := gh.ExecutePushAdd(ctx, client, cfg.Organization, target, value, permission); err != nil {
		return err
	}
	if !storeResult {
		return nil
	}
	db, err := store.InitDatabase()
	if err != nil {
		return fmt.Errorf("db init: %w", err)
	}
	defer db.Close()
	if err := gh.SyncPushAdd(ctx, client, db, cfg.Organization, target, value); err != nil {
		return fmt.Errorf("db sync: %w", err)
	}
	return nil
}

func resolvePushRemoveInput(in PushRemoveIn) (string, string, error) {
	var (
		target string
		value  string
		count  int
	)

	if team := strings.TrimSpace(in.Team); team != "" {
		if err := v.ValidateTeamSlug(team); err != nil {
			return "", "", err
		}
		target = "team"
		value = team
		count++
	}

	if user := strings.TrimSpace(in.User); user != "" {
		if err := v.ValidateUserName(user); err != nil {
			return "", "", err
		}
		target = "user"
		value = user
		count++
	}

	if pair := strings.TrimSpace(in.TeamUser); pair != "" {
		teamSlug, userName, err := v.ParseTeamUserPair(pair)
		if err != nil {
			return "", "", err
		}
		target = "team-user"
		value = fmt.Sprintf("%s/%s", teamSlug, userName)
		count++
	}

	if outside := strings.TrimSpace(in.OutsideUser); outside != "" {
		repoName, userLogin, err := v.ParseRepoUserPair(outside)
		if err != nil {
			return "", "", err
		}
		target = "outside-user"
		value = fmt.Sprintf("%s/%s", repoName, userLogin)
		count++
	}

	if repos := strings.TrimSpace(in.ReposUser); repos != "" {
		repoName, userLogin, err := v.ParseRepoUserPair(repos)
		if err != nil {
			return "", "", err
		}
		target = "repos-user"
		value = fmt.Sprintf("%s/%s", repoName, userLogin)
		count++
	}

	if count == 0 {
		return "", "", fmt.Errorf("Please specify one target (either --team, --user, --team-user, --outside-user, or --repos-user)")
	}
	if count > 1 {
		return "", "", fmt.Errorf("Please specify only one target (multiple selections are not allowed)")
	}
	return target, value, nil
}

func doPushRemove(ctx context.Context, cfg *appcfg.Config, target, value string, storeResult bool) error {
	client, err := gh.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client init: %w", err)
	}
	if err := gh.ExecutePushRemove(ctx, client, cfg.Organization, target, value); err != nil {
		return err
	}
	if !storeResult {
		return nil
	}
	db, err := store.InitDatabase()
	if err != nil {
		return fmt.Errorf("db init: %w", err)
	}
	defer db.Close()
	if err := gh.SyncPushRemove(ctx, client, db, cfg.Organization, target, value); err != nil {
		return fmt.Errorf("db sync: %w", err)
	}
	return nil
}

var permissionPriority = []string{"admin", "maintain", "push", "triage", "pull"}

func normalizePermissionValue(p string) string {
	return strings.ToLower(strings.TrimSpace(p))
}

func permissionRank(p string) int {
	for idx, key := range permissionPriority {
		if p == key {
			return idx
		}
	}
	return len(permissionPriority)
}

func maxPermission(current, candidate string) string {
	currentNorm := normalizePermissionValue(current)
	candidateNorm := normalizePermissionValue(candidate)

	if candidateNorm == "" {
		return currentNorm
	}
	if currentNorm == "" {
		return candidateNorm
	}
	if permissionRank(candidateNorm) < permissionRank(currentNorm) {
		return candidateNorm
	}
	return currentNorm
}

func normalizeOutsidePermission(p string) (string, error) {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return "", nil
	}
	switch val := strings.ToLower(trimmed); val {
	case "pull", "push", "admin":
		return val, nil
	case "read":
		return "pull", nil
	case "write":
		return "push", nil
	default:
		return "", fmt.Errorf("invalid permission for outside collaborator: choose from pull, push, admin (aliases: read, write)")
	}
}
