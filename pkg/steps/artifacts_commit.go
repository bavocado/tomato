package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CommitFeatureArtifacts stages and commits the intermediate artifacts under
// docs/specs/<feature>/ (design docs, reviews, reports, pr.json, …). It stages
// ONLY that directory so unrelated working-tree changes and the .tomato/
// runtime dir are left untouched.
//
// It is a no-op when there is nothing new to commit (e.g. an LLM step that did
// not change any artifact, or a step like `pr` that already committed). The
// step name is folded into the message so the history reads, for example,
// "chore(docs): design artifacts after design".
//
// Errors are non-fatal to a step's success: the artifact already landed on
// disk, and a failed commit should not reverse that. Callers surface the error
// as a warning.
func CommitFeatureArtifacts(repoDir, featureDir, feature, stepName string) error {
	rel, err := filepath.Rel(repoDir, featureDir)
	if err != nil {
		return fmt.Errorf("resolving feature dir: %w", err)
	}
	// Guard against an unexpected absolute/mismatched featureDir: only stage a
	// path that is genuinely under the repo.
	if rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("feature dir %q is not under repo root", featureDir)
	}

	if err := runGitCmd(repoDir, "add", "--", rel); err != nil {
		return fmt.Errorf("staging feature artifacts: %w", err)
	}
	// hasStagedChanges checks the whole index, which would misfire if an
	// earlier step left unrelated files staged. Scope the check to the feature
	// dir so we only commit when this step actually changed artifacts here.
	if !hasStagedPath(repoDir, rel) {
		return nil
	}
	msg := fmt.Sprintf("chore(docs): %s artifacts after %s", feature, stepName)
	if err := runGitCmd(repoDir, "commit", "-m", msg); err != nil {
		return fmt.Errorf("committing feature artifacts: %w", err)
	}
	pushCurrentBranch(repoDir)
	return nil
}

// hasStagedPath reports whether any staged change exists under path.
func hasStagedPath(repoDir, path string) bool {
	cmd := exec.Command("git", "diff", "--cached", "--quiet", "--", path)
	cmd.Dir = repoDir
	return cmd.Run() != nil
}

// CommitAllChanges stages and commits ALL working-tree changes (source code +
// docs artifacts). It is used after review_loop fix rounds so that impl fix
// changes to source files are committed alongside the docs artifacts, not left
// uncommitted in the working tree. .tomato/ stays untracked via .gitignore.
//
// It is a no-op when there is nothing staged after `git add -A`. Best-effort:
// errors are surfaced by the caller as warnings, never as step failures.
func CommitAllChanges(repoDir, feature, stepName string) error {
	if err := runGitCmd(repoDir, "add", "-A"); err != nil {
		return fmt.Errorf("staging all changes: %w", err)
	}
	if !hasStagedChanges(repoDir) {
		return nil
	}
	msg := fmt.Sprintf("fix: %s code changes after %s", feature, stepName)
	if err := runGitCmd(repoDir, "commit", "-m", msg); err != nil {
		return fmt.Errorf("committing code changes: %w", err)
	}
	pushCurrentBranch(repoDir)
	return nil
}

// pushCurrentBranch pushes the current branch to origin when a remote is
// configured. It is best-effort: no remote → silent skip; push failure →
// warning to stderr (the commit already succeeded, a push failure must not
// reverse it). Uses --force-with-lease so re-pushes after a prior push (e.g.
// preparePRBranch already pushed) are safe and never clobber remote work.
func pushCurrentBranch(repoDir string) {
	if getGitRemote(&StepConfig{RepoDir: repoDir}) == "" {
		return // no remote: local-only repo, skip
	}
	branch := getCurrentBranch(repoDir)
	if branch == "" || branch == "HEAD" {
		return
	}
	if err := runGitCmd(repoDir, "push", "origin", branch); err != nil {
		fmt.Fprintf(os.Stderr, "⚠  warning: failed to push %s: %v\n", branch, err)
	}
}
