package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ghub-desk/config"
	"ghub-desk/github"
	"ghub-desk/store"

	"github.com/alecthomas/kong"
)

var (
	// Version information - set by version.go
	appVersion = "dev"
	appCommit  = "none"
	appDate    = "unknown"
)

// SetVersionInfo sets the version information
func SetVersionInfo(version, commit, date string) {
	appVersion = version
	appCommit = commit
	appDate = date
}

// CLI represents the command line interface structure using Kong
type CLI struct {
	Pull    PullCmd    `cmd:"" help:"Fetch data from GitHub API"`
	View    ViewCmd    `cmd:"" help:"Display data from local database"`
	Push    PushCmd    `cmd:"" help:"Remove resources from GitHub"`
	Init    InitCmd    `cmd:"" help:"Initialize local database tables"`
	Version VersionCmd `cmd:"" help:"Show version information"`
}

// PullCmd represents the pull command structure
type PullCmd struct {
	Store        bool          `help:"Save to local SQLite database"`
	IntervalTime time.Duration `help:"Sleep interval between API requests" default:"3s"`

	Users           bool   `help:"Fetch organization members (basic info)"`
	DetailUsers     bool   `name:"detail-users" help:"Fetch organization members with detailed information"`
	Teams           bool   `help:"Fetch organization teams"`
	Repos           bool   `help:"Fetch organization repositories"`
	TeamsUsers      string `name:"teams-users" help:"Fetch team members for specific team"`
	AllTeamsUsers   bool   `name:"all-teams-users" help:"Fetch all team users"`
	TokenPermission bool   `name:"token-permission" help:"Fetch GitHub token permissions"`
	OutsideUsers    bool   `name:"outside-users" help:"Fetch organization outside collaborators"`
}

// ViewCmd represents the view command structure
type ViewCmd struct {
	Users           bool   `help:"Display users from database"`
	DetailUsers     bool   `name:"detail-users" help:"Display users with detailed info from database"`
	Teams           bool   `help:"Display teams from database"`
	Repos           bool   `help:"Display repositories from database"`
	TeamsUsers      string `name:"teams-users" help:"Display team members for specific team"`
	TokenPermission bool   `name:"token-permission" help:"Display token permissions from database"`
	OutsideUsers    bool   `name:"outside-users" help:"Display outside collaborators from database"`
}

// PushCmd represents the push command structure
type PushCmd struct {
	Remove RemoveCmd `cmd:"" help:"Remove resources from GitHub"`
}

// RemoveCmd represents the remove subcommand structure
type RemoveCmd struct {
	Exec     bool   `help:"Execute the operation (without this flag, runs in DRYRUN mode)"`
	Team     string `help:"Remove team from organization"`
	User     string `help:"Remove user from organization"`
	TeamUser string `name:"team-user" help:"Remove user from team (format: team/user)"`
}

// InitCmd represents the init command structure
type InitCmd struct{}

// VersionCmd represents the version command structure
type VersionCmd struct{}

// Execute is the main entry point for all commands
func Execute() error {
	cli := &CLI{}

	ctx := kong.Parse(cli,
		kong.Name("ghub-desk"),
		kong.Description("GitHub Organization Management CLI Tool"),
		kong.Vars{
			"version": fmt.Sprintf("%s (%s, built %s)", appVersion, appCommit, appDate),
		},
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
	)

	return ctx.Run()
}

// Run implements the pull command execution
func (p *PullCmd) Run() error {
	// Determine target from flags
	target, err := p.getTarget()
	if err != nil {
		return err
	}

	fmt.Printf("DEBUG: target=%s, store=%v, interval=%v\n", target, p.Store, p.IntervalTime)

	// Load configuration from environment variables
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Initialize GitHub client
	ctx := context.Background()
	client := github.InitClient(cfg.GitHubToken)

	var db *sql.DB
	if p.Store || target == "all-teams-users" {
		db, err = store.InitDatabase()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()
	}

	// Handle different target types with appropriate data fetching
	finalTarget := target
	if target == "teams-users" {
		finalTarget = p.TeamsUsers + "/users"
	}

	return github.HandlePullTarget(ctx, client, db, cfg.Organization, finalTarget, cfg.GitHubToken, p.Store, p.IntervalTime)
}

// getTarget returns the target based on the flags set
func (p *PullCmd) getTarget() (string, error) {
	targets := []struct {
		flag   bool
		name   string
		custom string
	}{
		{p.Users, "users", ""},
		{p.DetailUsers, "detail-users", ""},
		{p.Teams, "teams", ""},
		{p.Repos, "repos", ""},
		{p.TeamsUsers != "", "teams-users", p.TeamsUsers},
		{p.AllTeamsUsers, "all-teams-users", ""},
		{p.TokenPermission, "token-permission", ""},
		{p.OutsideUsers, "outside-users", ""},
	}

	var selectedTarget string
	var count int

	for _, t := range targets {
		if t.flag {
			count++
			if t.custom != "" {
				selectedTarget = t.name
			} else {
				selectedTarget = t.name
			}
		}
	}

	if count == 0 {
		return "", fmt.Errorf("target required: specify one of --users, --detail-users, --teams, --repos, --teams-users, --all-teams-users, --token-permission, --outside-users")
	}

	if count > 1 {
		return "", fmt.Errorf("only one target can be specified at a time")
	}

	return selectedTarget, nil
}

// Run implements the view command execution
func (v *ViewCmd) Run() error {
	// Determine target from flags
	target, err := v.getTarget()
	if err != nil {
		return err
	}

	// Initialize database
	db, err := store.InitDatabase()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	// Handle different target types
	finalTarget := target
	if target == "teams-users" {
		finalTarget = v.TeamsUsers + "/users"
	}

	return store.HandleViewTarget(db, finalTarget)
}

// getTarget returns the target based on the flags set for view command
func (v *ViewCmd) getTarget() (string, error) {
	targets := []struct {
		flag   bool
		name   string
		custom string
	}{
		{v.Users, "users", ""},
		{v.DetailUsers, "detail-users", ""},
		{v.Teams, "teams", ""},
		{v.Repos, "repos", ""},
		{v.TeamsUsers != "", "teams-users", v.TeamsUsers},
		{v.TokenPermission, "token-permission", ""},
		{v.OutsideUsers, "outside-users", ""},
	}

	var selectedTarget string
	var count int

	for _, t := range targets {
		if t.flag {
			count++
			if t.custom != "" {
				selectedTarget = t.name
			} else {
				selectedTarget = t.name
			}
		}
	}

	if count == 0 {
		return "", fmt.Errorf("target required: specify one of --users, --detail-users, --teams, --repos, --teams-users, --token-permission, --outside-users")
	}

	if count > 1 {
		return "", fmt.Errorf("only one target can be specified at a time")
	}

	return selectedTarget, nil
}

// Run implements the remove subcommand execution
func (r *RemoveCmd) Run() error {
	// Determine target from flags
	target, targetValue, err := r.getTarget()
	if err != nil {
		return err
	}

	fmt.Printf("DEBUG: target=%s, value=%s, exec=%v\n", target, targetValue, r.Exec)

	// Load configuration
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Initialize GitHub client
	client := github.InitClient(cfg.GitHubToken)
	ctx := context.Background()

	if r.Exec {
		fmt.Printf("Executing: Remove %s '%s' from organization %s\n", target, targetValue, cfg.Organization)
		err := github.ExecutePushRemove(ctx, client, cfg.Organization, target, targetValue)
		if err != nil {
			return fmt.Errorf("failed to execute remove: %w", err)
		}
		fmt.Println("Successfully removed.")
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
		return "", "", fmt.Errorf("target required: specify one of --team, --user, --team-user")
	}

	if count > 1 {
		return "", "", fmt.Errorf("only one target can be specified at a time")
	}

	return selectedTarget, selectedValue, nil
}

// Run implements the init command execution
func (i *InitCmd) Run() error {
	db, err := store.InitDatabase()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	fmt.Println("Database initialization completed")
	return nil
}

// Run implements the version command execution
func (v *VersionCmd) Run() error {
	fmt.Printf("ghub-desk version %s\n", appVersion)
	fmt.Printf("commit: %s\n", appCommit)
	fmt.Printf("built at: %s\n", appDate)
	return nil
}
