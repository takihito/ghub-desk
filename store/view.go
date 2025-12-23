package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"ghub-desk/session"
	"ghub-desk/validate"
)

// TargetRequest represents the requested view target including optional metadata.
type TargetRequest struct {
	Kind      string
	TeamSlug  string
	RepoName  string
	UserLogin string
}

// RepoTeamUserEntry represents a team member associated with a repository.
type RepoTeamUserEntry struct {
	TeamSlug       string
	TeamPermission string
	UserLogin      string
	Role           string
	Name           string
	Email          string
	Company        string
	Location       string
}

// HandleViewTarget processes different types of view targets
func HandleViewTarget(db *sql.DB, req TargetRequest, opts ViewOptions) error {
	format := opts.formatOrDefault()

	switch req.Kind {
	case "users", "detail-users":
		return ViewUsers(db, format)
	case "teams":
		return ViewTeams(db, format)
	case "repos", "repositories":
		return ViewRepositories(db, format)
	case "token-permission":
		return ViewTokenPermission(db, format)
	case "outside-users":
		return ViewOutsideUsers(db, format)
	case "user":
		if req.UserLogin == "" {
			return fmt.Errorf("user login must be specified when using user target")
		}
		if err := validate.ValidateUserName(req.UserLogin); err != nil {
			return fmt.Errorf("invalid user login: %w", err)
		}
		return ViewUser(db, req.UserLogin, format)
	case "user-teams":
		if req.UserLogin == "" {
			return fmt.Errorf("user login must be specified when using user-teams target")
		}
		if err := validate.ValidateUserName(req.UserLogin); err != nil {
			return fmt.Errorf("invalid user login: %w", err)
		}
		return ViewUserTeams(db, req.UserLogin, format)
	case "repos-users":
		if req.RepoName == "" {
			return fmt.Errorf("repository name must be specified when using repos-users target")
		}
		if err := validate.ValidateRepoName(req.RepoName); err != nil {
			return fmt.Errorf("invalid repository name: %w", err)
		}
		return ViewRepoUsers(db, req.RepoName, format)
	case "repos-teams":
		if req.RepoName == "" {
			return fmt.Errorf("repository name must be specified when using repos-teams target")
		}
		if err := validate.ValidateRepoName(req.RepoName); err != nil {
			return fmt.Errorf("invalid repository name: %w", err)
		}
		return ViewRepoTeams(db, req.RepoName, format)
	case "repos-teams-users":
		if req.RepoName == "" {
			return fmt.Errorf("repository name must be specified when using repos-teams-users target")
		}
		if err := validate.ValidateRepoName(req.RepoName); err != nil {
			return fmt.Errorf("invalid repository name: %w", err)
		}
		return ViewRepoTeamUsers(db, req.RepoName, format)
	case "all-repos-users":
		return ViewAllRepositoriesUsers(db, format)
	case "all-repos-teams":
		return ViewAllRepositoriesTeams(db, format)
	case "all-teams-users":
		return ViewAllTeamsUsers(db, format)
	case "team-repos":
		if req.TeamSlug == "" {
			return fmt.Errorf("team slug must be specified when using team-repos target")
		}
		if err := validate.ValidateTeamSlug(req.TeamSlug); err != nil {
			return fmt.Errorf("invalid team slug: %w", err)
		}
		return ViewTeamRepositories(db, req.TeamSlug, format)
	case "user-repos":
		if req.UserLogin == "" {
			return fmt.Errorf("user login must be specified when using user-repos target")
		}
		if err := validate.ValidateUserName(req.UserLogin); err != nil {
			return fmt.Errorf("invalid user login: %w", err)
		}
		return ViewUserRepositories(db, req.UserLogin, format)
	case "team-user":
		if req.TeamSlug == "" {
			return fmt.Errorf("team slug must be specified when using team-user target")
		}
		if err := validate.ValidateTeamSlug(req.TeamSlug); err != nil {
			return fmt.Errorf("invalid team slug: %w", err)
		}
		return ViewTeamUsers(db, req.TeamSlug, format)
	default:
		return fmt.Errorf("unknown target: %s", req.Kind)
	}
}

// printTableHeader prints a tab-separated header row followed by a matching underline row.
// Example: printTableHeader("ID", "Login") outputs:
// ID	Login
// --	-----
func printTableHeader(columns ...string) {
	if len(columns) == 0 {
		return
	}
	fmt.Println(strings.Join(columns, "\t"))

	under := make([]string, len(columns))
	for i, col := range columns {
		width := utf8.RuneCountInString(col)
		if width <= 0 {
			width = 1
		}
		under[i] = strings.Repeat("-", width)
	}
	fmt.Println(strings.Join(under, "\t"))
}

// ViewUsers displays users from the database
func ViewUsers(db *sql.DB, format OutputFormat) error {
	query := `SELECT id, login, name, email, company, location FROM ghub_users ORDER BY login`
	session.Debugf("SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	type userRecord struct {
		ID       int64  `json:"id" yaml:"id"`
		Login    string `json:"login" yaml:"login"`
		Name     string `json:"name" yaml:"name"`
		Email    string `json:"email" yaml:"email"`
		Company  string `json:"company" yaml:"company"`
		Location string `json:"location" yaml:"location"`
	}

	var records []userRecord
	for rows.Next() {
		var id int64
		var login, name, email, company, location sql.NullString
		err := rows.Scan(&id, &login, &name, &email, &company, &location)
		if err != nil {
			return fmt.Errorf("failed to scan user row: %w", err)
		}

		record := userRecord{
			ID:       id,
			Login:    login.String,
			Name:     name.String,
			Email:    email.String,
			Company:  company.String,
			Location: location.String,
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate user rows: %w", err)
	}

	tableFn := func() error {
		printTableHeader("ID", "Login", "Name", "Email", "Company", "Location")

		for _, record := range records {
			fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\n",
				record.ID,
				record.Login,
				record.Name,
				record.Email,
				record.Company,
				record.Location,
			)
		}
		return nil
	}

	return renderByFormat(format, tableFn, records)
}

// ViewUser displays a single user from the database
func ViewUser(db *sql.DB, userLogin string, format OutputFormat) error {
	if db == nil {
		return fmt.Errorf("database connection is required to view user")
	}
	cleanLogin := strings.TrimSpace(userLogin)
	if cleanLogin == "" {
		return fmt.Errorf("user login is required to view user")
	}

	query := `
		SELECT id, login, COALESCE(name, ''), COALESCE(email, ''), COALESCE(company, ''), COALESCE(location, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
		FROM ghub_users
		WHERE login = ?
	`
	session.Debugf("SQL: %s, ARGS: [%s]", query, cleanLogin)

	type userRecord struct {
		ID        int64  `json:"id" yaml:"id"`
		Login     string `json:"login" yaml:"login"`
		Name      string `json:"name" yaml:"name"`
		Email     string `json:"email" yaml:"email"`
		Company   string `json:"company" yaml:"company"`
		Location  string `json:"location" yaml:"location"`
		CreatedAt string `json:"created_at" yaml:"created_at"`
		UpdatedAt string `json:"updated_at" yaml:"updated_at"`
	}

	var record userRecord
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
		if format == FormatTable {
			fmt.Printf("No user found for login %s.\n", cleanLogin)
			fmt.Println("Run 'ghub-desk pull --users' first to populate user records.")
			return nil
		}
		payload := struct {
			User  string `json:"user" yaml:"user"`
			Found bool   `json:"found" yaml:"found"`
		}{
			User:  cleanLogin,
			Found: false,
		}
		return renderByFormat(format, nil, payload)
	}
	if err != nil {
		return fmt.Errorf("failed to query user %s: %w", cleanLogin, err)
	}

	record.Login = strings.TrimSpace(record.Login)
	record.Name = strings.TrimSpace(record.Name)
	record.Email = strings.TrimSpace(record.Email)
	record.Company = strings.TrimSpace(record.Company)
	record.Location = strings.TrimSpace(record.Location)
	record.CreatedAt = strings.TrimSpace(record.CreatedAt)
	record.UpdatedAt = strings.TrimSpace(record.UpdatedAt)

	tableFn := func() error {
		printTableHeader("ID", "Login", "Name", "Email", "Company", "Location", "Created At", "Updated At")

		normalize := func(s string) string {
			if s == "" {
				return "-"
			}
			return s
		}

		fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			record.ID,
			normalize(record.Login),
			normalize(record.Name),
			normalize(record.Email),
			normalize(record.Company),
			normalize(record.Location),
			normalize(record.CreatedAt),
			normalize(record.UpdatedAt),
		)
		return nil
	}

	return renderByFormat(format, tableFn, record)
}

// ViewTeams displays teams from the database
func ViewTeams(db *sql.DB, format OutputFormat) error {
	query := `SELECT id, slug, name, description, privacy FROM ghub_teams ORDER BY slug`
	session.Debugf("SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query teams: %w", err)
	}
	defer rows.Close()

	type teamRecord struct {
		ID          int64  `json:"id" yaml:"id"`
		Slug        string `json:"slug" yaml:"slug"`
		Name        string `json:"name" yaml:"name"`
		Description string `json:"description" yaml:"description"`
		Privacy     string `json:"privacy" yaml:"privacy"`
	}

	var records []teamRecord
	for rows.Next() {
		var id int64
		var slug, name, description, privacy sql.NullString
		err := rows.Scan(&id, &slug, &name, &description, &privacy)
		if err != nil {
			return fmt.Errorf("failed to scan team row: %w", err)
		}

		record := teamRecord{
			ID:          id,
			Slug:        slug.String,
			Name:        name.String,
			Description: description.String,
			Privacy:     privacy.String,
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate team rows: %w", err)
	}

	tableFn := func() error {
		printTableHeader("ID", "Slug", "Name", "Description", "Privacy")

		for _, record := range records {
			fmt.Printf("%d\t%s\t%s\t%s\t%s\n",
				record.ID,
				record.Slug,
				record.Name,
				record.Description,
				record.Privacy,
			)
		}
		return nil
	}

	return renderByFormat(format, tableFn, records)
}

// ViewRepositories displays repositories from the database
func ViewRepositories(db *sql.DB, format OutputFormat) error {
	query := `
		SELECT id, name, full_name, description, private, language, stargazers_count 
		FROM ghub_repos ORDER BY name`
	session.Debugf("SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query repositories: %w", err)
	}
	defer rows.Close()

	type repositoryRecord struct {
		ID          int64  `json:"id" yaml:"id"`
		Name        string `json:"name" yaml:"name"`
		FullName    string `json:"full_name" yaml:"full_name"`
		Description string `json:"description" yaml:"description"`
		Private     bool   `json:"private" yaml:"private"`
		Language    string `json:"language" yaml:"language"`
		Stars       int    `json:"stargazers_count" yaml:"stargazers_count"`
	}

	var records []repositoryRecord
	for rows.Next() {
		var id int64
		var private bool
		var name, fullName, description, language sql.NullString
		var stars int
		err := rows.Scan(&id, &name, &fullName, &description, &private, &language, &stars)
		if err != nil {
			return fmt.Errorf("failed to scan repository row: %w", err)
		}

		record := repositoryRecord{
			ID:          id,
			Name:        name.String,
			FullName:    fullName.String,
			Description: description.String,
			Private:     private,
			Language:    language.String,
			Stars:       stars,
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate repository rows: %w", err)
	}

	tableFn := func() error {
		printTableHeader("ID", "Name", "Full Name", "Description", "Private", "Language", "Stars")

		for _, record := range records {
			fmt.Printf("%d\t%s\t%s\t%s\t%t\t%s\t%d\n",
				record.ID,
				record.Name,
				record.FullName,
				record.Description,
				record.Private,
				record.Language,
				record.Stars,
			)
		}
		return nil
	}

	return renderByFormat(format, tableFn, records)
}

// ViewRepoUsers displays direct repository collaborators from the database
func ViewRepoUsers(db *sql.DB, repoName string, format OutputFormat) error {
	query := `
		SELECT ghub_user_id, user_login
		FROM ghub_repos_users
		WHERE repos_name = ?
		ORDER BY user_login`
	session.Debugf("SQL: %s, ARGS: [%s]", query, repoName)
	rows, err := db.Query(query, repoName)
	if err != nil {
		return fmt.Errorf("failed to query repository users: %w", err)
	}
	defer rows.Close()

	type repoUserRecord struct {
		UserID int64  `json:"user_id" yaml:"user_id"`
		Login  string `json:"login" yaml:"login"`
	}

	var records []repoUserRecord
	for rows.Next() {
		var userID sql.NullInt64
		var login sql.NullString
		if err := rows.Scan(&userID, &login); err != nil {
			return fmt.Errorf("failed to scan repository user row: %w", err)
		}
		record := repoUserRecord{
			UserID: userID.Int64,
			Login:  login.String,
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate repository user rows: %w", err)
	}

	tableFn := func() error {
		fmt.Printf("Repository: %s\n", repoName)
		printTableHeader("User ID", "Login")

		for _, record := range records {
			fmt.Printf("%d\t%s\n", record.UserID, record.Login)
		}
		return nil
	}

	payload := struct {
		Repository string           `json:"repository" yaml:"repository"`
		Users      []repoUserRecord `json:"users" yaml:"users"`
	}{
		Repository: repoName,
		Users:      records,
	}

	return renderByFormat(format, tableFn, payload)
}

// ViewRepoTeams displays repository teams from the database
func ViewRepoTeams(db *sql.DB, repoName string, format OutputFormat) error {
	query := `
		SELECT id, team_name, team_slug, description, privacy, permission
		FROM ghub_repos_teams
		WHERE repos_name = ?
		ORDER BY team_slug`
	session.Debugf("SQL: %s, ARGS: [%s]", query, repoName)
	rows, err := db.Query(query, repoName)
	if err != nil {
		return fmt.Errorf("failed to query repository teams: %w", err)
	}
	defer rows.Close()

	type repoTeamRecord struct {
		ID          int64  `json:"id" yaml:"id"`
		Slug        string `json:"team_slug" yaml:"team_slug"`
		Name        string `json:"team_name" yaml:"team_name"`
		Permission  string `json:"permission" yaml:"permission"`
		Privacy     string `json:"privacy" yaml:"privacy"`
		Description string `json:"description" yaml:"description"`
	}

	var records []repoTeamRecord
	for rows.Next() {
		var id sql.NullInt64
		var name, slug, description, privacy, permission sql.NullString
		if err := rows.Scan(&id, &name, &slug, &description, &privacy, &permission); err != nil {
			return fmt.Errorf("failed to scan repository team row: %w", err)
		}

		record := repoTeamRecord{
			ID:          id.Int64,
			Slug:        slug.String,
			Name:        name.String,
			Permission:  permission.String,
			Privacy:     privacy.String,
			Description: description.String,
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate repository team rows: %w", err)
	}

	tableFn := func() error {
		fmt.Printf("Repository: %s\n", repoName)
		printTableHeader("Team ID", "Slug", "Name", "Permission", "Privacy", "Description")

		for _, record := range records {
			fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\n",
				record.ID,
				record.Slug,
				record.Name,
				record.Permission,
				record.Privacy,
				record.Description,
			)
		}
		return nil
	}

	payload := struct {
		Repository string           `json:"repository" yaml:"repository"`
		Teams      []repoTeamRecord `json:"teams" yaml:"teams"`
	}{
		Repository: repoName,
		Teams:      records,
	}

	return renderByFormat(format, tableFn, payload)
}

// FetchRepoTeamUsers retrieves team members linked to a repository along with display names.
func FetchRepoTeamUsers(db *sql.DB, repoName string) (string, string, []RepoTeamUserEntry, error) {
	repoDisplay := repoName
	var fullName string

	if db == nil {
		return repoDisplay, fullName, nil, fmt.Errorf("database connection is required to fetch repository team users")
	}

	var (
		displayName sql.NullString
		fullRepo    sql.NullString
	)
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
		SELECT rt.team_slug,
		       COALESCE(rt.permission, ''),
		       COALESCE(u.login, tu.user_login),
		       COALESCE(tu.role, ''),
		       COALESCE(u.name, ''),
		       COALESCE(u.email, ''),
		       COALESCE(u.company, ''),
		       COALESCE(u.location, '')
		FROM ghub_repos_teams rt
		JOIN ghub_team_users tu ON tu.team_slug = rt.team_slug
		LEFT JOIN ghub_users u ON u.id = tu.ghub_user_id
		WHERE rt.repos_name = ?
		ORDER BY LOWER(rt.team_slug), LOWER(COALESCE(u.login, tu.user_login))
	`
	session.Debugf("SQL: %s, ARGS: [%s]", query, repoName)
	rows, err := db.Query(query, repoName)
	if err != nil {
		return repoDisplay, fullName, nil, fmt.Errorf("failed to query repository team users: %w", err)
	}
	defer rows.Close()

	var records []RepoTeamUserEntry
	for rows.Next() {
		var teamSlug, permission, login, role, name, email, company, location sql.NullString
		if err := rows.Scan(&teamSlug, &permission, &login, &role, &name, &email, &company, &location); err != nil {
			return repoDisplay, fullName, nil, fmt.Errorf("failed to scan repository team user row: %w", err)
		}
		records = append(records, RepoTeamUserEntry{
			TeamSlug:       strings.TrimSpace(teamSlug.String),
			TeamPermission: strings.TrimSpace(permission.String),
			UserLogin:      strings.TrimSpace(login.String),
			Role:           strings.TrimSpace(role.String),
			Name:           strings.TrimSpace(name.String),
			Email:          strings.TrimSpace(email.String),
			Company:        strings.TrimSpace(company.String),
			Location:       strings.TrimSpace(location.String),
		})
	}
	if err := rows.Err(); err != nil {
		return repoDisplay, fullName, nil, fmt.Errorf("failed to iterate repository team user rows: %w", err)
	}

	return repoDisplay, fullName, records, nil
}

// ViewRepoTeamUsers displays users belonging to teams associated with a repository.
func ViewRepoTeamUsers(db *sql.DB, repoName string, format OutputFormat) error {
	repoDisplay, _, records, err := FetchRepoTeamUsers(db, repoName)
	if err != nil {
		return err
	}

	tableFn := func() error {
		fmt.Printf("Repository: %s\n", repoDisplay)
		printTableHeader("Team Slug", "Team Permission", "User Login", "Role", "Name", "Email", "Company", "Location")

		for _, record := range records {
			fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				record.TeamSlug,
				record.TeamPermission,
				record.UserLogin,
				record.Role,
				record.Name,
				record.Email,
				record.Company,
				record.Location,
			)
		}
		return nil
	}

	payload := struct {
		Repository string              `json:"repository" yaml:"repository"`
		Members    []RepoTeamUserEntry `json:"members" yaml:"members"`
	}{
		Repository: repoDisplay,
		Members:    records,
	}

	return renderByFormat(format, tableFn, payload)
}

// ViewTeamRepositories displays repositories a team has access to.
func ViewTeamRepositories(db *sql.DB, teamSlug string, format OutputFormat) error {
	if db == nil {
		return fmt.Errorf("database connection is required to view team repositories")
	}
	cleanSlug := strings.TrimSpace(teamSlug)
	if cleanSlug == "" {
		return fmt.Errorf("team slug is required to view team repositories")
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
		return fmt.Errorf("failed to query repositories for team %s: %w", cleanSlug, err)
	}
	defer rows.Close()

	type repoEntry struct {
		RepoName    string `json:"repo_name" yaml:"repo_name"`
		FullName    string `json:"full_name" yaml:"full_name"`
		Permission  string `json:"permission" yaml:"permission"`
		Privacy     string `json:"privacy" yaml:"privacy"`
		Description string `json:"description" yaml:"description"`
	}

	var entries []repoEntry
	for rows.Next() {
		var repoName, fullName, permission, privacy, description, fallbackRepo sql.NullString
		if err := rows.Scan(&repoName, &fullName, &permission, &privacy, &description, &fallbackRepo); err != nil {
			return fmt.Errorf("failed to scan team repository row: %w", err)
		}
		name := strings.TrimSpace(repoName.String)
		if name == "" {
			name = strings.TrimSpace(fallbackRepo.String)
		}
		entries = append(entries, repoEntry{
			RepoName:    name,
			FullName:    strings.TrimSpace(fullName.String),
			Permission:  strings.TrimSpace(permission.String),
			Privacy:     strings.TrimSpace(privacy.String),
			Description: strings.TrimSpace(description.String),
		})
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate team repository rows: %w", err)
	}

	if len(entries) == 0 {
		if format == FormatTable {
			fmt.Printf("No repository access data found for team %s.\n", cleanSlug)
			fmt.Println("Run 'ghub-desk pull --all-repos-teams' to populate repository-team mappings.")
			return nil
		}
		payload := struct {
			Team         string      `json:"team" yaml:"team"`
			Repositories []repoEntry `json:"repositories" yaml:"repositories"`
		}{
			Team:         cleanSlug,
			Repositories: []repoEntry{},
		}
		return renderByFormat(format, nil, payload)
	}

	tableFn := func() error {
		fmt.Printf("Team: %s\n", cleanSlug)
		printTableHeader("Repo", "Full Name", "Permission", "Privacy", "Description")

		for _, entry := range entries {
			repo := entry.RepoName
			if repo == "" {
				repo = "-"
			}
			fullName := entry.FullName
			if fullName == "" {
				fullName = "-"
			}
			permission := entry.Permission
			if permission == "" {
				permission = "-"
			}
			privacy := entry.Privacy
			if privacy == "" {
				privacy = "-"
			}
			description := entry.Description
			if description == "" {
				description = "-"
			}

			fmt.Printf("%s\t%s\t%s\t%s\t%s\n",
				repo,
				fullName,
				permission,
				privacy,
				description,
			)
		}
		return nil
	}

	payload := struct {
		Team         string      `json:"team" yaml:"team"`
		Repositories []repoEntry `json:"repositories" yaml:"repositories"`
	}{
		Team:         cleanSlug,
		Repositories: entries,
	}

	return renderByFormat(format, tableFn, payload)
}

// ViewAllRepositoriesUsers displays direct collaborators for all repositories in the database.
func ViewAllRepositoriesUsers(db *sql.DB, format OutputFormat) error {
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
		return fmt.Errorf("failed to query repository users: %w", err)
	}
	defer rows.Close()

	type repoUserEntry struct {
		RepoName   string `json:"repo_name" yaml:"repo_name"`
		FullName   string `json:"full_name" yaml:"full_name"`
		Login      string `json:"user_login" yaml:"user_login"`
		UserName   string `json:"user_name" yaml:"user_name"`
		Permission string `json:"permission" yaml:"permission"`
	}

	var entries []repoUserEntry
	for rows.Next() {
		var repoName, fullName, login, userName, permission sql.NullString
		if err := rows.Scan(&repoName, &fullName, &login, &userName, &permission); err != nil {
			return fmt.Errorf("failed to scan repository user row: %w", err)
		}
		entry := repoUserEntry{
			RepoName:   strings.TrimSpace(repoName.String),
			FullName:   strings.TrimSpace(fullName.String),
			Login:      strings.TrimSpace(login.String),
			UserName:   strings.TrimSpace(userName.String),
			Permission: strings.TrimSpace(permission.String),
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate repository user rows: %w", err)
	}

	if len(entries) == 0 {
		if format == FormatTable {
			fmt.Println("No repository user data found in database.")
			fmt.Println("Run 'ghub-desk pull --all-repos-users' or 'ghub-desk pull --repos-users <repo>' first.")
			return nil
		}
		return renderByFormat(format, nil, entries)
	}

	tableFn := func() error {
		printTableHeader("Repo", "Full Name", "User Login", "User Name", "Permission")

		for _, entry := range entries {
			repo := entry.RepoName
			if repo == "" {
				repo = "-"
			}
			fullName := entry.FullName
			if fullName == "" {
				fullName = "-"
			}
			login := entry.Login
			if login == "" {
				login = "-"
			}
			userName := entry.UserName
			if userName == "" {
				userName = "-"
			}
			permission := entry.Permission
			if permission == "" {
				permission = "-"
			}

			fmt.Printf("%s\t%s\t%s\t%s\t%s\n",
				repo,
				fullName,
				login,
				userName,
				permission,
			)
		}
		return nil
	}

	return renderByFormat(format, tableFn, entries)
}

// ViewAllRepositoriesTeams displays all repository team assignments alongside repository metadata.
func ViewAllRepositoriesTeams(db *sql.DB, format OutputFormat) error {
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
		return fmt.Errorf("failed to query repository teams: %w", err)
	}
	defer rows.Close()

	type repoTeamEntry struct {
		RepoName    string `json:"repo_name" yaml:"repo_name"`
		FullName    string `json:"full_name" yaml:"full_name"`
		TeamSlug    string `json:"team_slug" yaml:"team_slug"`
		TeamName    string `json:"team_name" yaml:"team_name"`
		Permission  string `json:"permission" yaml:"permission"`
		Privacy     string `json:"privacy" yaml:"privacy"`
		Description string `json:"description" yaml:"description"`
	}

	var entries []repoTeamEntry
	for rows.Next() {
		var repoName, fullName, teamSlug, teamName, permission, privacy, description sql.NullString
		if err := rows.Scan(&repoName, &fullName, &teamSlug, &teamName, &permission, &privacy, &description); err != nil {
			return fmt.Errorf("failed to scan repository team row: %w", err)
		}
		entry := repoTeamEntry{
			RepoName:    strings.TrimSpace(repoName.String),
			FullName:    strings.TrimSpace(fullName.String),
			TeamSlug:    strings.TrimSpace(teamSlug.String),
			TeamName:    strings.TrimSpace(teamName.String),
			Permission:  strings.TrimSpace(permission.String),
			Privacy:     strings.TrimSpace(privacy.String),
			Description: strings.TrimSpace(description.String),
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate repository team rows: %w", err)
	}

	if len(entries) == 0 {
		if format == FormatTable {
			fmt.Println("No repository team data found in database.")
			fmt.Println("Run 'ghub-desk pull --all-repos-teams' or 'ghub-desk pull --repos-teams <repo>' first.")
			return nil
		}
		return renderByFormat(format, nil, entries)
	}

	tableFn := func() error {
		printTableHeader("Repo", "Full Name", "Team Slug", "Team Name", "Permission", "Privacy", "Description")

		for _, entry := range entries {
			repo := entry.RepoName
			if repo == "" {
				repo = "-"
			}
			fullName := entry.FullName
			if fullName == "" {
				fullName = "-"
			}
			teamSlug := entry.TeamSlug
			if teamSlug == "" {
				teamSlug = "-"
			}
			teamName := entry.TeamName
			if teamName == "" {
				teamName = "-"
			}
			permission := entry.Permission
			if permission == "" {
				permission = "-"
			}
			privacy := entry.Privacy
			if privacy == "" {
				privacy = "-"
			}
			description := entry.Description
			if description == "" {
				description = "-"
			}

			fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				repo,
				fullName,
				teamSlug,
				teamName,
				permission,
				privacy,
				description,
			)
		}
		return nil
	}

	return renderByFormat(format, tableFn, entries)
}

// ViewAllTeamsUsers displays all team membership entries from the database.
func ViewAllTeamsUsers(db *sql.DB, format OutputFormat) error {
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
		return fmt.Errorf("failed to query team users: %w", err)
	}
	defer rows.Close()

	type teamUserEntry struct {
		TeamSlug string `json:"team_slug" yaml:"team_slug"`
		TeamName string `json:"team_name" yaml:"team_name"`
		Login    string `json:"user_login" yaml:"user_login"`
		UserName string `json:"user_name" yaml:"user_name"`
		Role     string `json:"role" yaml:"role"`
	}

	var entries []teamUserEntry
	for rows.Next() {
		var teamSlug, teamName, login, userName, role sql.NullString
		if err := rows.Scan(&teamSlug, &teamName, &login, &userName, &role); err != nil {
			return fmt.Errorf("failed to scan team user row: %w", err)
		}
		entry := teamUserEntry{
			TeamSlug: strings.TrimSpace(teamSlug.String),
			TeamName: strings.TrimSpace(teamName.String),
			Login:    strings.TrimSpace(login.String),
			UserName: strings.TrimSpace(userName.String),
			Role:     strings.TrimSpace(role.String),
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate team user rows: %w", err)
	}

	if len(entries) == 0 {
		if format == FormatTable {
			fmt.Println("No team membership data found in database.")
			fmt.Println("Run 'ghub-desk pull --all-teams-users' or 'ghub-desk pull --team-user <team-slug>' first.")
			return nil
		}
		return renderByFormat(format, nil, entries)
	}

	tableFn := func() error {
		printTableHeader("Team Slug", "Team Name", "User Login", "User Name", "Role")

		for _, entry := range entries {
			slug := entry.TeamSlug
			if slug == "" {
				slug = "-"
			}
			name := entry.TeamName
			if name == "" {
				name = "-"
			}
			login := entry.Login
			if login == "" {
				login = "-"
			}
			fullName := entry.UserName
			if fullName == "" {
				fullName = "-"
			}
			role := entry.Role
			if role == "" {
				role = "-"
			}

			fmt.Printf("%s\t%s\t%s\t%s\t%s\n", slug, name, login, fullName, role)
		}
		return nil
	}

	return renderByFormat(format, tableFn, entries)
}

// ViewUserTeams displays teams a user belongs to.
func ViewUserTeams(db *sql.DB, userLogin string, format OutputFormat) error {
	if db == nil {
		return fmt.Errorf("database connection is required to view user teams")
	}
	cleanLogin := strings.TrimSpace(userLogin)
	if cleanLogin == "" {
		return fmt.Errorf("user login is required to view user teams")
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
		return fmt.Errorf("failed to query teams for user %s: %w", cleanLogin, err)
	}
	defer rows.Close()

	type teamEntry struct {
		TeamSlug string `json:"team_slug" yaml:"team_slug"`
		TeamName string `json:"team_name" yaml:"team_name"`
		Role     string `json:"role" yaml:"role"`
	}

	var entries []teamEntry
	for rows.Next() {
		var teamSlug, teamName, role sql.NullString
		if err := rows.Scan(&teamSlug, &teamName, &role); err != nil {
			return fmt.Errorf("failed to scan user team row: %w", err)
		}
		entries = append(entries, teamEntry{
			TeamSlug: strings.TrimSpace(teamSlug.String),
			TeamName: strings.TrimSpace(teamName.String),
			Role:     strings.TrimSpace(role.String),
		})
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate user team rows: %w", err)
	}

	if len(entries) == 0 {
		if format == FormatTable {
			fmt.Printf("No team membership data found for user %s.\n", cleanLogin)
			fmt.Println("Run 'ghub-desk pull --all-teams-users' or 'ghub-desk pull --team-user <team-slug>' first.")
			return nil
		}
		payload := struct {
			User  string      `json:"user" yaml:"user"`
			Teams []teamEntry `json:"teams" yaml:"teams"`
		}{
			User:  cleanLogin,
			Teams: []teamEntry{},
		}
		return renderByFormat(format, nil, payload)
	}

	tableFn := func() error {
		fmt.Printf("User: %s\n", cleanLogin)
		printTableHeader("Team Slug", "Team Name", "Role")

		for _, entry := range entries {
			slug := entry.TeamSlug
			if slug == "" {
				slug = "-"
			}
			name := entry.TeamName
			if name == "" {
				name = "-"
			}
			role := entry.Role
			if role == "" {
				role = "-"
			}

			fmt.Printf("%s\t%s\t%s\n", slug, name, role)
		}
		return nil
	}

	payload := struct {
		User  string      `json:"user" yaml:"user"`
		Teams []teamEntry `json:"teams" yaml:"teams"`
	}{
		User:  cleanLogin,
		Teams: entries,
	}

	return renderByFormat(format, tableFn, payload)
}

// ViewUserRepositories displays repositories a user can access along with access path and permission.
func ViewUserRepositories(db *sql.DB, userLogin string, format OutputFormat) error {
	if db == nil {
		return fmt.Errorf("database connection is required to view user repositories")
	}
	cleanLogin := strings.TrimSpace(userLogin)
	if cleanLogin == "" {
		return fmt.Errorf("user login is required to view repositories")
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
		return fmt.Errorf("failed to query direct repository access: %w", err)
	}
	defer directRows.Close()

	for directRows.Next() {
		var repoName, permission, fallback sql.NullString
		if err := directRows.Scan(&repoName, &permission, &fallback); err != nil {
			return fmt.Errorf("failed to scan direct access row: %w", err)
		}
		name := repoName.String
		if strings.TrimSpace(name) == "" {
			name = fallback.String
		}
		mergeRepoAccess(name, "Direct", permission.String)
	}
	if err := directRows.Err(); err != nil {
		return fmt.Errorf("failed to iterate direct access rows: %w", err)
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
		return fmt.Errorf("failed to query team-derived repository access: %w", err)
	}
	defer teamRows.Close()

	for teamRows.Next() {
		var repoName, teamSlug, teamName, permission, fallback sql.NullString
		if err := teamRows.Scan(&repoName, &teamSlug, &teamName, &permission, &fallback); err != nil {
			return fmt.Errorf("failed to scan team access row: %w", err)
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
		return fmt.Errorf("failed to iterate team access rows: %w", err)
	}

	if len(accessByRepoName) == 0 {
		if format == FormatTable {
			fmt.Printf("No repository access data found for user %s.\n", cleanLogin)
			fmt.Println("Run 'ghub-desk pull --all-repos-users' (or 'ghub-desk pull --repos-users <repo>'), 'ghub-desk pull --repos-teams', and 'ghub-desk pull --team-users <team-slug>' to populate the database.")
			return nil
		}
		payload := struct {
			User         string        `json:"user" yaml:"user"`
			Repositories []interface{} `json:"repositories" yaml:"repositories"`
		}{
			User:         cleanLogin,
			Repositories: []interface{}{},
		}
		return renderByFormat(format, nil, payload)
	}

	entries := make([]*repoAccessEntry, 0, len(accessByRepoName))
	for _, entry := range accessByRepoName {
		// Ensure stable output with direct access first, followed by alphabetical labels.
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

	type userRepoRecord struct {
		Repository string   `json:"repository" yaml:"repository"`
		AccessFrom []string `json:"access_from" yaml:"access_from"`
		Permission string   `json:"permission" yaml:"permission"`
	}

	records := make([]userRepoRecord, 0, len(entries))
	for _, entry := range entries {
		perm := entry.highest
		if perm == "" {
			perm = "-"
		}
		records = append(records, userRepoRecord{
			Repository: entry.repoName,
			AccessFrom: append([]string(nil), entry.sources...),
			Permission: perm,
		})
	}

	tableFn := func() error {
		fmt.Printf("User: %s\n", cleanLogin)
		printTableHeader("Repository", "Access From", "Permission")

		for _, record := range records {
			joined := strings.Join(record.AccessFrom, ", ")
			fmt.Printf("%s\t%s\t%s\n", record.Repository, joined, record.Permission)
		}
		return nil
	}

	payload := struct {
		User         string           `json:"user" yaml:"user"`
		Repositories []userRepoRecord `json:"repositories" yaml:"repositories"`
	}{
		User:         cleanLogin,
		Repositories: records,
	}

	return renderByFormat(format, tableFn, payload)
}

// ViewTeamUsers displays team members from the database
func ViewTeamUsers(db *sql.DB, teamSlug string, format OutputFormat) error {
	query := `
		SELECT ghub_user_id, user_login, role 
		FROM ghub_team_users 
		WHERE team_slug = ? 
		ORDER BY user_login`
	session.Debugf("SQL: %s, ARGS: [%s]", query, teamSlug)
	rows, err := db.Query(query, teamSlug)
	if err != nil {
		return fmt.Errorf("failed to query team users: %w", err)
	}
	defer rows.Close()

	type teamUserRecord struct {
		UserID int64  `json:"user_id" yaml:"user_id"`
		Login  string `json:"login" yaml:"login"`
		Role   string `json:"role" yaml:"role"`
	}

	var records []teamUserRecord
	for rows.Next() {
		var userID int64
		var login, role sql.NullString
		err := rows.Scan(&userID, &login, &role)
		if err != nil {
			return fmt.Errorf("failed to scan team user row: %w", err)
		}

		record := teamUserRecord{
			UserID: userID,
			Login:  login.String,
			Role:   role.String,
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate team user rows: %w", err)
	}

	tableFn := func() error {
		fmt.Printf("Team: %s\n", teamSlug)
		printTableHeader("User ID", "Login", "Role")

		for _, record := range records {
			fmt.Printf("%d\t%s\t%s\n", record.UserID, record.Login, record.Role)
		}
		return nil
	}

	payload := struct {
		TeamSlug string           `json:"team_slug" yaml:"team_slug"`
		Users    []teamUserRecord `json:"users" yaml:"users"`
	}{
		TeamSlug: teamSlug,
		Users:    records,
	}

	return renderByFormat(format, tableFn, payload)
}

// ViewTokenPermission displays token permissions from the database
func ViewTokenPermission(db *sql.DB, format OutputFormat) error {
	query := `
		SELECT scopes, x_oauth_scopes, x_accepted_oauth_scopes, x_accepted_github_permissions, x_github_media_type,
		       x_ratelimit_limit, x_ratelimit_remaining, x_ratelimit_reset,
		       created_at, updated_at
		FROM ghub_token_permissions 
		ORDER BY created_at DESC 
		LIMIT 1`
	session.Debugf("SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query token permissions: %w", err)
	}
	defer rows.Close()

	type tokenPermissionRecord struct {
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

	if !rows.Next() {
		if format == FormatTable {
			fmt.Println("No token permission data found in database.")
			fmt.Println("Run 'ghub-desk pull --token-permission' first.")
			return nil
		}
		return renderByFormat(format, nil, nil)
	}
	var scopes, oauthScopes, acceptedScopes, acceptedGitHubPermissions, mediaType, createdAt, updatedAt sql.NullString
	var rateLimit, rateRemaining, rateReset int

	err = rows.Scan(&scopes, &oauthScopes, &acceptedScopes, &acceptedGitHubPermissions, &mediaType,
		&rateLimit, &rateRemaining, &rateReset,
		&createdAt, &updatedAt)
	if err != nil {
		return fmt.Errorf("failed to scan token permission row: %w", err)
	}

	record := tokenPermissionRecord{
		Scopes:                    scopes.String,
		OAuthScopes:               oauthScopes.String,
		AcceptedOAuthScopes:       acceptedScopes.String,
		AcceptedGitHubPermissions: acceptedGitHubPermissions.String,
		GitHubMediaType:           mediaType.String,
		RateLimit:                 rateLimit,
		RateRemaining:             rateRemaining,
		RateReset:                 rateReset,
		CreatedAt:                 createdAt.String,
		UpdatedAt:                 updatedAt.String,
	}

	tableFn := func() error {
		fmt.Println("Token Permissions (from database):")
		fmt.Println("===================================")
		fmt.Printf("OAuth Scopes: %s\n", record.OAuthScopes)
		fmt.Printf("Accepted OAuth Scopes: %s\n", record.AcceptedOAuthScopes)
		fmt.Printf("Accepted GitHub Permissions: %s\n", record.AcceptedGitHubPermissions)
		fmt.Printf("GitHub Media Type: %s\n", record.GitHubMediaType)
		fmt.Printf("Rate Limit: %d\n", record.RateLimit)
		fmt.Printf("Rate Remaining: %d\n", record.RateRemaining)
		fmt.Printf("Rate Reset: %d\n", record.RateReset)
		fmt.Printf("Created At: %s\n", record.CreatedAt)
		fmt.Printf("Updated At: %s\n", record.UpdatedAt)
		return nil
	}

	return renderByFormat(format, tableFn, record)
}

// ViewOutsideUsers displays outside users from the database
func ViewOutsideUsers(db *sql.DB, format OutputFormat) error {
	query := `SELECT id, login, name, email, company, location FROM ghub_outside_users ORDER BY login`
	session.Debugf("SQL: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query outside users: %w", err)
	}
	defer rows.Close()

	type outsideUserRecord struct {
		ID       int64  `json:"id" yaml:"id"`
		Login    string `json:"login" yaml:"login"`
		Name     string `json:"name" yaml:"name"`
		Email    string `json:"email" yaml:"email"`
		Company  string `json:"company" yaml:"company"`
		Location string `json:"location" yaml:"location"`
	}

	var records []outsideUserRecord
	for rows.Next() {
		var id int64
		var login, name, email, company, location sql.NullString
		err := rows.Scan(&id, &login, &name, &email, &company, &location)
		if err != nil {
			return fmt.Errorf("failed to scan outside user row: %w", err)
		}

		record := outsideUserRecord{
			ID:       id,
			Login:    login.String,
			Name:     name.String,
			Email:    email.String,
			Company:  company.String,
			Location: location.String,
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate outside user rows: %w", err)
	}

	tableFn := func() error {
		fmt.Println("Outside Collaborators:")
		printTableHeader("ID", "Login", "Name", "Email", "Company", "Location")

		for _, record := range records {
			fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\n",
				record.ID,
				record.Login,
				record.Name,
				record.Email,
				record.Company,
				record.Location,
			)
		}
		return nil
	}

	return renderByFormat(format, tableFn, records)
}
