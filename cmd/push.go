package cmd

import (
	"context"
	"fmt"

	"ghub-desk/ghubclient"
	"ghub-desk/store"
)

// PushCmd represents the push command structure
type PushCmd struct {
	Remove RemoveCmd `cmd:"" help:"Remove resources from GitHub"`
	Add    AddCmd    `cmd:"" help:"Add resources to GitHub"`
}

// RemoveCmd represents the remove subcommand structure
type RemoveCmd struct {
	Exec        bool   `help:"Execute the operation (without this flag, runs in DRYRUN mode)"`
	Team        string `help:"Remove team from organization (team slug: 1–100 chars, lowercase alnum + hyphen)"`
	User        string `help:"Remove user from organization (username: 1–39 chars, alnum + hyphen, no leading/trailing hyphen)"`
	TeamUser    string `name:"team-user" help:"Remove user from team (format: team-slug/username)"`
	OutsideUser string `name:"outside-user" help:"Remove outside collaborator from repository (format: repo-name/username)"`
	ReposUser   string `name:"repos-user" help:"Remove repository collaborator (format: repo-name/username)"`
	NoStore     bool   `name:"no-store" help:"Do not update local SQLite database after executing the operation"`
}

// AddCmd represents the add subcommand structure
type AddCmd struct {
	Exec        bool   `help:"Execute the operation (without this flag, runs in DRYRUN mode)"`
	TeamUser    string `name:"team-user" help:"Add user to team (format: team-slug/username)"`
	OutsideUser string `name:"outside-user" help:"Invite outside collaborator to repository (format: repo-name/username)"`
	Permission  string `name:"permission" help:"Permission for outside collaborator (pull|push|admin, aliases: read→pull, write→push)."`
	NoStore     bool   `name:"no-store" help:"Do not update local SQLite database after executing the operation"`
}

// Run implements the remove subcommand execution
func (r *RemoveCmd) Run(cli *CLI) error {
	// Determine target from flags
	target, targetValue, err := r.getTarget()
	if err != nil {
		return err
	}

	cli.debugf("DEBUG: Push/Remove target='%s', value='%s', exec=%v\n", target, targetValue, r.Exec)

	// Load configuration once via CLI helper
	cfg, err := cli.Config()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	if cfg.DatabasePath != "" {
		store.SetDBPath(cfg.DatabasePath)
	}

	// Initialize GitHub client
	client, err := ghubclient.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client initialization error: %w", err)
	}
	ctx := context.Background()

	if r.Exec {
		fmt.Printf("Executing: Remove %s '%s' from organization %s\n", target, targetValue, cfg.Organization)
		err := ghubclient.ExecutePushRemove(ctx, client, cfg.Organization, target, targetValue)
		if err != nil {
			return fmt.Errorf("failed to execute remove: %w", err)
		}
		fmt.Println("Successfully removed.")
		if !r.NoStore {
			db, err := store.Connect()
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer db.Close()
			if err := ghubclient.SyncPushRemove(ctx, client, db, cfg.Organization, target, targetValue); err != nil {
				return fmt.Errorf("failed to update local database: %w", err)
			}
		}
	} else {
		fmt.Printf("DRYRUN: Would remove %s '%s' from organization %s\n", target, targetValue, cfg.Organization)
		fmt.Println("To execute, add the --exec flag.")
	}

	return nil
}

// getTarget returns the target and value based on the flags set for remove command
func (r *RemoveCmd) getTarget() (string, string, error) {
	targets := []struct {
		value string
		name  string
	}{
		{r.Team, "team"},
		{r.User, "user"},
		{r.TeamUser, "team-user"},
		{r.OutsideUser, "outside-user"},
		{r.ReposUser, "repos-user"},
	}

	var selectedTarget, selectedValue string
	var count int

	for _, t := range targets {
		if t.value != "" {
			count++
			selectedTarget = t.name
			selectedValue = t.value
		}
	}

	if count == 0 {
		return "", "", fmt.Errorf("target required: specify one of --team, --user, --team-user, --outside-user, --repos-user")
	}

	if count > 1 {
		return "", "", fmt.Errorf("only one target can be specified at a time")
	}

	// Validate argument formats
	switch selectedTarget {
	case "team":
		if err := validateTeamName(selectedValue); err != nil {
			return "", "", err
		}
	case "user":
		if err := validateUserName(selectedValue); err != nil {
			return "", "", err
		}
	case "team-user":
		if _, _, err := validateTeamUserPair(selectedValue); err != nil {
			return "", "", err
		}
	case "outside-user":
		if _, _, err := validateRepoUserPair(selectedValue); err != nil {
			return "", "", err
		}
	case "repos-user":
		if _, _, err := validateRepoUserPair(selectedValue); err != nil {
			return "", "", err
		}
	}

	return selectedTarget, selectedValue, nil
}

// Run implements the add subcommand execution
func (a *AddCmd) Run(cli *CLI) error {
	// Determine target from flags
	target, targetValue, permission, err := a.getTarget()
	if err != nil {
		return err
	}

	if permission != "" {
		cli.debugf("DEBUG: Push/Add target='%s', value='%s', permission='%s', exec=%v\n", target, targetValue, permission, a.Exec)
	} else {
		cli.debugf("DEBUG: Push/Add target='%s', value='%s', exec=%v\n", target, targetValue, a.Exec)
	}

	// Load configuration once via CLI helper
	cfg, err := cli.Config()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	if cfg.DatabasePath != "" {
		store.SetDBPath(cfg.DatabasePath)
	}

	// Initialize GitHub client
	client, err := ghubclient.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client initialization error: %w", err)
	}
	ctx := context.Background()

	if a.Exec {
		if permission != "" {
			fmt.Printf("Executing: Add %s '%s' (permission=%s) to organization %s\n", target, targetValue, permission, cfg.Organization)
		} else {
			fmt.Printf("Executing: Add %s '%s' to organization %s\n", target, targetValue, cfg.Organization)
		}
		err := ghubclient.ExecutePushAdd(ctx, client, cfg.Organization, target, targetValue, permission)
		if err != nil {
			return fmt.Errorf("failed to execute add: %w", err)
		}
		fmt.Println("Successfully added.")
		if !a.NoStore {
			db, err := store.Connect()
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer db.Close()
			if err := ghubclient.SyncPushAdd(ctx, client, db, cfg.Organization, target, targetValue); err != nil {
				return fmt.Errorf("failed to update local database: %w", err)
			}
		}
	} else {
		if permission != "" {
			fmt.Printf("DRYRUN: Would add %s '%s' (permission=%s) to organization %s\n", target, targetValue, permission, cfg.Organization)
		} else {
			fmt.Printf("DRYRUN: Would add %s '%s' to organization %s\n", target, targetValue, cfg.Organization)
		}
		fmt.Println("To execute, add the --exec flag.")
	}

	return nil
}

// getTarget returns the target and value based on the flags set for add command
func (a *AddCmd) getTarget() (string, string, string, error) {
	targets := []struct {
		value string
		name  string
	}{
		{a.TeamUser, "team-user"},
		{a.OutsideUser, "outside-user"},
	}

	var selectedTarget, selectedValue string
	var selectedPermission string
	var count int

	for _, t := range targets {
		if t.value != "" {
			count++
			selectedTarget = t.name
			selectedValue = t.value
		}
	}

	if count == 0 {
		return "", "", "", fmt.Errorf("target required: specify --team-user or --outside-user")
	}

	if count > 1 {
		return "", "", "", fmt.Errorf("only one target can be specified at a time")
	}

	switch selectedTarget {
	case "team-user":
		if a.Permission != "" {
			return "", "", "", fmt.Errorf("--permission can only be used with --outside-user")
		}
		if _, _, err := validateTeamUserPair(selectedValue); err != nil {
			return "", "", "", err
		}
	case "outside-user":
		if _, _, err := validateRepoUserPair(selectedValue); err != nil {
			return "", "", "", err
		}
		perm, err := validateOutsidePermission(a.Permission)
		if err != nil {
			return "", "", "", err
		}
		selectedPermission = perm
	}

	return selectedTarget, selectedValue, selectedPermission, nil
}
