package github

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
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
	Store        bool
	Stdout       bool
	Interval     time.Duration
	StartPage    int
	InitialCount int
	Resume       ResumeState
	Progress     ProgressReporter
}

// ResumeState captures the persisted progress of a previous pull execution.
type ResumeState struct {
	Endpoint string
	Metadata map[string]string
	LastPage int
	Count    int
}

// ProgressReporter updates persisted state as pull commands advance.
type ProgressReporter interface {
	Start(endpoint string, metadata map[string]string, page int, count int) error
	Page(endpoint string, metadata map[string]string, page int, count int) error
}

// ForEndpoint returns a copy of the options adjusted for the given endpoint and metadata.
func (opts PullOptions) ForEndpoint(endpoint string, metadata map[string]string) PullOptions {
	next := opts
	next.StartPage = 1
	next.InitialCount = 0
	// maps.Equal handles nil maps and ensures order-independent comparison.
	if opts.Resume.Endpoint == endpoint && maps.Equal(opts.Resume.Metadata, metadata) {
		next.StartPage = opts.Resume.LastPage + 1
		if next.StartPage < 1 {
			next.StartPage = 1
		}
		next.InitialCount = opts.Resume.Count
	}
	next.Resume = ResumeState{}
	return next
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
	case "all-repos-users":
		return PullAllReposUsers(ctx, client, db, org, opts)
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
	localOpts := opts.ForEndpoint("users", nil)

	if localOpts.Store && localOpts.InitialCount == 0 {
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
			if !localOpts.Store || db == nil {
				return nil
			}
			return store.StoreUsers(db, users)
		},
		db, org, localOpts, "users", nil,
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
	localOpts := opts.ForEndpoint("detail-users", nil)

	if localOpts.Store && localOpts.InitialCount == 0 {
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
				time.Sleep(localOpts.Interval)
			}
			detailedUsersList = append(detailedUsersList, detailedUsers...)
			if !localOpts.Store || db == nil {
				return nil
			}
			return store.StoreUsersWithDetails(db, detailedUsers)
		},
		db, org, localOpts, "detail-users", nil,
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
	localOpts := opts.ForEndpoint("teams", nil)

	if localOpts.Store && localOpts.InitialCount == 0 {
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
			if !localOpts.Store || db == nil {
				return nil
			}
			return store.StoreTeams(db, teams)
		},
		db, org, localOpts, "teams", nil,
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
	localOpts := opts.ForEndpoint("repos", nil)

	if localOpts.Store && localOpts.InitialCount == 0 {
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
			if !localOpts.Store || db == nil {
				return nil
			}
			return store.StoreRepositories(db, repos)
		},
		db, org, localOpts, "repos", nil,
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
	meta := map[string]string{"repo": repoName}
	localOpts := opts.ForEndpoint("repos-users", meta)

	if localOpts.Store && localOpts.InitialCount == 0 {
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
			if !localOpts.Store || db == nil {
				return nil
			}
			return store.StoreRepoUsers(db, repoName, users)
		},
		db, org, localOpts, "repos-users", meta,
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
	meta := map[string]string{"repo": repoName}
	localOpts := opts.ForEndpoint("repos-teams", meta)

	if localOpts.Store && localOpts.InitialCount == 0 {
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
			if !localOpts.Store || db == nil {
				return nil
			}
			return store.StoreRepoTeams(db, repoName, teams)
		},
		db, org, localOpts, "repos-teams", meta,
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

// PullAllReposUsers iterates all repositories and fetches their direct collaborators.
func PullAllReposUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, opts PullOptions) error {
	if db == nil {
		return fmt.Errorf("database connection is required to fetch all repository users")
	}

	repoNames, err := store.ListRepositoryNames(db)
	if err != nil {
		return fmt.Errorf("failed to load repositories from database: %w", err)
	}

	if len(repoNames) == 0 {
		fmt.Println("No repositories found in database. Please run 'ghub-desk pull --repos' first.")
		return nil
	}

	stdoutPayload := make([]struct {
		Repo  string         `json:"repo"`
		Users []*github.User `json:"users"`
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

	resumeState := opts.Resume
	resumeRepoIndex := -1
	if len(uniqueRepos) > 0 {
		var logMsg string
		resumeState, resumeRepoIndex, logMsg, _ = prepareResume(uniqueRepos, resumeState, "repos-users", "repo", "repo_index", "repository", "repository name")
		if logMsg != "" {
			fmt.Print(logMsg)
		}
	}

	fmt.Printf("Fetching users for %d repositories...\n", len(uniqueRepos))

	for idx, repoName := range uniqueRepos {
		if resumeState.Endpoint == "repos-users" && resumeRepoIndex >= 0 && idx < resumeRepoIndex {
			continue
		}

		fmt.Printf("Fetching users for repository %d/%d: %s\n", idx+1, len(uniqueRepos), repoName)
		meta := map[string]string{
			"repo":       repoName,
			"repo_index": strconv.Itoa(idx),
		}

		baseOpts := opts
		baseOpts.Resume = resumeState
		localOpts := baseOpts.ForEndpoint("repos-users", meta)

		if localOpts.Store && localOpts.InitialCount == 0 {
			if db == nil {
				return fmt.Errorf("database connection is required to store repository users")
			}
			if _, err := db.Exec(`DELETE FROM ghub_repos_users WHERE repos_name = ?`, repoName); err != nil {
				return fmt.Errorf("failed to clear repository users for %s: %w", repoName, err)
			}
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
				if !localOpts.Store || db == nil {
					return nil
				}
				return store.StoreRepoUsers(db, repoName, users)
			},
			db, org, localOpts, "repos-users", meta,
		)
		if err != nil {
			return fmt.Errorf("failed to fetch repository users for %s: %w", repoName, err)
		}

		if opts.Stdout {
			stdoutPayload = append(stdoutPayload, struct {
				Repo  string         `json:"repo"`
				Users []*github.User `json:"users"`
			}{
				Repo:  repoName,
				Users: users,
			})
		}

		if resumeState.Endpoint == "repos-users" && resumeRepoIndex >= 0 && idx == resumeRepoIndex {
			resumeState = ResumeState{}
			resumeRepoIndex = -1
		}
	}

	if opts.Stdout {
		if err := printJSON(stdoutPayload); err != nil {
			return err
		}
	}

	return nil
}

// PullAllReposTeams iterates all repositories and fetches their team assignments.
func PullAllReposTeams(ctx context.Context, client *github.Client, db *sql.DB, org string, opts PullOptions) error {
	if db == nil {
		return fmt.Errorf("database connection is required to fetch all repository teams")
	}

	repoNames, err := store.ListRepositoryNames(db)
	if err != nil {
		return fmt.Errorf("failed to load repositories from database: %w", err)
	}

	if len(repoNames) == 0 {
		fmt.Println("No repositories found in database. Please run 'ghub-desk pull --repos' first.")
		return nil
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

	resumeState := opts.Resume
	resumeRepoIndex := -1
	if len(uniqueRepos) > 0 {
		var logMsg string
		// Ensure the stored resume state still points at an existing repository name.
		// Matching by name prevents newly inserted repositories from causing the resume logic
		// to skip the original in-progress repository.
		resumeState, resumeRepoIndex, logMsg, _ = prepareResume(uniqueRepos, resumeState, "repos-teams", "repo", "repo_index", "repository", "repository name")
		if logMsg != "" {
			fmt.Print(logMsg)
		}
	}

	fmt.Printf("Fetching teams for %d repositories...\n", len(uniqueRepos))

	for idx, repoName := range uniqueRepos {
		if resumeState.Endpoint == "repos-teams" && resumeRepoIndex >= 0 && idx < resumeRepoIndex {
			continue
		}

		fmt.Printf("Fetching teams for repository %d/%d: %s\n", idx+1, len(uniqueRepos), repoName)
		meta := map[string]string{
			"repo":       repoName,
			"repo_index": strconv.Itoa(idx),
		}

		baseOpts := opts
		baseOpts.Resume = resumeState
		localOpts := baseOpts.ForEndpoint("repos-teams", meta)

		if localOpts.Store && localOpts.InitialCount == 0 {
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
				if !localOpts.Store || db == nil {
					return nil
				}
				return store.StoreRepoTeams(db, repoName, teams)
			},
			db, org, localOpts, "repos-teams", meta,
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

		if resumeState.Endpoint == "repos-teams" && resumeRepoIndex >= 0 && idx == resumeRepoIndex {
			// After processing the resumed repository, clear state so following repos start fresh.
			resumeState = ResumeState{}
			resumeRepoIndex = -1
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
	users, err := pullTeamUsers(ctx, client, db, org, teamSlug, nil, opts)
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

func pullTeamUsers(ctx context.Context, client *github.Client, db *sql.DB, org, teamSlug string, meta map[string]string, opts PullOptions) ([]*github.User, error) {
	metadata := map[string]string{"team": teamSlug}
	for k, v := range meta {
		metadata[k] = v
	}

	localOpts := opts.ForEndpoint("team-user", metadata)

	if localOpts.Store && localOpts.InitialCount == 0 {
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
			if !localOpts.Store || db == nil {
				return nil
			}
			return store.StoreTeamUsers(db, users, teamSlug)
		},
		db, org, localOpts, "team-user", metadata,
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
	resumeState := opts.Resume
	resumeTeamIndex := -1
	if len(teamSlugs) > 0 {
		var logMsg string
		// Similar to repositories, match the stored team slug against the current list so we
		// only skip ahead when the intended team still exists.
		resumeState, resumeTeamIndex, logMsg, _ = prepareResume(teamSlugs, resumeState, "team-user", "team", "team_index", "team", "team slug")
		if logMsg != "" {
			fmt.Print(logMsg)
		}
	}
	for i, teamSlug := range teamSlugs {
		if resumeState.Endpoint == "team-user" && resumeTeamIndex >= 0 && i < resumeTeamIndex {
			continue
		}

		fmt.Printf("Processing team %d/%d: %s\n", i+1, len(teamSlugs), teamSlug)
		meta := map[string]string{
			"team":       teamSlug,
			"team_index": strconv.Itoa(i),
		}
		baseOpts := opts
		baseOpts.Resume = resumeState

		// Ensure pullTeamUsers receives the tailored options, including resume state.
		users, err := pullTeamUsers(ctx, client, db, org, teamSlug, meta, baseOpts)
		if err != nil {
			fmt.Printf("Warning: failed to fetch users for team %s: %v\n", teamSlug, err)
			continue
		}
		if resumeState.Endpoint == "team-user" && i == resumeTeamIndex {
			resumeState = ResumeState{}
			resumeTeamIndex = -1
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
	pullOpts PullOptions,
	endpoint string,
	metadata map[string]string,
) ([]*T, error) {
	allItems := make([]*T, 0, DefaultPerPage*50)

	page := pullOpts.StartPage
	if page < 1 {
		page = 1
	}
	count := pullOpts.InitialCount

	if pullOpts.Progress != nil {
		if err := pullOpts.Progress.Start(endpoint, metadata, page-1, count); err != nil {
			return nil, err
		}
	}

	for {
		if err := ctx.Err(); err != nil {
			return allItems, err
		}

		// Make API call with pagination options
		listOpts := &github.ListOptions{Page: page, PerPage: DefaultPerPage}
		items, resp, err := listFunc(ctx, org, listOpts)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return allItems, ctx.Err()
			}
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

		if len(items) > 0 {
			if err := storeFunc(db, items); err != nil {
				return nil, fmt.Errorf("failed to store data: %w", err)
			}
			if pullOpts.Progress != nil {
				if err := pullOpts.Progress.Page(endpoint, metadata, page, count); err != nil {
					return nil, err
				}
			}
		}

		// Check if we've reached the last page
		if resp.NextPage == 0 {
			break
		}

		page = resp.NextPage

		// Rate limiting: sleep between requests to avoid hitting API limits
		if err := sleepWithContext(ctx, pullOpts.Interval); err != nil {
			return allItems, err
		}
	}

	return allItems, nil
}

// PullOutsideUsers fetches organization outside collaborators and optionally stores them in database
func PullOutsideUsers(ctx context.Context, client *github.Client, db *sql.DB, org string, opts PullOptions) error {
	localOpts := opts.ForEndpoint("outside-users", nil)

	if localOpts.Store && localOpts.InitialCount == 0 {
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
			if !localOpts.Store || db == nil {
				return nil
			}
			return store.StoreOutsideUsers(db, users)
		},
		db, org, localOpts, "outside-users", nil,
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

// prepareResume normalizes resume metadata for list-based targets, ensuring that the stored
// name still exists in the active list. When the metadata is stale it clears the resume state
// and returns a message so the caller can notify the user.
func prepareResume(names []string, state ResumeState, endpoint, nameKey, indexKey, label, identifier string) (ResumeState, int, string, string) {
	if state.Endpoint != endpoint {
		return state, -1, "", ""
	}

	meta := state.Metadata
	var name string
	if meta != nil {
		name = strings.TrimSpace(meta[nameKey])
	}
	if name != "" {
		for idx, current := range names {
			if current == name {
				return state, idx, "", name
			}
		}
		return ResumeState{}, -1, fmt.Sprintf("INFO: resume target %s '%s' not found in current list; restarting from first %s.\n", label, name, label), ""
	}

	var storedIndex string
	if meta != nil {
		storedIndex = strings.TrimSpace(meta[indexKey])
	}
	if storedIndex != "" {
		return ResumeState{}, -1, fmt.Sprintf("INFO: resume metadata missing %s; restarting from first %s (stored index=%s).\n", identifier, label, storedIndex), ""
	}

	return ResumeState{}, -1, "", ""
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stdout data: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
