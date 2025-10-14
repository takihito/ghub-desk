package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"ghub-desk/config"
	"ghub-desk/github"
	"ghub-desk/mcp"
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
	MCP     McpCmd     `cmd:"" help:"Start MCP server"`

	// internal cached config
	cfgOnce sync.Once
	cfg     *config.Config
	cfgErr  error
}

// Config returns the app configuration, loading it once per process.
func (cli *CLI) Config() (*config.Config, error) {
	cli.cfgOnce.Do(func() {
		// propagate debug flag to config package and load
		config.Debug = cli.Debug
		cli.cfg, cli.cfgErr = config.GetConfig(cli.ConfigPath)
	})
	return cli.cfg, cli.cfgErr
}

// CommonTargetOptions holds the shared target flags for pull and view commands
type CommonTargetOptions struct {
	Users           bool   `help:"Target: users"`
	DetailUsers     bool   `name:"detail-users" help:"Target: detail-users"`
	Teams           bool   `help:"Target: teams"`
	Repos           bool   `help:"Target: repos"`
	TeamUser        string `name:"team-user" aliases:"team-users" help:"Target: team-user (provide team slug: 1–100 chars, lowercase alnum + hyphen)"`
	RepoUsers       string `name:"repos-users" help:"Target: repos-users (provide repository name)"`
	RepoTeams       string `name:"repos-teams" help:"Target: repos-teams (provide repository name)"`
	UserRepos       string `name:"user-repos" help:"Target: user-repos (provide user login)"`
	TokenPermission bool   `name:"token-permission" help:"Target: token-permission"`
	OutsideUsers    bool   `name:"outside-users" help:"Target: outside-users"`
}

// TargetFlag represents an additional target option to evaluate.
type TargetFlag struct {
	Enabled bool
	Name    string
}

// GetTarget determines the single selected target from the common options.
func (c *CommonTargetOptions) GetTarget(extraTargets ...TargetFlag) (string, error) {
	targets := []struct {
		flag bool
		name string
	}{
		{c.Users, "users"},
		{c.DetailUsers, "detail-users"},
		{c.Teams, "teams"},
		{c.Repos, "repos"},
		{c.TeamUser != "", "team-user"},
		{c.RepoUsers != "", "repos-users"},
		{c.RepoTeams != "", "repos-teams"},
		{c.UserRepos != "", "user-repos"},
		{c.TokenPermission, "token-permission"},
		{c.OutsideUsers, "outside-users"},
	}
	for _, et := range extraTargets {
		targets = append(targets, struct {
			flag bool
			name string
		}{et.Enabled, et.Name})
	}

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
	NoStore      bool          `name:"no-store" help:"Do not save to local SQLite database"`
	Stdout       bool          `name:"stdout" help:"Print API response to stdout"`
	IntervalTime time.Duration `help:"Sleep interval between API requests" default:"3s"`
}

// ViewCmd represents the view command structure
type ViewCmd struct {
	CommonTargetOptions `embed:""`
	Settings            bool   `name:"settings" help:"Show application settings (masked)"`
	TargetPath          string `arg:"" optional:"" help:"Target path (e.g. team-slug/users)."`
}

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
	// Preload config once for commands that require GitHub access.
	// Keep view/init/version free from config requirement.
	cmdPath := ctx.Command()
	if cmdPath == "pull" || cmdPath == "push" {
		if _, err := cli.Config(); err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
	}

	return ctx.Run(&cli)
}

// Run implements the pull command execution
func (p *PullCmd) Run(cli *CLI) error {
	// Determine target from flags
	target, err := p.CommonTargetOptions.GetTarget(TargetFlag{Enabled: p.AllTeamsUsers, Name: "all-teams-users"})
	if err != nil {
		return err
	}

	storeData := !p.NoStore
	if cli.Debug {
		fmt.Printf("DEBUG: Pulling target='%s', store=%v, stdout=%v, interval=%v\n", target, storeData, p.Stdout, p.IntervalTime)
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
	client, err := github.InitClient(cfg)
	if err != nil {
		return fmt.Errorf("github client initialization error: %w", err)
	}
	ctx := context.Background()

	var db *sql.DB
	if storeData || target == "all-teams-users" {
		db, err = store.InitDatabase()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()
	}

	req := github.TargetRequest{Kind: target}
	switch target {
	case "team-user":
		if err := validateTeamName(p.TeamUser); err != nil {
			return err
		}
		req.TeamSlug = p.TeamUser
	case "repos-users":
		if err := validateRepoName(p.RepoUsers); err != nil {
			return err
		}
		req.RepoName = p.RepoUsers
	case "repos-teams":
		if err := validateRepoName(p.RepoTeams); err != nil {
			return err
		}
		req.RepoName = p.RepoTeams
	case "user-repos":
		if err := validateUserLogin(p.UserRepos); err != nil {
			return err
		}
		return fmt.Errorf("pull コマンドでは --user-repos を使用できません。view コマンドで --user-repos を指定してください")
	}
	return github.HandlePullTarget(
		ctx,
		client,
		db,
		cfg.Organization,
		req,
		cfg.GitHubToken,
		github.PullOptions{
			Store:    storeData,
			Stdout:   p.Stdout,
			Interval: p.IntervalTime,
		},
	)
}

// Run implements the view command execution
func (v *ViewCmd) Run(cli *CLI) error {
	if v.TargetPath != "" {
		slug, err := parseTeamUsersPath(v.TargetPath)
		if err != nil {
			return err
		}
		if v.TeamUser != "" && v.TeamUser != slug {
			return fmt.Errorf("フラグと引数で指定されたチームが一致しません")
		}
		v.TeamUser = slug
	}

	// Determine target from flags
	target, err := v.CommonTargetOptions.GetTarget(TargetFlag{Enabled: v.Settings, Name: "settings"})
	if err != nil {
		return err
	}

	if cli.Debug {
		fmt.Printf("DEBUG: Viewing target='%s'\n", target)
	}

	if target == "settings" {
		return ShowSettings(cli)
	}

	// Load config (non-validating) to optionally apply DB path without requiring auth
	if cfgNV, _ := config.LoadConfigNoValidate(cli.ConfigPath); cfgNV != nil && cfgNV.DatabasePath != "" {
		store.SetDBPath(cfgNV.DatabasePath)
	}
	// Initialize database for non-config views
	db, err := store.InitDatabase()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	req := store.TargetRequest{Kind: target}
	switch target {
	case "team-user":
		if err := validateTeamName(v.TeamUser); err != nil {
			return err
		}
		req.TeamSlug = v.TeamUser
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
	case "user-repos":
		if err := validateUserLogin(v.UserRepos); err != nil {
			return err
		}
		req.UserLogin = v.UserRepos
	}

	return store.HandleViewTarget(db, req)
}

func parseTeamUsersPath(path string) (string, error) {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return "", fmt.Errorf("表示対象の引数が空です。{team_slug}/users の形式で指定してください")
	}

	parts := strings.Split(cleaned, "/")
	if len(parts) != 2 || parts[1] != "users" {
		return "", fmt.Errorf("表示対象は {team_slug}/users の形式で指定してください")
	}

	teamSlug := parts[0]
	if teamSlug == "" {
		return "", fmt.Errorf("チームスラグが空です。{team_slug}/users の形式で指定してください")
	}

	return teamSlug, nil
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

	// Load configuration once via CLI helper
	cfg, err := cli.Config()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	if cfg.DatabasePath != "" {
		store.SetDBPath(cfg.DatabasePath)
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
		if !r.NoStore {
			db, err := store.InitDatabase()
			if err != nil {
				return fmt.Errorf("failed to initialize database: %w", err)
			}
			defer db.Close()
			if err := github.SyncPushRemove(ctx, client, db, cfg.Organization, target, targetValue); err != nil {
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
		return "", "", fmt.Errorf("target required: specify one of --team, --user, --team-user, --outside-user")
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

	if cli.Debug {
		if permission != "" {
			fmt.Printf("DEBUG: Push/Add target='%s', value='%s', permission='%s', exec=%v\n", target, targetValue, permission, a.Exec)
		} else {
			fmt.Printf("DEBUG: Push/Add target='%s', value='%s', exec=%v\n", target, targetValue, a.Exec)
		}
	}

	// Load configuration once via CLI helper
	cfg, err := cli.Config()
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
		if permission != "" {
			fmt.Printf("Executing: Add %s '%s' (permission=%s) to organization %s\n", target, targetValue, permission, cfg.Organization)
		} else {
			fmt.Printf("Executing: Add %s '%s' to organization %s\n", target, targetValue, cfg.Organization)
		}
		err := github.ExecutePushAdd(ctx, client, cfg.Organization, target, targetValue, permission)
		if err != nil {
			return fmt.Errorf("failed to execute add: %w", err)
		}
		fmt.Println("Successfully added.")
		if !a.NoStore {
			db, err := store.InitDatabase()
			if err != nil {
				return fmt.Errorf("failed to initialize database: %w", err)
			}
			defer db.Close()
			if err := github.SyncPushAdd(ctx, client, db, cfg.Organization, target, targetValue); err != nil {
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

// Run implements the init command execution
func (i *InitCmd) Run(cli *CLI) error {
	// Load config (non-validating) to optionally apply DB path
	if cfgNV, _ := config.LoadConfigNoValidate(cli.ConfigPath); cfgNV != nil && cfgNV.DatabasePath != "" {
		store.SetDBPath(cfgNV.DatabasePath)
	}
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

// McpCmd starts the MCP server
type McpCmd struct{}

func (m *McpCmd) Run(cli *CLI) error {
	// Load config to get MCP permissions and auth
	cfg, err := cli.Config()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	if cli.Debug {
		fmt.Printf("DEBUG: Starting MCP server (allow_pull=%v, allow_write=%v)\n", cfg.MCP.AllowPull, cfg.MCP.AllowWrite)
		fmt.Printf("DEBUG: Exposing tools: %v\n", mcp.AllowedTools(cfg))
	}
	ctx := context.Background()
	return mcp.Serve(ctx, cfg)
}
