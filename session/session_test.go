package session

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRemovePullSession(t *testing.T) {
	origPath := Path()
	t.Cleanup(func() {
		SetPath(origPath)
	})

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "session.json")
	SetPath(targetPath)

	key := "users|store:true|stdout:false|interval:1s"
	ps := NewPullSession(key, "users")
	ps.Store = true
	ps.Stdout = false
	ps.Interval = "1s"
	ps.Metadata = map[string]string{"phase": "init"}

	if err := SavePull(ps); err != nil {
		t.Fatalf("SavePull failed: %v", err)
	}

	loaded, err := LoadPull(key)
	if err != nil {
		t.Fatalf("LoadPull failed: %v", err)
	}

	if loaded.Target != "users" || !loaded.Store || loaded.Stdout {
		t.Fatalf("loaded session does not match: %+v", loaded)
	}

	rec := NewProgressRecorder(ps)
	meta := map[string]string{"phase": "users"}
	if err := rec.Start("users", meta, 0, 0); err != nil {
		t.Fatalf("recorder start failed: %v", err)
	}
	if err := rec.Page("users", meta, 2, 80); err != nil {
		t.Fatalf("recorder page failed: %v", err)
	}

	reloaded, err := LoadPull(key)
	if err != nil {
		t.Fatalf("LoadPull after record failed: %v", err)
	}
	if reloaded.LastPage != 2 || reloaded.FetchedCount != 80 {
		t.Fatalf("progress was not persisted: %+v", reloaded)
	}
	if reloaded.Endpoint != "users" {
		t.Fatalf("endpoint not stored, got %q", reloaded.Endpoint)
	}
	if reloaded.Metadata["phase"] != "users" {
		t.Fatalf("metadata not stored correctly: %+v", reloaded.Metadata)
	}

	if err := RemovePull(key); err != nil {
		t.Fatalf("RemovePull failed: %v", err)
	}

	if _, err := LoadPull(key); err == nil {
		t.Fatalf("expected ErrNotFound after removal")
	}
}

func TestPathUsesDatabaseDirectory(t *testing.T) {
	origPath := Path()
	t.Cleanup(func() {
		SetPath(origPath)
	})

	tmpDir := t.TempDir()
	custom := filepath.Join(tmpDir, "custom", "session.json")
	SetPath(custom)

	expected := filepath.Clean(custom)
	if got := Path(); got != expected {
		t.Fatalf("Path() = %q, want %q", got, expected)
	}
}
