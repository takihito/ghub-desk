package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	appcfg "ghub-desk/config"
	"ghub-desk/store"
	v "ghub-desk/validate"

	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// coreViewToolDefs lists the health check plus every read-only view_* tool. These are
// always registered regardless of MCP permission configuration.
var coreViewToolDefs = []toolDef{
	{name: "health", tier: tierCore, register: registerHealthTool},
	{name: "view_users", tier: tierCore, register: registerViewUsersTool},
	{name: "view_detail-users", tier: tierCore, register: registerViewDetailUsersTool},
	{name: "view_user", tier: tierCore, register: registerViewUserTool},
	{name: "view_user-teams", tier: tierCore, register: registerViewUserTeamsTool},
	{name: "view_teams", tier: tierCore, register: registerViewTeamsTool},
	{name: "view_repos", tier: tierCore, register: registerViewReposTool},
	{name: "view_team-user", tier: tierCore, register: registerViewTeamUserTool},
	{name: "view_repos-users", tier: tierCore, register: registerViewRepoUsersTool},
	{name: "view_repos-teams", tier: tierCore, register: registerViewRepoTeamsTool},
	{name: "view_repos-teams-users", tier: tierCore, register: registerViewRepoTeamsUsersTool},
	{name: "view_team-repos", tier: tierCore, register: registerViewTeamReposTool},
	{name: "view_all-teams-users", tier: tierCore, register: registerViewAllTeamsUsersTool},
	{name: "view_all-repos-users", tier: tierCore, register: registerViewAllReposUsersTool},
	{name: "view_all-repos-teams", tier: tierCore, register: registerViewAllReposTeamsTool},
	{name: "view_user-repos", tier: tierCore, register: registerViewUserReposTool},
	{name: "view_outside-users", tier: tierCore, register: registerViewOutsideUsersTool},
	{name: "view_settings", tier: tierCore, register: registerViewSettingsTool},
	{name: "view_token-permission", tier: tierCore, register: registerViewTokenPermissionTool},
	{name: "view_org-plan", tier: tierCore, register: registerViewOrgPlanTool},
}

type HealthOut struct {
	Status string `json:"status" jsonschema:"health status (ok)"`
	Time   string `json:"time" jsonschema:"server time in RFC3339"`
}

func registerHealthTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[struct{}, HealthOut](srv, &sdk.Tool{
		Name:        name,
		Title:       "Health Check",
		Description: "Returns server health status.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(_ context.Context, _ *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, HealthOut, error) {
		return nil, HealthOut{Status: "ok", Time: time.Now().UTC().Format(time.RFC3339)}, nil
	})
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

func registerViewUsersTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[struct{}, ViewUsersOut](srv, &sdk.Tool{
		Name:        name,
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
}

// registerViewDetailUsersTool exposes the same output shape as view_users for now.
func registerViewDetailUsersTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[struct{}, ViewUsersOut](srv, &sdk.Tool{
		Name:        name,
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

func registerViewUserTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[ViewUserIn, ViewUserOut](srv, &sdk.Tool{
		Name:        name,
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

func registerViewUserTeamsTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[ViewUserTeamsIn, ViewUserTeamsOut](srv, &sdk.Tool{
		Name:        name,
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

func registerViewTeamsTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[struct{}, ViewTeamsOut](srv, &sdk.Tool{
		Name:        name,
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

func registerViewReposTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[struct{}, ViewReposOut](srv, &sdk.Tool{
		Name:        name,
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

func registerViewTeamUserTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[ViewTeamUsersIn, ViewTeamUsersOut](srv, &sdk.Tool{
		Name:        name,
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

func registerViewRepoUsersTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[ViewRepoUsersIn, ViewRepoUsersOut](srv, &sdk.Tool{
		Name:        name,
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
			Permission: store.NormalizePermission(entry.Permission),
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

func registerViewRepoTeamsTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[ViewRepoTeamsIn, ViewRepoTeamsOut](srv, &sdk.Tool{
		Name:        name,
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

func registerViewRepoTeamsUsersTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[ViewRepoTeamsUsersIn, ViewRepoTeamsUsersOut](srv, &sdk.Tool{
		Name:        name,
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

func registerViewTeamReposTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[ViewTeamReposIn, ViewTeamReposOut](srv, &sdk.Tool{
		Name:        name,
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
			Permission:  store.NormalizePermission(entry.Permission),
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
			TeamPermission: store.NormalizePermission(e.TeamPermission),
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
			Permission:  store.NormalizePermission(entry.Permission),
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

func registerViewAllTeamsUsersTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[struct{}, ViewAllTeamsUsersOut](srv, &sdk.Tool{
		Name:        name,
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

func registerViewAllReposUsersTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[struct{}, ViewAllReposUsersOut](srv, &sdk.Tool{
		Name:        name,
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
			Permission: store.NormalizePermission(entry.Permission),
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

func registerViewAllReposTeamsTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[struct{}, ViewAllReposTeamsOut](srv, &sdk.Tool{
		Name:        name,
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
			Permission:  store.NormalizePermission(entry.Permission),
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

func registerViewUserReposTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[ViewUserReposIn, ViewUserReposOut](srv, &sdk.Tool{
		Name:        name,
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

type ViewOutsideUsersOut struct {
	Users []User `json:"users" jsonschema:"list of outside collaborators"`
}

func registerViewOutsideUsersTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[struct{}, ViewOutsideUsersOut](srv, &sdk.Tool{
		Name:        name,
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

func registerViewSettingsTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[struct{}, appcfg.Masked](srv, &sdk.Tool{
		Name:        name,
		Title:       "View Masked Settings",
		Description: "Show application configuration with secrets masked, useful for confirming MCP permissions. Usage: " + docsToolsURI + "#view_settings.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, appcfg.Masked, error) {
		return nil, appcfg.Mask(cfg), nil
	})
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

func registerViewTokenPermissionTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[struct{}, ViewTokenPermissionOut](srv, &sdk.Tool{
		Name:        name,
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
}

type ViewOrgPlanOut struct {
	Found         bool   `json:"found" jsonschema:"true when a cached organization plan snapshot exists"`
	Login         string `json:"login,omitempty" jsonschema:"organization login"`
	PlanName      string `json:"plan_name,omitempty" jsonschema:"contract plan name (e.g., free, team, enterprise)"`
	Seats         int    `json:"seats,omitempty" jsonschema:"contracted seats"`
	FilledSeats   int    `json:"filled_seats,omitempty" jsonschema:"seats currently in use"`
	PrivateRepos  int64  `json:"private_repos,omitempty" jsonschema:"private repository limit"`
	Collaborators int    `json:"collaborators,omitempty" jsonschema:"collaborator limit"`
	CreatedAt     string `json:"created_at,omitempty" jsonschema:"record created at"`
	UpdatedAt     string `json:"updated_at,omitempty" jsonschema:"record updated at"`
}

func registerViewOrgPlanTool(srv *sdk.Server, name string, _ *appcfg.Config) {
	sdk.AddTool[struct{}, ViewOrgPlanOut](srv, &sdk.Tool{
		Name:        name,
		Title:       "View Organization Plan",
		Description: "Show the cached organization plan (seats and contract info) from local database. Usage: " + docsToolsURI + "#view_org-plan.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, ViewOrgPlanOut, error) {
		out, err := getOrgPlan()
		if err != nil {
			return &sdk.CallToolResult{}, ViewOrgPlanOut{}, fmt.Errorf("failed to get organization plan: %w", err)
		}
		return nil, out, nil
	})
}

func getOrgPlan() (ViewOrgPlanOut, error) {
	db, err := store.InitDatabase()
	if err != nil {
		return ViewOrgPlanOut{}, err
	}
	defer db.Close()

	record, found, err := store.FetchOrgPlan(db)
	if err != nil {
		return ViewOrgPlanOut{}, err
	}
	if !found {
		return ViewOrgPlanOut{}, fmt.Errorf("no organization plan data; run pull_org-plan with store=true first")
	}

	return ViewOrgPlanOut{
		Found:         true,
		Login:         record.Login,
		PlanName:      record.PlanName,
		Seats:         record.Seats,
		FilledSeats:   record.FilledSeats,
		PrivateRepos:  record.PrivateRepos,
		Collaborators: record.Collaborators,
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	}, nil
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
