package steps

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetGitDiff(t *testing.T) {
	dir := t.TempDir()

	// Init a git repo
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// Create and commit a file
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	// Modify the file
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	runGit(t, dir, "add", ".")

	diff, err := getGitDiff(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "func main") {
		t.Errorf("expected diff to contain 'func main', got: %s", diff)
	}
}

func TestGetGitDiffNoChanges(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "file.go"), []byte("package main\n"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	diff, err := getGitDiff(dir)
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Errorf("expected empty diff, got: %s", diff)
	}
}

func TestGetGitDiffNotARepo(t *testing.T) {
	dir := t.TempDir()
	_, err := getGitDiff(dir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

// TestGetGitDiffShowsCommittedChangesOnFeatureBranch verifies that getGitDiff
// returns the diff between the current feature branch and origin/main, NOT
// just uncommitted working-tree changes. This is the real review scenario:
// CommitFeatureArtifacts already committed the changes, so `git diff HEAD` is
// empty — but review needs to see what the feature branch introduced.
func TestGetGitDiffShowsCommittedChangesOnFeatureBranch(t *testing.T) {
	dir := t.TempDir()

	// Set up a repo with a bare remote so origin/main exists.
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "checkout", "-b", "main")
	os.WriteFile(filepath.Join(dir, "base.go"), []byte("package main\n"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	bare := t.TempDir()
	runGit(t, bare, "init", "--bare")
	runGit(t, dir, "remote", "add", "origin", bare)
	runGit(t, dir, "push", "-u", "origin", "main")

	// Create a feature branch and commit a change (simulating what
	// CommitFeatureArtifacts does after impl).
	runGit(t, dir, "checkout", "-b", "tomato/my-feature")
	os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\n\nfunc New() {}\n"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "feat: add new.go")

	// Working tree is clean — git diff HEAD is empty.
	out, _ := exec.Command("git", "-C", dir, "diff", "HEAD").Output()
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("precondition: working tree should be clean, got diff: %q", string(out))
	}

	// getGitDiff should still return the feature branch's changes vs origin/main.
	diff, err := getGitDiff(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "func New") {
		t.Errorf("expected diff to contain committed feature changes, got: %s", diff)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s failed: %v", strings.Join(args, " "), err)
	}
}
