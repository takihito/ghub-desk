package cmd

import (
	"fmt"
	"strings"

	v "ghub-desk/validate"
)

func validateUserName(s string) error {
	if err := v.ValidateUserName(s); err != nil {
		return fmt.Errorf("invalid username: (%w)", err)
	}
	return nil
}

func validateUserLogin(s string) error {
	return validateUserName(strings.TrimSpace(s))
}

func validateTeamName(s string) error { // team slug
	if err := v.ValidateTeamSlug(s); err != nil {
		return fmt.Errorf("invalid team name (slug): (%w)", err)
	}
	return nil
}

func validateTeamUserPair(s string) (team string, user string, err error) {
	// Keep user-facing error while delegating parsing/validation
	team, user, err = v.ParseTeamUserPair(strings.TrimSpace(s))
	if err != nil {
		return "", "", fmt.Errorf("invalid team/user format. Please specify in the format {team_slug}/{user_name} (%v)", err)
	}
	return team, user, nil
}

func validateRepoName(s string) error {
	if err := v.ValidateRepoName(s); err != nil {
		return fmt.Errorf("invalid repository name: (%w)", err)
	}
	return nil
}

func validateRepoUserPair(s string) (repo string, user string, err error) {
	repo, user, err = v.ParseRepoUserPair(strings.TrimSpace(s))
	if err != nil {
		return "", "", fmt.Errorf("invalid repository/user format. Please specify in the format {repository}/{user_name} (%v)", err)
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
		return "", fmt.Errorf("invalid permission for outside collaborator: choose from pull, push, admin (aliases: read, write)")
	}
}
