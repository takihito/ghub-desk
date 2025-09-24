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
