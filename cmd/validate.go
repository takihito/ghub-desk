package cmd

import (
	"fmt"
	"strings"

	v "ghub-desk/validate"
)

func validateUserName(s string) error {
	if err := v.ValidateUserName(s); err != nil {
		return fmt.Errorf("ユーザー名が不正です: 1〜39文字の英数字とハイフンのみ、先頭・末尾のハイフン不可 (%v)", err)
	}
	return nil
}

func validateTeamName(s string) error { // team slug
	if err := v.ValidateTeamSlug(s); err != nil {
		return fmt.Errorf("チーム名(スラグ)が不正です: 小文字英数字とハイフンのみ、先頭・末尾のハイフン不可、長さは1〜100文字 (%v)", err)
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
