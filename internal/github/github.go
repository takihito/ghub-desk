package github

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
)

const (
	// API pagination settings
	DefaultPerPage = 100
	DefaultSleep   = 1 * time.Second
)

// InitGitHubClient creates and configures a GitHub API client with OAuth2 authentication
func InitGitHubClient(token string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc)
}

// HandlePullTarget processes different types of pull targets (users, teams, repos, team_users)
func HandlePullTarget(ctx context.Context, client *github.Client, db *sql.DB, org, target string, store bool) error {
	switch {
	case target == "users":
		return PullUsers(ctx, client, db, org, store)
	case target == "teams":
		return PullTeams(ctx, client, db, org, store)
	case target == "repos":
		return PullRepositories(ctx, client, db, org, store)
	case target == "all-teams-users":
		return PullAllTeamsUsers(ctx, client, db, org, store)
	case target == "token-permission":
		return PullTokenPermission(ctx, client, db, store)
	case strings.HasSuffix(target, "/users"):
		teamSlug := strings.TrimSuffix(target, "/users")
		return PullTeamUsers(ctx, client, db, org, teamSlug, store)
	default:
		return fmt.Errorf("unknown target: %s", target)
	}
}

// PullUsers fetches organization members and optionally stores them in database
func PullUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, store bool) error {
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

// PullTeams fetches organization teams and optionally stores them in database
func PullTeams(ctx context.Context, client *github.Client, db *sql.DB, org string, store bool) error {
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

// PullRepositories fetches organization repositories and optionally stores them in database
func PullRepositories(ctx context.Context, client *github.Client, db *sql.DB, org string, store bool) error {
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

// PullTeamUsers fetches team members and optionally stores them in database
func PullTeamUsers(ctx context.Context, client *github.Client, db *sql.DB, org, teamSlug string, store bool) error {
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

// PullAllTeamsUsers fetches users for all stored teams
func PullAllTeamsUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, store bool) error {
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
		if err := PullTeamUsers(ctx, client, db, org, teamSlug, store); err != nil {
			fmt.Printf("Warning: failed to fetch users for team %s: %v\n", teamSlug, err)
			continue
		}
	}

	fmt.Printf("Completed fetching users for all teams.\n")
	return nil
}

// PullTokenPermission fetches GitHub token permissions and optionally stores them in database
func PullTokenPermission(ctx context.Context, client *github.Client, db *sql.DB, store bool) error {
	// Make a simple API call to get token information from response headers
	_, resp, err := client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to get token information: %w", err)
	}

	// Extract permission information from response headers
	scopes := resp.Header.Get("X-OAuth-Scopes")
	acceptedScopes := resp.Header.Get("X-Accepted-OAuth-Scopes")
	acceptedGitHubPermissions := resp.Header.Get("X-Accepted-GitHub-Permissions")
	mediaType := resp.Header.Get("X-GitHub-Media-Type")
	rateLimit := resp.Rate.Limit
	rateRemaining := resp.Rate.Remaining
	rateReset := resp.Rate.Reset.Unix()

	fmt.Printf("Token Permissions:\n")
	fmt.Printf("  OAuth Scopes: %s\n", scopes)
	fmt.Printf("  Accepted OAuth Scopes: %s\n", acceptedScopes)
	fmt.Printf("  Accepted GitHub Permissions: %s\n", acceptedGitHubPermissions)
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
				scopes, x_oauth_scopes, x_accepted_oauth_scopes, x_accepted_github_permissions, x_github_media_type,
				x_ratelimit_limit, x_ratelimit_remaining, x_ratelimit_reset,
				created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			scopes, scopes, acceptedScopes, acceptedGitHubPermissions, mediaType,
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

// ExecutePushRemove executes the actual removal operation via GitHub API
func ExecutePushRemove(ctx context.Context, client *github.Client, org, target, resourceName string) error {
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

// formatTime converts a GitHub timestamp to a string, handling nil values
func formatTime(t github.Timestamp) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
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
