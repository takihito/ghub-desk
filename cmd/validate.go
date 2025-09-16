package cmd

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// Username: 1-39 chars, alphanumeric or hyphen, cannot start/end with hyphen
	reUser = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)
	// Team slug: lowercase alphanumeric or hyphen, cannot start/end with hyphen
	reTeam = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)
)

func validateUserName(s string) error {
	s = strings.TrimSpace(s)
	if !reUser.MatchString(s) {
		return fmt.Errorf("ユーザー名が不正です: 1〜39文字の英数字とハイフンのみ、先頭・末尾のハイフン不可")
	}
	return nil
}

func validateTeamName(s string) error {
	s = strings.TrimSpace(s)
	if len(s) == 0 || len(s) > 100 || !reTeam.MatchString(s) {
		return fmt.Errorf("チーム名(スラグ)が不正です: 小文字英数字とハイフンのみ、先頭・末尾のハイフン不可、長さは1〜100文字")
	}
	return nil
}

func validateTeamUserPair(s string) (team string, user string, err error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("チーム/ユーザー形式が正しくありません。{team_slug}/{user_name} の形式で指定してください")
	}
	team = strings.TrimSpace(parts[0])
	user = strings.TrimSpace(parts[1])
	if err := validateTeamName(team); err != nil {
		return "", "", fmt.Errorf("team-userのチーム名が不正です: %w", err)
	}
	if err := validateUserName(user); err != nil {
		return "", "", fmt.Errorf("team-userのユーザー名が不正です: %w", err)
	}
	return team, user, nil
}
