package cmd

import (
    "fmt"

    "ghub-desk/config"
    "gopkg.in/yaml.v3"
)

// ShowAppConfig loads application config and prints a masked YAML to stdout.
func ShowAppConfig(cli *CLI) error {
    cfg, err := cli.Config()
    if err != nil {
        return fmt.Errorf("configuration error: %w", err)
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
    }{
        Organization: cfg.Organization,
        GitHubToken:  maskSecret(cfg.GitHubToken),
    }
    safe.GitHubApp.AppID = cfg.GitHubApp.AppID
    safe.GitHubApp.InstallationID = cfg.GitHubApp.InstallationID
    if cfg.GitHubApp.PrivateKey != "" {
        safe.GitHubApp.PrivateKey = "[masked PEM]"
    }

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

