package steps

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeFeature(t *testing.T) {
	cases := map[string]string{
		"login":         "login",
		"feature/login": "feature-login",
		"  spaced  ":    "spaced",
		"a@b#c":         "a-b-c",
		"--trim--":      "trim",
	}
	for in, want := range cases {
		if got := sanitizeFeature(in); got != want {
			t.Errorf("sanitizeFeature(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveFeatureExplicitWins(t *testing.T) {
	// Explicit flag beats configured value and branch.
	if got := ResolveFeature("my-feat", "cfg-feat", "/nonexistent"); got != "my-feat" {
		t.Errorf("explicit flag should win, got %q", got)
	}
}

func TestResolveFeatureConfiguredBeatsBranch(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir, "feature/login")
	// No flag, but tomato.yaml feature set → it wins over the branch.
	if got := ResolveFeature("", "from-config", dir); got != "from-config" {
		t.Errorf("configured value should win over branch, got %q", got)
	}
}

func TestResolveFeatureFromBranch(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir, "feature/login")
	if got := ResolveFeature("", "", dir); got != "login" {
		t.Errorf("expected branch-derived 'login', got %q", got)
	}
}

func TestResolveFeatureFallback(t *testing.T) {
	// Not a git repo, no flag, no config → fallback.
	if got := ResolveFeature("", "", t.TempDir()); got != "current-feature" {
		t.Errorf("expected fallback 'current-feature', got %q", got)
	}
}

func gitInit(t *testing.T, dir, branch string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@t.com"},
		{"config", "user.name", "T"},
		{"checkout", "-b", branch},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0644)
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "i"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Run()
	}
}
