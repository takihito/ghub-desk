package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrNotFound indicates that the requested session entry was not found.
var ErrNotFound = errors.New("session entry not found")

// pullFileState represents the serialized session.json contents.
type pullFileState struct {
	Pull map[string]*PullSession `json:"pull"`
}

// PullSession describes the persisted state for an in-flight pull command.
type PullSession struct {
	Key          string            `json:"key"`
	Target       string            `json:"target"`
	Endpoint     string            `json:"endpoint"`
	LastPage     int               `json:"last_page"`
	FetchedCount int               `json:"fetched_count"`
	Store        bool              `json:"store"`
	Stdout       bool              `json:"stdout"`
	Interval     string            `json:"interval"`
	TableCleared bool              `json:"table_cleared"`
	TeamSlug     string            `json:"team_slug,omitempty"`
	RepoName     string            `json:"repo_name,omitempty"`
	UserLogin    string            `json:"user_login,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	UpdatedAt    string            `json:"updated_at"`
}

var (
	stateMutex sync.Mutex

	pathMutex  sync.RWMutex
	customPath string
)

// Path returns the location of the session file. It uses a custom override when set,
// falling back to the default configuration directory.
func Path() string {
	pathMutex.RLock()
	override := customPath
	pathMutex.RUnlock()
	if override != "" {
		return override
	}
	return defaultSessionPath()
}

// NewPullSession constructs a new pull session with the provided key and target.
func NewPullSession(key, target string) *PullSession {
	return &PullSession{
		Key:      key,
		Target:   target,
		Metadata: map[string]string{},
	}
}

// Clone returns a deep copy of the session for safe persistence.
func (ps *PullSession) Clone() *PullSession {
	cloned := *ps
	if ps.Metadata != nil {
		cloned.Metadata = make(map[string]string, len(ps.Metadata))
		for k, v := range ps.Metadata {
			cloned.Metadata[k] = v
		}
	}
	return &cloned
}

// SavePull stores or updates the pull session for the given key.
func SavePull(ps *PullSession) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	state, err := loadState()
	if err != nil {
		return err
	}

	if state.Pull == nil {
		state.Pull = make(map[string]*PullSession)
	}
	ps.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	state.Pull[ps.Key] = ps.Clone()

	return saveState(state)
}

// LoadPull retrieves the stored pull session for the provided key.
func LoadPull(key string) (*PullSession, error) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	state, err := loadState()
	if err != nil {
		return nil, err
	}

	session, ok := state.Pull[key]
	if !ok {
		return nil, ErrNotFound
	}
	return session.Clone(), nil
}

// RemovePull deletes the stored session associated with the key.
func RemovePull(key string) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	state, err := loadState()
	if err != nil {
		return err
	}

	if state.Pull != nil {
		delete(state.Pull, key)
	}

	return saveState(state)
}

func loadState() (*pullFileState, error) {
	path := Path()
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return &pullFileState{Pull: make(map[string]*PullSession)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	if len(content) == 0 {
		return &pullFileState{Pull: make(map[string]*PullSession)}, nil
	}

	var state pullFileState
	if err := json.Unmarshal(content, &state); err != nil {
		return nil, fmt.Errorf("failed to decode session file: %w", err)
	}

	if state.Pull == nil {
		state.Pull = make(map[string]*PullSession)
	}

	return &state, nil
}

func saveState(state *pullFileState) error {
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to ensure session directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode session file: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temporary session file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("failed to replace session file: %w", err)
	}
	return nil
}

// SetPath overrides the session file path. Empty string resets to the default location.
func SetPath(path string) {
	pathMutex.Lock()
	defer pathMutex.Unlock()
	if path == "" {
		customPath = ""
		return
	}
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		if abs, err := filepath.Abs(cleaned); err == nil {
			customPath = abs
			return
		}
	}
	customPath = cleaned
}

func defaultSessionPath() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".config", "ghub-desk", "session.json")
	}
	return "session.json"
}
