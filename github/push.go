package github

import (
	"context"
	"fmt"
	"strings"

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
