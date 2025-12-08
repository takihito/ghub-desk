package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	appcfg "ghub-desk/config"
	gh "ghub-desk/github"
	"ghub-desk/store"
	v "ghub-desk/validate"

	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// defaultListLimit is the common LIMIT used for list views.
	defaultListLimit    = 200
	teamUsersListLimit  = 500
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
func Serve(ctx context.Context, cfg *appcfg.Config) error {
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
	return srv.Run(ctx, &sdk.StdioTransport{})
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
	rows, err := db.Query(`SELECT id, login, name, email, company, location FROM ghub_users ORDER BY login LIMIT ?`, defaultListLimit)
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

func getUserProfile(login string) (ViewUserOut, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return ViewUserOut{}, err
	}
	defer db.Close()

	cleanLogin := strings.TrimSpace(login)
	if cleanLogin == "" {
		return ViewUserOut{}, fmt.Errorf("user login is required")
	}

	query := `
		SELECT id, login, COALESCE(name, ''), COALESCE(email, ''), COALESCE(company, ''), COALESCE(location, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
		FROM ghub_users
		WHERE login = ?
		LIMIT 1`
	row := db.QueryRow(query, cleanLogin)

	var rec UserProfile
	if err := row.Scan(&rec.ID, &rec.Login, &rec.Name, &rec.Email, &rec.Company, &rec.Location, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return ViewUserOut{Found: false, User: UserProfile{Login: cleanLogin}}, nil
		}
		return ViewUserOut{}, err
	}

	rec.Login = strings.TrimSpace(rec.Login)
	rec.Name = strings.TrimSpace(rec.Name)
	rec.Email = strings.TrimSpace(rec.Email)
	rec.Company = strings.TrimSpace(rec.Company)
	rec.Location = strings.TrimSpace(rec.Location)
	rec.CreatedAt = strings.TrimSpace(rec.CreatedAt)
	rec.UpdatedAt = strings.TrimSpace(rec.UpdatedAt)

	return ViewUserOut{
		Found: true,
		User:  rec,
	}, nil
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
	rows, err := db.Query(`SELECT id, slug, name, description, privacy FROM ghub_teams ORDER BY slug LIMIT ?`, defaultListLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Team
	for rows.Next() {
		var id int64
		var slug, name, description, privacy sql.NullString
		if err := rows.Scan(&id, &slug, &name, &description, &privacy); err != nil {
			return nil, err
		}
		res = append(res, Team{ID: id, Slug: slug.String, Name: name.String, Description: description.String, Privacy: privacy.String})
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
	rows, err := db.Query(`SELECT id, name, full_name, description, private, language, stargazers_count FROM ghub_repos ORDER BY name LIMIT ?`, defaultListLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Repo
	for rows.Next() {
		var id int64
		var name, fullName, description, language sql.NullString
		var private bool
		var stars int
		if err := rows.Scan(&id, &name, &fullName, &description, &private, &language, &stars); err != nil {
			return nil, err
		}
		res = append(res, Repo{ID: id, Name: name.String, FullName: fullName.String, Description: description.String, Private: private, Language: language.String, Stars: stars})
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
	rows, err := db.Query(`SELECT ghub_user_id, user_login, role FROM ghub_team_users WHERE team_slug = ? ORDER BY user_login LIMIT ?`, teamSlug, teamUsersListLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []TeamUser
	for rows.Next() {
		var id int64
		var login, role sql.NullString
		if err := rows.Scan(&id, &login, &role); err != nil {
			return nil, err
		}
		res = append(res, TeamUser{UserID: id, Login: login.String, Role: role.String})
	}
	return res, nil
}

func listUserTeams(userLogin string) (ViewUserTeamsOut, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return ViewUserTeamsOut{}, err
	}
	defer db.Close()

	cleanLogin := strings.TrimSpace(userLogin)
	if cleanLogin == "" {
		return ViewUserTeamsOut{}, fmt.Errorf("user login is required")
	}

	rows, err := db.Query(`
		SELECT 
			tu.team_slug,
			COALESCE(t.name, '') AS team_name,
			COALESCE(tu.role, '') AS role
		FROM ghub_team_users tu
		LEFT JOIN ghub_teams t ON t.slug = tu.team_slug
		WHERE tu.user_login = ?
		ORDER BY LOWER(tu.team_slug)
	`, cleanLogin)
	if err != nil {
		return ViewUserTeamsOut{}, err
	}
	defer rows.Close()

	out := ViewUserTeamsOut{User: cleanLogin}
	for rows.Next() {
		var teamSlug, teamName, role sql.NullString
		if err := rows.Scan(&teamSlug, &teamName, &role); err != nil {
			return ViewUserTeamsOut{}, err
		}
		out.Teams = append(out.Teams, UserTeam{
			TeamSlug: strings.TrimSpace(teamSlug.String),
			TeamName: strings.TrimSpace(teamName.String),
			Role:     strings.TrimSpace(role.String),
		})
	}
	if err := rows.Err(); err != nil {
		return ViewUserTeamsOut{}, err
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

	out := ViewRepoUsersOut{Repository: repoName}

	var (
		displayName sql.NullString
		fullName    sql.NullString
	)
	err = db.QueryRow(`SELECT name, full_name FROM ghub_repos WHERE name = ? LIMIT 1`, repoName).Scan(&displayName, &fullName)
	if err != nil && err != sql.ErrNoRows {
		return ViewRepoUsersOut{}, err
	}
	if err == nil {
		if trimmed := strings.TrimSpace(displayName.String); trimmed != "" {
			out.Repository = trimmed
		}
		out.FullName = strings.TrimSpace(fullName.String)
	}

	rows, err := db.Query(`
		SELECT ghub_user_id, user_login, COALESCE(permission, '')
		FROM ghub_repos_users
		WHERE repos_name = ?
		ORDER BY user_login`, repoName)
	if err != nil {
		return ViewRepoUsersOut{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var userID sql.NullInt64
		var login, permission sql.NullString
		if err := rows.Scan(&userID, &login, &permission); err != nil {
			return ViewRepoUsersOut{}, err
		}
		out.Users = append(out.Users, RepoUser{
			UserID:     userID.Int64,
			Login:      strings.TrimSpace(login.String),
			Permission: normalizePermissionValue(permission.String),
		})
	}
	if err := rows.Err(); err != nil {
		return ViewRepoUsersOut{}, err
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

	out := ViewRepoTeamsOut{Repository: repoName}

	var (
		displayName sql.NullString
		fullName    sql.NullString
	)
	err = db.QueryRow(`SELECT name, full_name FROM ghub_repos WHERE name = ? LIMIT 1`, repoName).Scan(&displayName, &fullName)
	if err != nil && err != sql.ErrNoRows {
		return ViewRepoTeamsOut{}, err
	}
	if err == nil {
		if trimmed := strings.TrimSpace(displayName.String); trimmed != "" {
			out.Repository = trimmed
		}
		out.FullName = strings.TrimSpace(fullName.String)
	}

	rows, err := db.Query(`
		SELECT id, team_slug, team_name, COALESCE(permission, ''), COALESCE(privacy, ''), COALESCE(description, '')
		FROM ghub_repos_teams
		WHERE repos_name = ?
		ORDER BY team_slug`, repoName)
	if err != nil {
		return ViewRepoTeamsOut{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id          sql.NullInt64
			slug        sql.NullString
			name        sql.NullString
			permission  sql.NullString
			privacy     sql.NullString
			description sql.NullString
		)
		if err := rows.Scan(&id, &slug, &name, &permission, &privacy, &description); err != nil {
			return ViewRepoTeamsOut{}, err
		}
		out.Teams = append(out.Teams, RepoTeam{
			ID:          id.Int64,
			Slug:        strings.TrimSpace(slug.String),
			Name:        strings.TrimSpace(name.String),
			Permission:  normalizePermissionValue(permission.String),
			Privacy:     strings.TrimSpace(privacy.String),
			Description: strings.TrimSpace(description.String),
		})
	}
	if err := rows.Err(); err != nil {
		return ViewRepoTeamsOut{}, err
	}

	return out, nil
}

func listTeamRepositories(teamSlug string) (ViewTeamReposOut, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return ViewTeamReposOut{}, err
	}
	defer db.Close()

	cleanSlug := strings.TrimSpace(teamSlug)
	if cleanSlug == "" {
		return ViewTeamReposOut{}, fmt.Errorf("team slug is required")
	}

	rows, err := db.Query(`
		SELECT 
			COALESCE(r.name, rt.repos_name) AS repo_name,
			COALESCE(r.full_name, '') AS repo_full_name,
			COALESCE(rt.permission, '') AS permission,
			COALESCE(rt.privacy, '') AS privacy,
			COALESCE(rt.description, '') AS description,
			rt.repos_name
		FROM ghub_repos_teams rt
		LEFT JOIN ghub_repos r ON r.name = rt.repos_name
		WHERE rt.team_slug = ?
		ORDER BY LOWER(repo_name)
	`, cleanSlug)
	if err != nil {
		return ViewTeamReposOut{}, err
	}
	defer rows.Close()

	out := ViewTeamReposOut{Team: cleanSlug}
	for rows.Next() {
		var repoName, fullName, permission, privacy, description, fallbackRepo sql.NullString
		if err := rows.Scan(&repoName, &fullName, &permission, &privacy, &description, &fallbackRepo); err != nil {
			return ViewTeamReposOut{}, err
		}
		name := strings.TrimSpace(repoName.String)
		if name == "" {
			name = strings.TrimSpace(fallbackRepo.String)
		}
		out.Repositories = append(out.Repositories, TeamRepository{
			RepoName:    name,
			FullName:    strings.TrimSpace(fullName.String),
			Permission:  normalizePermissionValue(permission.String),
			Privacy:     strings.TrimSpace(privacy.String),
			Description: strings.TrimSpace(description.String),
		})
	}
	if err := rows.Err(); err != nil {
		return ViewTeamReposOut{}, err
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

	rows, err := db.Query(`
		SELECT 
			tu.team_slug,
			COALESCE(t.name, '') AS team_name,
			tu.user_login,
			COALESCE(u.name, '') AS user_name,
			COALESCE(tu.role, '') AS role
		FROM ghub_team_users tu
		LEFT JOIN ghub_teams t ON t.slug = tu.team_slug
		LEFT JOIN ghub_users u ON u.login = tu.user_login
		ORDER BY LOWER(tu.team_slug), LOWER(tu.user_login)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AllTeamsUsersEntry
	for rows.Next() {
		var teamSlug, teamName, login, userName, role sql.NullString
		if err := rows.Scan(&teamSlug, &teamName, &login, &userName, &role); err != nil {
			return nil, err
		}
		entries = append(entries, AllTeamsUsersEntry{
			TeamSlug:  strings.TrimSpace(teamSlug.String),
			TeamName:  strings.TrimSpace(teamName.String),
			UserLogin: strings.TrimSpace(login.String),
			UserName:  strings.TrimSpace(userName.String),
			Role:      strings.TrimSpace(role.String),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
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

	rows, err := db.Query(`
		SELECT 
			COALESCE(r.name, ru.repos_name) AS repo_name,
			COALESCE(r.full_name, '') AS repo_full_name,
			ru.user_login,
			COALESCE(u.name, '') AS user_name,
			COALESCE(ru.permission, '') AS permission
		FROM ghub_repos_users ru
		LEFT JOIN ghub_repos r ON r.name = ru.repos_name
		LEFT JOIN ghub_users u ON u.login = ru.user_login
		ORDER BY LOWER(repo_name), LOWER(ru.user_login)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AllReposUsersEntry
	for rows.Next() {
		var repoName, fullName, login, userName, permission sql.NullString
		if err := rows.Scan(&repoName, &fullName, &login, &userName, &permission); err != nil {
			return nil, err
		}
		entries = append(entries, AllReposUsersEntry{
			RepoName:   strings.TrimSpace(repoName.String),
			FullName:   strings.TrimSpace(fullName.String),
			UserLogin:  strings.TrimSpace(login.String),
			UserName:   strings.TrimSpace(userName.String),
			Permission: normalizePermissionValue(permission.String),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
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

	rows, err := db.Query(`
		SELECT 
			COALESCE(r.name, rt.repos_name) AS repo_name,
			COALESCE(r.full_name, '') AS repo_full_name,
			rt.team_slug,
			COALESCE(rt.team_name, '') AS team_name,
			COALESCE(rt.permission, '') AS permission,
			COALESCE(rt.privacy, '') AS privacy,
			COALESCE(rt.description, '') AS description
		FROM ghub_repos_teams rt
		LEFT JOIN ghub_repos r ON r.name = rt.repos_name
		ORDER BY LOWER(repo_name), LOWER(rt.team_slug)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AllReposTeamsEntry
	for rows.Next() {
		var repoName, fullName, teamSlug, teamName, permission, privacy, description sql.NullString
		if err := rows.Scan(&repoName, &fullName, &teamSlug, &teamName, &permission, &privacy, &description); err != nil {
			return nil, err
		}
		entries = append(entries, AllReposTeamsEntry{
			RepoName:    strings.TrimSpace(repoName.String),
			FullName:    strings.TrimSpace(fullName.String),
			TeamSlug:    strings.TrimSpace(teamSlug.String),
			TeamName:    strings.TrimSpace(teamName.String),
			Permission:  normalizePermissionValue(permission.String),
			Privacy:     strings.TrimSpace(privacy.String),
			Description: strings.TrimSpace(description.String),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
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

	cleanLogin := strings.TrimSpace(userLogin)
	if cleanLogin == "" {
		return ViewUserReposOut{}, fmt.Errorf("user login is required")
	}

	type repoAccessEntry struct {
		repoName string
		highest  string
		sources  []string
		seen     map[string]struct{}
	}

	accessByRepo := make(map[string]*repoAccessEntry)
	mergeRepoAccess := func(repoName, sourceLabel, permission string) {
		name := strings.TrimSpace(repoName)
		if name == "" {
			return
		}
		entry, ok := accessByRepo[name]
		if !ok {
			entry = &repoAccessEntry{
				repoName: name,
				highest:  "",
				sources:  make([]string, 0, 2),
				seen:     make(map[string]struct{}),
			}
			accessByRepo[name] = entry
		}
		entry.highest = maxPermission(entry.highest, permission)

		displayPerm := normalizePermissionValue(permission)
		display := sourceLabel
		if displayPerm != "" {
			display = fmt.Sprintf("%s [%s]", sourceLabel, displayPerm)
		}
		if _, exists := entry.seen[display]; !exists {
			entry.sources = append(entry.sources, display)
			entry.seen[display] = struct{}{}
		}
	}

	directRows, err := db.Query(`
		SELECT COALESCE(r.name, ru.repos_name) AS repo_name,
		       COALESCE(ru.permission, ''),
		       ru.repos_name
		FROM ghub_repos_users ru
		LEFT JOIN ghub_repos r ON r.name = ru.repos_name
		WHERE ru.user_login = ?
	`, cleanLogin)
	if err != nil {
		return ViewUserReposOut{}, err
	}
	defer directRows.Close()

	for directRows.Next() {
		var repoName, permission, fallback sql.NullString
		if err := directRows.Scan(&repoName, &permission, &fallback); err != nil {
			return ViewUserReposOut{}, err
		}
		name := strings.TrimSpace(repoName.String)
		if name == "" {
			name = strings.TrimSpace(fallback.String)
		}
		if name == "" {
			continue
		}
		mergeRepoAccess(name, "Direct", permission.String)
	}
	if err := directRows.Err(); err != nil {
		return ViewUserReposOut{}, err
	}

	teamRows, err := db.Query(`
		SELECT COALESCE(r.name, rt.repos_name) AS repo_name,
		       rt.team_slug,
		       COALESCE(rt.team_name, ''),
		       COALESCE(rt.permission, ''),
		       rt.repos_name
		FROM ghub_team_users tu
		JOIN ghub_repos_teams rt ON rt.team_slug = tu.team_slug
		LEFT JOIN ghub_repos r ON r.name = rt.repos_name
		WHERE tu.user_login = ?
	`, cleanLogin)
	if err != nil {
		return ViewUserReposOut{}, err
	}
	defer teamRows.Close()

	for teamRows.Next() {
		var repoName, teamSlug, teamName, permission, fallback sql.NullString
		if err := teamRows.Scan(&repoName, &teamSlug, &teamName, &permission, &fallback); err != nil {
			return ViewUserReposOut{}, err
		}
		name := strings.TrimSpace(repoName.String)
		if name == "" {
			name = strings.TrimSpace(fallback.String)
		}
		if name == "" {
			continue
		}
		slug := strings.TrimSpace(teamSlug.String)
		if slug == "" {
			continue
		}
		label := fmt.Sprintf("Team:%s", slug)
		if displayName := strings.TrimSpace(teamName.String); displayName != "" {
			label = fmt.Sprintf("%s (%s)", label, displayName)
		}
		mergeRepoAccess(name, label, permission.String)
	}
	if err := teamRows.Err(); err != nil {
		return ViewUserReposOut{}, err
	}

	if len(accessByRepo) == 0 {
		return ViewUserReposOut{User: cleanLogin, Repositories: []UserRepoAccess{}}, nil
	}

	entries := make([]*repoAccessEntry, 0, len(accessByRepo))
	for _, entry := range accessByRepo {
		sort.Slice(entry.sources, func(i, j int) bool {
			si := entry.sources[i]
			sj := entry.sources[j]
			isDirect := strings.HasPrefix(si, "Direct")
			jsDirect := strings.HasPrefix(sj, "Direct")
			if isDirect != jsDirect {
				return isDirect
			}
			return si < sj
		})
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		li := strings.ToLower(entries[i].repoName)
		lj := strings.ToLower(entries[j].repoName)
		if li == lj {
			return entries[i].repoName < entries[j].repoName
		}
		return li < lj
	})

	output := make([]UserRepoAccess, 0, len(entries))
	for _, entry := range entries {
		output = append(output, UserRepoAccess{
			Repository: entry.repoName,
			AccessFrom: append([]string(nil), entry.sources...),
			Permission: entry.highest,
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
	rows, err := db.Query(`SELECT id, login, name, email, company, location FROM ghub_outside_users ORDER BY login LIMIT ?`, defaultListLimit)
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
		res = append(res, User{ID: id, Login: login.String, Name: name.String, Email: email.String, Company: company.String, Location: location.String})
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
	row := db.QueryRow(`SELECT x_oauth_scopes, x_accepted_oauth_scopes, x_accepted_github_permissions, x_github_media_type, x_ratelimit_limit, x_ratelimit_remaining, x_ratelimit_reset, created_at, updated_at FROM ghub_token_permissions ORDER BY created_at DESC LIMIT 1`)
	var out ViewTokenPermissionOut
	if err := row.Scan(&out.OAuthScopes, &out.AcceptedOAuthScopes, &out.AcceptedGitHubPermissions, &out.GitHubMediaType, &out.RateLimit, &out.RateRemaining, &out.RateReset, &out.CreatedAt, &out.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return ViewTokenPermissionOut{}, fmt.Errorf("no token permission data; run pull_token-permission with store=true first")
		}
		return ViewTokenPermissionOut{}, err
	}
	return out, nil
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
