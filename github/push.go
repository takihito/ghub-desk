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
	case "outside-user", "repos-user":
		repoName, username, err := validate.ParseRepoUserPair(resourceName)
		if err != nil {
			return fmt.Errorf("リポジトリ/ユーザー形式が正しくありません。{repository}/{user_name} の形式で指定してください")
		}
		if err := PushRemoveOutsideCollaborator(ctx, client, org, repoName, username); err != nil {
			return err
		}
		return nil

	default:
		return fmt.Errorf("サポートされていない削除対象: %s", target)
	}
}

// ExecutePushAdd executes the actual add operation via GitHub API
func ExecutePushAdd(ctx context.Context, client *github.Client, org, target, resourceName, permission string) error {
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
	case "outside-user":
		repoName, username, err := validate.ParseRepoUserPair(resourceName)
		if err != nil {
			return fmt.Errorf("リポジトリ/ユーザー形式が正しくありません。{repository}/{user_name} の形式で指定してください")
		}
		if err := PushAddOutsideCollaborator(ctx, client, org, repoName, username, permission); err != nil {
			return err
		}
		return nil

	default:
		return fmt.Errorf("サポートされていない追加対象: %s", target)
	}
}

// PushAddOutsideCollaborator adds an outside collaborator to a repository.
func PushAddOutsideCollaborator(ctx context.Context, client *github.Client, org, repoName, username, permission string) error {
	var opts *github.RepositoryAddCollaboratorOptions
	if permission != "" {
		opts = &github.RepositoryAddCollaboratorOptions{Permission: permission}
	}
	_, resp, err := client.Repositories.AddCollaborator(ctx, org, repoName, username, opts)
	scopePermission := FormatScopePermission(resp)
	if err != nil {
		return fmt.Errorf("リポジトリへのユーザー追加エラー: %v, Required permission scope: %s", err, scopePermission)
	}
	return nil
}

// PushRemoveOutsideCollaborator removes an outside collaborator from a repository.
func PushRemoveOutsideCollaborator(ctx context.Context, client *github.Client, org, repoName, username string) error {
	resp, err := client.Repositories.RemoveCollaborator(ctx, org, repoName, username)
	scopePermission := FormatScopePermission(resp)
	if err != nil {
		return fmt.Errorf("リポジトリからのユーザー削除エラー: %v, Required permission scope: %s", err, scopePermission)
	}
	return nil
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
	case "outside-user":
		repoName, userLogin, err := validate.ParseRepoUserPair(resourceName)
		if err != nil {
			return err
		}
		user, _, err := client.Users.Get(ctx, userLogin)
		if err != nil {
			return fmt.Errorf("ユーザー情報の取得に失敗しました: %w", err)
		}
		if err := store.UpsertRepoUser(db, repoName, user); err != nil {
			return fmt.Errorf("リポジトリユーザー情報の保存に失敗しました: %w", err)
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
	case "outside-user", "repos-user":
		repoName, userLogin, err := validate.ParseRepoUserPair(resourceName)
		if err != nil {
			return err
		}
		return store.DeleteRepoUser(db, repoName, userLogin)
	default:
		return fmt.Errorf("サポートされていない削除対象: %s", target)
	}
}
