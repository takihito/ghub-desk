package cmd

import (
    "fmt"
    "os"
    "strconv"

    "ghub-desk/config"

    "gopkg.in/yaml.v3"
)

// ShowSettings loads application settings and prints a masked YAML to stdout.
func ShowSettings(cli *CLI) error {
	cfg, err := loadConfigForView(cli.ConfigPath)
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

// loadConfigForView loads config from file and environment WITHOUT validation.
func loadConfigForView(customPath string) (*config.Config, error) {
    cfg := &config.Config{}

    // Resolve path via shared helper
    var configPath string
    isCustom := customPath != ""
    p, _ := config.ResolveConfigPath(customPath)
    configPath = p

    if configPath != "" {
        data, err := os.ReadFile(configPath)
        if err != nil {
            if isCustom {
                return nil, fmt.Errorf("config file not found: %s", configPath)
            }
        } else {
            expanded := os.ExpandEnv(string(data))
            if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil && isCustom {
                return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
            }
        }
    }

	// Overlay env vars (best-effort; ignore parse errors)
	if v := os.Getenv("GHUB_DESK_ORGANIZATION"); v != "" {
		cfg.Organization = v
	}
	if v := os.Getenv("GHUB_DESK_GITHUB_TOKEN"); v != "" {
		cfg.GitHubToken = v
	}
	if v := os.Getenv("GHUB_DESK_APP_ID"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.GitHubApp.AppID = n
		}
	}
	if v := os.Getenv("GHUB_DESK_INSTALLATION_ID"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.GitHubApp.InstallationID = n
		}
	}
	if v := os.Getenv("GHUB_DESK_PRIVATE_KEY"); v != "" {
		cfg.GitHubApp.PrivateKey = v
	}

	return cfg, nil
}
