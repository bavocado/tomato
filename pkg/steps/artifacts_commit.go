package steps

import (
	"fmt"
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
	return nil
}

// hasStagedPath reports whether any staged change exists under path.
func hasStagedPath(repoDir, path string) bool {
	cmd := exec.Command("git", "diff", "--cached", "--quiet", "--", path)
	cmd.Dir = repoDir
	return cmd.Run() != nil
}
