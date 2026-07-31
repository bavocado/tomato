package steps

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newCommitTestRepo sets up a local git repo with one committed file, ready
// for CommitAllChanges / CommitFeatureArtifacts tests.
func newCommitTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	return dir
}

func TestCommitAllChangesCommitsSourceWithVerb(t *testing.T) {
	dir := newCommitTestRepo(t)

	// impl writes a source file -> should be committed with "feat:" prefix.
	os.WriteFile(filepath.Join(dir, "handler.go"), []byte("package main\n\nfunc h() {}\n"), 0644)
	if err := CommitAllChanges(dir, "grape-f003", "impl", "feat"); err != nil {
		t.Fatalf("CommitAllChanges: %v", err)
	}

	// Working tree must be clean (the new file was committed).
	if hasWorkingTreeChanges(dir) {
		t.Errorf("expected clean working tree after commit")
	}
	// The commit message must carry the feat verb.
	out, err := runGitOut(t, dir, "log", "-1", "--format=%s")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "feat: grape-f003 code changes after impl") {
		t.Errorf("expected feat: message, got %q", out)
	}
}

func TestCommitAllChangesFixVerb(t *testing.T) {
	dir := newCommitTestRepo(t)

	os.WriteFile(filepath.Join(dir, "fix.go"), []byte("package main\n"), 0644)
	if err := CommitAllChanges(dir, "grape-f001", "fix-r1", "fix"); err != nil {
		t.Fatalf("CommitAllChanges: %v", err)
	}
	out, err := runGitOut(t, dir, "log", "-1", "--format=%s")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "fix: grape-f001 code changes after fix-r1") {
		t.Errorf("expected fix: message, got %q", out)
	}
}

func TestCommitAllChangesNoopWhenClean(t *testing.T) {
	dir := newCommitTestRepo(t)
	before, _ := runGitOut(t, dir, "rev-parse", "HEAD")
	// No working-tree changes -> CommitAllChanges is a no-op.
	if err := CommitAllChanges(dir, "grape-f003", "impl", "feat"); err != nil {
		t.Fatalf("CommitAllChanges on clean tree: %v", err)
	}
	after, _ := runGitOut(t, dir, "rev-parse", "HEAD")
	if before != after {
		t.Errorf("expected no new commit on clean tree, HEAD moved %s -> %s", before, after)
	}
}

// runGitOut runs git in dir and returns trimmed stdout (fatal on error).
func runGitOut(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
