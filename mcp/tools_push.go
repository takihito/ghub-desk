package mcp

import (
	"context"
	"fmt"
	"strings"

	appcfg "ghub-desk/config"
	"ghub-desk/ghubclient"
	"ghub-desk/store"
	v "ghub-desk/validate"

	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// writeToolDefs lists every push_* tool. These mutate GitHub state and are only
// registered when mcp.allow_write is enabled.
var writeToolDefs = []toolDef{
	{name: "push_add", tier: tierWrite, register: registerPushAddTool},
	{name: "push_remove", tier: tierWrite, register: registerPushRemoveTool},
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

func registerPushAddTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PushAddIn, any](srv, &sdk.Tool{
		Name:        name,
		Title:       "Push Add",
		Description: "Add users to teams or invite outside collaborators. Use team_user=\"team-slug/username\" or outside_user=\"repository/username\"; dry-run unless exec=true. Usage: " + docsToolsURI + ". Safety: " + docsSafetyURI + ".",
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
	}, func(ctx context.Context, req *sdk.CallToolRequest, in PushAddIn) (*sdk.CallToolResult, any, error) {
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
}

func registerPushRemoveTool(srv *sdk.Server, name string, cfg *appcfg.Config) {
	sdk.AddTool[PushRemoveIn, any](srv, &sdk.Tool{
		Name:        name,
		Title:       "Push Remove",
		Description: "Remove teams, users, or collaborators. Choose one target (team, user, team_user, outside_user, repos_user); dry-run unless exec=true. Usage: " + docsToolsURI + ". Safety: " + docsSafetyURI + ".",
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
	}, func(ctx context.Context, req *sdk.CallToolRequest, in PushRemoveIn) (*sdk.CallToolResult, any, error) {
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
			return "", "", "", fmt.Errorf("the permission flag can only be used with outside_user")
		}
		teamSlug, userName, err := v.ParseTeamUserPair(teamUser)
		if err != nil {
			return "", "", "", err
		}
		return "team-user", fmt.Sprintf("%s/%s", teamSlug, userName), "", nil
	}

	perm, err := v.NormalizeOutsidePermission(in.Permission)
	if err != nil {
		return "", "", "", err
	}
	repoName, userLogin, err := v.ParseRepoUserPair(outsideUser)
	if err != nil {
		return "", "", "", err
	}
	return "outside-user", fmt.Sprintf("%s/%s", repoName, userLogin), perm, nil
}

func doPushAdd(ctx context.Context, cfg *appcfg.Config, target, value, permission string, storeResult bool) error {
	client, err := ghubclient.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client init: %w", err)
	}
	if err := ghubclient.ExecutePushAdd(ctx, client, cfg.Organization, target, value, permission); err != nil {
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
	if err := ghubclient.SyncPushAdd(ctx, client, db, cfg.Organization, target, value); err != nil {
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
		return "", "", fmt.Errorf("please specify one target (either --team, --user, --team-user, --outside-user, or --repos-user)")
	}
	if count > 1 {
		return "", "", fmt.Errorf("please specify only one target (multiple selections are not allowed)")
	}
	return target, value, nil
}

func doPushRemove(ctx context.Context, cfg *appcfg.Config, target, value string, storeResult bool) error {
	client, err := ghubclient.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client init: %w", err)
	}
	if err := ghubclient.ExecutePushRemove(ctx, client, cfg.Organization, target, value); err != nil {
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
	if err := ghubclient.SyncPushRemove(ctx, client, db, cfg.Organization, target, value); err != nil {
		return fmt.Errorf("db sync: %w", err)
	}
	return nil
}
