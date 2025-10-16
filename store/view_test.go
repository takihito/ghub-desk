package store

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/go-github/v55/github"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// deferを使用してエラー時にもクリーンアップを保証
	// ただし、正常時は呼び出し元でClose()する責任がある
	var success bool
	defer func() {
		if !success {
			db.Close()
		}
	}()

	err = createTables(db)
	if err != nil {
		t.Fatalf("Failed to create tables: %v", err)
	}

	success = true
	return db
}

func captureOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = pw
	defer pr.Close()
	defer func() {
		os.Stdout = old
	}()

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, pr)
		close(done)
	}()

	callErr := fn()
	pw.Close()
	<-done

	return buf.String(), callErr
}

func TestHandleViewTarget(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name        string
		req         TargetRequest
		expectError bool
	}{
		{"users target", TargetRequest{Kind: "users"}, false},
		{"detail-users target", TargetRequest{Kind: "detail-users"}, false},
		{"teams target", TargetRequest{Kind: "teams"}, false},
		{"repos target", TargetRequest{Kind: "repos"}, false},
		{"repositories target", TargetRequest{Kind: "repositories"}, false},
		{"token-permission target", TargetRequest{Kind: "token-permission"}, false},
		{"outside-users target", TargetRequest{Kind: "outside-users"}, false},
		{"repos users target", TargetRequest{Kind: "repos-users", RepoName: "test-repo"}, false},
		{"repo teams target", TargetRequest{Kind: "repos-teams", RepoName: "test-repo"}, false},
		{"all repo teams target", TargetRequest{Kind: "all-repos-teams"}, false},
		{"all teams users target", TargetRequest{Kind: "all-teams-users"}, false},
		{"user repos target", TargetRequest{Kind: "user-repos", UserLogin: "octocat"}, false},
		{"team users target (slug)", TargetRequest{Kind: "team-user", TeamSlug: "test-team"}, false},
		{"unknown target", TargetRequest{Kind: "invalid target"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := HandleViewTarget(db, tt.req, ViewOptions{})
			if (err != nil) != tt.expectError {
				t.Errorf("HandleViewTarget() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestViewUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	users := []*github.User{
		{
			ID:       github.Int64(1),
			Login:    github.String("testuser1"),
			Name:     github.String("Test User 1"),
			Email:    github.String("test1@example.com"),
			Company:  github.String("Test Company"),
			Location: github.String("Test Location"),
		},
		{
			ID:    github.Int64(2),
			Login: github.String("testuser2"),
		},
	}

	err := StoreUsers(db, users)
	if err != nil {
		t.Fatalf("Failed to store test users: %v", err)
	}

	// Test ViewUsers - we can't easily test the output, but we can ensure it doesn't error
	err = ViewUsers(db, FormatTable)
	if err != nil {
		t.Errorf("ViewUsers() error = %v", err)
	}
}

func TestViewTeams(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	teams := []*github.Team{
		{
			ID:          github.Int64(1),
			Name:        github.String("Test Team 1"),
			Slug:        github.String("test-team-1"),
			Description: github.String("Test team description"),
			Privacy:     github.String("closed"),
		},
	}

	err := StoreTeams(db, teams)
	if err != nil {
		t.Fatalf("Failed to store test teams: %v", err)
	}

	// Test ViewTeams
	err = ViewTeams(db, FormatTable)
	if err != nil {
		t.Errorf("ViewTeams() error = %v", err)
	}
}

func TestViewRepositories(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repos := []*github.Repository{
		{
			ID:              github.Int64(1),
			Name:            github.String("test-repo"),
			FullName:        github.String("org/test-repo"),
			Description:     github.String("Test repository"),
			Private:         github.Bool(false),
			Language:        github.String("Go"),
			StargazersCount: github.Int(10),
		},
	}

	err := StoreRepositories(db, repos)
	if err != nil {
		t.Fatalf("Failed to store test repositories: %v", err)
	}

	// Test ViewRepositories
	err = ViewRepositories(db, FormatTable)
	if err != nil {
		t.Errorf("ViewRepositories() error = %v", err)
	}
}

func TestViewRepoUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repoName := "test-repo"
	users := []*github.User{
		{
			ID:    github.Int64(1),
			Login: github.String("collab1"),
		},
		{
			ID:    github.Int64(2),
			Login: github.String("collab2"),
		},
	}

	if err := StoreRepoUsers(db, repoName, users); err != nil {
		t.Fatalf("Failed to store repo users: %v", err)
	}

	if err := ViewRepoUsers(db, repoName, FormatTable); err != nil {
		t.Errorf("ViewRepoUsers() error = %v", err)
	}
}

func TestViewRepoUsersJSON(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repoName := "json-repo"
	users := []*github.User{
		{ID: github.Int64(10), Login: github.String("first")},
		{ID: github.Int64(20), Login: github.String("second")},
	}

	if err := StoreRepoUsers(db, repoName, users); err != nil {
		t.Fatalf("failed to store repo users: %v", err)
	}

	out, err := captureOutput(t, func() error {
		return ViewRepoUsers(db, repoName, FormatJSON)
	})
	if err != nil {
		t.Fatalf("ViewRepoUsers JSON error: %v", err)
	}

	result := struct {
		Repository string `json:"repository"`
		Users      []struct {
			UserID int64  `json:"user_id"`
			Login  string `json:"login"`
		} `json:"users"`
	}{}

	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, out)
	}

	if result.Repository != repoName {
		t.Fatalf("expected repository %q, got %q", repoName, result.Repository)
	}
	if len(result.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(result.Users))
	}
	if result.Users[0].Login != "first" || result.Users[1].Login != "second" {
		t.Fatalf("unexpected user order: %+v", result.Users)
	}
}

func TestViewRepoTeams(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repoName := "test-repo"
	teams := []*github.Team{
		{
			ID:          github.Int64(1),
			Name:        github.String("Team One"),
			Slug:        github.String("team-one"),
			Description: github.String("first team"),
			Privacy:     github.String("closed"),
			Permission:  github.String("push"),
		},
		{
			ID:          github.Int64(2),
			Name:        github.String("Team Two"),
			Slug:        github.String("team-two"),
			Description: github.String("second team"),
			Privacy:     github.String("secret"),
			Permission:  github.String("maintain"),
		},
	}

	if err := StoreRepoTeams(db, repoName, teams); err != nil {
		t.Fatalf("Failed to store repo teams: %v", err)
	}

	if err := ViewRepoTeams(db, repoName, FormatTable); err != nil {
		t.Errorf("ViewRepoTeams() error = %v", err)
	}
}

func TestViewAllRepositoriesTeams(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repos := []*github.Repository{
		{ID: github.Int64(1), Name: github.String("alpha"), FullName: github.String("org/alpha")},
		{ID: github.Int64(2), Name: github.String("beta"), FullName: github.String("org/beta")},
	}
	if err := StoreRepositories(db, repos); err != nil {
		t.Fatalf("failed to store repos: %v", err)
	}

	repoTeams := []*github.Team{
		{ID: github.Int64(1), Name: github.String("Alpha Maintainers"), Slug: github.String("alpha-maint"), Permission: github.String("maintain")},
	}
	if err := StoreRepoTeams(db, "alpha", repoTeams); err != nil {
		t.Fatalf("failed to store alpha teams: %v", err)
	}
	tRepoTeams := []*github.Team{
		{ID: github.Int64(2), Name: github.String("Beta Core"), Slug: github.String("beta-core"), Permission: github.String("push")},
	}
	if err := StoreRepoTeams(db, "beta", tRepoTeams); err != nil {
		t.Fatalf("failed to store beta teams: %v", err)
	}

	output, err := captureOutput(t, func() error {
		return ViewAllRepositoriesTeams(db, FormatTable)
	})
	if err != nil {
		t.Fatalf("ViewAllRepositoriesTeams returned error: %v", err)
	}

	expectMarkers := []string{
		"Repo\tFull Name\tTeam Slug\tTeam Name\tPermission\tPrivacy\tDescription",
		"alpha\torg/alpha\talpha-maint\tAlpha Maintainers\tmaintain\t-\t-",
		"beta\torg/beta\tbeta-core\tBeta Core\tpush\t-\t-",
	}
	for _, marker := range expectMarkers {
		if !strings.Contains(output, marker) {
			t.Fatalf("expected %q in output: %s", marker, output)
		}
	}
}

func TestViewAllTeamsUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	team := &github.Team{
		ID:   github.Int64(11),
		Name: github.String("Core Team"),
		Slug: github.String("core"),
	}
	if err := StoreTeams(db, []*github.Team{team}); err != nil {
		t.Fatalf("failed to store team: %v", err)
	}

	users := []*github.User{
		{
			ID:    github.Int64(21),
			Login: github.String("alice"),
			Name:  github.String("Alice"),
		},
		{
			ID:    github.Int64(22),
			Login: github.String("bob"),
			Name:  github.String("Bob B."),
		},
	}
	if err := StoreUsers(db, users); err != nil {
		t.Fatalf("failed to store users: %v", err)
	}
	if err := StoreTeamUsers(db, users, team.GetSlug()); err != nil {
		t.Fatalf("failed to store team users: %v", err)
	}

	output, err := captureOutput(t, func() error {
		return ViewAllTeamsUsers(db, FormatTable)
	})
	if err != nil {
		t.Fatalf("ViewAllTeamsUsers returned error: %v", err)
	}

	expectMarkers := []string{
		"Team Slug\tTeam Name\tUser Login\tUser Name\tRole",
		"core\tCore Team\talice\tAlice\tmember",
		"core\tCore Team\tbob\tBob B.\tmember",
	}
	for _, marker := range expectMarkers {
		if !strings.Contains(output, marker) {
			t.Fatalf("expected %q in output: %s", marker, output)
		}
	}
}

func TestViewUserRepositories(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := &github.Repository{
		ID:   github.Int64(101),
		Name: github.String("demo-repo"),
	}
	if err := StoreRepositories(db, []*github.Repository{repo}); err != nil {
		t.Fatalf("failed to store repo: %v", err)
	}

	user := &github.User{
		ID:    github.Int64(201),
		Login: github.String("alice"),
		Permissions: map[string]bool{
			"admin": true,
			"push":  true,
		},
	}
	if err := StoreRepoUsers(db, repo.GetName(), []*github.User{user}); err != nil {
		t.Fatalf("failed to store repo users: %v", err)
	}

	team := &github.Team{
		ID:          github.Int64(301),
		Name:        github.String("Dev Team"),
		Slug:        github.String("dev-team"),
		Description: github.String("development"),
	}
	if err := StoreTeams(db, []*github.Team{team}); err != nil {
		t.Fatalf("failed to store team: %v", err)
	}

	repoTeam := &github.Team{
		ID:          team.ID,
		Name:        team.Name,
		Slug:        team.Slug,
		Description: team.Description,
		Permission:  github.String("maintain"),
	}
	if err := StoreRepoTeams(db, repo.GetName(), []*github.Team{repoTeam}); err != nil {
		t.Fatalf("failed to store repo team: %v", err)
	}

	if err := StoreTeamUsers(db, []*github.User{{
		ID:    user.ID,
		Login: user.Login,
	}}, team.GetSlug()); err != nil {
		t.Fatalf("failed to store team users: %v", err)
	}

	output, err := captureOutput(t, func() error {
		return ViewUserRepositories(db, "alice", FormatTable)
	})
	if err != nil {
		t.Fatalf("ViewUserRepositories returned error: %v", err)
	}

	if !strings.Contains(output, "User: alice") {
		t.Fatalf("output missing user header: %s", output)
	}
	if !strings.Contains(output, "demo-repo") {
		t.Fatalf("output missing repository row: %s", output)
	}
	if !strings.Contains(output, "Direct [admin]") {
		t.Fatalf("output missing direct access entry: %s", output)
	}
	if !strings.Contains(output, "Team:dev-team") {
		t.Fatalf("output missing team access entry: %s", output)
	}
	if !strings.Contains(output, "maintain") {
		t.Fatalf("output missing team permission: %s", output)
	}
	if !strings.Contains(output, "admin") {
		t.Fatalf("output missing highest permission: %s", output)
	}
}

func TestViewUserRepositories_NoData(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	output, err := captureOutput(t, func() error {
		return ViewUserRepositories(db, "nobody", FormatTable)
	})
	if err != nil {
		t.Fatalf("ViewUserRepositories returned error: %v", err)
	}

	if !strings.Contains(output, "No repository access data found for user nobody.") {
		t.Fatalf("expected guidance message, got: %s", output)
	}
	if !strings.Contains(output, "pull --repos-users") {
		t.Fatalf("expected pull guidance, got: %s", output)
	}
}

func TestViewTeamUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// First, store a team
	teams := []*github.Team{
		{
			ID:   github.Int64(1),
			Name: github.String("Test Team"),
			Slug: github.String("test-team"),
		},
	}
	err := StoreTeams(db, teams)
	if err != nil {
		t.Fatalf("Failed to store team: %v", err)
	}

	// Then store team users
	users := []*github.User{
		{
			ID:    github.Int64(1),
			Login: github.String("testuser1"),
		},
	}

	err = StoreTeamUsers(db, users, "test-team")
	if err != nil {
		t.Fatalf("Failed to store team users: %v", err)
	}

	// Test ViewTeamUsers
	err = ViewTeamUsers(db, "test-team", FormatTable)
	if err != nil {
		t.Errorf("ViewTeamUsers() error = %v", err)
	}

	// Test with non-existent team
	err = ViewTeamUsers(db, "non-existent-team", FormatTable)
	if err != nil {
		t.Errorf("ViewTeamUsers() should handle non-existent team gracefully, error = %v", err)
	}
}

func TestViewTokenPermission(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Test with no data (should not error, just print message)
	err := ViewTokenPermission(db, FormatTable)
	if err != nil {
		t.Errorf("ViewTokenPermission() with no data error = %v", err)
	}

	_, err = db.Exec(`INSERT INTO ghub_token_permissions(
		scopes, x_oauth_scopes, x_accepted_oauth_scopes, x_accepted_github_permissions, 
		x_github_media_type, x_ratelimit_limit, x_ratelimit_remaining, x_ratelimit_reset,
		created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"repo,user", "repo,user", "repo,user,read:org", "permissions",
		"github.v3", 5000, 4999, 1609459200,
		"2023-01-01 12:00:00", "2023-01-01 12:00:00")
	if err != nil {
		t.Fatalf("Failed to insert test token permission: %v", err)
	}

	// Test ViewTokenPermission with data
	err = ViewTokenPermission(db, FormatTable)
	if err != nil {
		t.Errorf("ViewTokenPermission() with data error = %v", err)
	}
}

func TestViewOutsideUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Test with empty table
	err := ViewOutsideUsers(db, FormatTable)
	if err != nil {
		t.Errorf("ViewOutsideUsers() with empty table error = %v", err)
	}

	// Insert test outside user data
	_, err = db.Exec(`INSERT INTO ghub_outside_users(id, login, name, email, company, location, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		1, "outsideuser1", "Outside User 1", "outside@example.com", "External Corp", "Tokyo",
		"2023-01-01 12:00:00", "2023-01-01 12:00:00")
	if err != nil {
		t.Fatalf("Failed to insert test outside user: %v", err)
	}

	// Test ViewOutsideUsers with data
	err = ViewOutsideUsers(db, FormatTable)
	if err != nil {
		t.Errorf("ViewOutsideUsers() with data error = %v", err)
	}
}
