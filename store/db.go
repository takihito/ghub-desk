package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/go-github/v55/github"
	_ "modernc.org/sqlite"
)

const (
	// Database configuration
	DBFileName = "ghub-desk.db"
)

// InitDatabase creates and initializes the SQLite database with required tables
func InitDatabase() (*sql.DB, error) {
	db, err := sql.Open("sqlite", DBFileName)
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
			x_accepted_github_permissions TEXT,
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

// StoreUsers stores GitHub users in the database
func StoreUsers(db *sql.DB, users []*github.User) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	for _, u := range users {
		_, err := db.Exec(`INSERT OR REPLACE INTO users(id, login, name, email, company, location, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			u.GetID(), u.GetLogin(), u.GetName(), u.GetEmail(), u.GetCompany(), u.GetLocation(),
			now, now)
		if err != nil {
			return fmt.Errorf("failed to insert user %s: %w", u.GetLogin(), err)
		}
	}
	return nil
}

// StoreUsersWithDetails stores GitHub users with detailed information fetched individually
func StoreUsersWithDetails(ctx context.Context, client *github.Client, db *sql.DB, users []*github.User) error {
	now := time.Now().Format("2006-01-02 15:04:05")

	for i, u := range users {
		// Fetch detailed user information
		fmt.Printf("Fetching details for user %d/%d: %s\n", i+1, len(users), u.GetLogin())

		detailedUser, _, err := client.Users.Get(ctx, u.GetLogin())
		if err != nil {
			fmt.Printf("Warning: failed to fetch details for user %s: %v\n", u.GetLogin(), err)
			// Use basic user info if detailed fetch fails
			detailedUser = u
		}

		_, err = db.Exec(`INSERT OR REPLACE INTO users(id, login, name, email, company, location, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			detailedUser.GetID(), detailedUser.GetLogin(), detailedUser.GetName(), detailedUser.GetEmail(),
			detailedUser.GetCompany(), detailedUser.GetLocation(), now, now)
		if err != nil {
			return fmt.Errorf("failed to insert user %s: %w", detailedUser.GetLogin(), err)
		}

		// Rate limiting: sleep between requests to avoid hitting API limits
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// StoreTeams stores GitHub teams in the database
func StoreTeams(db *sql.DB, teams []*github.Team) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	for _, t := range teams {
		_, err := db.Exec(`INSERT OR REPLACE INTO teams(id, name, slug, description, privacy, permission, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			t.GetID(), t.GetName(), t.GetSlug(), t.GetDescription(), t.GetPrivacy(), t.GetPermission(),
			now, now)
		if err != nil {
			return fmt.Errorf("failed to insert team %s: %w", t.GetSlug(), err)
		}
	}
	return nil
}

// StoreRepositories stores GitHub repositories in the database
func StoreRepositories(db *sql.DB, repos []*github.Repository) error {
	for _, r := range repos {
		_, err := db.Exec(`INSERT OR REPLACE INTO repositories(id, name, full_name, description, private, language, size, stargazers_count, watchers_count, forks_count, created_at, updated_at, pushed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.GetID(), r.GetName(), r.GetFullName(), r.GetDescription(), r.GetPrivate(), r.GetLanguage(),
			r.GetSize(), r.GetStargazersCount(), r.GetWatchersCount(), r.GetForksCount(),
			formatTime(r.GetCreatedAt()), formatTime(r.GetUpdatedAt()), formatTime(r.GetPushedAt()))
		if err != nil {
			return fmt.Errorf("failed to insert repository %s: %w", r.GetName(), err)
		}
	}
	return nil
}

// StoreTeamUsers stores team users in the database
func StoreTeamUsers(db *sql.DB, users []*github.User, teamSlug string) error {
	// First get team ID from slug
	var teamID int64
	err := db.QueryRow(`SELECT id FROM teams WHERE slug = ?`, teamSlug).Scan(&teamID)
	if err != nil {
		return fmt.Errorf("failed to get team ID for slug %s: %w", teamSlug, err)
	}

	for _, u := range users {
		_, err := db.Exec(`INSERT OR REPLACE INTO team_users(team_id, user_id, user_login, team_slug, role, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			teamID, u.GetID(), u.GetLogin(), teamSlug, "member", time.Now().Format("2006-01-02 15:04:05"))
		if err != nil {
			return fmt.Errorf("failed to insert team user %s for team %s: %w", u.GetLogin(), teamSlug, err)
		}
	}
	return nil
}

// formatTime converts a GitHub timestamp to a string, handling nil values
func formatTime(t github.Timestamp) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
