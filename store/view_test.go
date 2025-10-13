package store

import (
	"database/sql"
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
		{"team users target (slug)", TargetRequest{Kind: "team-user", TeamSlug: "test-team"}, false},
		{"unknown target", TargetRequest{Kind: "invalid target"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := HandleViewTarget(db, tt.req)
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
	err = ViewUsers(db)
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
	err = ViewTeams(db)
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
	err = ViewRepositories(db)
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

	if err := ViewRepoUsers(db, repoName); err != nil {
		t.Errorf("ViewRepoUsers() error = %v", err)
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

	if err := ViewRepoTeams(db, repoName); err != nil {
		t.Errorf("ViewRepoTeams() error = %v", err)
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
	err = ViewTeamUsers(db, "test-team")
	if err != nil {
		t.Errorf("ViewTeamUsers() error = %v", err)
	}

	// Test with non-existent team
	err = ViewTeamUsers(db, "non-existent-team")
	if err != nil {
		t.Errorf("ViewTeamUsers() should handle non-existent team gracefully, error = %v", err)
	}
}

func TestViewTokenPermission(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Test with no data (should not error, just print message)
	err := ViewTokenPermission(db)
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
	err = ViewTokenPermission(db)
	if err != nil {
		t.Errorf("ViewTokenPermission() with data error = %v", err)
	}
}

func TestViewOutsideUsers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Test with empty table
	err := ViewOutsideUsers(db)
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
	err = ViewOutsideUsers(db)
	if err != nil {
		t.Errorf("ViewOutsideUsers() with data error = %v", err)
	}
}
