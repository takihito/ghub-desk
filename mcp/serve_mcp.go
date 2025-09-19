//go:build mcp_sdk

package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	appcfg "ghub-desk/config"
	gh "ghub-desk/github"
	"ghub-desk/store"

	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// defaultListLimit is the common LIMIT used for list views.
	defaultListLimit   = 200
	teamUsersListLimit = 500
)

var (
	reUser = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)
	reTeam = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)
)

const teamSlugPattern = "^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$"

func intPtr(i int) *int { return &i }

func validateTeamSlug(s string) error {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 100 || !reTeam.MatchString(s) {
		return fmt.Errorf("invalid team slug: lowercase alnum and hyphen only, no leading/trailing hyphen, length 1-100")
	}
	return nil
}

func validateUserName(s string) error {
	s = strings.TrimSpace(s)
	if !reUser.MatchString(s) {
		return fmt.Errorf("invalid username: 1-39 chars alnum or hyphen, no leading/trailing hyphen")
	}
	return nil
}

// Serve starts the MCP server using the go-sdk over stdio.
// Tools provided in phase 1:
// - health: simple readiness check
// - view.users: return users from local SQLite DB
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
	srv := sdk.NewServer(impl, &sdk.ServerOptions{HasTools: true})

	// health tool (no input)
	sdk.AddTool[struct{}, HealthOut](srv, &sdk.Tool{
		Name:        "health",
		Title:       "Health Check",
		Description: "Returns server health status.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(_ context.Context, _ *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, HealthOut, error) {
		return nil, HealthOut{Status: "ok", Time: time.Now().UTC().Format(time.RFC3339)}, nil
	})

	// view.users tool (no input for now)
	sdk.AddTool[struct{}, ViewUsersOut](srv, &sdk.Tool{
		Name:        "view.users",
		Title:       "View Users",
		Description: "List users from local database.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(_ context.Context, _ *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewUsersOut, error) {
		users, err := listUsers()
		if err != nil {
			// return as tool error (not protocol error)
			return &sdk.CallToolResult{}, ViewUsersOut{}, fmt.Errorf("failed to list users: %w", err)
		}
		return nil, ViewUsersOut{Users: users}, nil
	})

	// view.detail-users tool (same output shape as view.users for now)
	sdk.AddTool[struct{}, ViewUsersOut](srv, &sdk.Tool{
		Name:        "view.detail-users",
		Title:       "View Detail Users",
		Description: "List users with details from local database.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(_ context.Context, _ *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewUsersOut, error) {
		users, err := listUsers()
		if err != nil {
			return &sdk.CallToolResult{}, ViewUsersOut{}, fmt.Errorf("failed to list users: %w", err)
		}
		return nil, ViewUsersOut{Users: users}, nil
	})

	// view.teams
	sdk.AddTool[struct{}, ViewTeamsOut](srv, &sdk.Tool{
		Name:        "view.teams",
		Title:       "View Teams",
		Description: "List teams from local database.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewTeamsOut, error) {
		teams, err := listTeams()
		if err != nil {
			return &sdk.CallToolResult{}, ViewTeamsOut{}, fmt.Errorf("failed to list teams: %w", err)
		}
		return nil, ViewTeamsOut{Teams: teams}, nil
	})

	// view.repos
	sdk.AddTool[struct{}, ViewReposOut](srv, &sdk.Tool{
		Name:        "view.repos",
		Title:       "View Repositories",
		Description: "List repositories from local database.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewReposOut, error) {
		repos, err := listRepositories()
		if err != nil {
			return &sdk.CallToolResult{}, ViewReposOut{}, fmt.Errorf("failed to list repositories: %w", err)
		}
		return nil, ViewReposOut{Repositories: repos}, nil
	})

	// view.team-users {team}
	sdk.AddTool[ViewTeamUsersIn, ViewTeamUsersOut](srv, &sdk.Tool{
		Name:        "view.team-users",
		Title:       "View Team Users",
		Description: "List users in a specific team from local database.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"team": {
					Type:        "string",
					Title:       "Team Slug",
					Description: "team slug (lowercase alnum + hyphen)",
					MinLength:   intPtr(1),
					MaxLength:   intPtr(100),
					Pattern:     teamSlugPattern,
				},
			},
			Required: []string{"team"},
		},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in ViewTeamUsersIn) (*sdk.CallToolResult, ViewTeamUsersOut, error) {
		if in.Team == "" {
			return &sdk.CallToolResult{}, ViewTeamUsersOut{}, fmt.Errorf("team is required")
		}
		if err := validateTeamSlug(in.Team); err != nil {
			return &sdk.CallToolResult{}, ViewTeamUsersOut{}, err
		}
		users, err := listTeamUsers(in.Team)
		if err != nil {
			return &sdk.CallToolResult{}, ViewTeamUsersOut{}, fmt.Errorf("failed to list team users: %w", err)
		}
		return nil, ViewTeamUsersOut{Team: in.Team, Users: users}, nil
	})

	// view.outside-users
	sdk.AddTool[struct{}, ViewOutsideUsersOut](srv, &sdk.Tool{
		Name:        "view.outside-users",
		Title:       "View Outside Collaborators",
		Description: "List outside collaborators from local database.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewOutsideUsersOut, error) {
		users, err := listOutsideUsers()
		if err != nil {
			return &sdk.CallToolResult{}, ViewOutsideUsersOut{}, fmt.Errorf("failed to list outside users: %w", err)
		}
		return nil, ViewOutsideUsersOut{Users: users}, nil
	})

	// view.token-permission
	sdk.AddTool[struct{}, ViewTokenPermissionOut](srv, &sdk.Tool{
		Name:        "view.token-permission",
		Title:       "View Token Permission",
		Description: "Show token permission info from local database.",
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
		// pull.users {store?:bool, detail?:bool}
		sdk.AddTool[PullUsersIn, PullResult](srv, &sdk.Tool{
			Name:        "pull.users",
			Title:       "Pull Users",
			Description: "Fetch org users from GitHub; optionally store in DB.",
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullUsersIn) (*sdk.CallToolResult, PullResult, error) {
			target := "users"
			if in.Detail {
				target = "detail-users"
			}
			if err := doPull(ctx, cfg, target, in.Store, ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: target}, nil
		})

		// pull.teams {store?:bool}
		sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
			Name:        "pull.teams",
			Title:       "Pull Teams",
			Description: "Fetch teams from GitHub; optionally store in DB.",
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
			if err := doPull(ctx, cfg, "teams", in.Store, ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "teams"}, nil
		})

		// pull.repositories {store?:bool}
		sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
			Name:        "pull.repositories",
			Title:       "Pull Repositories",
			Description: "Fetch repositories from GitHub; optionally store in DB.",
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
			if err := doPull(ctx, cfg, "repos", in.Store, ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "repos"}, nil
		})

		// pull.team-users {team:string, store?:bool}
		sdk.AddTool[PullTeamUsersIn, PullResult](srv, &sdk.Tool{
			Name:        "pull.team-users",
			Title:       "Pull Team Users",
			Description: "Fetch users in a team from GitHub; optionally store in DB.",
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullTeamUsersIn) (*sdk.CallToolResult, PullResult, error) {
			if in.Team == "" {
				return &sdk.CallToolResult{}, PullResult{}, fmt.Errorf("team is required")
			}
			if err := validateTeamSlug(in.Team); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			if err := doPull(ctx, cfg, "teams-users", in.Store, in.Team); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "teams-users"}, nil
		})

		// pull.outside-users {store?:bool}
		sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
			Name:        "pull.outside-users",
			Title:       "Pull Outside Users",
			Description: "Fetch outside collaborators; optionally store in DB.",
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
			if err := doPull(ctx, cfg, "outside-users", in.Store, ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "outside-users"}, nil
		})

		// pull.token-permission {store?:bool}
		sdk.AddTool[PullCommonIn, PullResult](srv, &sdk.Tool{
			Name:        "pull.token-permission",
			Title:       "Pull Token Permission",
			Description: "Fetch token permission info; optionally store in DB.",
		}, func(ctx context.Context, req *sdk.CallToolRequest, in PullCommonIn) (*sdk.CallToolResult, PullResult, error) {
			if err := doPull(ctx, cfg, "token-permission", in.Store, ""); err != nil {
				return &sdk.CallToolResult{}, PullResult{}, err
			}
			return nil, PullResult{Ok: true, Target: "token-permission"}, nil
		})
	}

	// Respect config permissions if needed in the future for additional tools.
	// For phase 1, only non-destructive tools are registered.

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
	rows, err := db.Query(`SELECT id, name, full_name, description, private, language, stargazers_count FROM ghub_repositories ORDER BY name LIMIT ?`, defaultListLimit)
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

func listTeamUsers(teamSlug string) ([]TeamUser, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT user_id, user_login, role FROM ghub_team_users WHERE team_slug = ? ORDER BY user_login LIMIT ?`, teamSlug, teamUsersListLimit)
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
			return ViewTokenPermissionOut{}, fmt.Errorf("no token permission data; run pull.token-permission with store=true first")
		}
		return ViewTokenPermissionOut{}, err
	}
	return out, nil
}

// Pull inputs/outputs
type PullCommonIn struct {
	Store bool `json:"store,omitempty"`
}

type PullUsersIn struct {
	Store  bool `json:"store,omitempty"`
	Detail bool `json:"detail,omitempty"`
}

type PullTeamUsersIn struct {
	Team  string `json:"team"`
	Store bool   `json:"store,omitempty"`
}

type PullResult struct {
	Ok     bool   `json:"ok"`
	Target string `json:"target"`
}

func doPull(ctx context.Context, cfg *appcfg.Config, target string, storeData bool, teamSlug string) error {
	client, err := gh.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client init: %w", err)
	}
	var db *sql.DB
	if storeData {
		db, err = store.InitDatabase()
		if err != nil {
			return fmt.Errorf("db init: %w", err)
		}
		defer db.Close()
	}
	finalTarget := target
	if target == "teams-users" && teamSlug != "" {
		finalTarget = teamSlug + "/users"
	}
	return gh.HandlePullTarget(ctx, client, db, cfg.Organization, finalTarget, cfg.GitHubToken, storeData, gh.DefaultSleep)
}
