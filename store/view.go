package store

import (
	"database/sql"
	"fmt"

	"ghub-desk/validate"
)

// TargetRequest represents the requested view target including optional metadata.
type TargetRequest struct {
	Kind     string
	TeamSlug string
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
		fmt.Println("Run 'ghub-desk pull --token-permission --store' first.")
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
