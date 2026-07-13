package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SessionRef is legacy state from the old shared claude-session flow.
// Runtime code clears this file and no longer resumes it.
type SessionRef struct {
	SessionID string `json:"session_id"`
}

// SessionPath returns the on-disk path for the run-scoped session file:
// <repoDir>/.tomato/session.json
func SessionPath(repoDir string) string {
	return filepath.Join(repoDir, ".tomato", "session.json")
}

// LoadSession reads legacy persisted session state. A missing or malformed file
// yields an empty SessionRef.
func LoadSession(repoDir string) SessionRef {
	data, err := os.ReadFile(SessionPath(repoDir))
	if err != nil {
		return SessionRef{}
	}
	var ref SessionRef
	if err := json.Unmarshal(data, &ref); err != nil {
		return SessionRef{}
	}
	return ref
}

// SaveSession persists legacy session state. New runtime code should not call
// this; it remains for compatibility with old tests/tools.
func SaveSession(repoDir string, ref SessionRef) error {
	path := SessionPath(repoDir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating session dir: %w", err)
	}
	data, err := json.MarshalIndent(ref, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ClearSession removes any persisted legacy session.
func ClearSession(repoDir string) error {
	path := SessionPath(repoDir)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
