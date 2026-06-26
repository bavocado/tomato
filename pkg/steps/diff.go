package steps

import (
	"fmt"
	"os/exec"
)

// getGitDiff returns the staged + unstaged diff of the working tree.
// Returns empty string if there are no changes.
func getGitDiff(repoDir string) (string, error) {
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		// If HEAD doesn't exist (no commits yet), try diff without HEAD
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