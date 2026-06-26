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

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s failed: %v", strings.Join(args, " "), err)
	}
}