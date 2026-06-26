package steps

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

	adapterBin := GlobalAdapterBin
	if adapterBin == "" {
		return &model.StepResult{
			Success: false,
			Error:   "no adapter configured for 'pr' role. Set TOMATO_ADAPTER_BIN env or configure in tomato.yaml",
		}
	}

	input := struct {
		Branch string `json:"branch"`
		Repo   string `json:"repo"`
		Title  string `json:"title"`
		Draft  bool   `json:"draft"`
	}{
		Branch: branch,
		Repo:   getGitRemote(cfg),
		Title:  fmt.Sprintf("feat: %s", cfg.Feature),
		Draft:  true,
	}
	inputJSON, _ := json.Marshal(input)

	cmd := exec.Command(adapterBin, "create-pr")
	cmd.Dir = cfg.RepoDir
	cmd.Env = os.Environ()
	cmd.Stdin = strings.NewReader(string(inputJSON))

	output, err := cmd.Output()
	if err != nil {
		return &model.StepResult{Success: false, Error: fmt.Sprintf("adapter create-pr failed: %v", err)}
	}

	// Write pr.md with PR info
	var result struct {
		PRRef string `json:"pr_ref"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return &model.StepResult{Success: false, Error: fmt.Sprintf("invalid adapter output: %v", err)}
	}

	prContent := fmt.Sprintf("# PR: %s\n\n- PR: %s\n- URL: %s\n- Branch: %s\n", cfg.Feature, result.PRRef, result.URL, branch)
	prPath := filepath.Join(cfg.FeatureDir, "pr.md")
	if err := writeFile(prPath, prContent); err != nil {
		return &model.StepResult{Success: false, Error: fmt.Sprintf("writing pr.md: %v", err)}
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