package github

import (
	"context"

	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
)

// InitClient creates and configures a GitHub API client with OAuth2 authentication
func InitClient(token string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc)
}
