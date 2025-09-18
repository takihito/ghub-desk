package cmd

import (
	"fmt"

	"ghub-desk/config"

	"gopkg.in/yaml.v3"
)

// ShowSettings loads application settings and prints a masked YAML to stdout.
func ShowSettings(cli *CLI) error {
	// Use shared loader without validation. It errors only when a custom --config is invalid.
	cfg, err := config.LoadConfigNoValidate(cli.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	out, err := renderMaskedConfigYAML(cfg)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

// renderMaskedConfigYAML returns YAML of config with secrets masked.
func renderMaskedConfigYAML(cfg *config.Config) (string, error) {
	safe := struct {
		Organization string `yaml:"organization"`
		GitHubToken  string `yaml:"github_token"`
		GitHubApp    struct {
			AppID          int64  `yaml:"app_id"`
			InstallationID int64  `yaml:"installation_id"`
			PrivateKey     string `yaml:"private_key"`
		} `yaml:"github_app"`
		MCP struct {
			AllowPull  bool `yaml:"allow_pull"`
			AllowWrite bool `yaml:"allow_write"`
		} `yaml:"mcp"`
		DatabasePath string `yaml:"database_path"`
	}{
		Organization: cfg.Organization,
		GitHubToken:  maskSecret(cfg.GitHubToken),
	}
	safe.GitHubApp.AppID = cfg.GitHubApp.AppID
	safe.GitHubApp.InstallationID = cfg.GitHubApp.InstallationID
	if cfg.GitHubApp.PrivateKey != "" {
		safe.GitHubApp.PrivateKey = "[masked PEM]"
	}
	safe.MCP.AllowPull = cfg.MCP.AllowPull
	safe.MCP.AllowWrite = cfg.MCP.AllowWrite
	safe.DatabasePath = cfg.DatabasePath

	b, err := yaml.Marshal(&safe)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}
	return string(b), nil
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	// Keep last 4 characters if reasonably long, else mask fully
	if len(s) > 8 {
		return "[masked]…" + s[len(s)-4:]
	}
	return "[masked]"
}
