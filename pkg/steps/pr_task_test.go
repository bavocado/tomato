package steps

import (
	"io"
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

func TestRunPRPrintsCreatedPRURL(t *testing.T) {
	repo := t.TempDir()
	newGitRepo(t, repo)
	featureDir := filepath.Join(repo, "docs", "specs", "current-feature")
	os.MkdirAll(featureDir, 0755)

	logPath := filepath.Join(repo, "adapter.log")
	bin := fakeAdapter(t, repo, logPath, `{"pr_ref":"42","url":"https://example.com/pr/42"}`)

	reg := adapter.NewRegistry()
	reg.Set("pr", &adapter.Bridge{Bin: bin})

	cfg := &StepConfig{RepoDir: repo, FeatureDir: featureDir, Feature: "current-feature", Adapters: reg}
	stdout := captureStdout(t, func() {
		result := runPR(cfg, nil)
		if !result.Success {
			t.Fatalf("runPR failed: %s", result.Error)
		}
	})

	if !strings.Contains(stdout, "https://example.com/pr/42") {
		t.Fatalf("expected stdout to contain PR URL, got %q", stdout)
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

func TestPreparePRBranchCreatesFeatureBranchFromMainAndCommits(t *testing.T) {
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@t.com"},
		{"config", "user.name", "T"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	os.WriteFile(filepath.Join(repo, "initial.txt"), []byte("initial"), 0644)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")
	// Ensure branch is main for this test.
	runGit(t, repo, "branch", "-M", "main")

	os.WriteFile(filepath.Join(repo, "generated.go"), []byte("package main\n"), 0644)

	branch, err := preparePRBranch(repo, "data-models")
	if err != nil {
		t.Fatal(err)
	}
	if branch != "tomato/data-models" {
		t.Fatalf("expected tomato/data-models branch, got %s", branch)
	}
	current := getCurrentBranch(repo)
	if current != "tomato/data-models" {
		t.Fatalf("expected current branch tomato/data-models, got %s", current)
	}
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected clean working tree after preparePRBranch, got %q", string(out))
	}
}

// addBareRemote creates a bare git repo and adds it as "origin" of the given
// working repo, then pushes main so origin/main exists. Returns the bare path.
func addBareRemote(t *testing.T, repo string) string {
	t.Helper()
	bare := t.TempDir()
	runGit(t, bare, "init", "--bare")
	runGit(t, repo, "remote", "add", "origin", bare)
	runGit(t, repo, "push", "-u", "origin", "main")
	return bare
}

// TestPreparePRBranchFromOriginMain verifies that when a remote origin exists,
// the feature branch is created from origin/main (the fetched remote tip), not
// from the local HEAD. We simulate a diverged state: local main is behind
// origin/main, and preparePRBranch must land on the newer origin/main commit.
func TestPreparePRBranchFromOriginMain(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "t@t.com")
	runGit(t, repo, "config", "user.name", "T")
	runGit(t, repo, "checkout", "-b", "main")
	os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base"), 0644)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	addBareRemote(t, repo)

	// Advance origin/main with an extra commit, then roll local main back so
	// local HEAD is behind origin/main.
	os.WriteFile(filepath.Join(repo, "remote-extra.txt"), []byte("from-remote"), 0644)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "remote-extra")
	runGit(t, repo, "push", "origin", "main")
	runGit(t, repo, "reset", "--hard", "HEAD~1")

	// Sanity: local HEAD no longer has remote-extra.txt.
	if _, err := os.Stat(filepath.Join(repo, "remote-extra.txt")); !os.IsNotExist(err) {
		t.Fatalf("precondition: local should be behind origin/main")
	}

	branch, err := preparePRBranch(repo, "feat-x")
	if err != nil {
		t.Fatalf("preparePRBranch: %v", err)
	}
	if branch != "tomato/feat-x" {
		t.Fatalf("expected tomato/feat-x, got %s", branch)
	}
	// The new branch must be based on origin/main, so remote-extra.txt exists.
	if _, err := os.Stat(filepath.Join(repo, "remote-extra.txt")); err != nil {
		t.Fatalf("expected branch based on origin/main (remote-extra.txt missing): %v", err)
	}
}

// TestPreparePRBranchPreservesDirtyArtifactsWhenSwitchingFromOriginMain
// reproduces the workflow failure where impl rewrites architecture.md and the
// following pr step tries to checkout a branch from origin/main. The dirty
// artifact must be carried onto the new branch and committed, not block checkout.
func TestPreparePRBranchPreservesDirtyArtifactsWhenSwitchingFromOriginMain(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "t@t.com")
	runGit(t, repo, "config", "user.name", "T")
	runGit(t, repo, "checkout", "-b", "main")

	featureDir := filepath.Join(repo, "docs", "specs", "openai-endpoint-forwarder")
	archPath := filepath.Join(featureDir, "architecture.md")
	os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base"), 0644)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	addBareRemote(t, repo)

	// Simulate design having created and committed the feature artifact locally,
	// while origin/main still does not contain this feature directory.
	os.MkdirAll(featureDir, 0755)
	os.WriteFile(archPath, []byte("old architecture"), 0644)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "design artifacts")

	// Simulate the rewrite-arch post-hook changing the tracked artifact
	// immediately before pr runs.
	os.WriteFile(archPath, []byte("rewritten architecture"), 0644)

	branch, err := preparePRBranch(repo, "openai-endpoint-forwarder")
	if err != nil {
		t.Fatalf("preparePRBranch should preserve dirty artifacts instead of failing checkout: %v", err)
	}
	if branch != "tomato/openai-endpoint-forwarder" {
		t.Fatalf("expected tomato/openai-endpoint-forwarder, got %s", branch)
	}
	data, err := os.ReadFile(archPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "rewritten architecture" {
		t.Fatalf("dirty artifact content not preserved, got %q", string(data))
	}
	if getCurrentBranch(repo) != branch {
		t.Fatalf("expected current branch %s, got %s", branch, getCurrentBranch(repo))
	}
	out, err := exec.Command("git", "-C", repo, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected clean working tree after committing preserved artifact, got %q", string(out))
	}
}

// TestPreparePRBranchSuffixesExistingBranch verifies that when tomato/<feature>
// already exists, a new tomato/<feature>-2 (then -3, …) is created and the old
// branch is preserved.
func TestPreparePRBranchSuffixesExistingBranch(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "t@t.com")
	runGit(t, repo, "config", "user.name", "T")
	runGit(t, repo, "checkout", "-b", "main")
	os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base"), 0644)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")

	// Pre-create tomato/feat-y so the first run must pick -2.
	runGit(t, repo, "branch", "tomato/feat-y")

	branch, err := preparePRBranch(repo, "feat-y")
	if err != nil {
		t.Fatalf("preparePRBranch: %v", err)
	}
	if branch != "tomato/feat-y-2" {
		t.Fatalf("expected tomato/feat-y-2, got %s", branch)
	}
	// Old branch still exists.
	out, err := exec.Command("git", "-C", repo, "rev-parse", "--verify", "tomato/feat-y").Output()
	if err != nil {
		t.Fatalf("old branch tomato/feat-y should still exist: %v", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Fatal("tomato/feat-y ref is empty")
	}
}

// TestPreparePRBranchFallsBackToHEADWithoutRemote verifies that without an
// origin remote, the feature branch is created from the local current HEAD
// (the existing behavior), so pure-local repos and unit tests keep working.
func TestPreparePRBranchFallsBackToHEADWithoutRemote(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "t@t.com")
	runGit(t, repo, "config", "user.name", "T")
	runGit(t, repo, "checkout", "-b", "main")
	os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base"), 0644)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	// No remote configured.

	branch, err := preparePRBranch(repo, "local-feat")
	if err != nil {
		t.Fatalf("preparePRBranch: %v", err)
	}
	if branch != "tomato/local-feat" {
		t.Fatalf("expected tomato/local-feat, got %s", branch)
	}
	if getCurrentBranch(repo) != "tomato/local-feat" {
		t.Fatalf("expected to be on tomato/local-feat, got %s", getCurrentBranch(repo))
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

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old
	data, _ := io.ReadAll(r)
	r.Close()
	return string(data)
}
