package cmd

import (
	"fmt"
	"strings"

	v "ghub-desk/validate"
)

func validateUserName(s string) error {
	if err := v.ValidateUserName(s); err != nil {
		return fmt.Errorf("ユーザー名が不正です: (%w)", err)
	}
	return nil
}

func validateUserLogin(s string) error {
	return validateUserName(strings.TrimSpace(s))
}

func validateTeamName(s string) error { // team slug
	if err := v.ValidateTeamSlug(s); err != nil {
		return fmt.Errorf("チーム名(スラグ)が不正です: (%w)", err)
	}
	return nil
}

func validateTeamUserPair(s string) (team string, user string, err error) {
	// Keep Japanese user-facing error while delegating parsing/validation
	team, user, err = v.ParseTeamUserPair(strings.TrimSpace(s))
	if err != nil {
		return "", "", fmt.Errorf("チーム/ユーザー形式が正しくありません。{team_slug}/{user_name} の形式で指定してください (%v)", err)
	}
	return team, user, nil
}

func validateRepoName(s string) error {
	if err := v.ValidateRepoName(s); err != nil {
		return fmt.Errorf("リポジトリ名が不正です: (%w)", err)
	}
	return nil
}

func validateRepoUserPair(s string) (repo string, user string, err error) {
	repo, user, err = v.ParseRepoUserPair(strings.TrimSpace(s))
	if err != nil {
		return "", "", fmt.Errorf("リポジトリ/ユーザー形式が正しくありません。{repository}/{user_name} の形式で指定してください (%v)", err)
	}
	return repo, user, nil
}

func validateOutsidePermission(s string) (string, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", nil
	}
	switch val := strings.ToLower(trimmed); val {
	case "pull", "push", "admin":
		return val, nil
	case "read":
		return "pull", nil
	case "write":
		return "push", nil
	default:
		return "", fmt.Errorf("外部コラボレーターの権限が不正です: pull, push, admin（エイリアス: read, write）から選択してください")
	}
}
