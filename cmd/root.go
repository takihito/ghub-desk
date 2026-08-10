package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"ghub-desk/config"
	"ghub-desk/debuglog"

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
	Debug      bool   `help:"Enable debug logging."`
	LogPath    string `name:"log-path" help:"Write logs to the given file (appends)." type:"path"`
	ConfigPath string `name:"config" short:"c" help:"Path to config file." type:"path"`

	Pull    PullCmd      `cmd:"" help:"Fetch data from GitHub API (resumable; session_path stores progress and validation ensures repository/team names still exist)"`
	View    ViewCmd      `cmd:"" help:"Display data from local database"`
	Audit   AuditLogsCmd `cmd:"" name:"auditlogs" help:"Fetch audit log entries from GitHub"`
	Push    PushCmd      `cmd:"" help:"Manipulate resources on GitHub"`
	Init    InitCmd      `cmd:"" help:"Initialize local database tables"`
	Version VersionCmd   `cmd:"" help:"Show version information"`
	MCP     McpCmd       `cmd:"" help:"Start MCP server"`

	// internal cached config
	cfgOnce sync.Once
	cfg     *config.Config
	cfgErr  error

	debugWriter io.Writer
	logger      *slog.Logger
}

// debugf writes debug messages via the configured slog logger.
func (cli *CLI) debugf(format string, args ...interface{}) {
	if !cli.Debug || cli.logger == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	msg = strings.TrimSuffix(msg, "\n")
	cli.logger.Debug(msg)
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
	AllTeamsUsers   bool   `name:"all-teams-users" help:"Target: all-teams-users"`
	AllReposUsers   bool   `name:"all-repos-users" help:"Target: all-repos-users"`
	TeamUser        string `name:"team-user" aliases:"team-users" help:"Target: team-user (provide team slug: 1–100 chars, lowercase alnum + hyphen)"`
	RepoUsers       string `name:"repos-users" help:"Target: repos-users (provide repository name)"`
	RepoTeams       string `name:"repos-teams" help:"Target: repos-teams (provide repository name)"`
	RepoTeamsUsers  string `name:"repos-teams-users" help:"Target: repository team users (provide repository name)"`
	AllReposTeams   bool   `name:"all-repos-teams" help:"Target: all-repos-teams"`
	User            string `name:"user" help:"Target: user (provide user login)"`
	UserTeams       string `name:"user-teams" help:"Target: user-teams (provide user login)"`
	UserRepos       string `name:"user-repos" help:"Target: user-repos (provide user login)"`
	TeamRepos       string `name:"team-repos" help:"Target: team-repos (provide team slug)"`
	TokenPermission bool   `name:"token-permission" help:"Target: token-permission"`
	OutsideUsers    bool   `name:"outside-users" help:"Target: outside-users"`
	OrgPlan         bool   `name:"org-plan" help:"Target: org-plan (organization seats and plan)"`
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
		{c.AllTeamsUsers, "all-teams-users"},
		{c.AllReposUsers, "all-repos-users"},
		{c.TeamUser != "", "team-user"},
		{c.RepoUsers != "", "repos-users"},
		{c.RepoTeams != "", "repos-teams"},
		{c.RepoTeamsUsers != "", "repos-teams-users"},
		{c.AllReposTeams, "all-repos-teams"},
		{c.User != "", "user"},
		{c.UserTeams != "", "user-teams"},
		{c.UserRepos != "", "user-repos"},
		{c.TeamRepos != "", "team-repos"},
		{c.TokenPermission, "token-permission"},
		{c.OutsideUsers, "outside-users"},
		{c.OrgPlan, "org-plan"},
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

// Execute is the main entry point for all commands
func Execute() (io.Writer, func(), error) {
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
	logWriter := os.Stderr
	var logCloser io.Closer
	if cli.LogPath != "" {
		f, err := os.OpenFile(cli.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return logWriter, nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logWriter = f
		logCloser = f
	}

	handlerLevel := slog.LevelInfo
	if cli.Debug {
		handlerLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		Level: handlerLevel,
	}))
	cli.logger = logger
	cli.debugWriter = logWriter

	if cli.Debug {
		debuglog.EnableDebugWithWriter(logWriter)
	}

	cleanup := func() {
		if logCloser != nil {
			logCloser.Close()
		}
	}
	if cli.ConfigPath == "" {
		if w := config.LegacyConfigDirWarning(); w != "" {
			fmt.Fprint(os.Stderr, w)
		}
	}

	// Preload config once for commands that require GitHub access.
	// Keep view/init/version free from config requirement.
	cmdPath := ctx.Command()
	if cmdPath == "pull" || cmdPath == "auditlogs" || strings.HasPrefix(cmdPath, "push") {
		if _, err := cli.Config(); err != nil {
			return logWriter, cleanup, fmt.Errorf("configuration error: %w", err)
		}
	}

	return logWriter, cleanup, ctx.Run(&cli)
}
