package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SessionRef persists the claude CLI session id for a single workflow run so
// subsequent LLM steps can resume the same conversation (--resume <id>) instead
// of starting cold. Shared across every LLM step in one run regardless of
// whether the workflow includes a design step.
type SessionRef struct {
	SessionID string `json:"session_id"`
}

// SessionPath returns the on-disk path for the run-scoped session file:
// <repoDir>/.tomato/session.json
func SessionPath(repoDir string) string {
	return filepath.Join(repoDir, ".tomato", "session.json")
}

// LoadSession reads the persisted session id. A missing or malformed file
// yields an empty SessionRef and no error — callers treat an empty SessionID as
// "no session yet, start a new one".
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

// SaveSession persists the session id so the next LLM step in the same run can
// resume it.
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

// ClearSession removes any persisted session so the next run starts fresh
// rather than resuming a stale session from a prior run.
func ClearSession(repoDir string) error {
	path := SessionPath(repoDir)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
