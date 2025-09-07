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
