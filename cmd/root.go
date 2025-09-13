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
	Debug      bool   `help:"Enable debug mode."`
	ConfigPath string `name:"config" short:"c" help:"Path to config file." type:"path"`

	Pull    PullCmd    `cmd:"" help:"Fetch data from GitHub API"`
	View    ViewCmd    `cmd:"" help:"Display data from local database"`
	Push    PushCmd    `cmd:"" help:"Manipulate resources on GitHub"`
	Init    InitCmd    `cmd:"" help:"Initialize local database tables"`
	Version VersionCmd `cmd:"" help:"Show version information"`
}

// CommonTargetOptions holds the shared target flags for pull and view commands
type CommonTargetOptions struct {
	Users           bool   `help:"Target: users"`
	DetailUsers     bool   `name:"detail-users" help:"Target: detail-users"`
	Teams           bool   `help:"Target: teams"`
	Repos           bool   `help:"Target: repos"`
	TeamsUsers      string `name:"teams-users" help:"Target: team-users (provide team slug)"`
	TokenPermission bool   `name:"token-permission" help:"Target: token-permission"`
	OutsideUsers    bool   `name:"outside-users" help:"Target: outside-users"`
}

// GetTarget determines the single selected target from the common options.
func (c *CommonTargetOptions) GetTarget(extraTargets ...struct {
	flag bool
	name string
}) (string, error) {
	targets := []struct {
		flag bool
		name string
	}{
		{c.Users, "users"},
		{c.DetailUsers, "detail-users"},
		{c.Teams, "teams"},
		{c.Repos, "repos"},
		{c.TeamsUsers != "", "teams-users"},
		{c.TokenPermission, "token-permission"},
		{c.OutsideUsers, "outside-users"},
	}
	targets = append(targets, extraTargets...)

	var selectedTarget string
	count := 0
	for _, t := range targets {
		if t.flag {
			count++
			selectedTarget = t.name
		}
	}

	if count == 0 {
		return "", fmt.Errorf("at least one target flag must be specified")
	}
	if count > 1 {
		return "", fmt.Errorf("only one target can be specified at a time")
	}
	return selectedTarget, nil
}

// PullCmd represents the pull command structure
type PullCmd struct {
	CommonTargetOptions `embed:""`
	AllTeamsUsers       bool `name:"all-teams-users" help:"Target: all-teams-users"`

	// Options
	Store        bool          `help:"Save to local SQLite database"`
	IntervalTime time.Duration `help:"Sleep interval between API requests" default:"3s"`
}

// ViewCmd represents the view command structure
type ViewCmd struct {
	CommonTargetOptions `embed:""`
}

// PushCmd represents the push command structure
type PushCmd struct {
	Remove RemoveCmd `cmd:"" help:"Remove resources from GitHub"`
	Add    AddCmd    `cmd:"" help:"Add resources to GitHub"`
}

// RemoveCmd represents the remove subcommand structure
type RemoveCmd struct {
	Exec     bool   `help:"Execute the operation (without this flag, runs in DRYRUN mode)"`
	Team     string `help:"Remove team from organization"`
	User     string `help:"Remove user from organization"`
	TeamUser string `name:"team-user" help:"Remove user from team (format: team/user)"`
}

// AddCmd represents the add subcommand structure
type AddCmd struct {
	Exec     bool   `help:"Execute the operation (without this flag, runs in DRYRUN mode)"`
	TeamUser string `name:"team-user" help:"Add user to team (format: team/user)"`
}

// InitCmd represents the init command structure
type InitCmd struct{}

// VersionCmd represents the version command structure
type VersionCmd struct{}

// Execute is the main entry point for all commands
func Execute() error {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("ghub-desk"),
		kong.Description("GitHub Organization Management CLI Tool"),
		kong.Vars{
			"version": fmt.Sprintf("%s (%s, built %s)", appVersion, appCommit, appDate),
		},
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
	)

	return ctx.Run(&cli)
}

// Run implements the pull command execution
func (p *PullCmd) Run(cli *CLI) error {
	// Determine target from flags
	target, err := p.CommonTargetOptions.GetTarget(struct {
		flag bool
		name string
	}{p.AllTeamsUsers, "all-teams-users"})
	if err != nil {
		return err
	}

	if cli.Debug {
		fmt.Printf("DEBUG: Pulling target='%s', store=%v, interval=%v\n", target, p.Store, p.IntervalTime)
	}

	// Load configuration
	config.Debug = cli.Debug
	cfg, err := config.GetConfig(cli.ConfigPath)
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Initialize GitHub client
	client, err := github.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client initialization error: %w", err)
	}
	ctx := context.Background()

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

// Run implements the view command execution
func (v *ViewCmd) Run(cli *CLI) error {
	// Determine target from flags
	target, err := v.CommonTargetOptions.GetTarget()
	if err != nil {
		return err
	}

	if cli.Debug {
		fmt.Printf("DEBUG: Viewing target='%s'\n", target)
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

// Run implements the remove subcommand execution
func (r *RemoveCmd) Run(cli *CLI) error {
	// Determine target from flags
	target, targetValue, err := r.getTarget()
	if err != nil {
		return err
	}

	if cli.Debug {
		fmt.Printf("DEBUG: Push/Remove target='%s', value='%s', exec=%v\n", target, targetValue, r.Exec)
	}

	// Load configuration
	config.Debug = cli.Debug
	cfg, err := config.GetConfig(cli.ConfigPath)
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Initialize GitHub client
	client, err := github.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client initialization error: %w", err)
	}
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

// Run implements the add subcommand execution
func (a *AddCmd) Run(cli *CLI) error {
	// Determine target from flags
	target, targetValue, err := a.getTarget()
	if err != nil {
		return err
	}

	if cli.Debug {
		fmt.Printf("DEBUG: Push/Add target='%s', value='%s', exec=%v\n", target, targetValue, a.Exec)
	}

	// Load configuration
	config.Debug = cli.Debug
	cfg, err := config.GetConfig(cli.ConfigPath)
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Initialize GitHub client
	client, err := github.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client initialization error: %w", err)
	}
	ctx := context.Background()

	if a.Exec {
		fmt.Printf("Executing: Add %s '%s' to organization %s\n", target, targetValue, cfg.Organization)
		err := github.ExecutePushAdd(ctx, client, cfg.Organization, target, targetValue)
		if err != nil {
			return fmt.Errorf("failed to execute add: %w", err)
		}
		fmt.Println("Successfully added.")
	} else {
		fmt.Printf("DRYRUN: Would add %s '%s' to organization %s\n", target, targetValue, cfg.Organization)
		fmt.Println("To execute, add the --exec flag.")
	}

	return nil
}

// getTarget returns the target and value based on the flags set for add command
func (a *AddCmd) getTarget() (string, string, error) {
	targets := []struct {
		value string
		name  string
	}{
		{a.TeamUser, "team-user"},
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
		return "", "", fmt.Errorf("target required: specify --team-user")
	}

	if count > 1 {
		return "", "", fmt.Errorf("only one target can be specified at a time")
	}

	return selectedTarget, selectedValue, nil
}

// Run implements the init command execution
func (i *InitCmd) Run(cli *CLI) error {
	db, err := store.InitDatabase()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	fmt.Println("Database initialization completed")
	return nil
}

// Run implements the version command execution
func (v *VersionCmd) Run(cli *CLI) error {
	fmt.Printf("ghub-desk version %s\n", appVersion)
	fmt.Printf("commit: %s\n", appCommit)
	fmt.Printf("built at: %s\n", appDate)
	return nil
}
