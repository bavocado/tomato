package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bavocado/tomato/pkg/config"
	"github.com/bavocado/tomato/pkg/steps"
)

// Engine loads a tomato.yaml and provides workflow scheduling.
type Engine struct {
	Config      *config.Config
	Workflows   map[string]config.WorkflowDef
	RepoDir     string
	AdapterBins map[string]string
}

// NewEngine creates an engine by loading tomato.yaml from the given directory.
func NewEngine(dir string) (*Engine, error) {
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, err
	}

	adapterBins := map[string]string{}
	for role, adapterName := range cfg.Roles {
		if adapter, ok := cfg.Adapters[adapterName]; ok {
			adapterBins[role] = adapter.Bin
		}
	}

	if len(adapterBins) > 0 {
		for _, bin := range adapterBins {
			steps.GlobalAdapterBin = bin
			break
		}
	} else if envBin := os.Getenv("TOMATO_ADAPTER_BIN"); envBin != "" {
		steps.GlobalAdapterBin = envBin
	}

	return &Engine{
		Config:      cfg,
		Workflows:   cfg.Workflows,
		RepoDir:     dir,
		AdapterBins: adapterBins,
	}, nil
}

// HasWorkflow checks if a named workflow exists.
func (e *Engine) HasWorkflow(name string) bool {
	_, ok := e.Workflows[name]
	return ok
}

// GetSteps returns the step names for a workflow.
func (e *Engine) GetSteps(name string) []string {
	wf, ok := e.Workflows[name]
	if !ok {
		return nil
	}
	names := make([]string, len(wf.Steps))
	for i, s := range wf.Steps {
		names[i] = s.Name
	}
	return names
}

// Run executes a named workflow step by step.
func (e *Engine) Run(workflowName string) error {
	wf, ok := e.Workflows[workflowName]
	if !ok {
		return fmt.Errorf("workflow %q not found", workflowName)
	}

	for i, stepCfg := range wf.Steps {
		if stepCfg.IsMetaStep && stepCfg.Name == "review_loop" {
			fmt.Printf("▶ [%d/%d] review_loop (max_rounds=%d)\n", i+1, len(wf.Steps), stepCfg.MaxRounds)
			if err := e.runReviewLoop(stepCfg); err != nil {
				return err
			}
			continue
		}

		fmt.Printf("▶ [%d/%d] %s\n", i+1, len(wf.Steps), stepCfg.Name)
		stepFn, err := steps.Get(stepCfg.Name)
		if err != nil {
			return fmt.Errorf("step %d (%s): %w", i, stepCfg.Name, err)
		}

featureDir := filepath.Join(e.RepoDir, "docs", "specs", "current-feature")
			stepConfig := &steps.StepConfig{
				RepoDir:        e.RepoDir,
				FeatureDir:     featureDir,
				Feature:        "current-feature",
				ModelName:      e.resolveModel(stepCfg.Name),
				AnthropicURL:   e.Config.Anthropic.BaseURL,
				AnthropicKey:   e.Config.Anthropic.AuthToken,
				AnthropicModel: e.Config.Anthropic.Model,
			}
			stepConfig.LLMStream = steps.NewLLMStream(stepConfig)

		result := stepFn(stepConfig, nil)
		if !result.Success {
			fmt.Printf("✗ %s failed: %s\n", stepCfg.Name, result.Error)
			return fmt.Errorf("step %q failed: %s", stepCfg.Name, result.Error)
		}
		fmt.Printf("✓ %s completed (run: %s)\n", stepCfg.Name, result.RunID)
	}

	return nil
}

func (e *Engine) runReviewLoop(cfg config.WorkflowStep) error {
	maxRounds := cfg.MaxRounds
	if maxRounds < 1 {
		maxRounds = 1
	}
	onFail := cfg.OnFail
	if onFail == "" {
		onFail = "stop"
	}

	reviewFn, _ := steps.Get("review")
	implFn, _ := steps.Get("impl")

	for round := 1; round <= maxRounds+1; round++ {
		featureDir := filepath.Join(e.RepoDir, "docs", "specs", "current-feature")

reviewCfg := &steps.StepConfig{
				RepoDir:        e.RepoDir,
				FeatureDir:     featureDir,
				Feature:        "current-feature",
				ModelName:      e.resolveModel("review"),
				AnthropicURL:   e.Config.Anthropic.BaseURL,
				AnthropicKey:   e.Config.Anthropic.AuthToken,
				AnthropicModel: e.Config.Anthropic.Model,
			}
			reviewCfg.LLMStream = steps.NewLLMStream(reviewCfg)

		fmt.Printf("  review round %d...\n", round)
		result := reviewFn(reviewCfg, []string{fmt.Sprintf("r%d", round)})
		if !result.Success {
			return fmt.Errorf("review round %d failed: %s", round, result.Error)
		}

		reviewPath := filepath.Join(featureDir, "reviews", fmt.Sprintf("r%d-comments.md", round))
		if !steps.HasBlockingIssues(reviewPath) {
			fmt.Printf("✓ review_loop converged in round %d\n", round)
			e.callAdapterCfg("review", "mark-pr-ready", `{}`)
			return nil
		}

		if round <= maxRounds {
			fmt.Printf("  → round %d found blocking issues, fixing...\n", round)
implCfg := &steps.StepConfig{
					RepoDir:        e.RepoDir,
					FeatureDir:     featureDir,
					Feature:        "current-feature",
					ModelName:      e.resolveModel("impl"),
					AnthropicURL:   e.Config.Anthropic.BaseURL,
					AnthropicKey:   e.Config.Anthropic.AuthToken,
					AnthropicModel: e.Config.Anthropic.Model,
				}
				implCfg.LLMStream = steps.NewLLMStream(implCfg)
			fixResult := implFn(implCfg, nil)
			if !fixResult.Success {
				return fmt.Errorf("fix round %d failed: %s", round, fixResult.Error)
			}
			e.callAdapterCfg("pr", "update-pr", `{}`)
		} else {
			e.callAdapterCfg("review", "mark-pr-failed", `{}`)
			data, _ := os.ReadFile(reviewPath)
			fmt.Fprintf(os.Stderr, "✗ review_loop exhausted after %d rounds\n", round)
			fmt.Fprintf(os.Stderr, "  Final comments: %s\n", string(data))

			switch onFail {
			case "continue":
				return nil
			case "ask":
				fmt.Println("  Accept and continue? [Y/n]")
				var input string
				fmt.Scanln(&input)
				if input == "n" || input == "N" {
					return fmt.Errorf("review_loop aborted by user")
				}
				return nil
			case "stop":
				fallthrough
			default:
				return fmt.Errorf("review_loop exhausted: blocking issues remain after %d rounds", round)
			}
		}
	}
	return nil
}

func (e *Engine) resolveModel(stepName string) string {
	if m, ok := e.Config.Models.Steps[stepName]; ok {
		return m
	}
	return e.Config.Models.Default
}

func (e *Engine) callAdapterCfg(role, subcommand, stdinJSON string) string {
	bin := steps.GlobalAdapterBin
	if bin == "" {
		return ""
	}
	cmd := exec.Command(bin, subcommand)
	cmd.Dir = e.RepoDir
	cmd.Stdin = strings.NewReader(stdinJSON)
	cmd.Env = os.Environ()
	out, _ := cmd.Output()
	return string(out)
}