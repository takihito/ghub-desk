package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"ghub-desk/validate"
)

// TargetRequest represents the requested view target including optional metadata.
type TargetRequest struct {
	Kind      string
	TeamSlug  string
	RepoName  string
	UserLogin string
}

// HandleViewTarget processes different types of view targets
func HandleViewTarget(db *sql.DB, req TargetRequest) error {
	switch req.Kind {
	case "users", "detail-users":
		return ViewUsers(db)
	case "teams":
		return ViewTeams(db)
	case "repos", "repositories":
		return ViewRepositories(db)
	case "token-permission":
		return ViewTokenPermission(db)
	case "outside-users":
		return ViewOutsideUsers(db)
	case "repos-users":
		if req.RepoName == "" {
			return fmt.Errorf("repository name must be specified when using repos-users target")
		}
		if err := validate.ValidateRepoName(req.RepoName); err != nil {
			return fmt.Errorf("invalid repository name: %w", err)
		}
		return ViewRepoUsers(db, req.RepoName)
	case "repos-teams":
		if req.RepoName == "" {
			return fmt.Errorf("repository name must be specified when using repos-teams target")
		}
		if err := validate.ValidateRepoName(req.RepoName); err != nil {
			return fmt.Errorf("invalid repository name: %w", err)
		}
		return ViewRepoTeams(db, req.RepoName)
	case "all-repos-teams":
		return ViewAllRepositoriesTeams(db)
	case "all-teams-users":
		return ViewAllTeamsUsers(db)
	case "user-repos":
		if req.UserLogin == "" {
			return fmt.Errorf("user login must be specified when using user-repos target")
		}
		if err := validate.ValidateUserName(req.UserLogin); err != nil {
			return fmt.Errorf("invalid user login: %w", err)
		}
		return ViewUserRepositories(db, req.UserLogin)
	case "team-user":
		if req.TeamSlug == "" {
			return fmt.Errorf("team slug must be specified when using team-user target")
		}
		if err := validate.ValidateTeamSlug(req.TeamSlug); err != nil {
			return fmt.Errorf("invalid team slug: %w", err)
		}
		return ViewTeamUsers(db, req.TeamSlug)
	default:
		return fmt.Errorf("unknown target: %s", req.Kind)
	}
}

// ViewUsers displays users from the database
func ViewUsers(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, login, name, email, company, location FROM ghub_users ORDER BY login`)
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

// ViewTeams displays teams from the database
func ViewTeams(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, slug, name, description, privacy FROM ghub_teams ORDER BY slug`)
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

// ViewRepositories displays repositories from the database
func ViewRepositories(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT id, name, full_name, description, private, language, stargazers_count 
		FROM ghub_repositories ORDER BY name`)
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

// ViewRepoUsers displays direct repository collaborators from the database
func ViewRepoUsers(db *sql.DB, repoName string) error {
	rows, err := db.Query(`
		SELECT user_id, user_login
		FROM repo_users
		WHERE repo_name = ?
		ORDER BY user_login`, repoName)
	if err != nil {
		return fmt.Errorf("failed to query repository users: %w", err)
	}
	defer rows.Close()

	fmt.Printf("Repository: %s\n", repoName)
	fmt.Println("User ID\tLogin")
	fmt.Println("-------\t-----")

	for rows.Next() {
		var userID sql.NullInt64
		var login sql.NullString
		if err := rows.Scan(&userID, &login); err != nil {
			return fmt.Errorf("failed to scan repository user row: %w", err)
		}
		fmt.Printf("%d\t%s\n", userID.Int64, login.String)
	}
	return nil
}

// ViewRepoTeams displays repository teams from the database
func ViewRepoTeams(db *sql.DB, repoName string) error {
	rows, err := db.Query(`
		SELECT id, team_name, team_slug, description, privacy, permission
		FROM repo_teams
		WHERE repo_name = ?
		ORDER BY team_slug`, repoName)
	if err != nil {
		return fmt.Errorf("failed to query repository teams: %w", err)
	}
	defer rows.Close()

	fmt.Printf("Repository: %s\n", repoName)
	fmt.Println("Team ID\tSlug\tName\tPermission\tPrivacy\tDescription")
	fmt.Println("-------\t----\t----\t----------\t-------\t-----------")

	for rows.Next() {
		var id sql.NullInt64
		var name, slug, description, privacy, permission sql.NullString
		if err := rows.Scan(&id, &name, &slug, &description, &privacy, &permission); err != nil {
			return fmt.Errorf("failed to scan repository team row: %w", err)
		}

		fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\n",
			id.Int64,
			slug.String,
			name.String,
			permission.String,
			privacy.String,
			description.String,
		)
	}
	return nil
}

// ViewAllRepositoriesTeams displays all repository team assignments alongside repository metadata.
func ViewAllRepositoriesTeams(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT 
			COALESCE(r.name, rt.repo_name) AS repo_name,
			COALESCE(r.full_name, '') AS repo_full_name,
			rt.team_slug,
			COALESCE(rt.team_name, '') AS team_name,
			COALESCE(rt.permission, '') AS permission,
			COALESCE(rt.privacy, '') AS privacy,
			COALESCE(rt.description, '') AS description
		FROM repo_teams rt
		LEFT JOIN ghub_repositories r ON r.name = rt.repo_name
		ORDER BY LOWER(repo_name), LOWER(rt.team_slug)
	`)
	if err != nil {
		return fmt.Errorf("failed to query repository teams: %w", err)
	}
	defer rows.Close()

	type repoTeamEntry struct {
		repoName    string
		fullName    string
		teamSlug    string
		teamName    string
		permission  string
		privacy     string
		description string
	}

	var entries []repoTeamEntry
	for rows.Next() {
		var repoName, fullName, teamSlug, teamName, permission, privacy, description sql.NullString
		if err := rows.Scan(&repoName, &fullName, &teamSlug, &teamName, &permission, &privacy, &description); err != nil {
			return fmt.Errorf("failed to scan repository team row: %w", err)
		}
		entry := repoTeamEntry{
			repoName:    strings.TrimSpace(repoName.String),
			fullName:    strings.TrimSpace(fullName.String),
			teamSlug:    strings.TrimSpace(teamSlug.String),
			teamName:    strings.TrimSpace(teamName.String),
			permission:  strings.TrimSpace(permission.String),
			privacy:     strings.TrimSpace(privacy.String),
			description: strings.TrimSpace(description.String),
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate repository team rows: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No repository team data found in database.")
		fmt.Println("Run 'ghub-desk pull --all-repos-teams' or 'ghub-desk pull --repos-teams <repo>' first.")
		return nil
	}

	fmt.Println("Repo\tFull Name\tTeam Slug\tTeam Name\tPermission\tPrivacy\tDescription")
	fmt.Println("----\t---------\t---------\t---------\t----------\t-------\t-----------")

	for _, entry := range entries {
		repo := entry.repoName
		if repo == "" {
			repo = "-"
		}
		fullName := entry.fullName
		if fullName == "" {
			fullName = "-"
		}
		teamSlug := entry.teamSlug
		if teamSlug == "" {
			teamSlug = "-"
		}
		teamName := entry.teamName
		if teamName == "" {
			teamName = "-"
		}
		permission := entry.permission
		if permission == "" {
			permission = "-"
		}
		privacy := entry.privacy
		if privacy == "" {
			privacy = "-"
		}
		description := entry.description
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

// ViewAllTeamsUsers displays all team membership entries from the database.
func ViewAllTeamsUsers(db *sql.DB) error {
	rows, err := db.Query(`
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
	`)
	if err != nil {
		return fmt.Errorf("failed to query team users: %w", err)
	}
	defer rows.Close()

	type teamUserEntry struct {
		teamSlug string
		teamName string
		login    string
		userName string
		role     string
	}

	var entries []teamUserEntry
	for rows.Next() {
		var teamSlug, teamName, login, userName, role sql.NullString
		if err := rows.Scan(&teamSlug, &teamName, &login, &userName, &role); err != nil {
			return fmt.Errorf("failed to scan team user row: %w", err)
		}
		entry := teamUserEntry{
			teamSlug: strings.TrimSpace(teamSlug.String),
			teamName: strings.TrimSpace(teamName.String),
			login:    strings.TrimSpace(login.String),
			userName: strings.TrimSpace(userName.String),
			role:     strings.TrimSpace(role.String),
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate team user rows: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No team membership data found in database.")
		fmt.Println("Run 'ghub-desk pull --all-teams-users' or 'ghub-desk pull --team-user <team-slug>' first.")
		return nil
	}

	fmt.Println("Team Slug\tTeam Name\tUser Login\tUser Name\tRole")
	fmt.Println("---------\t---------\t----------\t---------\t----")

	for _, entry := range entries {
		slug := entry.teamSlug
		if slug == "" {
			slug = "-"
		}
		name := entry.teamName
		if name == "" {
			name = "-"
		}
		login := entry.login
		if login == "" {
			login = "-"
		}
		fullName := entry.userName
		if fullName == "" {
			fullName = "-"
		}
		role := entry.role
		if role == "" {
			role = "-"
		}

		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", slug, name, login, fullName, role)
	}

	return nil
}

// ViewUserRepositories displays repositories a user can access along with access path and permission.
func ViewUserRepositories(db *sql.DB, userLogin string) error {
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

	directRows, err := db.Query(`
		SELECT COALESCE(r.name, ru.repo_name) AS repo_name,
		       COALESCE(ru.permission, ''),
		       ru.repo_name
		FROM repo_users ru
		LEFT JOIN ghub_repositories r ON r.name = ru.repo_name
		WHERE ru.user_login = ?
	`, cleanLogin)
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

	teamRows, err := db.Query(`
		SELECT COALESCE(r.name, rt.repo_name) AS repo_name,
		       rt.team_slug,
		       COALESCE(rt.team_name, ''),
		       COALESCE(rt.permission, ''),
		       rt.repo_name
		FROM ghub_team_users tu
		JOIN repo_teams rt ON rt.team_slug = tu.team_slug
		LEFT JOIN ghub_repositories r ON r.name = rt.repo_name
		WHERE tu.user_login = ?
	`, cleanLogin)
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
		fmt.Printf("No repository access data found for user %s.\n", cleanLogin)
		fmt.Println("Run 'ghub-desk pull --repos-users', 'ghub-desk pull --repos-teams', and 'ghub-desk pull --team-users <team-slug>' to populate the database.")
		return nil
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

	fmt.Printf("User: %s\n", cleanLogin)
	fmt.Println("Repository\tAccess From\tPermission")
	fmt.Println("----------\t-----------\t----------")

	for _, entry := range entries {
		perm := entry.highest
		if perm == "" {
			perm = "-"
		}
		fmt.Printf("%s\t%s\t%s\n", entry.repoName, strings.Join(entry.sources, ", "), perm)
	}

	return nil
}

// ViewTeamUsers displays team members from the database
func ViewTeamUsers(db *sql.DB, teamSlug string) error {
	rows, err := db.Query(`
		SELECT user_id, user_login, role 
		FROM ghub_team_users 
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

// ViewTokenPermission displays token permissions from the database
func ViewTokenPermission(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT scopes, x_oauth_scopes, x_accepted_oauth_scopes, x_accepted_github_permissions, x_github_media_type,
		       x_ratelimit_limit, x_ratelimit_remaining, x_ratelimit_reset,
		       created_at, updated_at
		FROM ghub_token_permissions 
		ORDER BY created_at DESC 
		LIMIT 1`)
	if err != nil {
		return fmt.Errorf("failed to query token permissions: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		fmt.Println("No token permission data found in database.")
		fmt.Println("Run 'ghub-desk pull --token-permission' first.")
		return nil
	}

	var scopes, oauthScopes, acceptedScopes, acceptedGitHubPermissions, mediaType, createdAt, updatedAt sql.NullString
	var rateLimit, rateRemaining, rateReset int

	err = rows.Scan(&scopes, &oauthScopes, &acceptedScopes, &acceptedGitHubPermissions, &mediaType,
		&rateLimit, &rateRemaining, &rateReset,
		&createdAt, &updatedAt)
	if err != nil {
		return fmt.Errorf("failed to scan token permission row: %w", err)
	}

	fmt.Println("Token Permissions (from database):")
	fmt.Println("===================================")
	fmt.Printf("OAuth Scopes: %s\n", oauthScopes.String)
	fmt.Printf("Accepted OAuth Scopes: %s\n", acceptedScopes.String)
	fmt.Printf("Accepted GitHub Permissions: %s\n", acceptedGitHubPermissions.String)
	fmt.Printf("GitHub Media Type: %s\n", mediaType.String)
	fmt.Printf("Rate Limit: %d\n", rateLimit)
	fmt.Printf("Rate Remaining: %d\n", rateRemaining)
	fmt.Printf("Rate Reset: %d\n", rateReset)
	fmt.Printf("Created At: %s\n", createdAt.String)
	fmt.Printf("Updated At: %s\n", updatedAt.String)

	return nil
}

// ViewOutsideUsers displays outside users from the database
func ViewOutsideUsers(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, login, name, email, company, location FROM ghub_outside_users ORDER BY login`)
	if err != nil {
		return fmt.Errorf("failed to query outside users: %w", err)
	}
	defer rows.Close()

	fmt.Println("Outside Collaborators:")
	fmt.Println("ID\tLogin\tName\tEmail\tCompany\tLocation")
	fmt.Println("--\t-----\t----\t-----\t-------\t--------")

	for rows.Next() {
		var id int64
		var login, name, email, company, location sql.NullString
		err := rows.Scan(&id, &login, &name, &email, &company, &location)
		if err != nil {
			return fmt.Errorf("failed to scan outside user row: %w", err)
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
