package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"

	"ghub-desk/config"
)

// InitClient initializes and returns a GitHub client based on the provided configuration.
func InitClient(cfg *config.Config) (*github.Client, error) {
	patConfigured := cfg.GitHubToken != ""
	appConfigured := cfg.GitHubApp.AppID != 0 && cfg.GitHubApp.InstallationID != 0 && cfg.GitHubApp.PrivateKey != ""

	if appConfigured {
		// Use GitHub App authentication
		tr, err := ghinstallation.New(http.DefaultTransport, cfg.GitHubApp.AppID, cfg.GitHubApp.InstallationID, []byte(cfg.GitHubApp.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to create github app transport: %w", err)
		}
		return github.NewClient(&http.Client{Transport: tr}), nil
	} else if patConfigured {
		// Use Personal Access Token authentication
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: cfg.GitHubToken},
		)
		tc := oauth2.NewClient(context.Background(), ts)
		return github.NewClient(tc), nil
	}

	return nil, fmt.Errorf("no valid authentication method found in configuration")
}