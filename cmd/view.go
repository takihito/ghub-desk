package cmd

import (
	"fmt"
	"strings"

	"ghub-desk/config"
	"ghub-desk/store"
)

// ViewCmd represents the view command structure
type ViewCmd struct {
	CommonTargetOptions `embed:""`
	Settings            bool   `name:"settings" help:"Show application settings (masked)"`
	Format              string `name:"format" default:"table" help:"Output format (table|json|yaml)"`
	TargetPath          string `arg:"" optional:"" help:"Target path (e.g. team-slug/users)."`
}

// Run implements the view command execution
func (v *ViewCmd) Run(cli *CLI) error {
	if v.TargetPath != "" {
		slug, err := parseTeamUsersPath(v.TargetPath)
		if err != nil {
			return err
		}
		if v.TeamUser != "" && v.TeamUser != slug {
			return fmt.Errorf("The team specified by the flag and the argument do not match")
		}
		v.TeamUser = slug
	}

	// Determine target from flags
	target, err := v.CommonTargetOptions.GetTarget(
		TargetFlag{Enabled: v.Settings, Name: "settings"},
	)
	if err != nil {
		return err
	}

	selectedFormat, err := store.ParseOutputFormat(v.Format)
	if err != nil {
		return err
	}

	cli.debugf("DEBUG: Viewing target='%s', format='%s'\n", target, selectedFormat)

	if target == "settings" {
		return ShowSettings(cli)
	}

	// Load config (non-validating) to optionally apply DB path without requiring auth
	if cfgNV, _ := config.LoadConfigNoValidate(cli.ConfigPath); cfgNV != nil && cfgNV.DatabasePath != "" {
		store.SetDBPath(cfgNV.DatabasePath)
	}
	// Initialize database for non-config views
	db, err := store.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	req := store.TargetRequest{Kind: target}
	switch target {
	case "team-user":
		if err := validateTeamName(v.TeamUser); err != nil {
			return err
		}
		req.TeamSlug = v.TeamUser
	case "user":
		if err := validateUserLogin(v.User); err != nil {
			return err
		}
		req.UserLogin = v.User
	case "user-teams":
		if err := validateUserLogin(v.UserTeams); err != nil {
			return err
		}
		req.UserLogin = v.UserTeams
	case "repos-users":
		if err := validateRepoName(v.RepoUsers); err != nil {
			return err
		}
		req.RepoName = v.RepoUsers
	case "repos-teams":
		if err := validateRepoName(v.RepoTeams); err != nil {
			return err
		}
		req.RepoName = v.RepoTeams
	case "repos-teams-users":
		if err := validateRepoName(v.RepoTeamsUsers); err != nil {
			return err
		}
		req.RepoName = v.RepoTeamsUsers
	case "user-repos":
		if err := validateUserLogin(v.UserRepos); err != nil {
			return err
		}
		req.UserLogin = v.UserRepos
	case "team-repos":
		if err := validateTeamName(v.TeamRepos); err != nil {
			return err
		}
		req.TeamSlug = v.TeamRepos
	}

	return store.HandleViewTarget(db, req, store.ViewOptions{Format: selectedFormat})
}

func parseTeamUsersPath(path string) (string, error) {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return "", fmt.Errorf("target argument is empty. Please specify in the format {team_slug}/users")
	}

	parts := strings.Split(cleaned, "/")
	if len(parts) != 2 || parts[1] != "users" {
		return "", fmt.Errorf("target must be in the format {team_slug}/users")
	}

	teamSlug := parts[0]
	if teamSlug == "" {
		return "", fmt.Errorf("team slug is empty. Please specify in the format {team_slug}/users")
	}

	return teamSlug, nil
}
