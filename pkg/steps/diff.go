package steps

import (
	"fmt"
	"os/exec"
)

// getGitDiff returns the diff of changes introduced by the current feature
// branch. When origin/main exists (the normal workflow case), it diffs
// origin/main...HEAD so that already-committed changes (which
// CommitFeatureArtifacts staged) are visible to the review step. Without a
// remote, it falls back to the working-tree diff vs HEAD.
func getGitDiff(repoDir string) (string, error) {
	// Prefer the feature-branch diff against origin/main. The three-dot
	// notation shows changes on the current branch since it diverged from
	// origin/main — exactly what review needs to see.
	if getGitRemote(&StepConfig{RepoDir: repoDir}) != "" {
		cmd := exec.Command("git", "diff", "origin/main...HEAD")
		cmd.Dir = repoDir
		out, err := cmd.Output()
		if err == nil {
			return string(out), nil
		}
		// origin/main might not exist yet (e.g. fresh repo with remote but
		// no main branch pushed). Fall through to HEAD diff.
	}

	// Fallback: working-tree diff vs HEAD (covers no-remote repos and the
	// first commit where HEAD doesn't resolve yet).
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		// If HEAD doesn't exist (no commits yet), try diff without HEAD.
		cmd2 := exec.Command("git", "diff", "--cached")
		cmd2.Dir = repoDir
		out2, err2 := cmd2.Output()
		if err2 != nil {
			return "", fmt.Errorf("getting git diff: %w", err)
		}
		return string(out2), nil
	}
	return string(out), nil
}
