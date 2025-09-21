package validate

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Patterns and length constraints are exported for reuse (e.g., JSON Schema).
const (
	UserNamePattern = "^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$"
	TeamSlugPattern = "^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$"

	UserNameMin = 1
	UserNameMax = 39

	TeamSlugMin = 1
	TeamSlugMax = 100
)

var (
	reUser = regexp.MustCompile(UserNamePattern)
	reTeam = regexp.MustCompile(TeamSlugPattern)
)

// Sentinel errors for classification by callers.
var (
	ErrInvalidUserName = errors.New("invalid username")
	ErrInvalidTeamSlug = errors.New("invalid team slug")
	ErrInvalidPair     = errors.New("invalid team/user pair")
)

// ValidateUserName checks GitHub username rule: 1-39 chars, alnum or hyphen,
// cannot start/end with hyphen.
func ValidateUserName(s string) error {
	s = strings.TrimSpace(s)
	if len(s) < UserNameMin || len(s) > UserNameMax || !reUser.MatchString(s) {
		return fmt.Errorf("%w: 1-39 chars alnum or hyphen, no leading/trailing hyphen", ErrInvalidUserName)
	}
	return nil
}

// ValidateTeamSlug checks team slug rule: lowercase alnum or hyphen,
// cannot start/end with hyphen. Length 1-100.
func ValidateTeamSlug(s string) error {
	s = strings.TrimSpace(s)
	if len(s) < TeamSlugMin || len(s) > TeamSlugMax || !reTeam.MatchString(s) {
		return fmt.Errorf("%w: lowercase alnum and hyphen only, no leading/trailing hyphen, length 1-100", ErrInvalidTeamSlug)
	}
	return nil
}

// ParseTeamUserPair parses "{team_slug}/{user_name}" and validates both parts.
func ParseTeamUserPair(s string) (team string, user string, err error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("%w: expected {team_slug}/{user_name}", ErrInvalidPair)
	}
	team = strings.TrimSpace(parts[0])
	user = strings.TrimSpace(parts[1])
	if err := ValidateTeamSlug(team); err != nil {
		return "", "", fmt.Errorf("%w: team slug invalid: %v", ErrInvalidPair, err)
	}
	if err := ValidateUserName(user); err != nil {
		return "", "", fmt.Errorf("%w: user name invalid: %v", ErrInvalidPair, err)
	}
	return team, user, nil
}
