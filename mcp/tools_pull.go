package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	appcfg "ghub-desk/config"
	"ghub-desk/ghubclient"
	"ghub-desk/store"
	v "ghub-desk/validate"

	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultPullInterval = 3 * time.Second

// pullToolDefs lists every pull_* tool. These call the GitHub API (non-destructively) and
// are only registered when mcp.allow_pull is enabled.
var pullToolDefs = []toolDef{
	{name: "pull_users", tier: tierPull, register: registerPullUsersTool},
	{name: "pull_detail-users", tier: tierPull, register: registerPullDetailUsersTool},
	{name: "pull_teams", tier: tierPull, register: registerPullTeamsTool},
	{name: "pull_repositories", tier: tierPull, register: registerPullRepositoriesTool},
	{name: "pull_all-teams-users", tier: tierPull, register: registerPullAllTeamsUsersTool},
	{name: "pull_all-repos-users", tier: tierPull, register: registerPullAllReposUsersTool},
	{name: "pull_all-repos-teams", tier: tierPull, register: registerPullAllReposTeamsTool},
	{name: "pull_team-user", tier: tierPull, register: registerPullTeamUserTool},
	{name: "pull_repos-users", tier: tierPull, register: registerPullRepoUsersTool},
	{name: "pull_repos-teams", tier: tierPull, register: registerPullRepoTeamsTool},
	{name: "pull_outside-users", tier: tierPull, register: registerPullOutsideUsersTool},
	{name: "pull_token-permission", tier: tierPull, register: registerPullTokenPermissionTool},
	{name: "pull_org-plan", tier: tierPull, register: registerPullOrgPlanTool},
}

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

// pullSchema builds the input schema for a pull_* tool, layering tool-specific properties
// and required fields on top of the shared no_store/stdout/interval_seconds options.
func pullSchema(extra map[string]*jsonschema.Schema, required []string) *jsonschema.Schema {
	schema := &jsonschema.Schema{
		Type:       "object",
		Properties: pullOptionProperties(extra),
	}
	if len(required) > 0 {
		schema.Required = required
	}
	return schema
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

func registerPullUsersTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
		Name:        name,
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
}

func registerPullDetailUsersTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
		Name:        name,
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
}

func registerPullTeamsTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
		Name:        name,
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
}

func registerPullRepositoriesTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
		Name:        name,
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
}

func registerPullAllTeamsUsersTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
		Name:        name,
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
}

func registerPullAllReposUsersTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
		Name:        name,
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
}

func registerPullAllReposTeamsTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
		Name:        name,
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
}

func registerPullTeamUserTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PullTeamUsersIn, PullResult](srv, &sdk.Tool{
		Name:        name,
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
}

func registerPullRepoUsersTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PullRepoTargetIn, PullResult](srv, &sdk.Tool{
		Name:        name,
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
}

func registerPullRepoTeamsTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PullRepoTargetIn, PullResult](srv, &sdk.Tool{
		Name:        name,
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
}

func registerPullOutsideUsersTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
		Name:        name,
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
}

func registerPullTokenPermissionTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
		Name:        name,
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

func registerPullOrgPlanTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
		Name:        name,
		Title:       "Pull Organization Plan",
		Description: "Fetch the organization plan (seats and contract info); optionally store it in SQLite. Requires a token with organization member/admin access. Usage: " + docsToolsURI + "#pull_org-plan.",
		InputSchema: pullSchema(nil, nil),
	}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
		opts := resolvePullOptions(in.NoStore, in.Stdout, in.IntervalSeconds)
		if err := doPull(ctx, cfg, "org-plan", opts, "", ""); err != nil {
			return &sdk.CallToolResult{}, PullResult{}, err
		}
		return nil, PullResult{Ok: true, Target: "org-plan"}, nil
	})
}

// resolvePullOptions converts MCP inputs to GitHub pull options.
// Default is to save, disable only when `no_store` is specified. The interval is specified in seconds, with a default of 3 seconds.
func resolvePullOptions(noStore, stdout bool, intervalSeconds float64) ghubclient.PullOptions {
	interval := defaultPullInterval
	if intervalSeconds > 0 {
		ms := math.Round(intervalSeconds * 1000)
		interval = time.Duration(ms) * time.Millisecond
	}
	return ghubclient.PullOptions{
		Store:    !noStore,
		Stdout:   stdout,
		Interval: interval,
	}
}

func doPull(ctx context.Context, cfg *appcfg.Config, target string, opts ghubclient.PullOptions, teamSlug, repoName string) error {
	client, err := ghubclient.InitClient(cfg)
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
	req := ghubclient.TargetRequest{Kind: target}
	if teamSlug != "" {
		req.TeamSlug = teamSlug
	}
	if repoName != "" {
		req.RepoName = repoName
	}
	if opts.Interval <= 0 {
		opts.Interval = defaultPullInterval
	}
	return ghubclient.HandlePullTarget(ctx, client, db, cfg.Organization, req, opts)
}
