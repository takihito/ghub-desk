// Package main implements a CLI tool for GitHub organization management.
// This tool allows users to fetch, view, and manage GitHub organization data
// including users, teams, repositories, and team memberships.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
	_ "modernc.org/sqlite"
)

const (
	// Environment variable names
	envOrg         = "GHUB_DESK_ORGANIZATION"
	envGithubToken = "GHUB_DESK_GITHUB_TOKEN"

	// Database configuration
	dbFileName = "ghub-desk.db"

	// API pagination settings
	defaultPerPage = 100
	defaultSleep   = 1 * time.Second
)

// main is the entry point of the CLI application
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "pull":
		pullCmd(os.Args[2:])
	case "view":
		viewCmd(os.Args[2:])
	case "push":
		pushCmd(os.Args[2:])
	case "init":
		if err := initCmd(); err != nil {
			fmt.Fprintf(os.Stderr, "Initialization failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Database tables initialized successfully")
		return
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

// initCmd initializes the database with required tables
func initCmd() error {
	db, err := initDatabase()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	fmt.Println("Database initialization completed")
	return nil
}

// usage displays help information for the CLI tool
func usage() {
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

// Config holds the application configuration loaded from environment variables
type Config struct {
	Organization string
	GitHubToken  string
}

// getConfig loads and validates configuration from environment variables
func getConfig() (*Config, error) {
	org := os.Getenv(envOrg)
	if org == "" {
		return nil, fmt.Errorf("environment variable %s is required", envOrg)
	}

	token := os.Getenv(envGithubToken)
	if token == "" {
		return nil, fmt.Errorf("environment variable %s is required", envGithubToken)
	}

	return &Config{
		Organization: org,
		GitHubToken:  token,
	}, nil
}

// initGitHubClient creates and configures a GitHub API client with OAuth2 authentication
func initGitHubClient(token string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc)
}

// initDatabase creates and initializes the SQLite database with required tables
func initDatabase() (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, nil
}

// createTables creates all required database tables if they don't exist
func createTables(db *sql.DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			login TEXT UNIQUE,
			name TEXT,
			email TEXT,
			company TEXT,
			location TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS teams (
			id INTEGER PRIMARY KEY,
			name TEXT,
			slug TEXT UNIQUE,
			description TEXT,
			privacy TEXT,
			permission TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS repositories (
			id INTEGER PRIMARY KEY,
			name TEXT UNIQUE,
			full_name TEXT,
			description TEXT,
			private BOOLEAN,
			language TEXT,
			size INTEGER,
			stargazers_count INTEGER,
			watchers_count INTEGER,
			forks_count INTEGER,
			created_at TEXT,
			updated_at TEXT,
			pushed_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS team_users (
			team_id INTEGER,
			user_id INTEGER,
			user_login TEXT,
			team_slug TEXT,
			role TEXT,
			created_at TEXT,
			PRIMARY KEY (team_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS token_permissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			scopes TEXT,
			x_oauth_scopes TEXT,
			x_accepted_oauth_scopes TEXT,
			x_github_media_type TEXT,
			x_ratelimit_limit INTEGER,
			x_ratelimit_remaining INTEGER,
			x_ratelimit_reset INTEGER,
			created_at TEXT,
			updated_at TEXT
		)`,
	}

	for _, query := range tables {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

// pullCmd handles the 'pull' command to fetch data from GitHub API
func pullCmd(args []string) {
	// Parse flags according to new specification
	var target string
	var store bool
	var teamName string // For --teams-users

	for i, arg := range args {
		switch arg {
		case "--store":
			store = true
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
		fmt.Fprintln(os.Stderr, "pull対象を指定してください")
		fmt.Fprintln(os.Stderr, "利用可能な対象: --users, --teams, --repos, --teams-users <team_name>, --all-teams-users, --token-permission")
		os.Exit(1)
	}

	if target == "teams-users" && teamName == "" {
		fmt.Fprintln(os.Stderr, "--teams-users にはチーム名を指定してください")
		os.Exit(1)
	}

	// Debug message to verify flag parsing
	if store {
		fmt.Printf("DEBUG: --store flag detected, will save to database\n")
	} else {
		fmt.Printf("DEBUG: --store flag not detected, will not save to database\n")
	}

	// Load configuration from environment variables
	config, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize GitHub client
	ctx := context.Background()
	client := initGitHubClient(config.GitHubToken)

	var db *sql.DB
	if store || target == "all-teams-users" {
		db, err = initDatabase()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Database initialization error: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()
	}

	// Handle different target types with appropriate data fetching
	finalTarget := target
	if target == "teams-users" {
		finalTarget = teamName + "/users" // Convert to legacy format for handlePullTarget
	}

	err = handlePullTarget(ctx, client, db, config.Organization, finalTarget, store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to pull %s: %v\n", target, err)
		os.Exit(1)
	}

	fmt.Printf("Successfully completed pulling %s for organization %s\n", target, config.Organization)
}

// handlePullTarget processes different types of pull targets (users, teams, repos, team_users)
func handlePullTarget(ctx context.Context, client *github.Client, db *sql.DB, org, target string, store bool) error {
	switch {
	case target == "users":
		return pullUsers(ctx, client, db, org, store)
	case target == "teams":
		return pullTeams(ctx, client, db, org, store)
	case target == "repos":
		return pullRepositories(ctx, client, db, org, store)
	case target == "all-teams-users":
		return pullAllTeamsUsers(ctx, client, db, org, store)
	case target == "token-permission":
		return pullTokenPermission(ctx, client, db, store)
	case strings.HasSuffix(target, "/users"):
		teamSlug := strings.TrimSuffix(target, "/users")
		return pullTeamUsers(ctx, client, db, org, teamSlug, store)
	default:
		return fmt.Errorf("unknown target: %s", target)
	}
}

// pullUsers fetches organization members and optionally stores them in database
func pullUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, store bool) error {
	if store {
		if _, err := db.Exec(`DELETE FROM users`); err != nil {
			return fmt.Errorf("failed to clear users table: %w", err)
		}
	}

	return fetchAndStore(
		ctx, client,
		func(ctx context.Context, org string, opts *github.ListOptions) ([]*github.User, *github.Response, error) {
			memberOpts := &github.ListMembersOptions{ListOptions: *opts}
			return client.Organizations.ListMembers(ctx, org, memberOpts)
		},
		func(db *sql.DB, users []*github.User) error {
			if !store || db == nil {
				return nil
			}
			return storeUsers(db, users)
		},
		db, org,
	)
}

// pullTeams fetches organization teams and optionally stores them in database
func pullTeams(ctx context.Context, client *github.Client, db *sql.DB, org string, store bool) error {
	if store {
		if _, err := db.Exec(`DELETE FROM teams`); err != nil {
			return fmt.Errorf("failed to clear teams table: %w", err)
		}
	}

	return fetchAndStore(
		ctx, client,
		func(ctx context.Context, org string, opts *github.ListOptions) ([]*github.Team, *github.Response, error) {
			return client.Teams.ListTeams(ctx, org, opts)
		},
		func(db *sql.DB, teams []*github.Team) error {
			if !store || db == nil {
				return nil
			}
			return storeTeams(db, teams)
		},
		db, org,
	)
}

// pullRepositories fetches organization repositories and optionally stores them in database
func pullRepositories(ctx context.Context, client *github.Client, db *sql.DB, org string, store bool) error {
	if store {
		if _, err := db.Exec(`DELETE FROM repositories`); err != nil {
			return fmt.Errorf("failed to clear repositories table: %w", err)
		}
	}

	return fetchAndStore(
		ctx, client,
		func(ctx context.Context, org string, opts *github.ListOptions) ([]*github.Repository, *github.Response, error) {
			repoOpts := &github.RepositoryListByOrgOptions{ListOptions: *opts}
			return client.Repositories.ListByOrg(ctx, org, repoOpts)
		},
		func(db *sql.DB, repos []*github.Repository) error {
			if !store || db == nil {
				return nil
			}
			return storeRepositories(db, repos)
		},
		db, org,
	)
}

// pullTeamUsers fetches team members and optionally stores them in database
func pullTeamUsers(ctx context.Context, client *github.Client, db *sql.DB, org, teamSlug string, store bool) error {
	if store {
		if _, err := db.Exec(`DELETE FROM team_users WHERE team_slug = ?`, teamSlug); err != nil {
			return fmt.Errorf("failed to clear team_users table: %w", err)
		}
	}

	return fetchAndStore(
		ctx, client,
		func(ctx context.Context, org string, opts *github.ListOptions) ([]*github.User, *github.Response, error) {
			teamOpts := &github.TeamListTeamMembersOptions{ListOptions: *opts}
			return client.Teams.ListTeamMembersBySlug(ctx, org, teamSlug, teamOpts)
		},
		func(db *sql.DB, users []*github.User) error {
			if !store || db == nil {
				return nil
			}
			return storeTeamUsers(db, users, teamSlug)
		},
		db, org,
	)
}

// pullAllTeamsUsers fetches users for all stored teams
func pullAllTeamsUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, store bool) error {
	// Get all team slugs from the database
	rows, err := db.Query(`SELECT slug FROM teams`)
	if err != nil {
		return fmt.Errorf("failed to query teams: %w", err)
	}
	defer rows.Close()

	var teamSlugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return fmt.Errorf("failed to scan team slug: %w", err)
		}
		teamSlugs = append(teamSlugs, slug)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating team slugs: %w", err)
	}

	if len(teamSlugs) == 0 {
		fmt.Println("No teams found in database. Please run 'ghub-desk pull teams --store' first.")
		return nil
	}

	fmt.Printf("Fetching users for %d teams...\n", len(teamSlugs))

	// Fetch users for each team
	for i, teamSlug := range teamSlugs {
		fmt.Printf("Processing team %d/%d: %s\n", i+1, len(teamSlugs), teamSlug)
		if err := pullTeamUsers(ctx, client, db, org, teamSlug, store); err != nil {
			fmt.Printf("Warning: failed to fetch users for team %s: %v\n", teamSlug, err)
			continue
		}
	}

	fmt.Printf("Completed fetching users for all teams.\n")
	return nil
}

// pullTokenPermission fetches GitHub token permissions and optionally stores them in database
func pullTokenPermission(ctx context.Context, client *github.Client, db *sql.DB, store bool) error {
	// Make a simple API call to get token information from response headers
	_, resp, err := client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to get token information: %w", err)
	}

	// Extract permission information from response headers
	scopes := resp.Header.Get("X-OAuth-Scopes")
	acceptedScopes := resp.Header.Get("X-Accepted-OAuth-Scopes")
	mediaType := resp.Header.Get("X-GitHub-Media-Type")
	rateLimit := resp.Rate.Limit
	rateRemaining := resp.Rate.Remaining
	rateReset := resp.Rate.Reset.Unix()

	fmt.Printf("Token Permissions:\n")
	fmt.Printf("  OAuth Scopes: %s\n", scopes)
	fmt.Printf("  Accepted OAuth Scopes: %s\n", acceptedScopes)
	fmt.Printf("  GitHub Media Type: %s\n", mediaType)
	fmt.Printf("  Rate Limit: %d\n", rateLimit)
	fmt.Printf("  Rate Remaining: %d\n", rateRemaining)
	fmt.Printf("  Rate Reset: %d\n", rateReset)

	if store && db != nil {
		// Clear existing token permission data
		if _, err := db.Exec(`DELETE FROM token_permissions`); err != nil {
			return fmt.Errorf("failed to clear token_permissions table: %w", err)
		}

		// Store new token permission data
		now := time.Now().Format(time.RFC3339)
		_, err = db.Exec(`
			INSERT INTO token_permissions (
				scopes, x_oauth_scopes, x_accepted_oauth_scopes, x_github_media_type,
				x_ratelimit_limit, x_ratelimit_remaining, x_ratelimit_reset,
				created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			scopes, scopes, acceptedScopes, mediaType,
			rateLimit, rateRemaining, rateReset,
			now, now,
		)
		if err != nil {
			return fmt.Errorf("failed to store token permissions: %w", err)
		}
		fmt.Printf("Token permission information stored in database\n")
	}

	return nil
}

// fetchAndStore is a generic function that handles GitHub API pagination
// and stores the fetched data into the database.
// It abstracts the common pattern of:
// 1. Making paginated API calls to GitHub
// 2. Processing each page of results
// 3. Storing the data in SQLite database
// 4. Handling rate limiting and errors
//
// Type parameter T represents the type of GitHub API response data
// (e.g., *github.User, *github.Team, *github.Repository)
func fetchAndStore[T any](
	ctx context.Context,
	client *github.Client,
	listFunc func(ctx context.Context, org string, opts *github.ListOptions) ([]*T, *github.Response, error),
	storeFunc func(db *sql.DB, items []*T) error,
	db *sql.DB,
	org string,
) error {
	var allItems []*T
	page := 1

	for {
		fmt.Printf("Fetching page %d...\n", page)

		// Make API call with pagination options
		opts := &github.ListOptions{Page: page, PerPage: defaultPerPage}
		items, resp, err := listFunc(ctx, org, opts)
		if err != nil {
			return fmt.Errorf("failed to fetch page %d: %w", page, err)
		}

		allItems = append(allItems, items...)
		fmt.Printf("Retrieved %d items from page %d\n", len(items), page)

		// Check if we've reached the last page
		if resp.NextPage == 0 {
			break
		}

		page = resp.NextPage

		// Rate limiting: sleep between requests to avoid hitting API limits
		time.Sleep(defaultSleep)
	}

	// Store all fetched data in the database
	if len(allItems) > 0 {
		if err := storeFunc(db, allItems); err != nil {
			return fmt.Errorf("failed to store data: %w", err)
		}
		fmt.Printf("Successfully stored %d items in database\n", len(allItems))
	}

	return nil
}

// Database storage functions for different entity types

// storeUsers stores GitHub users in the database
func storeUsers(db *sql.DB, users []*github.User) error {
	for _, user := range users {
		_, err := db.Exec(`
			INSERT OR REPLACE INTO users (id, login, name, email, company, location, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			user.GetID(),
			user.GetLogin(),
			user.GetName(),
			user.GetEmail(),
			user.GetCompany(),
			user.GetLocation(),
			formatTime(user.GetCreatedAt()),
			formatTime(user.GetUpdatedAt()),
		)
		if err != nil {
			return fmt.Errorf("failed to store user %s: %w", user.GetLogin(), err)
		}
	}
	return nil
}

// storeTeams stores GitHub teams in the database
func storeTeams(db *sql.DB, teams []*github.Team) error {
	for _, team := range teams {
		_, err := db.Exec(`
			INSERT OR REPLACE INTO teams (id, name, slug, description, privacy, permission, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			team.GetID(),
			team.GetName(),
			team.GetSlug(),
			team.GetDescription(),
			team.GetPrivacy(),
			team.GetPermission(),
			time.Now().Format(time.RFC3339), // Teams don't have created_at in API
			time.Now().Format(time.RFC3339), // Teams don't have updated_at in API
		)
		if err != nil {
			return fmt.Errorf("failed to store team %s: %w", team.GetSlug(), err)
		}
	}
	return nil
}

// storeRepositories stores GitHub repositories in the database
func storeRepositories(db *sql.DB, repos []*github.Repository) error {
	for _, repo := range repos {
		_, err := db.Exec(`
			INSERT OR REPLACE INTO repositories (
				id, name, full_name, description, private, language, size,
				stargazers_count, watchers_count, forks_count,
				created_at, updated_at, pushed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			repo.GetID(),
			repo.GetName(),
			repo.GetFullName(),
			repo.GetDescription(),
			repo.GetPrivate(),
			repo.GetLanguage(),
			repo.GetSize(),
			repo.GetStargazersCount(),
			repo.GetWatchersCount(),
			repo.GetForksCount(),
			formatTime(repo.GetCreatedAt()),
			formatTime(repo.GetUpdatedAt()),
			formatTime(repo.GetPushedAt()),
		)
		if err != nil {
			return fmt.Errorf("failed to store repository %s: %w", repo.GetName(), err)
		}
	}
	return nil
}

// storeTeamUsers stores team users in the database
func storeTeamUsers(db *sql.DB, users []*github.User, teamSlug string) error {
	// First get team ID from slug
	var teamID int64
	err := db.QueryRow(`SELECT id FROM teams WHERE slug = ?`, teamSlug).Scan(&teamID)
	if err != nil {
		return fmt.Errorf("failed to find team ID for slug %s: %w", teamSlug, err)
	}

	for _, user := range users {
		_, err := db.Exec(`
			INSERT OR REPLACE INTO team_users (team_id, user_id, user_login, team_slug, role, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			teamID,
			user.GetID(),
			user.GetLogin(),
			teamSlug,
			"member", // Default role, could be enhanced to get actual role
			time.Now().Format(time.RFC3339),
		)
		if err != nil {
			return fmt.Errorf("failed to store team user %s: %w", user.GetLogin(), err)
		}
	}
	return nil
}

// formatTime converts a GitHub timestamp to a string, handling nil values
func formatTime(t github.Timestamp) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// pushCmd handles the 'push' command and its subcommands
func pushCmd(args []string) {
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
		fmt.Fprintln(os.Stderr, "pushサブコマンドを指定してください (現在は --remove のみサポート)")
		os.Exit(1)
	}

	if target == "" || resourceName == "" {
		fmt.Fprintln(os.Stderr, "削除対象を指定してください")
		fmt.Fprintln(os.Stderr, "利用可能な対象: --team <name>, --user <name>, --team-user <team>/<user>")
		os.Exit(1)
	}

	// Load configuration
	config, err := getConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "設定の読み込みに失敗しました: %v\n", err)
		os.Exit(1)
	}

	// Initialize GitHub client
	client := initGitHubClient(config.GitHubToken)
	ctx := context.Background()

	if exec {
		fmt.Printf("%s %s を削除します (実行)\n", target, resourceName)
		// Execute the actual removal
		err := executePushRemove(ctx, client, config.Organization, target, resourceName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "削除に失敗しました: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("削除が完了しました\n")
	} else {
		fmt.Printf("%s %s を削除します (DRYRUN)\n", target, resourceName)
		fmt.Println("実際に削除するには --exec フラグを追加してください")
	}
}

// viewCmd handles the 'view' command to display data from local database
func viewCmd(args []string) {
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
		fmt.Fprintln(os.Stderr, "view対象を指定してください")
		fmt.Fprintln(os.Stderr, "利用可能な対象: --users, --teams, --repos, --teams-users <team_name>, --token-permission")
		os.Exit(1)
	}

	if target == "teams-users" && teamName == "" {
		fmt.Fprintln(os.Stderr, "--teams-users にはチーム名を指定してください")
		os.Exit(1)
	}

	db, err := initDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database initialization error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Handle different target types
	finalTarget := target
	if target == "teams-users" {
		finalTarget = teamName + "/users" // Convert to legacy format for handleViewTarget
	}

	err = handleViewTarget(db, finalTarget)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to view %s: %v\n", target, err)
		os.Exit(1)
	}
}

// handleViewTarget processes different types of view targets
func handleViewTarget(db *sql.DB, target string) error {
	switch target {
	case "users":
		return viewUsers(db)
	case "teams":
		return viewTeams(db)
	case "repos", "repositories":
		return viewRepositories(db)
	case "token-permission":
		return viewTokenPermission(db)
	default:
		if strings.HasSuffix(target, "/users") {
			teamSlug := strings.TrimSuffix(target, "/users")
			return viewTeamUsers(db, teamSlug)
		}
		return fmt.Errorf("unknown target: %s", target)
	}
}

// viewUsers displays users from the database
func viewUsers(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, login, name, email, company, location FROM users ORDER BY login`)
	if err != nil {
		return fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	fmt.Println("ID\tLogin\tName\tEmail\tCompany\tLocation")
	fmt.Println("--\t-----\t----\t-----\t-------\t--------")

	for rows.Next() {
		var id int64
		var login, name, email, company, location sql.NullString
		err := rows.Scan(&id, &login, &name, &email, &company, &location)
		if err != nil {
			return fmt.Errorf("failed to scan user row: %w", err)
		}

		fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\n",
			id,
			login.String,
			name.String,
			email.String,
			company.String,
			location.String,
		)
	}
	return nil
}

// viewTeams displays teams from the database
func viewTeams(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, slug, name, description, privacy FROM teams ORDER BY slug`)
	if err != nil {
		return fmt.Errorf("failed to query teams: %w", err)
	}
	defer rows.Close()

	fmt.Println("ID\tSlug\tName\tDescription\tPrivacy")
	fmt.Println("--\t----\t----\t-----------\t-------")

	for rows.Next() {
		var id int64
		var slug, name, description, privacy sql.NullString
		err := rows.Scan(&id, &slug, &name, &description, &privacy)
		if err != nil {
			return fmt.Errorf("failed to scan team row: %w", err)
		}

		fmt.Printf("%d\t%s\t%s\t%s\t%s\n",
			id,
			slug.String,
			name.String,
			description.String,
			privacy.String,
		)
	}
	return nil
}

// viewRepositories displays repositories from the database
func viewRepositories(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT id, name, full_name, description, private, language, stargazers_count 
		FROM repositories ORDER BY name`)
	if err != nil {
		return fmt.Errorf("failed to query repositories: %w", err)
	}
	defer rows.Close()

	fmt.Println("ID\tName\tFull Name\tDescription\tPrivate\tLanguage\tStars")
	fmt.Println("--\t----\t---------\t-----------\t-------\t--------\t-----")

	for rows.Next() {
		var id int64
		var private bool
		var name, fullName, description, language sql.NullString
		var stars int
		err := rows.Scan(&id, &name, &fullName, &description, &private, &language, &stars)
		if err != nil {
			return fmt.Errorf("failed to scan repository row: %w", err)
		}

		fmt.Printf("%d\t%s\t%s\t%s\t%t\t%s\t%d\n",
			id,
			name.String,
			fullName.String,
			description.String,
			private,
			language.String,
			stars,
		)
	}
	return nil
}

// viewTeamUsers displays team members from the database
func viewTeamUsers(db *sql.DB, teamSlug string) error {
	rows, err := db.Query(`
		SELECT user_id, user_login, role 
		FROM team_users 
		WHERE team_slug = ? 
		ORDER BY user_login`, teamSlug)
	if err != nil {
		return fmt.Errorf("failed to query team users: %w", err)
	}
	defer rows.Close()

	fmt.Printf("Team: %s\n", teamSlug)
	fmt.Println("User ID\tLogin\tRole")
	fmt.Println("-------\t-----\t----")

	for rows.Next() {
		var userID int64
		var login, role sql.NullString
		err := rows.Scan(&userID, &login, &role)
		if err != nil {
			return fmt.Errorf("failed to scan team user row: %w", err)
		}

		fmt.Printf("%d\t%s\t%s\n", userID, login.String, role.String)
	}
	return nil
}

// viewTokenPermission displays token permissions from the database
func viewTokenPermission(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT scopes, x_oauth_scopes, x_accepted_oauth_scopes, x_github_media_type,
		       x_ratelimit_limit, x_ratelimit_remaining, x_ratelimit_reset,
		       created_at, updated_at
		FROM token_permissions 
		ORDER BY created_at DESC 
		LIMIT 1`)
	if err != nil {
		return fmt.Errorf("failed to query token permissions: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		fmt.Println("No token permission data found in database.")
		fmt.Println("Run 'ghub-desk pull --token-permission --store' first.")
		return nil
	}

	var scopes, oauthScopes, acceptedScopes, mediaType, createdAt, updatedAt sql.NullString
	var rateLimit, rateRemaining, rateReset int

	err = rows.Scan(&scopes, &oauthScopes, &acceptedScopes, &mediaType,
		&rateLimit, &rateRemaining, &rateReset,
		&createdAt, &updatedAt)
	if err != nil {
		return fmt.Errorf("failed to scan token permission row: %w", err)
	}

	fmt.Println("Token Permissions (from database):")
	fmt.Println("===================================")
	fmt.Printf("OAuth Scopes: %s\n", oauthScopes.String)
	fmt.Printf("Accepted OAuth Scopes: %s\n", acceptedScopes.String)
	fmt.Printf("GitHub Media Type: %s\n", mediaType.String)
	fmt.Printf("Rate Limit: %d\n", rateLimit)
	fmt.Printf("Rate Remaining: %d\n", rateRemaining)
	fmt.Printf("Rate Reset: %d\n", rateReset)
	fmt.Printf("Created At: %s\n", createdAt.String)
	fmt.Printf("Updated At: %s\n", updatedAt.String)

	return nil
}

// executePushRemove executes the actual removal operation via GitHub API
func executePushRemove(ctx context.Context, client *github.Client, org, target, resourceName string) error {
	switch target {
	case "team":
		// Remove team from organization
		_, err := client.Teams.DeleteTeamBySlug(ctx, org, resourceName)
		if err != nil {
			return fmt.Errorf("チーム削除エラー: %v", err)
		}
		return nil

	case "user":
		// Remove user from organization
		_, err := client.Organizations.RemoveMember(ctx, org, resourceName)
		if err != nil {
			return fmt.Errorf("ユーザー削除エラー: %v", err)
		}
		return nil

	case "team-user":
		// Parse team/user format
		parts := strings.Split(resourceName, "/")
		if len(parts) != 2 {
			return fmt.Errorf("チーム/ユーザー形式が正しくありません。{team_name}/{user_name} の形式で指定してください")
		}
		teamSlug := parts[0]
		username := parts[1]

		// Remove user from team
		_, err := client.Teams.RemoveTeamMembershipBySlug(ctx, org, teamSlug, username)
		if err != nil {
			return fmt.Errorf("チームからのユーザー削除エラー: %v", err)
		}
		return nil

	default:
		return fmt.Errorf("サポートされていない削除対象: %s", target)
	}
}
