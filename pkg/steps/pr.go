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

func preparePRBranch(repoDir, feature string) (string, error) {
	branch := getCurrentBranch(repoDir)
	if branch == "" {
		return "", fmt.Errorf("not on a git branch; initialize git and commit first")
	}
	if branch == "main" || branch == "master" {
		branch = "tomato/" + sanitizeBranchPart(feature)
		if err := runGitCmd(repoDir, "checkout", "-B", branch); err != nil {
			return "", fmt.Errorf("creating PR branch %s: %w", branch, err)
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
	if getGitRemote(&StepConfig{RepoDir: repoDir}) != "" {
		if err := runGitCmd(repoDir, "push", "-u", "origin", branch); err != nil {
			return "", fmt.Errorf("pushing PR branch %s: %w", branch, err)
		}
	}
	return branch, nil
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
