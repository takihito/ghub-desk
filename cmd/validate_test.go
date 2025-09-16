package cmd

import (
    "strings"
    "testing"
)

func TestValidateUserName(t *testing.T) {
    ok := []string{"a", "abc", "user-123", "A3", "Name9-9"}
    ng := []string{"a@b", "user name", "", strings.Repeat("x", 40), "-abc", "abc-", "under_score"}

    for _, s := range ok {
        if err := validateUserName(s); err != nil {
            t.Errorf("want ok, got err for %q: %v", s, err)
        }
    }
    for _, s := range ng {
        if err := validateUserName(s); err == nil {
            t.Errorf("want err, got ok for %q", s)
        }
    }
}

func TestValidateTeamName(t *testing.T) {
    ok := []string{"a", "team123", "team-name", strings.Repeat("a", 50), strings.Repeat("a", 100)}
    ng := []string{"bad team", "bad$team", strings.Repeat("Z", 3), "Team", "-abc", "abc-", "team_name", strings.Repeat("a", 101)}

    for _, s := range ok {
        if err := validateTeamName(s); err != nil {
            t.Errorf("want ok, got err for %q: %v", s, err)
        }
    }
    for _, s := range ng {
        if err := validateTeamName(s); err == nil {
            t.Errorf("want err, got ok for %q", s)
        }
    }
}

func TestValidateTeamUserPair(t *testing.T) {
    if team, user, err := validateTeamUserPair("good-team/user-ok"); err != nil {
        t.Fatalf("unexpected err: %v", err)
    } else if team != "good-team" || user != "user-ok" {
        t.Fatalf("unexpected parts: %s/%s", team, user)
    }

    cases := []string{
        "no-slash",
        "bad team/user",
        "team/bad$user",
        "-team/user",
        "team/-user",
        "Team/user",             // uppercase invalid for team slug
        "good-team/" + strings.Repeat("u", 40), // user too long (40)
        "good-team/user-",       // user ends with hyphen
    }
    for _, c := range cases {
        if _, _, err := validateTeamUserPair(c); err == nil {
            t.Errorf("want err for %q", c)
        }
    }
}
