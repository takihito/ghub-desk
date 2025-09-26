package github

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ghub-desk/store"
	"ghub-desk/validate"

	"github.com/google/go-github/v55/github"
)

const (
	// API pagination settings
	DefaultPerPage = 100
	DefaultSleep   = 1 * time.Second
)

// TargetRequest represents the requested pull target including optional metadata.
type TargetRequest struct {
	Kind     string
	TeamSlug string
}

// HandlePullTarget processes different types of pull targets (users, teams, repos, team_users)
func HandlePullTarget(ctx context.Context, client *github.Client, db *sql.DB, org string, req TargetRequest, token string, storeData bool, intervalTime time.Duration) error {
	switch req.Kind {
	case "users":
		return PullUsers(ctx, client, db, org, storeData, intervalTime)
	case "detail-users":
		return PullDetailUsers(ctx, client, db, org, token, storeData, intervalTime)
	case "teams":
		return PullTeams(ctx, client, db, org, storeData, intervalTime)
	case "repos":
		return PullRepositories(ctx, client, db, org, storeData, intervalTime)
	case "all-teams-users":
		return PullAllTeamsUsers(ctx, client, db, org, storeData, intervalTime)
	case "token-permission":
		return PullTokenPermission(ctx, client, db, storeData, intervalTime)
	case "outside-users":
		return PullOutsideUsers(ctx, client, db, org, storeData, intervalTime)
	case "teams-users":
		if req.TeamSlug == "" {
			return fmt.Errorf("team slug must be specified when using teams-users target")
		}
		if err := validate.ValidateTeamSlug(req.TeamSlug); err != nil {
			return fmt.Errorf("invalid team slug: %w", err)
		}
		return PullTeamUsers(ctx, client, db, org, req.TeamSlug, storeData, intervalTime)
	default:
		return fmt.Errorf("unknown target: %s", req.Kind)
	}
}

// PullUsers fetches organization members and optionally stores them in database
func PullUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, storeData bool, intervalTime time.Duration) error {
	if storeData {
		if _, err := db.Exec(`DELETE FROM ghub_users`); err != nil {
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
		db, org, intervalTime,
	)
}

// PullDetailUsers fetches organization members with detailed information and optionally stores them in database
func PullDetailUsers(ctx context.Context, client *github.Client, db *sql.DB, org, token string, storeData bool, intervalTime time.Duration) error {
	if storeData {
		if _, err := db.Exec(`DELETE FROM ghub_users`); err != nil {
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
			var detailedUsers []*github.User
			for i, u := range users {
				fmt.Printf("Fetching details for user %d/%d: %s\n", i+1, len(users), u.GetLogin())

				detailedUser, _, err := client.Users.Get(ctx, u.GetLogin())
				if err != nil {
					fmt.Printf("Warning: failed to fetch details for user %s: %v\n", u.GetLogin(), err)
					// Use basic user info if detailed fetch fails
					detailedUser = u
				}
				detailedUsers = append(detailedUsers, detailedUser)

				// Rate limiting: sleep between requests to avoid hitting API limits
				time.Sleep(intervalTime)
			}
			return store.StoreUsersWithDetails(db, detailedUsers)
		},
		db, org, intervalTime,
	)
}

// PullTeams fetches organization teams and optionally stores them in database
func PullTeams(ctx context.Context, client *github.Client, db *sql.DB, org string, storeData bool, intervalTime time.Duration) error {
	if storeData {
		if _, err := db.Exec(`DELETE FROM ghub_teams`); err != nil {
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
		db, org, intervalTime,
	)
}

// PullRepositories fetches organization repositories and optionally stores them in database
func PullRepositories(ctx context.Context, client *github.Client, db *sql.DB, org string, storeData bool, intervalTime time.Duration) error {
	if storeData {
		if _, err := db.Exec(`DELETE FROM ghub_repositories`); err != nil {
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
		db, org, intervalTime,
	)
}

// PullTeamUsers fetches team members and optionally stores them in database
func PullTeamUsers(ctx context.Context, client *github.Client, db *sql.DB, org, teamSlug string, storeData bool, intervalTime time.Duration) error {
	if storeData {
		if _, err := db.Exec(`DELETE FROM ghub_team_users WHERE team_slug = ?`, teamSlug); err != nil {
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
		db, org, intervalTime,
	)
}

// PullAllTeamsUsers fetches users for all stored teams
func PullAllTeamsUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, storeData bool, intervalTime time.Duration) error {
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
		if err := PullTeamUsers(ctx, client, db, org, teamSlug, storeData, intervalTime); err != nil {
			fmt.Printf("Warning: failed to fetch users for team %s: %v\n", teamSlug, err)
			continue
		}
	}

	fmt.Printf("Completed fetching users for all teams.\n")
	return nil
}

// PullTokenPermission fetches GitHub token permissions and optionally stores them in database
func PullTokenPermission(ctx context.Context, client *github.Client, db *sql.DB, storeData bool, intervalTime time.Duration) error {
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
		if _, err := db.Exec(`DELETE FROM ghub_token_permissions`); err != nil {
			return fmt.Errorf("failed to clear token_permissions table: %w", err)
		}

		// Store new token permission data
		now := time.Now().Format(time.RFC3339)
		_, err = db.Exec(`
			INSERT INTO ghub_token_permissions (
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
	intervalTime time.Duration,
) error {
	var allItems []*T
	page := 1
	count := 0

	for {
		// Make API call with pagination options
		opts := &github.ListOptions{Page: page, PerPage: DefaultPerPage}
		items, resp, err := listFunc(ctx, org, opts)
		if err != nil {
			if resp != nil {
				scopePermission := fmt.Errorf("X-Accepted-OAuth-Scopes:%s, X-Accepted-GitHub-Permissions:%s",
					resp.Header.Get("X-Accepted-OAuth-Scopes"), resp.Header.Get("X-Accepted-GitHub-Permissions"))
				return fmt.Errorf("failed to fetch page %d: %w, Required permission scope: %w", page, err, scopePermission)
			}
			return fmt.Errorf("failed to fetch page %d: %w", page, err)
		}

		allItems = append(allItems, items...)
		count += len(items)
		fmt.Printf("- %d件取得しました\n", count)

		// Check if we've reached the last page
		if resp.NextPage == 0 {
			break
		}

		page = resp.NextPage

		// Rate limiting: sleep between requests to avoid hitting API limits
		time.Sleep(intervalTime)
	}

	// Store all fetched data in the database
	if len(allItems) > 0 {
		if err := storeFunc(db, allItems); err != nil {
			return fmt.Errorf("failed to store data: %w", err)
		}
	}

	return nil
}

// PullOutsideUsers fetches organization outside collaborators and optionally stores them in database
func PullOutsideUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, storeData bool, intervalTime time.Duration) error {
	if storeData {
		if _, err := db.Exec(`DELETE FROM ghub_outside_users`); err != nil {
			return fmt.Errorf("failed to clear outside_users table: %w", err)
		}
		fmt.Printf("Fetching outside collaborators from GitHub API and storing in database...\n")
	} else {
		fmt.Printf("Fetching outside collaborators from GitHub API...\n")
	}

	return fetchAndStore(
		ctx, client,
		func(ctx context.Context, org string, opts *github.ListOptions) ([]*github.User, *github.Response, error) {
			users, resp, err := client.Organizations.ListOutsideCollaborators(ctx, org, &github.ListOutsideCollaboratorsOptions{
				ListOptions: *opts,
			})
			return users, resp, err
		},
		func(db *sql.DB, users []*github.User) error {
			if !storeData || db == nil {
				return nil
			}
			return store.StoreOutsideUsers(db, users)
		},
		db, org, intervalTime,
	)
}
