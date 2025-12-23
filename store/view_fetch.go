package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"ghub-desk/session"
)

// UserEntry represents a user record stored in the database.
type UserEntry struct {
	ID       int64  `json:"id" yaml:"id"`
	Login    string `json:"login" yaml:"login"`
	Name     string `json:"name" yaml:"name"`
	Email    string `json:"email" yaml:"email"`
	Company  string `json:"company" yaml:"company"`
	Location string `json:"location" yaml:"location"`
}

// UserProfileEntry represents a user profile with audit timestamps.
type UserProfileEntry struct {
	ID        int64  `json:"id" yaml:"id"`
	Login     string `json:"login" yaml:"login"`
	Name      string `json:"name" yaml:"name"`
	Email     string `json:"email" yaml:"email"`
	Company   string `json:"company" yaml:"company"`
	Location  string `json:"location" yaml:"location"`
	CreatedAt string `json:"created_at" yaml:"created_at"`
	UpdatedAt string `json:"updated_at" yaml:"updated_at"`
}

// TeamEntry represents a team record stored in the database.
type TeamEntry struct {
	ID          int64  `json:"id" yaml:"id"`
	Slug        string `json:"slug" yaml:"slug"`
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Privacy     string `json:"privacy" yaml:"privacy"`
}

// RepositoryEntry represents a repository record stored in the database.
type RepositoryEntry struct {
	ID          int64  `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	FullName    string `json:"full_name" yaml:"full_name"`
	Description string `json:"description" yaml:"description"`
	Private     bool   `json:"private" yaml:"private"`
	Language    string `json:"language" yaml:"language"`
	Stars       int    `json:"stargazers_count" yaml:"stargazers_count"`
}

// RepoUserEntry represents a direct collaborator on a repository.
type RepoUserEntry struct {
	UserID     int64  `json:"user_id" yaml:"user_id"`
	Login      string `json:"login" yaml:"login"`
	Permission string `json:"permission,omitempty" yaml:"permission,omitempty"`
}

// RepoTeamEntry represents a team associated with a repository.
type RepoTeamEntry struct {
	ID          int64  `json:"id" yaml:"id"`
	Slug        string `json:"team_slug" yaml:"team_slug"`
	Name        string `json:"team_name" yaml:"team_name"`
	Permission  string `json:"permission" yaml:"permission"`
	Privacy     string `json:"privacy" yaml:"privacy"`
	Description string `json:"description" yaml:"description"`
}

// TeamUserEntry represents a user belonging to a team.
type TeamUserEntry struct {
	UserID int64  `json:"user_id" yaml:"user_id"`
	Login  string `json:"login" yaml:"login"`
	Role   string `json:"role" yaml:"role"`
}

// UserTeamEntry represents a team that a user belongs to.
type UserTeamEntry struct {
	TeamSlug string `json:"team_slug" yaml:"team_slug"`
	TeamName string `json:"team_name" yaml:"team_name"`
	Role     string `json:"role" yaml:"role"`
}

// TeamRepositoryEntry represents a repository accessible by a team.
type TeamRepositoryEntry struct {
	RepoName    string `json:"repo_name" yaml:"repo_name"`
	FullName    string `json:"full_name" yaml:"full_name"`
	Permission  string `json:"permission" yaml:"permission"`
	Privacy     string `json:"privacy" yaml:"privacy"`
	Description string `json:"description" yaml:"description"`
}

// AllTeamsUsersEntry represents a flattened team-user relationship.
type AllTeamsUsersEntry struct {
	TeamSlug  string `json:"team_slug" yaml:"team_slug"`
	TeamName  string `json:"team_name" yaml:"team_name"`
	UserLogin string `json:"user_login" yaml:"user_login"`
	UserName  string `json:"user_name" yaml:"user_name"`
	Role      string `json:"role" yaml:"role"`
}

// AllReposUsersEntry represents a flattened repository-user relationship.
type AllReposUsersEntry struct {
	RepoName   string `json:"repo_name" yaml:"repo_name"`
	FullName   string `json:"full_name" yaml:"full_name"`
	UserLogin  string `json:"user_login" yaml:"user_login"`
	UserName   string `json:"user_name" yaml:"user_name"`
	Permission string `json:"permission" yaml:"permission"`
}

// AllReposTeamsEntry represents a flattened repository-team relationship.
type AllReposTeamsEntry struct {
	RepoName    string `json:"repo_name" yaml:"repo_name"`
	FullName    string `json:"full_name" yaml:"full_name"`
	TeamSlug    string `json:"team_slug" yaml:"team_slug"`
	TeamName    string `json:"team_name" yaml:"team_name"`
	Permission  string `json:"permission" yaml:"permission"`
	Privacy     string `json:"privacy" yaml:"privacy"`
	Description string `json:"description" yaml:"description"`
}

// UserRepoAccessEntry represents repository access for a user with access paths.
type UserRepoAccessEntry struct {
	Repository string   `json:"repository" yaml:"repository"`
	AccessFrom []string `json:"access_from" yaml:"access_from"`
	Permission string   `json:"permission" yaml:"permission"`
}

// TokenPermissionEntry represents stored token permission metadata.
type TokenPermissionEntry struct {
	Scopes                    string `json:"scopes" yaml:"scopes"`
	OAuthScopes               string `json:"oauth_scopes" yaml:"oauth_scopes"`
	AcceptedOAuthScopes       string `json:"accepted_oauth_scopes" yaml:"accepted_oauth_scopes"`
	AcceptedGitHubPermissions string `json:"accepted_github_permissions" yaml:"accepted_github_permissions"`
	GitHubMediaType           string `json:"github_media_type" yaml:"github_media_type"`
	RateLimit                 int    `json:"rate_limit" yaml:"rate_limit"`
	RateRemaining             int    `json:"rate_remaining" yaml:"rate_remaining"`
	RateReset                 int    `json:"rate_reset" yaml:"rate_reset"`
	CreatedAt                 string `json:"created_at" yaml:"created_at"`
	UpdatedAt                 string `json:"updated_at" yaml:"updated_at"`
}

// FetchUsers retrieves all users ordered by login.
func FetchUsers(db *sql.DB) ([]UserEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required to fetch users")
	}
	query := `SELECT id, login, name, email, company, location FROM ghub_users ORDER BY login`
	session.Debugf("SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var records []UserEntry
	for rows.Next() {
		var id int64
		var login, name, email, company, location sql.NullString
		if err := rows.Scan(&id, &login, &name, &email, &company, &location); err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}
		records = append(records, UserEntry{
			ID:       id,
			Login:    login.String,
			Name:     name.String,
			Email:    email.String,
			Company:  company.String,
			Location: location.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate user rows: %w", err)
	}
	return records, nil
}

// FetchUserProfile retrieves a single user profile.
func FetchUserProfile(db *sql.DB, userLogin string) (UserProfileEntry, bool, error) {
	if db == nil {
		return UserProfileEntry{}, false, fmt.Errorf("database connection is required to fetch user")
	}
	cleanLogin := strings.TrimSpace(userLogin)
	if cleanLogin == "" {
		return UserProfileEntry{}, false, fmt.Errorf("user login is required to fetch user")
	}

	query := `
		SELECT id, login, COALESCE(name, ''), COALESCE(email, ''), COALESCE(company, ''), COALESCE(location, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
		FROM ghub_users
		WHERE login = ?
	`
	session.Debugf("SQL: %s, ARGS: [%s]", query, cleanLogin)

	var record UserProfileEntry
	err := db.QueryRow(query, cleanLogin).Scan(
		&record.ID,
		&record.Login,
		&record.Name,
		&record.Email,
		&record.Company,
		&record.Location,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return UserProfileEntry{Login: cleanLogin}, false, nil
	}
	if err != nil {
		return UserProfileEntry{}, false, fmt.Errorf("failed to query user %s: %w", cleanLogin, err)
	}

	record.Login = strings.TrimSpace(record.Login)
	record.Name = strings.TrimSpace(record.Name)
	record.Email = strings.TrimSpace(record.Email)
	record.Company = strings.TrimSpace(record.Company)
	record.Location = strings.TrimSpace(record.Location)
	record.CreatedAt = strings.TrimSpace(record.CreatedAt)
	record.UpdatedAt = strings.TrimSpace(record.UpdatedAt)

	return record, true, nil
}

// FetchTeams retrieves all teams ordered by slug.
func FetchTeams(db *sql.DB) ([]TeamEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required to fetch teams")
	}
	query := `SELECT id, slug, name, description, privacy FROM ghub_teams ORDER BY slug`
	session.Debugf("SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query teams: %w", err)
	}
	defer rows.Close()

	var records []TeamEntry
	for rows.Next() {
		var id int64
		var slug, name, description, privacy sql.NullString
		if err := rows.Scan(&id, &slug, &name, &description, &privacy); err != nil {
			return nil, fmt.Errorf("failed to scan team row: %w", err)
		}
		records = append(records, TeamEntry{
			ID:          id,
			Slug:        slug.String,
			Name:        name.String,
			Description: description.String,
			Privacy:     privacy.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate team rows: %w", err)
	}
	return records, nil
}

// FetchRepositories retrieves all repositories ordered by name.
func FetchRepositories(db *sql.DB) ([]RepositoryEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required to fetch repositories")
	}
	query := `
		SELECT id, name, full_name, description, private, language, stargazers_count 
		FROM ghub_repos ORDER BY name`
	session.Debugf("SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query repositories: %w", err)
	}
	defer rows.Close()

	var records []RepositoryEntry
	for rows.Next() {
		var id int64
		var private bool
		var name, fullName, description, language sql.NullString
		var stars int
		if err := rows.Scan(&id, &name, &fullName, &description, &private, &language, &stars); err != nil {
			return nil, fmt.Errorf("failed to scan repository row: %w", err)
		}
		records = append(records, RepositoryEntry{
			ID:          id,
			Name:        name.String,
			FullName:    fullName.String,
			Description: description.String,
			Private:     private,
			Language:    language.String,
			Stars:       stars,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate repository rows: %w", err)
	}
	return records, nil
}

// FetchOutsideUsers retrieves outside collaborators ordered by login.
func FetchOutsideUsers(db *sql.DB) ([]UserEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required to fetch outside users")
	}
	query := `SELECT id, login, name, email, company, location FROM ghub_outside_users ORDER BY login`
	session.Debugf("SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query outside users: %w", err)
	}
	defer rows.Close()

	var records []UserEntry
	for rows.Next() {
		var id int64
		var login, name, email, company, location sql.NullString
		if err := rows.Scan(&id, &login, &name, &email, &company, &location); err != nil {
			return nil, fmt.Errorf("failed to scan outside user row: %w", err)
		}
		records = append(records, UserEntry{
			ID:       id,
			Login:    login.String,
			Name:     name.String,
			Email:    email.String,
			Company:  company.String,
			Location: location.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate outside user rows: %w", err)
	}
	return records, nil
}

// FetchRepoUsers retrieves direct collaborators for a repository along with metadata.
func FetchRepoUsers(db *sql.DB, repoName string) (string, string, []RepoUserEntry, error) {
	if db == nil {
		return repoName, "", nil, fmt.Errorf("database connection is required to fetch repository users")
	}
	repoDisplay := repoName
	var fullName string

	var displayName sql.NullString
	var fullRepo sql.NullString
	err := db.QueryRow(`SELECT name, full_name FROM ghub_repos WHERE name = ? LIMIT 1`, repoName).Scan(&displayName, &fullRepo)
	if err != nil && err != sql.ErrNoRows {
		return repoDisplay, fullName, nil, fmt.Errorf("failed to fetch repository metadata: %w", err)
	}
	if err == nil {
		if trimmed := strings.TrimSpace(displayName.String); trimmed != "" {
			repoDisplay = trimmed
		}
		fullName = strings.TrimSpace(fullRepo.String)
	}

	query := `
		SELECT ghub_user_id, user_login, COALESCE(permission, '')
		FROM ghub_repos_users
		WHERE repos_name = ?
		ORDER BY user_login`
	session.Debugf("SQL: %s, ARGS: [%s]", query, repoName)
	rows, err := db.Query(query, repoName)
	if err != nil {
		return repoDisplay, fullName, nil, fmt.Errorf("failed to query repository users: %w", err)
	}
	defer rows.Close()

	var records []RepoUserEntry
	for rows.Next() {
		var userID sql.NullInt64
		var login, permission sql.NullString
		if err := rows.Scan(&userID, &login, &permission); err != nil {
			return repoDisplay, fullName, nil, fmt.Errorf("failed to scan repository user row: %w", err)
		}
		records = append(records, RepoUserEntry{
			UserID:     userID.Int64,
			Login:      strings.TrimSpace(login.String),
			Permission: normalizePermissionValue(permission.String),
		})
	}
	if err := rows.Err(); err != nil {
		return repoDisplay, fullName, nil, fmt.Errorf("failed to iterate repository user rows: %w", err)
	}
	return repoDisplay, fullName, records, nil
}

// FetchRepoTeams retrieves teams associated with a repository along with metadata.
func FetchRepoTeams(db *sql.DB, repoName string) (string, string, []RepoTeamEntry, error) {
	if db == nil {
		return repoName, "", nil, fmt.Errorf("database connection is required to fetch repository teams")
	}
	repoDisplay := repoName
	var fullName string

	var displayName sql.NullString
	var fullRepo sql.NullString
	err := db.QueryRow(`SELECT name, full_name FROM ghub_repos WHERE name = ? LIMIT 1`, repoName).Scan(&displayName, &fullRepo)
	if err != nil && err != sql.ErrNoRows {
		return repoDisplay, fullName, nil, fmt.Errorf("failed to fetch repository metadata: %w", err)
	}
	if err == nil {
		if trimmed := strings.TrimSpace(displayName.String); trimmed != "" {
			repoDisplay = trimmed
		}
		fullName = strings.TrimSpace(fullRepo.String)
	}

	query := `
		SELECT id, team_slug, team_name, COALESCE(permission, ''), COALESCE(privacy, ''), COALESCE(description, '')
		FROM ghub_repos_teams
		WHERE repos_name = ?
		ORDER BY team_slug`
	session.Debugf("SQL: %s, ARGS: [%s]", query, repoName)
	rows, err := db.Query(query, repoName)
	if err != nil {
		return repoDisplay, fullName, nil, fmt.Errorf("failed to query repository teams: %w", err)
	}
	defer rows.Close()

	var records []RepoTeamEntry
	for rows.Next() {
		var id sql.NullInt64
		var slug, name, permission, privacy, description sql.NullString
		if err := rows.Scan(&id, &slug, &name, &permission, &privacy, &description); err != nil {
			return repoDisplay, fullName, nil, fmt.Errorf("failed to scan repository team row: %w", err)
		}
		records = append(records, RepoTeamEntry{
			ID:          id.Int64,
			Slug:        strings.TrimSpace(slug.String),
			Name:        strings.TrimSpace(name.String),
			Permission:  normalizePermissionValue(permission.String),
			Privacy:     strings.TrimSpace(privacy.String),
			Description: strings.TrimSpace(description.String),
		})
	}
	if err := rows.Err(); err != nil {
		return repoDisplay, fullName, nil, fmt.Errorf("failed to iterate repository team rows: %w", err)
	}
	return repoDisplay, fullName, records, nil
}

// FetchUserTeams retrieves teams that a user belongs to.
func FetchUserTeams(db *sql.DB, userLogin string) ([]UserTeamEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required to fetch user teams")
	}
	cleanLogin := strings.TrimSpace(userLogin)
	if cleanLogin == "" {
		return nil, fmt.Errorf("user login is required to fetch user teams")
	}

	query := `
		SELECT 
			tu.team_slug,
			COALESCE(t.name, '') AS team_name,
			COALESCE(tu.role, '') AS role
		FROM ghub_team_users tu
		LEFT JOIN ghub_teams t ON t.slug = tu.team_slug
		WHERE tu.user_login = ?
		ORDER BY LOWER(tu.team_slug)
	`
	session.Debugf("SQL: %s, ARGS: [%s]", query, cleanLogin)
	rows, err := db.Query(query, cleanLogin)
	if err != nil {
		return nil, fmt.Errorf("failed to query teams for user %s: %w", cleanLogin, err)
	}
	defer rows.Close()

	var entries []UserTeamEntry
	for rows.Next() {
		var teamSlug, teamName, role sql.NullString
		if err := rows.Scan(&teamSlug, &teamName, &role); err != nil {
			return nil, fmt.Errorf("failed to scan user team row: %w", err)
		}
		entries = append(entries, UserTeamEntry{
			TeamSlug: strings.TrimSpace(teamSlug.String),
			TeamName: strings.TrimSpace(teamName.String),
			Role:     strings.TrimSpace(role.String),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate user team rows: %w", err)
	}
	return entries, nil
}

// FetchTeamUsers retrieves users belonging to a team.
func FetchTeamUsers(db *sql.DB, teamSlug string) ([]TeamUserEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required to fetch team users")
	}
	cleanSlug := strings.TrimSpace(teamSlug)
	if cleanSlug == "" {
		return nil, fmt.Errorf("team slug is required to fetch team users")
	}

	query := `
		SELECT ghub_user_id, user_login, role 
		FROM ghub_team_users 
		WHERE team_slug = ? 
		ORDER BY user_login`
	session.Debugf("SQL: %s, ARGS: [%s]", query, cleanSlug)
	rows, err := db.Query(query, cleanSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to query team users: %w", err)
	}
	defer rows.Close()

	var records []TeamUserEntry
	for rows.Next() {
		var userID int64
		var login, role sql.NullString
		if err := rows.Scan(&userID, &login, &role); err != nil {
			return nil, fmt.Errorf("failed to scan team user row: %w", err)
		}
		records = append(records, TeamUserEntry{
			UserID: userID,
			Login:  login.String,
			Role:   role.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate team user rows: %w", err)
	}
	return records, nil
}

// FetchTeamRepositories retrieves repositories a team can access.
func FetchTeamRepositories(db *sql.DB, teamSlug string) ([]TeamRepositoryEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required to fetch team repositories")
	}
	cleanSlug := strings.TrimSpace(teamSlug)
	if cleanSlug == "" {
		return nil, fmt.Errorf("team slug is required to fetch team repositories")
	}

	query := `
		SELECT 
			COALESCE(r.name, rt.repos_name) AS repo_name,
			COALESCE(r.full_name, '') AS repo_full_name,
			COALESCE(rt.permission, '') AS permission,
			COALESCE(rt.privacy, '') AS privacy,
			COALESCE(rt.description, '') AS description,
			rt.repos_name
		FROM ghub_repos_teams rt
		LEFT JOIN ghub_repos r ON r.name = rt.repos_name
		WHERE rt.team_slug = ?
		ORDER BY LOWER(repo_name)
	`
	session.Debugf("SQL: %s, ARGS: [%s]", query, cleanSlug)
	rows, err := db.Query(query, cleanSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to query repositories for team %s: %w", cleanSlug, err)
	}
	defer rows.Close()

	var entries []TeamRepositoryEntry
	for rows.Next() {
		var repoName, fullName, permission, privacy, description, fallbackRepo sql.NullString
		if err := rows.Scan(&repoName, &fullName, &permission, &privacy, &description, &fallbackRepo); err != nil {
			return nil, fmt.Errorf("failed to scan team repository row: %w", err)
		}
		name := strings.TrimSpace(repoName.String)
		if name == "" {
			name = strings.TrimSpace(fallbackRepo.String)
		}
		entries = append(entries, TeamRepositoryEntry{
			RepoName:    name,
			FullName:    strings.TrimSpace(fullName.String),
			Permission:  normalizePermissionValue(permission.String),
			Privacy:     strings.TrimSpace(privacy.String),
			Description: strings.TrimSpace(description.String),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate team repository rows: %w", err)
	}
	return entries, nil
}

// FetchAllRepositoriesUsers retrieves flattened repository-user relationships.
func FetchAllRepositoriesUsers(db *sql.DB) ([]AllReposUsersEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required to fetch repository users")
	}
	query := `
		SELECT 
			COALESCE(r.name, ru.repos_name) AS repo_name,
			COALESCE(r.full_name, '') AS repo_full_name,
			ru.user_login,
			COALESCE(u.name, '') AS user_name,
			COALESCE(ru.permission, '') AS permission
		FROM ghub_repos_users ru
		LEFT JOIN ghub_repos r ON r.name = ru.repos_name
		LEFT JOIN ghub_users u ON u.login = ru.user_login
		ORDER BY LOWER(repo_name), LOWER(ru.user_login)
	`
	session.Debugf("SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query repository users: %w", err)
	}
	defer rows.Close()

	var entries []AllReposUsersEntry
	for rows.Next() {
		var repoName, fullName, login, userName, permission sql.NullString
		if err := rows.Scan(&repoName, &fullName, &login, &userName, &permission); err != nil {
			return nil, fmt.Errorf("failed to scan repository user row: %w", err)
		}
		entries = append(entries, AllReposUsersEntry{
			RepoName:   strings.TrimSpace(repoName.String),
			FullName:   strings.TrimSpace(fullName.String),
			UserLogin:  strings.TrimSpace(login.String),
			UserName:   strings.TrimSpace(userName.String),
			Permission: normalizePermissionValue(permission.String),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate repository user rows: %w", err)
	}
	return entries, nil
}

// FetchAllRepositoriesTeams retrieves flattened repository-team relationships.
func FetchAllRepositoriesTeams(db *sql.DB) ([]AllReposTeamsEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required to fetch repository teams")
	}
	query := `
		SELECT 
			COALESCE(r.name, rt.repos_name) AS repo_name,
			COALESCE(r.full_name, '') AS repo_full_name,
			rt.team_slug,
			COALESCE(rt.team_name, '') AS team_name,
			COALESCE(rt.permission, '') AS permission,
			COALESCE(rt.privacy, '') AS privacy,
			COALESCE(rt.description, '') AS description
		FROM ghub_repos_teams rt
		LEFT JOIN ghub_repos r ON r.name = rt.repos_name
		ORDER BY LOWER(repo_name), LOWER(rt.team_slug)
	`
	session.Debugf("SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query repository teams: %w", err)
	}
	defer rows.Close()

	var entries []AllReposTeamsEntry
	for rows.Next() {
		var repoName, fullName, teamSlug, teamName, permission, privacy, description sql.NullString
		if err := rows.Scan(&repoName, &fullName, &teamSlug, &teamName, &permission, &privacy, &description); err != nil {
			return nil, fmt.Errorf("failed to scan repository team row: %w", err)
		}
		entries = append(entries, AllReposTeamsEntry{
			RepoName:    strings.TrimSpace(repoName.String),
			FullName:    strings.TrimSpace(fullName.String),
			TeamSlug:    strings.TrimSpace(teamSlug.String),
			TeamName:    strings.TrimSpace(teamName.String),
			Permission:  normalizePermissionValue(permission.String),
			Privacy:     strings.TrimSpace(privacy.String),
			Description: strings.TrimSpace(description.String),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate repository team rows: %w", err)
	}
	return entries, nil
}

// FetchAllTeamsUsers retrieves flattened team-user relationships.
func FetchAllTeamsUsers(db *sql.DB) ([]AllTeamsUsersEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required to fetch team users")
	}
	query := `
		SELECT 
			tu.team_slug,
			COALESCE(t.name, '') AS team_name,
			tu.user_login,
			COALESCE(u.name, '') AS user_name,
			COALESCE(tu.role, '') AS role
		FROM ghub_team_users tu
		LEFT JOIN ghub_teams t ON t.slug = tu.team_slug
		LEFT JOIN ghub_users u ON u.login = tu.user_login
		ORDER BY LOWER(tu.team_slug), LOWER(tu.user_login)
	`
	session.Debugf("SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query team users: %w", err)
	}
	defer rows.Close()

	var entries []AllTeamsUsersEntry
	for rows.Next() {
		var teamSlug, teamName, login, userName, role sql.NullString
		if err := rows.Scan(&teamSlug, &teamName, &login, &userName, &role); err != nil {
			return nil, fmt.Errorf("failed to scan team user row: %w", err)
		}
		entries = append(entries, AllTeamsUsersEntry{
			TeamSlug:  strings.TrimSpace(teamSlug.String),
			TeamName:  strings.TrimSpace(teamName.String),
			UserLogin: strings.TrimSpace(login.String),
			UserName:  strings.TrimSpace(userName.String),
			Role:      strings.TrimSpace(role.String),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate team user rows: %w", err)
	}
	return entries, nil
}

// FetchUserRepositories aggregates repository access for a user.
func FetchUserRepositories(db *sql.DB, userLogin string) ([]UserRepoAccessEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required to fetch user repositories")
	}
	cleanLogin := strings.TrimSpace(userLogin)
	if cleanLogin == "" {
		return nil, fmt.Errorf("user login is required to fetch repositories")
	}

	type repoAccessEntry struct {
		repoName string
		highest  string
		sources  []string
		seen     map[string]struct{}
	}

	accessByRepoName := make(map[string]*repoAccessEntry)
	mergeRepoAccess := func(repoName, sourceLabel, permission string) {
		name := strings.TrimSpace(repoName)
		if name == "" {
			return
		}
		entry, ok := accessByRepoName[name]
		if !ok {
			entry = &repoAccessEntry{
				repoName: name,
				highest:  "",
				sources:  make([]string, 0, 2),
				seen:     make(map[string]struct{}),
			}
			accessByRepoName[name] = entry
		}
		entry.highest = maxPermission(entry.highest, permission)

		displayPerm := normalizePermissionValue(permission)
		display := sourceLabel
		if displayPerm != "" {
			display = fmt.Sprintf("%s [%s]", sourceLabel, displayPerm)
		}
		if _, exists := entry.seen[display]; !exists {
			entry.sources = append(entry.sources, display)
			entry.seen[display] = struct{}{}
		}
	}

	directQuery := `
		SELECT COALESCE(r.name, ru.repos_name) AS repo_name,
		       COALESCE(ru.permission, ''),
		       ru.repos_name
		FROM ghub_repos_users ru
		LEFT JOIN ghub_repos r ON r.name = ru.repos_name
		WHERE ru.user_login = ?
	`
	session.Debugf("SQL: %s, ARGS: [%s]", directQuery, cleanLogin)
	directRows, err := db.Query(directQuery, cleanLogin)
	if err != nil {
		return nil, fmt.Errorf("failed to query direct repository access: %w", err)
	}
	defer directRows.Close()

	for directRows.Next() {
		var repoName, permission, fallback sql.NullString
		if err := directRows.Scan(&repoName, &permission, &fallback); err != nil {
			return nil, fmt.Errorf("failed to scan direct access row: %w", err)
		}
		name := repoName.String
		if strings.TrimSpace(name) == "" {
			name = fallback.String
		}
		mergeRepoAccess(name, "Direct", permission.String)
	}
	if err := directRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate direct access rows: %w", err)
	}

	teamQuery := `
		SELECT COALESCE(r.name, rt.repos_name) AS repo_name,
		       rt.team_slug,
		       COALESCE(rt.team_name, ''),
		       COALESCE(rt.permission, ''),
		       rt.repos_name
		FROM ghub_team_users tu
		JOIN ghub_repos_teams rt ON rt.team_slug = tu.team_slug
		LEFT JOIN ghub_repos r ON r.name = rt.repos_name
		WHERE tu.user_login = ?
	`
	session.Debugf("SQL: %s, ARGS: [%s]", teamQuery, cleanLogin)
	teamRows, err := db.Query(teamQuery, cleanLogin)
	if err != nil {
		return nil, fmt.Errorf("failed to query team-derived repository access: %w", err)
	}
	defer teamRows.Close()

	for teamRows.Next() {
		var repoName, teamSlug, teamName, permission, fallback sql.NullString
		if err := teamRows.Scan(&repoName, &teamSlug, &teamName, &permission, &fallback); err != nil {
			return nil, fmt.Errorf("failed to scan team access row: %w", err)
		}
		name := repoName.String
		if strings.TrimSpace(name) == "" {
			name = fallback.String
		}
		if strings.TrimSpace(name) == "" {
			continue
		}

		slug := strings.TrimSpace(teamSlug.String)
		if slug == "" {
			continue
		}
		label := fmt.Sprintf("Team:%s", slug)
		if displayName := strings.TrimSpace(teamName.String); displayName != "" {
			label = fmt.Sprintf("%s (%s)", label, displayName)
		}
		mergeRepoAccess(name, label, permission.String)
	}
	if err := teamRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate team access rows: %w", err)
	}

	if len(accessByRepoName) == 0 {
		return []UserRepoAccessEntry{}, nil
	}

	entries := make([]*repoAccessEntry, 0, len(accessByRepoName))
	for _, entry := range accessByRepoName {
		sort.Slice(entry.sources, func(i, j int) bool {
			si := entry.sources[i]
			sj := entry.sources[j]
			isDirect := strings.HasPrefix(si, "Direct")
			jsDirect := strings.HasPrefix(sj, "Direct")
			if isDirect != jsDirect {
				return isDirect
			}
			return si < sj
		})
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		li := strings.ToLower(entries[i].repoName)
		lj := strings.ToLower(entries[j].repoName)
		if li == lj {
			return entries[i].repoName < entries[j].repoName
		}
		return li < lj
	})

	output := make([]UserRepoAccessEntry, 0, len(entries))
	for _, entry := range entries {
		perm := entry.highest
		if perm == "" {
			perm = "-"
		}
		output = append(output, UserRepoAccessEntry{
			Repository: entry.repoName,
			AccessFrom: append([]string(nil), entry.sources...),
			Permission: perm,
		})
	}

	return output, nil
}

// FetchTokenPermission retrieves the latest token permission metadata.
func FetchTokenPermission(db *sql.DB) (TokenPermissionEntry, bool, error) {
	if db == nil {
		return TokenPermissionEntry{}, false, fmt.Errorf("database connection is required to fetch token permission")
	}
	query := `
		SELECT scopes, x_oauth_scopes, x_accepted_oauth_scopes, x_accepted_github_permissions, x_github_media_type,
		       x_ratelimit_limit, x_ratelimit_remaining, x_ratelimit_reset,
		       created_at, updated_at
		FROM ghub_token_permissions 
		ORDER BY created_at DESC 
		LIMIT 1`
	session.Debugf("SQL: %s", query)
	row := db.QueryRow(query)

	var record TokenPermissionEntry
	err := row.Scan(
		&record.Scopes,
		&record.OAuthScopes,
		&record.AcceptedOAuthScopes,
		&record.AcceptedGitHubPermissions,
		&record.GitHubMediaType,
		&record.RateLimit,
		&record.RateRemaining,
		&record.RateReset,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return TokenPermissionEntry{}, false, nil
	}
	if err != nil {
		return TokenPermissionEntry{}, false, fmt.Errorf("failed to query token permissions: %w", err)
	}

	return record, true, nil
}
