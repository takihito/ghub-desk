package store

import (
	"database/sql"
	"fmt"
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
	records, err := FetchUsers(db)
	if err != nil {
		return err
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
	record, found, err := FetchUserProfile(db, userLogin)
	if err != nil {
		return err
	}
	cleanLogin := strings.TrimSpace(userLogin)
	if !found {
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
	records, err := FetchTeams(db)
	if err != nil {
		return err
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
	records, err := FetchRepositories(db)
	if err != nil {
		return err
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
	repoDisplay, _, records, err := FetchRepoUsers(db, repoName)
	if err != nil {
		return err
	}

	type repoUserRecord struct {
		UserID int64  `json:"user_id" yaml:"user_id"`
		Login  string `json:"login" yaml:"login"`
	}

	viewRecords := make([]repoUserRecord, 0, len(records))
	for _, record := range records {
		viewRecords = append(viewRecords, repoUserRecord{
			UserID: record.UserID,
			Login:  record.Login,
		})
	}

	tableFn := func() error {
		fmt.Printf("Repository: %s\n", repoDisplay)
		printTableHeader("User ID", "Login")

		for _, record := range viewRecords {
			fmt.Printf("%d\t%s\n", record.UserID, record.Login)
		}
		return nil
	}

	payload := struct {
		Repository string           `json:"repository" yaml:"repository"`
		Users      []repoUserRecord `json:"users" yaml:"users"`
	}{
		Repository: repoDisplay,
		Users:      viewRecords,
	}

	return renderByFormat(format, tableFn, payload)
}

// ViewRepoTeams displays repository teams from the database
func ViewRepoTeams(db *sql.DB, repoName string, format OutputFormat) error {
	repoDisplay, _, records, err := FetchRepoTeams(db, repoName)
	if err != nil {
		return err
	}

	tableFn := func() error {
		fmt.Printf("Repository: %s\n", repoDisplay)
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
		Repository string          `json:"repository" yaml:"repository"`
		Teams      []RepoTeamEntry `json:"teams" yaml:"teams"`
	}{
		Repository: repoDisplay,
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
	entries, err := FetchTeamRepositories(db, teamSlug)
	if err != nil {
		return err
	}
	cleanSlug := strings.TrimSpace(teamSlug)

	if len(entries) == 0 {
		if format == FormatTable {
			fmt.Printf("No repository access data found for team %s.\n", cleanSlug)
			fmt.Println("Run 'ghub-desk pull --all-repos-teams' to populate repository-team mappings.")
			return nil
		}
		payload := struct {
			Team         string                `json:"team" yaml:"team"`
			Repositories []TeamRepositoryEntry `json:"repositories" yaml:"repositories"`
		}{
			Team:         cleanSlug,
			Repositories: []TeamRepositoryEntry{},
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
		Team         string                `json:"team" yaml:"team"`
		Repositories []TeamRepositoryEntry `json:"repositories" yaml:"repositories"`
	}{
		Team:         cleanSlug,
		Repositories: entries,
	}

	return renderByFormat(format, tableFn, payload)
}

// ViewAllRepositoriesUsers displays direct collaborators for all repositories in the database.
func ViewAllRepositoriesUsers(db *sql.DB, format OutputFormat) error {
	entries, err := FetchAllRepositoriesUsers(db)
	if err != nil {
		return err
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
			login := entry.UserLogin
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
	entries, err := FetchAllRepositoriesTeams(db)
	if err != nil {
		return err
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
	entries, err := FetchAllTeamsUsers(db)
	if err != nil {
		return err
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
			login := entry.UserLogin
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
	entries, err := FetchUserTeams(db, userLogin)
	if err != nil {
		return err
	}
	cleanLogin := strings.TrimSpace(userLogin)

	if len(entries) == 0 {
		if format == FormatTable {
			fmt.Printf("No team membership data found for user %s.\n", cleanLogin)
			fmt.Println("Run 'ghub-desk pull --all-teams-users' or 'ghub-desk pull --team-user <team-slug>' first.")
			return nil
		}
		payload := struct {
			User  string          `json:"user" yaml:"user"`
			Teams []UserTeamEntry `json:"teams" yaml:"teams"`
		}{
			User:  cleanLogin,
			Teams: []UserTeamEntry{},
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
		User  string          `json:"user" yaml:"user"`
		Teams []UserTeamEntry `json:"teams" yaml:"teams"`
	}{
		User:  cleanLogin,
		Teams: entries,
	}

	return renderByFormat(format, tableFn, payload)
}

// ViewUserRepositories displays repositories a user can access along with access path and permission.
func ViewUserRepositories(db *sql.DB, userLogin string, format OutputFormat) error {
	entries, err := FetchUserRepositories(db, userLogin)
	if err != nil {
		return err
	}
	cleanLogin := strings.TrimSpace(userLogin)

	if len(entries) == 0 {
		if format == FormatTable {
			fmt.Printf("No repository access data found for user %s.\n", cleanLogin)
			fmt.Println("Run 'ghub-desk pull --all-repos-users' (or 'ghub-desk pull --repos-users <repo>'), 'ghub-desk pull --repos-teams', and 'ghub-desk pull --team-users <team-slug>' to populate the database.")
			return nil
		}
		payload := struct {
			User         string                `json:"user" yaml:"user"`
			Repositories []UserRepoAccessEntry `json:"repositories" yaml:"repositories"`
		}{
			User:         cleanLogin,
			Repositories: []UserRepoAccessEntry{},
		}
		return renderByFormat(format, nil, payload)
	}

	tableFn := func() error {
		fmt.Printf("User: %s\n", cleanLogin)
		printTableHeader("Repository", "Access From", "Permission")

		for _, record := range entries {
			joined := strings.Join(record.AccessFrom, ", ")
			fmt.Printf("%s\t%s\t%s\n", record.Repository, joined, record.Permission)
		}
		return nil
	}

	payload := struct {
		User         string                `json:"user" yaml:"user"`
		Repositories []UserRepoAccessEntry `json:"repositories" yaml:"repositories"`
	}{
		User:         cleanLogin,
		Repositories: entries,
	}

	return renderByFormat(format, tableFn, payload)
}

// ViewTeamUsers displays team members from the database
func ViewTeamUsers(db *sql.DB, teamSlug string, format OutputFormat) error {
	records, err := FetchTeamUsers(db, teamSlug)
	if err != nil {
		return err
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
		TeamSlug string          `json:"team_slug" yaml:"team_slug"`
		Users    []TeamUserEntry `json:"users" yaml:"users"`
	}{
		TeamSlug: teamSlug,
		Users:    records,
	}

	return renderByFormat(format, tableFn, payload)
}

// ViewTokenPermission displays token permissions from the database
func ViewTokenPermission(db *sql.DB, format OutputFormat) error {
	record, found, err := FetchTokenPermission(db)
	if err != nil {
		return err
	}
	if !found {
		if format == FormatTable {
			fmt.Println("No token permission data found in database.")
			fmt.Println("Run 'ghub-desk pull --token-permission' first.")
			return nil
		}
		return renderByFormat(format, nil, nil)
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
	records, err := FetchOutsideUsers(db)
	if err != nil {
		return err
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
