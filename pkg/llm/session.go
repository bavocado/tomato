package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SessionRef is the persisted claude session id for a repo, used to share one
// claude session across all LLM steps in a `tomato run` (see NewLLMStream).
type SessionRef struct {
	SessionID string `json:"session_id"`
}

// SessionPath returns the on-disk path for the run-scoped session file:
// <repoDir>/.tomato/session.json
func SessionPath(repoDir string) string {
	return filepath.Join(repoDir, ".tomato", "session.json")
}

// LoadSession reads the persisted session id. A missing or malformed file
// yields an empty SessionRef (caller starts a fresh session).
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

// SaveSession persists the session id so the next LLM step can resume it.
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

// ClearSession removes any persisted session, forcing the next step to start a
// fresh claude session (used at the start of each `tomato run`).
func ClearSession(repoDir string) error {
	path := SessionPath(repoDir)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
