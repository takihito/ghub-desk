package github

import (
	"context"
	"fmt"
	"net/http"

	"ghub-desk/session"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"

	"ghub-desk/config"
)

// loggingTransport wraps an http.RoundTripper to log requests.
type loggingTransport struct {
	transport http.RoundTripper
}

// RoundTrip logs the request and delegates to the wrapped transport.
func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	session.Debugf("API: %s %s", req.Method, req.URL)
	return t.transport.RoundTrip(req)
}

// InitClient initializes and returns a GitHub client based on the provided configuration.
func InitClient(cfg *config.Config) (*github.Client, error) {
	patConfigured := cfg.GitHubToken != ""
	appConfigured := cfg.GitHubApp.AppID != 0 && cfg.GitHubApp.InstallationID != 0 && cfg.GitHubApp.PrivateKey != ""

	var httpClient *http.Client

	if appConfigured {
		// Use GitHub App authentication
		baseTransport := http.DefaultTransport
		tr, err := ghinstallation.New(baseTransport, cfg.GitHubApp.AppID, cfg.GitHubApp.InstallationID, []byte(cfg.GitHubApp.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to create github app transport: %w", err)
		}
		var transport http.RoundTripper = tr
		if config.Debug {
			transport = &loggingTransport{transport: transport}
		}
		httpClient = &http.Client{Transport: transport}
	} else if patConfigured {
		// Use Personal Access Token authentication
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: cfg.GitHubToken},
		)
		tc := oauth2.NewClient(context.Background(), ts)
		if config.Debug {
			baseTransport := tc.Transport
			if baseTransport == nil {
				baseTransport = http.DefaultTransport
			}
			tc.Transport = &loggingTransport{transport: baseTransport}
		}
		httpClient = tc
	} else {
		return nil, fmt.Errorf("no valid authentication method found in configuration")
	}

	return github.NewClient(httpClient), nil
}
