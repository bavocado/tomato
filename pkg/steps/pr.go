package steps

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bavocado/tomato/pkg/adapter"
	"github.com/bavocado/tomato/pkg/model"
)

func init() {
	Register("pr", runPR)
}

func runPR(cfg *StepConfig, args []string) *model.StepResult {
	// Ensure code is on a PR-capable feature branch with generated changes committed.
	branch, err := preparePRBranch(cfg.RepoDir, cfg.Feature)
	if err != nil {
		return &model.StepResult{Success: false, Error: err.Error()}
	}

	br := cfg.Adapters.For("pr")
	if br == nil {
		return &model.StepResult{
			Success: false,
			Error:   "no adapter configured for 'pr' role. Configure adapters+roles in tomato.yaml or set TOMATO_ADAPTER_BIN",
		}
	}

	body := fmt.Sprintf("Implemented by tomato for %q.", cfg.Feature)
	if sig := tomatoSignature(cfg.RepoDir); sig != "" {
		body += "\n\n" + sig
	}

	input := struct {
		Branch string `json:"branch"`
		Repo   string `json:"repo"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Draft  bool   `json:"draft"`
	}{
		Branch: branch,
		Repo:   getGitRemote(cfg),
		Title:  fmt.Sprintf("feat: %s", cfg.Feature),
		Body:   body,
		Draft:  true,
	}
	inputJSON, _ := json.Marshal(input)

	output, err := br.Execute(adapter.CmdCreatePR, string(inputJSON), nil)
	if err != nil {
		return &model.StepResult{Success: false, Error: fmt.Sprintf("adapter create-pr failed: %v", err)}
	}

	var result struct {
		PRRef string `json:"pr_ref"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return &model.StepResult{Success: false, Error: fmt.Sprintf("invalid adapter output: %v", err)}
	}

	// Human-readable pr.md plus machine-readable pr.json for downstream steps.
	prContent := fmt.Sprintf("# PR: %s\n\n- PR: %s\n- URL: %s\n- Branch: %s\n", cfg.Feature, result.PRRef, result.URL, branch)
	if err := writeFile(filepath.Join(cfg.FeatureDir, "pr.md"), prContent); err != nil {
		return &model.StepResult{Success: false, Error: fmt.Sprintf("writing pr.md: %v", err)}
	}
	if err := WritePRRef(cfg.FeatureDir, PRRef{PRRef: result.PRRef, URL: result.URL, Branch: branch}); err != nil {
		return &model.StepResult{Success: false, Error: fmt.Sprintf("writing pr.json: %v", err)}
	}
	if result.URL != "" {
		fmt.Printf("✓ PR created: %s\n", result.URL)
	}

	return &model.StepResult{StepName: "pr", Success: true}
}

func getCurrentBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// preparePRBranch ensures the working tree is on a PR-capable feature branch
// before staging and committing generated changes. The feature branch is based
// on the freshest available main:
//
//   - When an `origin` remote exists, it fetches `origin/main` and bases the
//     branch on the remote tip so the PR starts from the latest upstream state
//     (not a stale local main).
//   - Without a remote, it falls back to the current local HEAD so pure-local
//     repos and unit tests keep working.
//
// Branch naming: the first choice is `tomato/<feature>`. If that branch already
// exists, a `-N` suffix is appended (`tomato/<feature>-2`, `-3`, …) and the old
// branch is preserved — `git checkout -b` (not `-B`) never overwrites it.
//
// After switching, generated changes are staged with `git add -A`, committed as
// `feat: <feature>`, and pushed when `origin` is configured.
func preparePRBranch(repoDir, feature string) (string, error) {
	branch, base, hasRemote := resolvePRBranch(repoDir, feature)
	if base != "" {
		// `-b` (not `-B`): never overwrite an existing branch; pickFeatureBranchName
		// already guaranteed a free name when a new branch is needed.
		if err := runGitCmd(repoDir, "checkout", "-b", branch, base); err != nil {
			return "", fmt.Errorf("creating PR branch %s from %s: %w", branch, base, err)
		}
	}

	if err := runGitCmd(repoDir, "add", "-A"); err != nil {
		return "", fmt.Errorf("staging changes: %w", err)
	}
	if hasStagedChanges(repoDir) {
		msg := fmt.Sprintf("feat: %s", feature)
		if err := runGitCmd(repoDir, "commit", "-m", msg); err != nil {
			return "", fmt.Errorf("committing changes for PR: %w", err)
		}
	}

	// Push only when a remote is configured. Unit tests use local repos without remotes.
	if hasRemote {
		if err := runGitCmd(repoDir, "push", "-u", "origin", branch); err != nil {
			return "", fmt.Errorf("pushing PR branch %s: %w", branch, err)
		}
	}
	return branch, nil
}

// resolvePRBranch decides the branch to commit onto and, when a fresh branch is
// needed, the base to create it from. Returns (branch, base, hasRemote):
//
//   - With an `origin` remote: fetch origin/main and base a new
//     `tomato/<feature>[-N]` branch on it. base = "origin/main".
//   - Without a remote, on main/master: create `tomato/<feature>[-N]` from HEAD
//     (the legacy behavior). base = "HEAD".
//   - Without a remote, on another branch: stay on the current branch (the
//     legacy behavior — never hijack an unrelated feature branch). base = "".
//
// Staying on the current branch when there is no remote AND no main preserves
// the original semantics that existing tests and local-only workflows rely on.
func resolvePRBranch(repoDir, feature string) (branch, base string, hasRemote bool) {
	if getGitRemote(&StepConfig{RepoDir: repoDir}) != "" {
		_ = runGitCmd(repoDir, "fetch", "origin", "main")
		if refExists(repoDir, "origin/main") {
			name, _ := pickFeatureBranchName(repoDir, feature)
			return name, "origin/main", true
		}
		// Remote exists but origin/main is unavailable: fall through to HEAD.
	}
	current := getCurrentBranch(repoDir)
	if current == "main" || current == "master" {
		name, _ := pickFeatureBranchName(repoDir, feature)
		return name, "HEAD", getGitRemote(&StepConfig{RepoDir: repoDir}) != ""
	}
	// Already on a feature branch with no usable main: keep it.
	return current, "", false
}

// pickFeatureBranchName returns a free `tomato/<feature>` name, appending a
// `-N` suffix (-2, -3, …) when the base name is already taken so prior branches
// are preserved rather than overwritten.
func pickFeatureBranchName(repoDir, feature string) (string, error) {
	prefix := "tomato/" + sanitizeBranchPart(feature)
	name := prefix
	for n := 2; refExists(repoDir, name); n++ {
		name = fmt.Sprintf("%s-%d", prefix, n)
	}
	return name, nil
}

// refExists reports whether the given ref resolves.
func refExists(repoDir, ref string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", ref)
	cmd.Dir = repoDir
	return cmd.Run() == nil
}

func hasStagedChanges(repoDir string) bool {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = repoDir
	return cmd.Run() != nil
}

func runGitCmd(repoDir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}

func sanitizeBranchPart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "current-feature"
	}
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	return s
}

func getGitRemote(cfg *StepConfig) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = cfg.RepoDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
