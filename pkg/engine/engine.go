package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bavocado/tomato/pkg/archive"
	"github.com/bavocado/tomato/pkg/budget"
	"github.com/bavocado/tomato/pkg/config"
	"github.com/bavocado/tomato/pkg/steps"
)

// Engine loads a tomato.yaml and provides workflow scheduling.
type Engine struct {
	Config      *config.Config
	Workflows   map[string]config.WorkflowDef
	RepoDir     string
	AdapterBins map[string]string
	Tracker     *budget.Tracker
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

	tracker := budget.NewTracker()
	tracker.InitFromConfig(
		cfg.Budget.Mode,
		cfg.Budget.PerStep,
		cfg.Budget.GlobalPerRun,
		cfg.Budget.OnExceed,
		cfg.Budget.DegradeTo,
	)

	return &Engine{
		Config:      cfg,
		Workflows:   cfg.Workflows,
		RepoDir:     dir,
		AdapterBins: adapterBins,
		Tracker:     tracker,
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
			AnthropicURL:   e.Config.Anthropic.ResolvedBaseURL(),
			AnthropicKey:   e.Config.Anthropic.ResolvedAuthToken(),
			AnthropicModel: e.Config.Anthropic.ResolvedModel(),
			BudgetTracker:  e.Tracker,
		}
		stepConfig.LLMStream = steps.NewLLMStream(stepConfig)

		result := stepFn(stepConfig, nil)
		if !result.Success {
			fmt.Printf("✗ %s failed: %s\n", stepCfg.Name, result.Error)
			return fmt.Errorf("step %q failed: %s", stepCfg.Name, result.Error)
		}
		fmt.Printf("✓ %s completed (run: %s)\n", stepCfg.Name, result.RunID)

		// Post-hook: after impl, archive the design trio to v<N>/ and rewrite
		// architecture.md to reflect the real, as-implemented architecture
		// (design §2.8). Archiving copies (root retains the trio) so the
		// rewrite overwrites only architecture.md, leaving the design-intent
		// snapshot frozen in v<N>/.
		if stepCfg.Name == "impl" && result.Success {
			featureDir := filepath.Join(e.RepoDir, "docs", "specs", "current-feature")
			ver, err := archive.ArchiveTrio(featureDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "⚠  warning: failed to archive design trio: %v\n", err)
			} else {
				fmt.Printf("📦 design trio archived to v%d/\n", ver)
			}
			if e.Config.Impl.RewriteArchEnabled() {
				if err := e.rewriteArchitecture(featureDir); err != nil {
					fmt.Fprintf(os.Stderr, "⚠  warning: failed to rewrite architecture.md: %v\n", err)
				} else {
					fmt.Printf("🔄 architecture.md rewritten to reflect real implementation\n")
				}
			}
		}
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
			AnthropicURL:   e.Config.Anthropic.ResolvedBaseURL(),
			AnthropicKey:   e.Config.Anthropic.ResolvedAuthToken(),
			AnthropicModel: e.Config.Anthropic.ResolvedModel(),
			BudgetTracker:  e.Tracker,
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
				BudgetTracker:  e.Tracker,
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
				return e.askOnFail(os.Stdin)
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

// askOnFail implements the on_fail: ask policy. It prompts on an interactive
// terminal, returning nil to accept-and-continue or an error to abort. When the
// input is not a terminal (CI, piped, /dev/null), it does NOT block waiting on
// stdin — it fails safe by aborting with a clear message, since "ask" means a
// human decision is required and none is available.
func (e *Engine) askOnFail(in *os.File) error {
	if !isInteractive(in) {
		return fmt.Errorf("review_loop exhausted: blocking issues remain and on_fail=ask requires an interactive terminal (none detected); rerun interactively or set on_fail to stop/continue")
	}

	fmt.Println("  Accept and continue? [Y/n]")
	var input string
	fmt.Fscanln(in, &input)
	if input == "n" || input == "N" {
		return fmt.Errorf("review_loop aborted by user")
	}
	return nil
}

// isInteractive reports whether f is a character device (a terminal), as
// opposed to a pipe, regular file, or /dev/null.
func isInteractive(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// rewriteArchitecture runs the §2.8 post-impl architecture rewrite, regenerating
// the root architecture.md from the actual implementation. Failures are
// non-fatal (the impl step itself already succeeded) and are surfaced as warnings.
func (e *Engine) rewriteArchitecture(featureDir string) error {
	cfg := &steps.StepConfig{
		RepoDir:        e.RepoDir,
		FeatureDir:     featureDir,
		Feature:        "current-feature",
		ModelName:      e.resolveModel("design"),
		AnthropicURL:   e.Config.Anthropic.BaseURL,
		AnthropicKey:   e.Config.Anthropic.AuthToken,
		AnthropicModel: e.Config.Anthropic.Model,
		BudgetTracker:  e.Tracker,
	}
	cfg.LLMStream = steps.NewLLMStream(cfg)

	result := steps.RewriteArchitecture(cfg)
	if !result.Success {
		return fmt.Errorf("%s", result.Error)
	}
	return nil
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
