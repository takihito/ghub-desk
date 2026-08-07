package cmd

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"ghub-desk/config"
	"ghub-desk/store"
)

// InitCmd groups the init subcommands
type InitCmd struct {
	DB     InitDBCmd     `cmd:"" help:"Initialize the SQLite database tables"`
	Config InitConfigCmd `cmd:"" help:"Generate a config.yaml skeleton"`
}

// InitDBCmd initializes the database schema
type InitDBCmd struct {
	TargetFile string `name:"target-file" type:"path" help:"SQLite database file path to initialize (e.g. ~/data/ghub-desk.db)"`
}

// InitConfigCmd creates a configuration file from the example template
type InitConfigCmd struct {
	TargetFile string `name:"target-file" type:"path" help:"Destination config file path (default: ~/.ghub-desk/config.yaml)"`
}

//go:embed config_template.yaml
var exampleConfigTemplate string

// expandUserPath expands a leading ~ to the current user's home directory.
func expandUserPath(p string) (string, error) {
	if p == "" || p[0] != '~' {
		return p, nil
	}
	if len(p) > 1 && p[1] != '/' && p[1] != '\\' {
		return "", fmt.Errorf("cannot expand user-specific home in path %q", p)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}
	if len(p) == 1 {
		return home, nil
	}
	return filepath.Join(home, p[2:]), nil
}

// Run implements the init db subcommand execution
func (i *InitDBCmd) Run(cli *CLI) error {
	var (
		dbPath   string
		explicit bool
	)

	if i.TargetFile != "" {
		expanded, err := expandUserPath(i.TargetFile)
		if err != nil {
			return err
		}
		dbPath = filepath.Clean(expanded)
		explicit = true
	} else {
		configPath := cli.ConfigPath
		if configPath != "" {
			expanded, err := expandUserPath(configPath)
			if err != nil {
				return err
			}
			configPath = expanded
		}

		cfgNV, err := config.LoadConfigNoValidate(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if cfgNV.DatabasePath != "" {
			expanded, err := expandUserPath(cfgNV.DatabasePath)
			if err != nil {
				return err
			}
			dbPath = filepath.Clean(expanded)
		}
	}

	if explicit {
		if _, err := os.Stat(dbPath); err == nil {
			fmt.Fprintf(os.Stderr, "Database file already exists at %s; skipping\n", dbPath)
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to check database file: %w", err)
		}
	}

	if dbPath != "" {
		if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}
		store.SetDBPath(dbPath)
	} else {
		store.SetDBPath("")
	}

	selectedPath := store.DBFileName
	if store.DBPath != "" {
		selectedPath = store.DBPath
	}

	db, err := store.InitDatabase()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	fmt.Printf("Database initialization completed (path: %s)\n", selectedPath)
	return nil
}

// Run implements the init config subcommand execution
func (i *InitConfigCmd) Run(cli *CLI) error {
	targetPath := i.TargetFile
	if targetPath == "" {
		targetPath = cli.ConfigPath
	}
	if targetPath == "" {
		resolved, err := config.ResolveConfigPath("")
		if err != nil {
			return fmt.Errorf("failed to resolve default config path: %w", err)
		}
		targetPath = resolved
	}

	expanded, err := expandUserPath(targetPath)
	if err != nil {
		return err
	}
	targetPath = filepath.Clean(expanded)

	if _, err := os.Stat(targetPath); err == nil {
		fmt.Fprintf(os.Stderr, "Config file already exists at %s; skipping\n", targetPath)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to check config file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(targetPath, []byte(exampleConfigTemplate), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Config file created at %s\n", targetPath)
	return nil
}
