package github

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"ghub-desk/store"
	"ghub-desk/validate"

	"github.com/google/go-github/v55/github"
)

// ExecutePushRemove executes the actual removal operation via GitHub API
func ExecutePushRemove(ctx context.Context, client *github.Client, org, target, resourceName string) error {
	switch target {
	case "team":
		// Remove team from organization
		resp, err := client.Teams.DeleteTeamBySlug(ctx, org, resourceName)
		scopePermission := FormatScopePermission(resp)
		if err != nil {
			return fmt.Errorf("チーム削除エラー: %v, Required permission scope: %s", err, scopePermission)
		}
		return nil

	case "user":
		// Remove user from organization
		resp, err := client.Organizations.RemoveMember(ctx, org, resourceName)
		scopePermission := FormatScopePermission(resp)
		if err != nil {
			return fmt.Errorf("ユーザー削除エラー: %v, Required permission scope: %s", err, scopePermission)
		}
		return nil

	case "team-user":
		// Parse team/user format
		parts := strings.Split(resourceName, "/")
		if len(parts) != 2 {
			return fmt.Errorf("チーム/ユーザー形式が正しくありません。{team_slug}/{user_name} の形式で指定してください")
		}
		teamSlug := parts[0]
		username := parts[1]

		// Remove user from team
		resp, err := client.Teams.RemoveTeamMembershipBySlug(ctx, org, teamSlug, username)
		scopePermission := FormatScopePermission(resp)
		if err != nil {
			return fmt.Errorf("チームからのユーザー削除エラー: %v, Required permission scope: %s", err, scopePermission)
		}
		return nil

	default:
		return fmt.Errorf("サポートされていない削除対象: %s", target)
	}
}

// ExecutePushAdd executes the actual add operation via GitHub API
func ExecutePushAdd(ctx context.Context, client *github.Client, org, target, resourceName string) error {
	switch target {
	case "team-user":
		// Parse team/user format
		parts := strings.Split(resourceName, "/")
		if len(parts) != 2 {
			return fmt.Errorf("チーム/ユーザー形式が正しくありません。{team_slug}/{user_name} の形式で指定してください")
		}
		teamSlug := parts[0]
		username := parts[1]

		// Add user to team
		membership := &github.TeamAddTeamMembershipOptions{
			Role: "member", // Default role is "member"
		}

		_, resp, err := client.Teams.AddTeamMembershipBySlug(ctx, org, teamSlug, username, membership)
		scopePermission := FormatScopePermission(resp)
		if err != nil {
			return fmt.Errorf("チームへのユーザー追加エラー: %v, Required permission scope: %s", err, scopePermission)
		}
		return nil

	default:
		return fmt.Errorf("サポートされていない追加対象: %s", target)
	}
}

func FormatScopePermission(resp *github.Response) string {
	scopePermission := "ResponseHeaderScopePermission:undef"
	if resp != nil && resp.Header != nil {
		scopePermission = fmt.Sprintf("X-Accepted-OAuth-Scopes:%s, X-Accepted-GitHub-Permissions:%s",
			resp.Header.Get("X-Accepted-OAuth-Scopes"), resp.Header.Get("X-Accepted-GitHub-Permissions"))
	}
	return scopePermission
}

// SyncPushAdd reflects side effects of push add operations into the local database.
func SyncPushAdd(ctx context.Context, client *github.Client, db *sql.DB, org, target, resourceName string) error {
	switch target {
	case "team-user":
		teamSlug, userLogin, err := validate.ParseTeamUserPair(resourceName)
		if err != nil {
			return err
		}
		team, _, err := client.Teams.GetTeamBySlug(ctx, org, teamSlug)
		if err != nil {
			return fmt.Errorf("チーム情報の取得に失敗しました: %w", err)
		}
		if err := store.StoreTeams(db, []*github.Team{team}); err != nil {
			return fmt.Errorf("チーム情報の保存に失敗しました: %w", err)
		}
		user, _, err := client.Users.Get(ctx, userLogin)
		if err != nil {
			return fmt.Errorf("ユーザー情報の取得に失敗しました: %w", err)
		}
		if err := store.StoreUsers(db, []*github.User{user}); err != nil {
			return fmt.Errorf("ユーザー情報の保存に失敗しました: %w", err)
		}
		membership, _, err := client.Teams.GetTeamMembershipBySlug(ctx, org, teamSlug, userLogin)
		if err != nil {
			return fmt.Errorf("チームメンバー情報の取得に失敗しました: %w", err)
		}
		role := "member"
		if membership != nil && membership.Role != nil && membership.GetRole() != "" {
			role = membership.GetRole()
		}
		if err := store.UpsertTeamUser(db, teamSlug, team.GetID(), user, role); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("サポートされていない追加対象: %s", target)
	}
}

// SyncPushRemove reflects side effects of push remove operations into the local database.
func SyncPushRemove(ctx context.Context, client *github.Client, db *sql.DB, org, target, resourceName string) error {
	switch target {
	case "team":
		return store.DeleteTeamBySlug(db, resourceName)
	case "user":
		return store.DeleteUserByLogin(db, resourceName)
	case "team-user":
		teamSlug, userLogin, err := validate.ParseTeamUserPair(resourceName)
		if err != nil {
			return err
		}
		return store.DeleteTeamUser(db, teamSlug, userLogin)
	default:
		return fmt.Errorf("サポートされていない削除対象: %s", target)
	}
}
