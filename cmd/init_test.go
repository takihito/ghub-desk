package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"ghub-desk/store"
)

func TestInitDBCmdCreatesDatabaseAtExplicitPath(t *testing.T) {
	dir := t.TempDir()

	dbPath := filepath.Join(dir, "custom.db")
	defer store.SetDBPath("")

	cmd := InitDBCmd{TargetFile: dbPath}
	if err := cmd.Run(&CLI{}); err != nil {
		t.Fatalf("InitDBCmd.Run returned error: %v", err)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected database file at %s: %v", dbPath, err)
	}

	infoBefore, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("failed to stat db file: %v", err)
	}

	if err := cmd.Run(&CLI{}); err != nil {
		t.Fatalf("InitDBCmd.Run on existing file returned error: %v", err)
	}

	infoAfter, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("failed to stat db file after second run: %v", err)
	}

	if !infoAfter.ModTime().Equal(infoBefore.ModTime()) {
		t.Fatalf("expected db file not to be modified; before=%v after=%v", infoBefore.ModTime(), infoAfter.ModTime())
	}
}

func TestInitDBCmdUsesConfigDatabasePath(t *testing.T) {
	dir := t.TempDir()

	configPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "nested", "config.db")
	configContent := "database_path: \"" + dbPath + "\"\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	defer store.SetDBPath("")

	cmd := InitDBCmd{}
	cli := &CLI{ConfigPath: configPath}
	if err := cmd.Run(cli); err != nil {
		t.Fatalf("InitDBCmd.Run returned error: %v", err)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected database file via config at %s: %v", dbPath, err)
	}
}

func TestInitConfigCmdCreatesConfigAndSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "config.yaml")

	cmd := InitConfigCmd{TargetFile: target}
	if err := cmd.Run(&CLI{}); err != nil {
		t.Fatalf("InitConfigCmd.Run returned error: %v", err)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read generated config: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("generated config file was empty")
	}

	infoBefore, err := os.Stat(target)
	if err != nil {
		t.Fatalf("failed to stat generated config: %v", err)
	}

	if err := cmd.Run(&CLI{}); err != nil {
		t.Fatalf("InitConfigCmd.Run on existing file returned error: %v", err)
	}

	infoAfter, err := os.Stat(target)
	if err != nil {
		t.Fatalf("failed to stat config after second run: %v", err)
	}

	if !infoAfter.ModTime().Equal(infoBefore.ModTime()) {
		t.Fatalf("expected config file not to be modified; before=%v after=%v", infoBefore.ModTime(), infoAfter.ModTime())
	}
}
