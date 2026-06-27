package steps

import (
	"fmt"
	"os/exec"
	"strings"
)

// tomatoSignature returns a chain-of-provenance footer recording the current
// HEAD commit hash, matching the prepare-commit-msg hook's format. It is
// stamped into tomato-created PR descriptions so tomato's own outputs follow
// the same signing convention as its commits. Returns "" when HEAD cannot be
// resolved (e.g. no commits yet).
func tomatoSignature(repoDir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	hash := strings.TrimSpace(string(out))
	if hash == "" {
		return ""
	}
	return fmt.Sprintf("🍅 Tomato signature\nTomato-Parent: %s", hash)
}
