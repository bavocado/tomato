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
	// Get the current git branch
	branch := getCurrentBranch(cfg.RepoDir)
	if branch == "" {
		return &model.StepResult{Success: false, Error: "not on a git branch; commit changes first"}
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

func getGitRemote(cfg *StepConfig) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = cfg.RepoDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
