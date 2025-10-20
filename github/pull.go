package github

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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
	Kind      string
	TeamSlug  string
	RepoName  string
	UserLogin string
}

// PullOptions controls how data fetched from GitHub should be handled locally.
type PullOptions struct {
	Store    bool
	Stdout   bool
	Interval time.Duration
}

// HandlePullTarget processes different types of pull targets (users, teams, repos, team_users)
func HandlePullTarget(ctx context.Context, client *github.Client, db *sql.DB, org string, req TargetRequest, token string, opts PullOptions) error {
	switch req.Kind {
	case "users":
		return PullUsers(ctx, client, db, org, opts)
	case "detail-users":
		return PullDetailUsers(ctx, client, db, org, token, opts)
	case "teams":
		return PullTeams(ctx, client, db, org, opts)
	case "repos":
		return PullRepositories(ctx, client, db, org, opts)
	case "repos-users":
		if req.RepoName == "" {
			return fmt.Errorf("repository name must be specified when using repos-users target")
		}
		return PullRepoUsers(ctx, client, db, org, req.RepoName, opts)
	case "repos-teams":
		if req.RepoName == "" {
			return fmt.Errorf("repository name must be specified when using repos-teams target")
		}
		if err := validate.ValidateRepoName(req.RepoName); err != nil {
			return fmt.Errorf("invalid repository name: %w", err)
		}
		return PullRepoTeams(ctx, client, db, org, req.RepoName, opts)
	case "all-repos-teams":
		return PullAllReposTeams(ctx, client, db, org, opts)
	case "all-teams-users":
		return PullAllTeamsUsers(ctx, client, db, org, opts)
	case "token-permission":
		return PullTokenPermission(ctx, client, db, opts)
	case "outside-users":
		return PullOutsideUsers(ctx, client, db, org, opts)
	case "team-user":
		if req.TeamSlug == "" {
			return fmt.Errorf("team slug must be specified when using team-user target")
		}
		if err := validate.ValidateTeamSlug(req.TeamSlug); err != nil {
			return fmt.Errorf("invalid team slug: %w", err)
		}
		return PullTeamUsers(ctx, client, db, org, req.TeamSlug, opts)
	default:
		return fmt.Errorf("unknown target: %s", req.Kind)
	}
}

// PullUsers fetches organization members and optionally stores them in database
func PullUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, opts PullOptions) error {
	if opts.Store {
		if db == nil {
			return fmt.Errorf("database connection is required to store users")
		}
		if _, err := db.Exec(`DELETE FROM ghub_users`); err != nil {
			return fmt.Errorf("failed to clear users table: %w", err)
		}
	}

	users, err := fetchAndStore(
		ctx, client,
		func(ctx context.Context, org string, opts *github.ListOptions) ([]*github.User, *github.Response, error) {
			memberOpts := &github.ListMembersOptions{ListOptions: *opts}
			return client.Organizations.ListMembers(ctx, org, memberOpts)
		},
		func(db *sql.DB, users []*github.User) error {
			if db == nil {
				return nil
			}
			return store.StoreUsers(db, users)
		},
		db, org, opts.Interval, opts.Store,
	)
	if err != nil {
		return err
	}

	if opts.Stdout {
		if err := printJSON(users); err != nil {
			return err
		}
	}

	return nil
}

// PullDetailUsers fetches organization members with detailed information and optionally stores them in database
func PullDetailUsers(ctx context.Context, client *github.Client, db *sql.DB, org, token string, opts PullOptions) error {
	if opts.Store {
		if db == nil {
			return fmt.Errorf("database connection is required to store detailed users")
		}
		if _, err := db.Exec(`DELETE FROM ghub_users`); err != nil {
			return fmt.Errorf("failed to clear users table: %w", err)
		}
	}

	detailedUsersList := make([]*github.User, 0)
	_, err := fetchAndStore(
		ctx, client,
		func(ctx context.Context, org string, optsList *github.ListOptions) ([]*github.User, *github.Response, error) {
			memberOpts := &github.ListMembersOptions{ListOptions: *optsList}
			return client.Organizations.ListMembers(ctx, org, memberOpts)
		},
		func(db *sql.DB, users []*github.User) error {
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
				time.Sleep(opts.Interval)
			}
			detailedUsersList = append(detailedUsersList, detailedUsers...)
			if db == nil {
				return nil
			}
			return store.StoreUsersWithDetails(db, detailedUsers)
		},
		db, org, opts.Interval, opts.Store,
	)
	if err != nil {
		return err
	}

	if opts.Stdout {
		if err := printJSON(detailedUsersList); err != nil {
			return err
		}
	}

	return nil
}

// PullTeams fetches organization teams and optionally stores them in database
func PullTeams(ctx context.Context, client *github.Client, db *sql.DB, org string, opts PullOptions) error {
	if opts.Store {
		if db == nil {
			return fmt.Errorf("database connection is required to store teams")
		}
		if _, err := db.Exec(`DELETE FROM ghub_teams`); err != nil {
			return fmt.Errorf("failed to clear teams table: %w", err)
		}
	}

	teams, err := fetchAndStore(
		ctx, client,
		func(ctx context.Context, org string, optsList *github.ListOptions) ([]*github.Team, *github.Response, error) {
			return client.Teams.ListTeams(ctx, org, optsList)
		},
		func(db *sql.DB, teams []*github.Team) error {
			if db == nil {
				return nil
			}
			return store.StoreTeams(db, teams)
		},
		db, org, opts.Interval, opts.Store,
	)
	if err != nil {
		return err
	}

	if opts.Stdout {
		if err := printJSON(teams); err != nil {
			return err
		}
	}

	return nil
}

// PullRepositories fetches organization repositories and optionally stores them in database
func PullRepositories(ctx context.Context, client *github.Client, db *sql.DB, org string, opts PullOptions) error {
	if opts.Store {
		if db == nil {
			return fmt.Errorf("database connection is required to store repositories")
		}
		if _, err := db.Exec(`DELETE FROM ghub_repos`); err != nil {
			return fmt.Errorf("failed to clear repositories table: %w", err)
		}
	}

	repos, err := fetchAndStore(
		ctx, client,
		func(ctx context.Context, org string, optsList *github.ListOptions) ([]*github.Repository, *github.Response, error) {
			repoOpts := &github.RepositoryListByOrgOptions{ListOptions: *optsList}
			return client.Repositories.ListByOrg(ctx, org, repoOpts)
		},
		func(db *sql.DB, repos []*github.Repository) error {
			if db == nil {
				return nil
			}
			return store.StoreRepositories(db, repos)
		},
		db, org, opts.Interval, opts.Store,
	)
	if err != nil {
		return err
	}

	if opts.Stdout {
		if err := printJSON(repos); err != nil {
			return err
		}
	}

	return nil
}

// PullRepoUsers fetches direct repository collaborators and optionally stores them in database
func PullRepoUsers(ctx context.Context, client *github.Client, db *sql.DB, org, repoName string, opts PullOptions) error {
	if opts.Store {
		if db == nil {
			return fmt.Errorf("database connection is required to store repository users")
		}
		if _, err := db.Exec(`DELETE FROM ghub_repos_users WHERE repos_name = ?`, repoName); err != nil {
			return fmt.Errorf("failed to clear repository users for %s: %w", repoName, err)
		}
		// TODO: replace with upsert-based synchronization to avoid full delete for large repositories.
	}

	users, err := fetchAndStore(
		ctx, client,
		func(ctx context.Context, org string, optsList *github.ListOptions) ([]*github.User, *github.Response, error) {
			collabOpts := &github.ListCollaboratorsOptions{
				Affiliation: "direct",
				ListOptions: *optsList,
			}
			return client.Repositories.ListCollaborators(ctx, org, repoName, collabOpts)
		},
		func(db *sql.DB, users []*github.User) error {
			if db == nil {
				return nil
			}
			return store.StoreRepoUsers(db, repoName, users)
		},
		db, org, opts.Interval, opts.Store,
	)
	if err != nil {
		return err
	}

	if opts.Stdout {
		if err := printJSON(users); err != nil {
			return err
		}
	}

	return nil
}

// PullRepoTeams fetches repository teams and optionally stores them in database
func PullRepoTeams(ctx context.Context, client *github.Client, db *sql.DB, org, repoName string, opts PullOptions) error {
	if opts.Store {
		if db == nil {
			return fmt.Errorf("database connection is required to store repository teams")
		}
		if _, err := db.Exec(`DELETE FROM ghub_repos_teams WHERE repos_name = ?`, repoName); err != nil {
			return fmt.Errorf("failed to clear repository teams for %s: %w", repoName, err)
		}
	}

	teams, err := fetchAndStore(
		ctx, client,
		func(ctx context.Context, org string, optsList *github.ListOptions) ([]*github.Team, *github.Response, error) {
			return client.Repositories.ListTeams(ctx, org, repoName, optsList)
		},
		func(db *sql.DB, teams []*github.Team) error {
			if db == nil {
				return nil
			}
			return store.StoreRepoTeams(db, repoName, teams)
		},
		db, org, opts.Interval, opts.Store,
	)
	if err != nil {
		return err
	}

	if opts.Stdout {
		if err := printJSON(teams); err != nil {
			return err
		}
	}

	return nil
}

// PullAllReposTeams iterates all repositories and fetches their team assignments.
func PullAllReposTeams(ctx context.Context, client *github.Client, db *sql.DB, org string, opts PullOptions) error {
	repoNames := make([]string, 0)
	if db != nil {
		names, err := store.ListRepositoryNames(db)
		if err != nil {
			return fmt.Errorf("failed to load repositories from database: %w", err)
		}
		repoNames = append(repoNames, names...)
	}

	if len(repoNames) == 0 {
		fmt.Println("Repository list not found in local database. Fetching from GitHub API...")
		repos, err := fetchAndStore(
			ctx, client,
			func(ctx context.Context, org string, optsList *github.ListOptions) ([]*github.Repository, *github.Response, error) {
				repoOpts := &github.RepositoryListByOrgOptions{ListOptions: *optsList}
				return client.Repositories.ListByOrg(ctx, org, repoOpts)
			},
			func(db *sql.DB, repos []*github.Repository) error {
				if !opts.Store || db == nil {
					return nil
				}
				return store.StoreRepositories(db, repos)
			},
			db, org, opts.Interval, opts.Store,
		)
		if err != nil {
			return fmt.Errorf("failed to fetch repositories: %w", err)
		}
		for _, repo := range repos {
			if name := strings.TrimSpace(repo.GetName()); name != "" {
				repoNames = append(repoNames, name)
			}
		}
	}

	if len(repoNames) == 0 {
		return fmt.Errorf("no repositories available. Run 'ghub-desk pull --repos' first")
	}

	stdoutPayload := make([]struct {
		Repo  string         `json:"repo"`
		Teams []*github.Team `json:"teams"`
	}, 0, len(repoNames))

	seen := make(map[string]struct{}, len(repoNames))
	uniqueRepos := make([]string, 0, len(repoNames))
	for _, name := range repoNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		uniqueRepos = append(uniqueRepos, trimmed)
	}

	for idx, repoName := range uniqueRepos {
		fmt.Printf("Fetching teams for repository %d/%d: %s\n", idx+1, len(uniqueRepos), repoName)
		if opts.Store {
			if db == nil {
				return fmt.Errorf("database connection is required to store repository teams")
			}
			if _, err := db.Exec(`DELETE FROM ghub_repos_teams WHERE repos_name = ?`, repoName); err != nil {
				return fmt.Errorf("failed to clear repository teams for %s: %w", repoName, err)
			}
		}

		teams, err := fetchAndStore(
			ctx, client,
			func(ctx context.Context, org string, optsList *github.ListOptions) ([]*github.Team, *github.Response, error) {
				return client.Repositories.ListTeams(ctx, org, repoName, optsList)
			},
			func(db *sql.DB, teams []*github.Team) error {
				if db == nil {
					return nil
				}
				return store.StoreRepoTeams(db, repoName, teams)
			},
			db, org, opts.Interval, opts.Store,
		)
		if err != nil {
			return fmt.Errorf("failed to fetch repository teams for %s: %w", repoName, err)
		}

		if opts.Stdout {
			stdoutPayload = append(stdoutPayload, struct {
				Repo  string         `json:"repo"`
				Teams []*github.Team `json:"teams"`
			}{
				Repo:  repoName,
				Teams: teams,
			})
		}
	}

	if opts.Stdout {
		if err := printJSON(stdoutPayload); err != nil {
			return err
		}
	}

	return nil
}

// PullTeamUsers fetches team members and optionally stores them in database
func PullTeamUsers(ctx context.Context, client *github.Client, db *sql.DB, org, teamSlug string, opts PullOptions) error {
	users, err := pullTeamUsers(ctx, client, db, org, teamSlug, opts)
	if err != nil {
		return err
	}

	if opts.Stdout {
		output := struct {
			Team  string         `json:"team"`
			Users []*github.User `json:"users"`
		}{
			Team:  teamSlug,
			Users: users,
		}
		if err := printJSON(output); err != nil {
			return err
		}
	}

	return nil
}

func pullTeamUsers(ctx context.Context, client *github.Client, db *sql.DB, org, teamSlug string, opts PullOptions) ([]*github.User, error) {
	if opts.Store {
		if db == nil {
			return nil, fmt.Errorf("database connection is required to store team users")
		}
		if _, err := db.Exec(`DELETE FROM ghub_team_users WHERE team_slug = ?`, teamSlug); err != nil {
			return nil, fmt.Errorf("failed to clear team_users table: %w", err)
		}
		// TODO ? : check store.StoreTeams(db, []*github.Team{team});
		// TODO DBを正規化して一度に組織を取得する手順(pull --all-organization-data? --cmd-list "--teams,--team-users a-team,")を考える
	}

	users, err := fetchAndStore(
		ctx, client,
		func(ctx context.Context, org string, optsList *github.ListOptions) ([]*github.User, *github.Response, error) {
			teamOpts := &github.TeamListTeamMembersOptions{ListOptions: *optsList}
			return client.Teams.ListTeamMembersBySlug(ctx, org, teamSlug, teamOpts)
		},
		func(db *sql.DB, users []*github.User) error {
			if db == nil {
				return nil
			}
			return store.StoreTeamUsers(db, users, teamSlug)
		},
		db, org, opts.Interval, opts.Store,
	)
	if err != nil {
		return nil, err
	}

	if users == nil {
		users = make([]*github.User, 0)
	}

	return users, nil
}

// PullAllTeamsUsers fetches users for all stored teams
func PullAllTeamsUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, opts PullOptions) error {
	if db == nil {
		return fmt.Errorf("database connection is required to fetch all team users")
	}

	// Get all team slugs from the database
	rows, err := db.Query(`SELECT slug FROM ghub_teams`)
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
		fmt.Println("No teams found in database. Please run 'ghub-desk pull teams' first.")
		return nil
	}

	fmt.Printf("Fetching users for %d teams...\n", len(teamSlugs))

	// Fetch users for each team
	var stdoutResults []struct {
		Team  string         `json:"team"`
		Users []*github.User `json:"users"`
	}
	for i, teamSlug := range teamSlugs {
		fmt.Printf("Processing team %d/%d: %s\n", i+1, len(teamSlugs), teamSlug)
		users, err := pullTeamUsers(ctx, client, db, org, teamSlug, opts)
		if err != nil {
			fmt.Printf("Warning: failed to fetch users for team %s: %v\n", teamSlug, err)
			continue
		}
		if opts.Stdout {
			stdoutResults = append(stdoutResults, struct {
				Team  string         `json:"team"`
				Users []*github.User `json:"users"`
			}{
				Team:  teamSlug,
				Users: users,
			})
		}
	}

	fmt.Printf("Completed fetching users for all teams.\n")

	if opts.Stdout {
		if err := printJSON(stdoutResults); err != nil {
			return err
		}
	}

	return nil
}

// PullTokenPermission fetches GitHub token permissions and optionally stores them in database
func PullTokenPermission(ctx context.Context, client *github.Client, db *sql.DB, opts PullOptions) error {
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

	if opts.Stdout {
		output := map[string]any{
			"oauth_scopes":                x_oauth_scopes,
			"accepted_oauth_scopes":       acceptedScopes,
			"accepted_github_permissions": acceptedGitHubPermissions,
			"github_media_type":           mediaType,
			"rate_limit":                  rateLimit,
			"rate_remaining":              rateRemaining,
			"rate_reset":                  rateReset,
		}
		if err := printJSON(output); err != nil {
			return err
		}
	}

	if opts.Store {
		if db == nil {
			return fmt.Errorf("database connection is required to store token permissions")
		}
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
	store bool,
) ([]*T, error) {
	allItems := make([]*T, 0, DefaultPerPage*50)
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
				return nil, fmt.Errorf("failed to fetch page %d: %w, Required permission scope: %w", page, err, scopePermission)
			}
			return nil, fmt.Errorf("failed to fetch page %d: %w", page, err)
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
	if store && len(allItems) > 0 {
		if err := storeFunc(db, allItems); err != nil {
			return nil, fmt.Errorf("failed to store data: %w", err)
		}
	}

	return allItems, nil
}

// PullOutsideUsers fetches organization outside collaborators and optionally stores them in database
func PullOutsideUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, opts PullOptions) error {
	if opts.Store {
		if db == nil {
			return fmt.Errorf("database connection is required to store outside users")
		}
		if _, err := db.Exec(`DELETE FROM ghub_outside_users`); err != nil {
			return fmt.Errorf("failed to clear outside_users table: %w", err)
		}
		fmt.Printf("Fetching outside collaborators from GitHub API and storing in database...\n")
	} else {
		fmt.Printf("Fetching outside collaborators from GitHub API...\n")
	}

	users, err := fetchAndStore(
		ctx, client,
		func(ctx context.Context, org string, optsList *github.ListOptions) ([]*github.User, *github.Response, error) {
			users, resp, err := client.Organizations.ListOutsideCollaborators(ctx, org, &github.ListOutsideCollaboratorsOptions{
				ListOptions: *optsList,
			})
			return users, resp, err
		},
		func(db *sql.DB, users []*github.User) error {
			if !opts.Store || db == nil {
				return nil
			}
			return store.StoreOutsideUsers(db, users)
		},
		db, org, opts.Interval, opts.Store,
	)
	if err != nil {
		return err
	}

	if opts.Stdout {
		if err := printJSON(users); err != nil {
			return err
		}
	}

	return nil
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stdout data: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
