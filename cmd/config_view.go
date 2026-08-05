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
	b, err := yaml.Marshal(config.Mask(cfg))
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}
	return string(b), nil
}
