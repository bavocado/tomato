package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadState(t *testing.T) {
	dir := t.TempDir()
	s := WorkflowState{
		Workflow:       "default",
		Feature:        "login",
		CurrentStep:    "review_loop",
		FailedStep:     "review_loop",
		CompletedSteps: []string{"spec", "design", "impl", "pr"},
		LastRunID:      "2026-06-30-abc123",
	}

	if err := Save(dir, s); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir, "default", "login")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Workflow != "default" || loaded.Feature != "login" {
		t.Fatalf("wrong state loaded: %#v", loaded)
	}
	if loaded.FailedStep != "review_loop" {
		t.Fatalf("expected failed step review_loop, got %s", loaded.FailedStep)
	}
	if len(loaded.CompletedSteps) != 4 {
		t.Fatalf("expected 4 completed steps, got %d", len(loaded.CompletedSteps))
	}
	if loaded.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be set")
	}
}

func TestLoadMissingState(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir, "default", "login")
	if err == nil {
		t.Fatal("expected missing state error")
	}
}

func TestClearState(t *testing.T) {
	dir := t.TempDir()
	s := WorkflowState{Workflow: "default", Feature: "login", FailedStep: "design"}
	if err := Save(dir, s); err != nil {
		t.Fatal(err)
	}
	if err := Clear(dir, "default", "login"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".tomato", "state", "default-login.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected state file removed, err=%v", err)
	}
}

func TestStatePathSanitizesNames(t *testing.T) {
	dir := t.TempDir()
	path := Path(dir, "my/workflow", "feature/name")
	if filepath.Base(path) != "my-workflow-feature-name.json" {
		t.Fatalf("unexpected state path: %s", path)
	}
}

func TestSaveCreatesStateDirectory(t *testing.T) {
	dir := t.TempDir()
	s := WorkflowState{Workflow: "default", Feature: "f", CurrentStep: "spec"}
	if err := Save(dir, s); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".tomato", "state")); err != nil {
		t.Fatalf("expected state dir: %v", err)
	}
}