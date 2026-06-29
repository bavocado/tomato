package steps

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bavocado/tomato/pkg/adapter"
)

// fakeAdapter writes a bash script that records its argv and stdin to logPath
// and prints the given JSON to stdout. It returns the script path for use as a
// Bridge.Bin. Skips the test on platforms without /bin/sh.
func fakeAdapter(t *testing.T, dir, logPath, stdoutJSON string) string {
	t.Helper()
	script := filepath.Join(dir, "fake-adapter.sh")
	body := "#!/bin/sh\n" +
		"echo \"argv: $@\" >> '" + logPath + "'\n" +
		"cat >> '" + logPath + "'\n" +
		"echo >> '" + logPath + "'\n" +
		"cat <<'JSON'\n" + stdoutJSON + "\nJSON\n"
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return script
}

func newGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@t.com"},
		{"config", "user.name", "T"},
		{"checkout", "-b", "feature-x"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	// One commit so HEAD/branch resolve.
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0644)
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Run()
	}
}

func TestRunPRUsesAdapterAndWritesRefs(t *testing.T) {
	repo := t.TempDir()
	newGitRepo(t, repo)
	featureDir := filepath.Join(repo, "docs", "specs", "current-feature")
	os.MkdirAll(featureDir, 0755)

	logPath := filepath.Join(repo, "adapter.log")
	bin := fakeAdapter(t, repo, logPath, `{"pr_ref":"42","url":"https://example.com/pr/42"}`)

	reg := adapter.NewRegistry()
	reg.Set("pr", &adapter.Bridge{Bin: bin})

	cfg := &StepConfig{RepoDir: repo, FeatureDir: featureDir, Feature: "current-feature", Adapters: reg}
	result := runPR(cfg, nil)
	if !result.Success {
		t.Fatalf("runPR failed: %s", result.Error)
	}

	logData, _ := os.ReadFile(logPath)
	log := string(logData)
	if !strings.Contains(log, "argv: create-pr") {
		t.Errorf("adapter not invoked with create-pr; log:\n%s", log)
	}
	if !strings.Contains(log, `"branch":"feature-x"`) {
		t.Errorf("adapter stdin missing branch; log:\n%s", log)
	}
	// The PR body must carry the Tomato signature (chain-of-provenance).
	if !strings.Contains(log, "Tomato-Parent:") {
		t.Errorf("create-pr payload missing Tomato signature; log:\n%s", log)
	}

	// pr.md and pr.json must be written with the returned ref.
	if _, err := os.Stat(filepath.Join(featureDir, "pr.md")); err != nil {
		t.Errorf("pr.md not written: %v", err)
	}
	ref := ReadPRRef(featureDir)
	if ref.PRRef != "42" {
		t.Errorf("pr.json pr_ref = %q, want 42", ref.PRRef)
	}
	if ref.Branch != "feature-x" {
		t.Errorf("pr.json branch = %q, want feature-x", ref.Branch)
	}
}

func TestRunPRNoAdapterFails(t *testing.T) {
	repo := t.TempDir()
	newGitRepo(t, repo)
	featureDir := filepath.Join(repo, "docs", "specs", "current-feature")
	os.MkdirAll(featureDir, 0755)

	cfg := &StepConfig{RepoDir: repo, FeatureDir: featureDir, Feature: "f", Adapters: adapter.NewRegistry()}
	result := runPR(cfg, nil)
	if result.Success {
		t.Error("expected failure when no 'pr' adapter configured")
	}
	if !strings.Contains(result.Error, "pr") {
		t.Errorf("error should mention the pr role: %s", result.Error)
	}
}

func TestRunTaskUsesAdapterAndWritesRef(t *testing.T) {
	repo := t.TempDir()
	featureDir := filepath.Join(repo, "docs", "specs", "current-feature")
	os.MkdirAll(featureDir, 0755)
	os.WriteFile(filepath.Join(featureDir, "prd.md"), []byte("# PRD"), 0644)

	logPath := filepath.Join(repo, "adapter.log")
	bin := fakeAdapter(t, repo, logPath, `{"task_ref":"ISSUE-7","url":"https://example.com/i/7"}`)

	reg := adapter.NewRegistry()
	reg.Set("task", &adapter.Bridge{Bin: bin})

	cfg := &StepConfig{RepoDir: repo, FeatureDir: featureDir, Feature: "current-feature", Adapters: reg}
	result := runTask(cfg, nil)
	if !result.Success {
		t.Fatalf("runTask failed: %s", result.Error)
	}

	logData, _ := os.ReadFile(logPath)
	if !strings.Contains(string(logData), "argv: create-task") {
		t.Errorf("adapter not invoked with create-task; log:\n%s", string(logData))
	}

	ref := ReadTaskRef(featureDir)
	if ref.TaskRef != "ISSUE-7" {
		t.Errorf("task.json task_ref = %q, want ISSUE-7", ref.TaskRef)
	}
}
