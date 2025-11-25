package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"ghub-desk/session"

	"github.com/google/go-github/v55/github"
	_ "modernc.org/sqlite"
)

// DBTX is an interface that abstracts *sql.DB and *sql.Tx.
type DBTX interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

var (
	// ErrRepoNotFound is returned when a repository is not found in the local database.
	ErrRepoNotFound = errors.New("repository not found")
)



const (
	// Database configuration
	DBFileName         = "ghub-desk.db"
	sqliteMaxVariables = 999
)

func insertOrReplaceBatch(db DBTX, table string, columns []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}

	columnCount := len(columns)
	if columnCount == 0 {
		return fmt.Errorf("no columns provided for %s", table)
	}
	if columnCount > sqliteMaxVariables {
		return fmt.Errorf("column count %d exceeds SQLite limit %d for %s", columnCount, sqliteMaxVariables, table)
	}

	values := make([]string, columnCount)
	for i := range values {
		values[i] = "?"
	}
	plHolder := "(" + strings.Join(values, ",") + ")"
	batchSize := sqliteMaxVariables / columnCount
	if batchSize == 0 {
		batchSize = 1
	}

	for start := 0; start < len(rows); start += batchSize {
		end := start + batchSize
		if end > len(rows) {
			end = len(rows)
		}

		placeholders := make([]string, 0, end-start)
		args := make([]any, 0, columnCount*(end-start))

		for _, row := range rows[start:end] {
			if len(row) != columnCount {
				return fmt.Errorf("expected %d values for %s insert, got %d", columnCount, table, len(row))
			}
			placeholders = append(placeholders, plHolder)
			args = append(args, row...)
		}

		query := fmt.Sprintf(
			"INSERT OR REPLACE INTO %s(%s) VALUES %s",
			table,
			strings.Join(columns, ", "),
			strings.Join(placeholders, ","),
		)
		session.Debugf("SQL: %s, ARGS: %v", query, args)
		if _, err := db.Exec(query, args...); err != nil {
			return fmt.Errorf("failed to insert into %s: %w", table, err)
		}
	}

	return nil
}

// DBPath is the runtime-configured SQLite file path. If empty, DBFileName is used.
var DBPath string

// SetDBPath sets a custom SQLite file path. Empty resets to default.
func SetDBPath(path string) {
	DBPath = path
}

func dbPath() string {
	if DBPath != "" {
		return DBPath
	}
	return DBFileName
}

// Connect opens a connection to the SQLite database.
func Connect() (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return db, nil
}

// InitDatabase creates and initializes the SQLite database with required tables.
// It ensures the database file and all necessary tables are created.
func InitDatabase() (*sql.DB, error) {
	db, err := Connect()
	if err != nil {
		return nil, err
	}

	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, nil
}

// createTables creates all required database tables if they don't exist
func createTables(db DBTX) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS ghub_users (
			id INTEGER PRIMARY KEY,
			login TEXT UNIQUE,
			name TEXT,
			email TEXT,
			company TEXT,
			location TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS ghub_teams (
			id INTEGER PRIMARY KEY,
			name TEXT,
			slug TEXT UNIQUE,
			description TEXT,
			privacy TEXT,
			permission TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS ghub_repos (
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
		`CREATE TABLE IF NOT EXISTS ghub_team_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ghub_team_id INTEGER,
			ghub_user_id INTEGER,
			user_login TEXT,
			team_slug TEXT,
			role TEXT,
			created_at TEXT,
			UNIQUE (ghub_team_id, ghub_user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS ghub_token_permissions (
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
		`CREATE TABLE IF NOT EXISTS ghub_outside_users (
			id INTEGER PRIMARY KEY,
			login TEXT UNIQUE,
			name TEXT,
			email TEXT,
			company TEXT,
			location TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS ghub_repos_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ghub_repos_id INTEGER,
			repos_name TEXT,
			user_login TEXT,
			ghub_user_id INTEGER,
			permission TEXT,
			created_at TEXT,
			updated_at TEXT,
			UNIQUE (repos_name, user_login)
		)`,
		`CREATE TABLE IF NOT EXISTS ghub_repos_teams (
			id INTEGER NOT NULL,
			ghub_repos_id INTEGER,
			repos_name TEXT NOT NULL,
			ghub_team_id INTEGER,
			team_name TEXT NOT NULL,
			team_slug TEXT NOT NULL,
			description TEXT,
			privacy TEXT,
			permission TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (repos_name, id)
		)`,
	}

	for _, query := range tables {
		session.Debugf("SQL: %s", query)
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// indexes
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_token_permissions_created_at ON ghub_token_permissions(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_ghub_repos_users_repos_name ON ghub_repos_users(repos_name)`,
		`CREATE INDEX IF NOT EXISTS idx_ghub_repos_users_user_login ON ghub_repos_users(user_login)`,
		`CREATE INDEX IF NOT EXISTS idx_ghub_repos_teams_repos_name ON ghub_repos_teams(repos_name)`,
	}
	for _, idx := range indexes {
		session.Debugf("SQL: %s", idx)
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

var permissionPriority = []string{"admin", "maintain", "push", "triage", "pull"}

// selectHighestPermission returns the most privileged permission that is true in the GitHub permissions map.
func selectHighestPermission(perms map[string]bool) string {
	if len(perms) == 0 {
		return ""
	}
	for _, key := range permissionPriority {
		if perms[key] {
			return key
		}
	}
	return ""
}

// normalizePermissionValue trims surrounding whitespace and lowercases the permission string for consistent storage.
func normalizePermissionValue(p string) string {
	return strings.ToLower(strings.TrimSpace(p))
}

// resolvedCollaboratorPermission extracts and normalizes the highest permission for a repository collaborator.
func resolvedCollaboratorPermission(u *github.User) string {
	if u == nil {
		return ""
	}
	highest := selectHighestPermission(u.Permissions)
	return normalizePermissionValue(highest)
}

// ListRepositoryNames returns repository names stored in ghub_repos ordered alphabetically.
func ListRepositoryNames(db DBTX) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required to list repositories")
	}
	query := `SELECT name FROM ghub_repos ORDER BY name`
	session.Debugf("SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query repositories: %w", err)
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name sql.NullString
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan repository name: %w", err)
		}
		if trimmed := strings.TrimSpace(name.String); trimmed != "" {
			names = append(names, trimmed)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository iteration failed: %w", err)
	}
	return names, nil
}

// permissionRank reports the priority index of a permission; unknown values are ranked lowest.
func permissionRank(p string) int {
	for idx, key := range permissionPriority {
		if p == key {
			return idx
		}
	}
	return len(permissionPriority)
}

func maxPermission(current, candidate string) string {
	currentNorm := normalizePermissionValue(current)
	candidateNorm := normalizePermissionValue(candidate)

	if candidateNorm == "" {
		return currentNorm
	}
	if currentNorm == "" {
		return candidateNorm
	}
	if permissionRank(candidateNorm) < permissionRank(currentNorm) {
		return candidateNorm
	}
	return currentNorm
}

// StoreUsers stores GitHub users in the database
func StoreUsers(db DBTX, users []*github.User) error {
	if len(users) == 0 {
		return nil
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	rows := make([][]any, 0, len(users))
	for _, u := range users {
		rows = append(rows, []any{
			u.GetID(),
			u.GetLogin(),
			u.GetName(),
			u.GetEmail(),
			u.GetCompany(),
			u.GetLocation(),
			now,
			now,
		})
	}

	columns := []string{"id", "login", "name", "email", "company", "location", "created_at", "updated_at"}
	if err := insertOrReplaceBatch(db, "ghub_users", columns, rows); err != nil {
		return fmt.Errorf("failed to insert users: %w", err)
	}
	return nil
}

// StoreUsersWithDetails stores GitHub users with detailed information
// This function expects that detailed user information has already been fetched by the caller
func StoreUsersWithDetails(db DBTX, users []*github.User) error {
	return StoreUsers(db, users)
}

// StoreTeams stores GitHub teams in the database
func StoreTeams(db DBTX, teams []*github.Team) error {
	if len(teams) == 0 {
		return nil
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	rows := make([][]any, 0, len(teams))
	for _, t := range teams {
		rows = append(rows, []any{
			t.GetID(),
			t.GetName(),
			t.GetSlug(),
			t.GetDescription(),
			t.GetPrivacy(),
			t.GetPermission(),
			now,
			now,
		})
	}

	columns := []string{"id", "name", "slug", "description", "privacy", "permission", "created_at", "updated_at"}
	if err := insertOrReplaceBatch(db, "ghub_teams", columns, rows); err != nil {
		return fmt.Errorf("failed to insert teams: %w", err)
	}
	return nil
}

// StoreRepositories stores GitHub repositories in the database
func StoreRepositories(db DBTX, repos []*github.Repository) error {
	if len(repos) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(repos))
	for _, r := range repos {
		rows = append(rows, []any{
			r.GetID(),
			r.GetName(),
			r.GetFullName(),
			r.GetDescription(),
			r.GetPrivate(),
			r.GetLanguage(),
			r.GetSize(),
			r.GetStargazersCount(),
			r.GetWatchersCount(),
			r.GetForksCount(),
			formatTime(r.GetCreatedAt()),
			formatTime(r.GetUpdatedAt()),
			formatTime(r.GetPushedAt()),
		})
	}

	columns := []string{"id", "name", "full_name", "description", "private", "language", "size", "stargazers_count", "watchers_count", "forks_count", "created_at", "updated_at", "pushed_at"}
	if err := insertOrReplaceBatch(db, "ghub_repos", columns, rows); err != nil {
		return fmt.Errorf("failed to insert repositories: %w", err)
	}
	return nil
}

// StoreTeamUsers stores team users in the database
func StoreTeamUsers(db DBTX, users []*github.User, teamSlug string) error {
	// First get team ID from slug
	var teamID int64
	query := `SELECT id FROM ghub_teams WHERE slug = ?`
	session.Debugf("SQL: %s, ARGS: [%s]", query, teamSlug)
	err := db.QueryRow(query, teamSlug).Scan(&teamID)
	if err != nil {
		// TODO: Consider how to get them all at once
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("team %s data not found. Please run `ghub-desk pull --teams` first to get team information: %w", teamSlug, err)
		}
		return fmt.Errorf("failed to get team ID for slug %s: %w", teamSlug, err)
	}

	if len(users) == 0 {
		return nil
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	rows := make([][]any, 0, len(users))
	for _, u := range users {
		rows = append(rows, []any{
			teamID,
			u.GetID(),
			u.GetLogin(),
			teamSlug,
			"member",
			now,
		})
	}

	columns := []string{"ghub_team_id", "ghub_user_id", "user_login", "team_slug", "role", "created_at"}
	if err := insertOrReplaceBatch(db, "ghub_team_users", columns, rows); err != nil {
		return fmt.Errorf("failed to insert team users for %s: %w", teamSlug, err)
	}
	return nil
}

// UpsertTeamUser adds or updates a single team membership relation in the local database.
func UpsertTeamUser(db DBTX, teamSlug string, teamID int64, user *github.User, role string) error {
	if db == nil {
		return fmt.Errorf("database connection is required to upsert team user")
	}
	if user == nil {
		return fmt.Errorf("user information is required to upsert team user")
	}
	if teamID == 0 {
		return fmt.Errorf("team ID is required to upsert team user")
	}
	if role == "" {
		role = "member"
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	query := `INSERT OR REPLACE INTO ghub_team_users(ghub_team_id, ghub_user_id, user_login, team_slug, role, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	args := []any{teamID, user.GetID(), user.GetLogin(), teamSlug, role, now}
	session.Debugf("SQL: %s, ARGS: %v", query, args)
	_, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to upsert team user %s for team %s: %w", user.GetLogin(), teamSlug, err)
	}
	return nil
}

// DeleteTeamBySlug removes a team and its memberships from the local database.
func DeleteTeamBySlug(db DBTX, teamSlug string) error {
	if db == nil {
		return fmt.Errorf("database connection is required to delete team")
	}
	query1 := `DELETE FROM ghub_team_users WHERE team_slug = ?`
	session.Debugf("SQL: %s, ARGS: [%s]", query1, teamSlug)
	if _, err := db.Exec(query1, teamSlug); err != nil {
		return fmt.Errorf("failed to delete team users for team %s: %w", teamSlug, err)
	}
	query2 := `DELETE FROM ghub_teams WHERE slug = ?`
	session.Debugf("SQL: %s, ARGS: [%s]", query2, teamSlug)
	if _, err := db.Exec(query2, teamSlug); err != nil {
		return fmt.Errorf("failed to delete team %s: %w", teamSlug, err)
	}
	return nil
}

// DeleteUserByLogin removes a user and related memberships from the local database.
func DeleteUserByLogin(db DBTX, login string) error {
	if db == nil {
		return fmt.Errorf("database connection is required to delete user")
	}
	query1 := `DELETE FROM ghub_team_users WHERE user_login = ?`
	session.Debugf("SQL: %s, ARGS: [%s]", query1, login)
	if _, err := db.Exec(query1, login); err != nil {
		return fmt.Errorf("failed to delete team memberships for user %s: %w", login, err)
	}
	query2 := `DELETE FROM ghub_users WHERE login = ?`
	session.Debugf("SQL: %s, ARGS: [%s]", query2, login)
	if _, err := db.Exec(query2, login); err != nil {
		return fmt.Errorf("failed to delete user %s: %w", login, err)
	}
	return nil
}

// DeleteTeamUser removes a membership relation between a team and a user from the local database.
func DeleteTeamUser(db DBTX, teamSlug, userLogin string) error {
	if db == nil {
		return fmt.Errorf("database connection is required to delete team user relation")
	}
	query := `DELETE FROM ghub_team_users WHERE team_slug = ? AND user_login = ?`
	session.Debugf("SQL: %s, ARGS: [%s, %s]", query, teamSlug, userLogin)
	if _, err := db.Exec(query, teamSlug, userLogin); err != nil {
		return fmt.Errorf("failed to delete team user relation %s/%s: %w", teamSlug, userLogin, err)
	}
	return nil
}

// StoreOutsideUsers stores GitHub outside collaborators in the database
func StoreOutsideUsers(db DBTX, users []*github.User) error {
	if len(users) == 0 {
		return nil
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	rows := make([][]any, 0, len(users))
	for _, u := range users {
		rows = append(rows, []any{
			u.GetID(),
			u.GetLogin(),
			u.GetName(),
			u.GetEmail(),
			u.GetCompany(),
			u.GetLocation(),
			now,
			now,
		})
	}

	columns := []string{"id", "login", "name", "email", "company", "location", "created_at", "updated_at"}
	if err := insertOrReplaceBatch(db, "ghub_outside_users", columns, rows); err != nil {
		return fmt.Errorf("failed to store outside users: %w", err)
	}
	return nil
}

// StoreRepoUsers stores collaborators for a specific repository in the database.
func StoreRepoUsers(db DBTX, repoName string, users []*github.User) error {
	if db == nil {
		return fmt.Errorf("database connection is required to store repository users")
	}
	if repoName == "" {
		return fmt.Errorf("repository name is required to store repository users")
	}
	if len(users) == 0 {
		return nil
	}

	repoID, repoFound, err := lookupRepositoryID(db, repoName)
	if err != nil {
		return fmt.Errorf("failed to look up repository ID for %s: %w", repoName, err)
	}
	if !repoFound {
		fmt.Printf("WARNING: repository '%s' not found in ghub_repos. Run 'ghub-desk pull --repos' first to populate repository metadata.\n", repoName)
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	rows := make([][]any, 0, len(users))
	var repoIDValue any
	if repoFound {
		repoIDValue = repoID
	}
	for _, u := range users {
		resolvedPermission := resolvedCollaboratorPermission(u)
		rows = append(rows, []any{
			repoIDValue,
			repoName,
			u.GetLogin(),
			u.GetID(),
			resolvedPermission,
			now,
			now,
		})
	}

	columns := []string{"ghub_repos_id", "repos_name", "user_login", "ghub_user_id", "permission", "created_at", "updated_at"}
	if err := insertOrReplaceBatch(db, "ghub_repos_users", columns, rows); err != nil {
		return fmt.Errorf("failed to store repository users for %s: %w", repoName, err)
	}
	return nil
}

// StoreRepoTeams stores repository teams in the database.
func StoreRepoTeams(db DBTX, repoName string, teams []*github.Team) error {
	if db == nil {
		return fmt.Errorf("database connection is required to store repository teams")
	}
	if repoName == "" {
		return fmt.Errorf("repository name is required to store repository teams")
	}

	if len(teams) == 0 {
		return nil
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	repoID, repoFound, err := lookupRepositoryID(db, repoName)
	if err != nil {
		return fmt.Errorf("failed to look up repository ID for %s: %w", repoName, err)
	}
	if !repoFound {
		return fmt.Errorf("repository '%s' not found: %w", repoName, ErrRepoNotFound)
	}
	var repoIDValue any
	if repoFound {
		repoIDValue = repoID
	}

	rows := make([][]any, 0, len(teams))
	for _, t := range teams {
		teamID := t.GetID()
		teamLocalID, teamFound, err := lookupTeamID(db, t.GetSlug())
		if err != nil {
			return fmt.Errorf("failed to look up team ID for %s: %w", t.GetSlug(), err)
		}
		if !teamFound {
			fmt.Printf("WARNING: team '%s' not found in ghub_teams. Run 'ghub-desk pull --teams' first to populate team metadata.\n", t.GetSlug())
		}
		var teamLocalIDValue any
		if teamFound {
			teamLocalIDValue = teamLocalID
		}

		rows = append(rows, []any{
			repoName,
			teamID,
			repoIDValue,
			teamLocalIDValue,
			t.GetName(),
			t.GetSlug(),
			t.GetDescription(),
			t.GetPrivacy(),
			t.GetPermission(),
			now,
			now,
		})
	}

	columns := []string{"repos_name", "id", "ghub_repos_id", "ghub_team_id", "team_name", "team_slug", "description", "privacy", "permission", "created_at", "updated_at"}
	if err := insertOrReplaceBatch(db, "ghub_repos_teams", columns, rows); err != nil {
		return fmt.Errorf("failed to store repository teams for %s: %w", repoName, err)
	}
	return nil
}

// UpsertRepoUser adds or updates a single repository collaborator entry.
func UpsertRepoUser(db DBTX, repoName string, user *github.User) error {
	if db == nil {
		return fmt.Errorf("database connection is required to upsert repository user")
	}
	if repoName == "" {
		return fmt.Errorf("repository name is required to upsert repository user")
	}
	if user == nil {
		return fmt.Errorf("user information is required to upsert repository user")
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	resolvedPermission := resolvedCollaboratorPermission(user)
	repoID, repoFound, err := lookupRepositoryID(db, repoName)
	if err != nil {
		return fmt.Errorf("failed to look up repository ID for %s: %w", repoName, err)
	}
	var repoIDValue any
	if repoFound {
		repoIDValue = repoID
	} else {
		fmt.Printf("WARNING: repository '%s' not found in ghub_repos. Run 'ghub-desk pull --repos' first to populate repository metadata.\n", repoName)
	}
	query := `INSERT OR REPLACE INTO ghub_repos_users(ghub_repos_id, repos_name, user_login, ghub_user_id, permission, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	args := []any{repoIDValue, repoName, user.GetLogin(), user.GetID(), resolvedPermission, now, now}
	session.Debugf("SQL: %s, ARGS: %v", query, args)
	_, err = db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to upsert repository user %s for repo %s: %w", user.GetLogin(), repoName, err)
	}
	return nil
}

// DeleteRepoUser removes a repository collaborator entry from the database.
func DeleteRepoUser(db DBTX, repoName, userLogin string) error {
	if db == nil {
		return fmt.Errorf("database connection is required to delete repository user")
	}
	if repoName == "" || userLogin == "" {
		return fmt.Errorf("repository name and user login are required to delete repository user")
	}
	query := `DELETE FROM ghub_repos_users WHERE repos_name = ? AND user_login = ?`
	session.Debugf("SQL: %s, ARGS: [%s, %s]", query, repoName, userLogin)
	if _, err := db.Exec(query, repoName, userLogin); err != nil {
		return fmt.Errorf("failed to delete repository user %s for repo %s: %w", userLogin, repoName, err)
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

func lookupRepositoryID(db DBTX, repoName string) (int64, bool, error) {
	if db == nil {
		return 0, false, fmt.Errorf("database connection is required to look up repository ID")
	}
	var repoID int64
	query := `SELECT id FROM ghub_repos WHERE name = ?`
	session.Debugf("SQL: %s, ARGS: [%s]", query, repoName)
	err := db.QueryRow(query, repoName).Scan(&repoID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return repoID, true, nil
}

func lookupTeamID(db DBTX, teamSlug string) (int64, bool, error) {
	if db == nil {
		return 0, false, fmt.Errorf("database connection is required to look up team ID")
	}
	var teamID int64
	query := `SELECT id FROM ghub_teams WHERE slug = ?`
	session.Debugf("SQL: %s, ARGS: [%s]", query, teamSlug)
	err := db.QueryRow(query, teamSlug).Scan(&teamID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return teamID, true, nil
}

// ClearTable deletes all rows from a specified table.
func ClearTable(db DBTX, tableName string) error {
	query := "DELETE FROM " + tableName
	session.Debugf("SQL: %s", query)
	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("failed to clear table %s: %w", tableName, err)
	}
	return nil
}
