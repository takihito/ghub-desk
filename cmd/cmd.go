package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"ghub-desk/internal/config"
	ghubclient "ghub-desk/internal/github"
	"ghub-desk/internal/store"
)

// Execute is the main entry point for command execution
func Execute() error {
	if len(os.Args) < 2 {
		Usage()
		return fmt.Errorf("no command specified")
	}

	cmd := os.Args[1]
	switch cmd {
	case "pull":
		return PullCmd(os.Args[2:])
	case "view":
		return ViewCmd(os.Args[2:])
	case "push":
		return PushCmd(os.Args[2:])
	case "init":
		return InitCmd()
	case "help", "-h", "--help":
		Usage()
		return nil
	default:
		Usage()
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

// Usage displays help information for the CLI tool
func Usage() {
	fmt.Print(`ghub-desk - GitHub Organization Management CLI Tool

USAGE:
    ghub-desk <command> [options] [arguments]

COMMANDS:
    pull [--store] --<target>      Fetch data from GitHub API
                                   Targets: --users, --teams, --repos, --teams-users <team_name>, --all-teams-users, --token-permission
                                   --store: Save to local SQLite database
    
    view --<target>                Display data from local database
                                   Targets: --users, --teams, --repos, --teams-users <team_name>, --token-permission
    
    push --remove --<target>       Remove resources from GitHub
                                   [--exec]: Execute (without this flag, runs in DRYRUN mode)
                                   Targets: --team <name>, --user <name>, --team-user <team>/<user>
    
    init                           Initialize local database tables
    
    help                           Show this help message

ENVIRONMENT VARIABLES:
    GHUB_DESK_ORGANIZATION     GitHub organization name (required)
    GHUB_DESK_GITHUB_TOKEN     GitHub personal access token (required)

EXAMPLES:
    # Fetch and store organization members
    ghub-desk pull --store --users
    
    # View stored teams
    ghub-desk view --teams
    
    # Fetch team members (without storing)
    ghub-desk pull --teams-users engineering
    
    # Fetch all team users
    ghub-desk pull --all-teams-users
    
    # Remove team (DRYRUN)
    ghub-desk push --remove --team old-team
    
    # Remove user (Execute)
    ghub-desk push --remove --user old-user --exec
    
    # Initialize database
    ghub-desk init

For more information, visit: https://github.com/your-org/ghub-desk
`)
}

// InitCmd initializes the database with required tables
func InitCmd() error {
	db, err := store.InitDatabase()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	fmt.Println("Database initialization completed")
	return nil
}

// PullCmd handles the 'pull' command to fetch data from GitHub API
func PullCmd(args []string) error {
	// Parse flags according to new specification
	var target string
	var storeFlag bool
	var teamName string // For --teams-users

	for i, arg := range args {
		switch arg {
		case "--store":
			storeFlag = true
		case "--users":
			target = "users"
		case "--teams":
			target = "teams"
		case "--repos":
			target = "repos"
		case "--teams-users":
			target = "teams-users"
			// Next argument should be team name
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				teamName = args[i+1]
			}
		case "--all-teams-users":
			target = "all-teams-users"
		case "--token-permission":
			target = "token-permission"
		}
	}

	if target == "" {
		return fmt.Errorf("pull対象を指定してください\n利用可能な対象: --users, --teams, --repos, --teams-users <team_name>, --all-teams-users, --token-permission")
	}

	if target == "teams-users" && teamName == "" {
		return fmt.Errorf("--teams-users にはチーム名を指定してください")
	}

	// Debug message to verify flag parsing
	if storeFlag {
		fmt.Printf("DEBUG: --store flag detected, will save to database\n")
	} else {
		fmt.Printf("DEBUG: --store flag not detected, will not save to database\n")
	}

	// Load configuration from environment variables
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Initialize GitHub client
	ctx := context.Background()
	client := ghubclient.InitGitHubClient(cfg.GitHubToken)

	var db *sql.DB
	if storeFlag || target == "all-teams-users" {
		dbInstance, err := store.InitDatabase()
		if err != nil {
			return fmt.Errorf("database initialization error: %w", err)
		}
		defer dbInstance.Close()
		db = dbInstance
	}

	// Handle different target types with appropriate data fetching
	finalTarget := target
	if target == "teams-users" {
		finalTarget = teamName + "/users" // Convert to legacy format for handlePullTarget
	}

	err = ghubclient.HandlePullTarget(ctx, client, db, cfg.Organization, finalTarget, storeFlag)
	if err != nil {
		return fmt.Errorf("failed to pull %s: %w", target, err)
	}

	fmt.Printf("Successfully completed pulling %s for organization %s\n", target, cfg.Organization)
	return nil
}

// ViewCmd handles the 'view' command to display data from local database
func ViewCmd(args []string) error {
	// Parse flags according to new specification
	var target string
	var teamName string // For --teams-users

	for i, arg := range args {
		switch arg {
		case "--users":
			target = "users"
		case "--teams":
			target = "teams"
		case "--repos":
			target = "repos"
		case "--teams-users":
			target = "teams-users"
			// Next argument should be team name
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				teamName = args[i+1]
			}
		case "--token-permission":
			target = "token-permission"
		}
	}

	if target == "" {
		return fmt.Errorf("view対象を指定してください\n利用可能な対象: --users, --teams, --repos, --teams-users <team_name>, --token-permission")
	}

	if target == "teams-users" && teamName == "" {
		return fmt.Errorf("--teams-users にはチーム名を指定してください")
	}

	db, err := store.InitDatabase()
	if err != nil {
		return fmt.Errorf("database initialization error: %w", err)
	}
	defer db.Close()

	// Handle different target types
	finalTarget := target
	if target == "teams-users" {
		finalTarget = teamName + "/users" // Convert to legacy format for handleViewTarget
	}

	err = store.HandleViewTarget(db, finalTarget)
	if err != nil {
		return fmt.Errorf("failed to view %s: %w", target, err)
	}

	return nil
}

// PushCmd handles the 'push' command and its subcommands
func PushCmd(args []string) error {
	// Parse flags according to new specification
	var remove bool
	var exec bool
	var target string
	var resourceName string

	for i, arg := range args {
		switch arg {
		case "--remove":
			remove = true
		case "--exec":
			exec = true
		case "--team":
			target = "team"
			// Next argument should be team name
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				resourceName = args[i+1]
			}
		case "--user":
			target = "user"
			// Next argument should be user name
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				resourceName = args[i+1]
			}
		case "--team-user":
			target = "team-user"
			// Next argument should be team/user name
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				resourceName = args[i+1]
			}
		}
	}

	if !remove {
		return fmt.Errorf("pushサブコマンドを指定してください (現在は --remove のみサポート)")
	}

	if target == "" || resourceName == "" {
		return fmt.Errorf("削除対象を指定してください\n利用可能な対象: --team <name>, --user <name>, --team-user <team>/<user>")
	}

	// Load configuration
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("設定の読み込みに失敗しました: %w", err)
	}

	// Initialize GitHub client
	client := ghubclient.InitGitHubClient(cfg.GitHubToken)
	ctx := context.Background()

	if exec {
		fmt.Printf("%s %s を削除します (実行)\n", target, resourceName)
		// Execute the actual removal
		err := ghubclient.ExecutePushRemove(ctx, client, cfg.Organization, target, resourceName)
		if err != nil {
			return fmt.Errorf("削除に失敗しました: %w", err)
		}
		fmt.Printf("削除が完了しました\n")
	} else {
		fmt.Printf("%s %s を削除します (DRYRUN)\n", target, resourceName)
		fmt.Println("実際に削除するには --exec フラグを追加してください")
	}

	return nil
}
