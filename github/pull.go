package github

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"ghub-desk/store"

	"github.com/google/go-github/v55/github"
)

const (
	// API pagination settings
	DefaultPerPage = 100
	DefaultSleep   = 1 * time.Second
)

// HandlePullTarget processes different types of pull targets (users, teams, repos, team_users)
func HandlePullTarget(ctx context.Context, client *github.Client, db *sql.DB, org, target, token string, storeData bool) error {
	switch {
	case target == "users":
		return PullUsers(ctx, client, db, org, storeData)
	case target == "detail-users":
		return PullDetailUsers(ctx, client, db, org, token, storeData)
	case target == "teams":
		return PullTeams(ctx, client, db, org, storeData)
	case target == "repos":
		return PullRepositories(ctx, client, db, org, storeData)
	case target == "all-teams-users":
		return PullAllTeamsUsers(ctx, client, db, org, storeData)
	case target == "token-permission":
		return PullTokenPermission(ctx, client, db, storeData)
	case strings.HasSuffix(target, "/users"):
		teamSlug := strings.TrimSuffix(target, "/users")
		return PullTeamUsers(ctx, client, db, org, teamSlug, storeData)
	default:
		return fmt.Errorf("unknown target: %s", target)
	}
}

// PullUsers fetches organization members and optionally stores them in database
func PullUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, storeData bool) error {
	if storeData {
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
			if !storeData || db == nil {
				return nil
			}
			return store.StoreUsers(db, users)
		},
		db, org,
	)
}

// PullDetailUsers fetches organization members with detailed information and optionally stores them in database
func PullDetailUsers(ctx context.Context, client *github.Client, db *sql.DB, org, token string, storeData bool) error {
	if storeData {
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
			if !storeData || db == nil {
				return nil
			}
			// Fetch detailed user information for each user
			return store.StoreUsersWithDetails(ctx, client, db, users, token, org)
		},
		db, org,
	)
}

// PullTeams fetches organization teams and optionally stores them in database
func PullTeams(ctx context.Context, client *github.Client, db *sql.DB, org string, storeData bool) error {
	if storeData {
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
			if !storeData || db == nil {
				return nil
			}
			return store.StoreTeams(db, teams)
		},
		db, org,
	)
}

// PullRepositories fetches organization repositories and optionally stores them in database
func PullRepositories(ctx context.Context, client *github.Client, db *sql.DB, org string, storeData bool) error {
	if storeData {
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
			if !storeData || db == nil {
				return nil
			}
			return store.StoreRepositories(db, repos)
		},
		db, org,
	)
}

// PullTeamUsers fetches team members and optionally stores them in database
func PullTeamUsers(ctx context.Context, client *github.Client, db *sql.DB, org, teamSlug string, storeData bool) error {
	if storeData {
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
			if !storeData || db == nil {
				return nil
			}
			return store.StoreTeamUsers(db, users, teamSlug)
		},
		db, org,
	)
}

// PullAllTeamsUsers fetches users for all stored teams
func PullAllTeamsUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, storeData bool) error {
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
		if err := PullTeamUsers(ctx, client, db, org, teamSlug, storeData); err != nil {
			fmt.Printf("Warning: failed to fetch users for team %s: %v\n", teamSlug, err)
			continue
		}
	}

	fmt.Printf("Completed fetching users for all teams.\n")
	return nil
}

// PullTokenPermission fetches GitHub token permissions and optionally stores them in database
func PullTokenPermission(ctx context.Context, client *github.Client, db *sql.DB, storeData bool) error {
	// Make a simple API call to get token information from response headers
	_, resp, err := client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to get token information: %w", err)
	}

	// Extract permission information from response headers
	x_oauth_scopes := resp.Header.Get("X-OAuth-Scopes")
	acceptedScopes := resp.Header.Get("X-Accepted-OAuth-Scopes")
	acceptedGitHubPermissions := resp.Header.Get("X-Accepted-GitHub-Permissions")
	mediaType := resp.Header.Get("X-GitHub-Media-Type")
	rateLimit := resp.Rate.Limit
	rateRemaining := resp.Rate.Remaining
	rateReset := resp.Rate.Reset.Unix()

	fmt.Printf("Token Permissions:\n")
	fmt.Printf("  OAuth Scopes: %s\n", x_oauth_scopes)
	fmt.Printf("  Accepted OAuth Scopes: %s\n", acceptedScopes)
	fmt.Printf("  Accepted GitHub Permissions: %s\n", acceptedGitHubPermissions)
	fmt.Printf("  GitHub Media Type: %s\n", mediaType)
	fmt.Printf("  Rate Limit: %d\n", rateLimit)
	fmt.Printf("  Rate Remaining: %d\n", rateRemaining)
	fmt.Printf("  Rate Reset: %d\n", rateReset)

	if storeData && db != nil {
		// Clear existing token permission data
		if _, err := db.Exec(`DELETE FROM token_permissions`); err != nil {
			return fmt.Errorf("failed to clear token_permissions table: %w", err)
		}

		// Store new token permission data
		now := time.Now().Format(time.RFC3339)
		_, err = db.Exec(`
			INSERT INTO token_permissions (
				scopes, x_oauth_scopes, x_accepted_oauth_scopes, x_accepted_github_permissions, x_github_media_type,
				x_ratelimit_limit, x_ratelimit_remaining, x_ratelimit_reset,
				created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"", x_oauth_scopes, acceptedScopes, acceptedGitHubPermissions, mediaType,
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
		opts := &github.ListOptions{Page: page, PerPage: DefaultPerPage}
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
		time.Sleep(DefaultSleep)
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
