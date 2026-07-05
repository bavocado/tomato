package llm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ref := SessionRef{SessionID: "abc-123"}

	if err := SaveSession(dir, ref); err != nil {
		t.Fatal(err)
	}

	loaded := LoadSession(dir)
	if loaded.SessionID != "abc-123" {
		t.Errorf("expected abc-123, got %q", loaded.SessionID)
	}
}

func TestLoadSessionMissingReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	loaded := LoadSession(dir)
	if loaded.SessionID != "" {
		t.Errorf("expected empty SessionID for missing file, got %q", loaded.SessionID)
	}
}

func TestLoadSessionMalformedReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(SessionPath(dir), []byte("not json"), 0644)
	loaded := LoadSession(dir)
	if loaded.SessionID != "" {
		t.Errorf("expected empty for malformed file, got %q", loaded.SessionID)
	}
}

func TestClearSession(t *testing.T) {
	dir := t.TempDir()
	SaveSession(dir, SessionRef{SessionID: "x"})
	if err := ClearSession(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".tomato", "session.json")); !os.IsNotExist(err) {
		t.Errorf("expected session file removed, got err=%v", err)
	}
}

func TestClearSessionMissingIsNoop(t *testing.T) {
	dir := t.TempDir()
	if err := ClearSession(dir); err != nil {
		t.Errorf("clearing a missing session should be a no-op, got %v", err)
	}
}
