package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bavocado/tomato/pkg/adapter"
	"github.com/bavocado/tomato/pkg/archive"
	"github.com/bavocado/tomato/pkg/budget"
	"github.com/bavocado/tomato/pkg/config"
	"github.com/bavocado/tomato/pkg/customstep"
	"github.com/bavocado/tomato/pkg/llm"
	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
	"github.com/bavocado/tomato/pkg/state"
	"github.com/bavocado/tomato/pkg/steps"
)

// Engine loads a tomato.yaml and provides workflow scheduling.
type Engine struct {
	Config    *config.Config
	Workflows map[string]config.WorkflowDef
	RepoDir   string
	Feature   string
	Adapters  *adapter.Registry
	Tracker   *budget.Tracker
	// LLMStream, when non-nil, overrides the default LLM stream factory for
	// every step (built-in and custom). Production leaves it nil so each
	// step binds its provider via steps.NewLLMStream; tests and future
	// replay/dry-run modes inject a function here.
	LLMStream runner.LLMFunc
}

// NewEngine creates an engine by loading tomato.yaml from the given directory.
// The feature defaults to the current git branch (or "current-feature"); set
// Engine.Feature afterwards to override (e.g. from a --feature flag).
func NewEngine(dir string) (*Engine, error) {
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, err
	}

	adapters := BuildRegistry(cfg)

	tracker := budget.NewTracker()
	tracker.InitFromConfig(
		cfg.Budget.Mode,
		cfg.Budget.PerStep,
		cfg.Budget.GlobalPerRun,
		cfg.Budget.OnExceed,
		cfg.Budget.DegradeTo,
	)

	return &Engine{
		Config:    cfg,
		Workflows: cfg.Workflows,
		RepoDir:   dir,
		Feature:   steps.ResolveFeature("", cfg.Feature, dir),
		Adapters:  adapters,
		Tracker:   tracker,
	}, nil
}

// featureDir returns the artifact directory for the engine's current feature.
func (e *Engine) featureDir() string {
	return steps.FeatureDir(e.RepoDir, e.Feature)
}

// BuildRegistry resolves the role→adapter mapping from tomato.yaml. Adapter env
// values are expanded against the process environment (so "${GITHUB_TOKEN}"
// works). When no roles are configured, a TOMATO_ADAPTER_BIN fallback serves
// the built-in roles for backward compatibility.
func BuildRegistry(cfg *config.Config) *adapter.Registry {
	reg := adapter.NewRegistry()
	for role, name := range cfg.Roles {
		a, ok := cfg.Adapters[name]
		if !ok {
			continue
		}
		env := map[string]string{}
		for k, v := range a.Env {
			env[k] = os.ExpandEnv(v)
		}
		reg.Set(role, &adapter.Bridge{Bin: a.Bin, Env: env})
	}
	if len(cfg.Roles) == 0 {
		if bin := os.Getenv("TOMATO_ADAPTER_BIN"); bin != "" {
			for _, role := range []string{"pr", "task", "review"} {
				reg.Set(role, &adapter.Bridge{Bin: bin})
			}
		}
	}
	return reg
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

// RunOptions controls workflow execution.
type RunOptions struct {
	From   string
	Resume bool
	Force  bool
}

// Run executes a named workflow step by step.
func (e *Engine) Run(workflowName string) error {
	return e.RunWithOptions(workflowName, RunOptions{})
}

func (e *Engine) planSteps(workflowName string, opts RunOptions) []config.WorkflowStep {
	steps, _ := e.planStepsChecked(workflowName, opts)
	return steps
}

func (e *Engine) planStepsChecked(workflowName string, opts RunOptions) ([]config.WorkflowStep, error) {
	if opts.From != "" && opts.Resume {
		return nil, fmt.Errorf("--from and --resume cannot be used together")
	}
	wf, ok := e.Workflows[workflowName]
	if !ok {
		return nil, fmt.Errorf("workflow %q not found", workflowName)
	}
	if opts.Resume {
		s, err := state.Load(e.RepoDir, workflowName, e.Feature)
		if err != nil {
			return nil, fmt.Errorf("--resume: %w", err)
		}
		if s.FailedStep == "" {
			return nil, fmt.Errorf("no failed step recorded for workflow %q feature %q; nothing to resume", workflowName, e.Feature)
		}
		opts.From = s.FailedStep
	}
	if opts.From == "" {
		return wf.Steps, nil
	}
	for i, s := range wf.Steps {
		if s.Name == opts.From {
			return wf.Steps[i:], nil
		}
	}
	return nil, fmt.Errorf("--from step %q not found in workflow %q", opts.From, workflowName)
}

func (e *Engine) RunWithOptions(workflowName string, opts RunOptions) error {
	stepsToRun, err := e.planStepsChecked(workflowName, opts)
	if err != nil {
		return err
	}

	// Start each run with a fresh claude session so context from a prior run
	// does not leak in. --resume within the run reuses this session across
	// every LLM step (design §2.9 — shared session for cross-step context).
	if !opts.Resume {
		llm.ClearSession(e.RepoDir)
	}

	// completed tracks finished step names so a mid-workflow failure can
	// persist resumable state (design §3.2, Task 3).
	completed := make([]string, 0, len(stepsToRun))

	for i, stepCfg := range stepsToRun {
		if stepCfg.IsMetaStep && stepCfg.Name == "review_loop" {
			fmt.Printf("▶ [%d/%d] review_loop (max_rounds=%d)\n", i+1, len(stepsToRun), stepCfg.MaxRounds)
			if err := e.runReviewLoop(stepCfg); err != nil {
				return err
			}
			continue
		}

		fmt.Printf("▶ [%d/%d] %s\n", i+1, len(stepsToRun), stepCfg.Name)
		featureDir := e.featureDir()
		stepConfig := e.stepConfig(featureDir, e.Feature, stepCfg.Name)
		result, err := e.executeStep(stepCfg.Name, stepConfig)
		if err != nil {
			return fmt.Errorf("step %d (%s): %w", i, stepCfg.Name, err)
		}
		if !result.Success {
			fmt.Printf("✗ %s failed: %s\n", stepCfg.Name, result.Error)
			if saveErr := state.Save(e.RepoDir, state.WorkflowState{
				Workflow:       workflowName,
				Feature:        e.Feature,
				CurrentStep:    stepCfg.Name,
				FailedStep:     stepCfg.Name,
				CompletedSteps: completed,
				LastRunID:      result.RunID,
			}); saveErr != nil {
				fmt.Fprintf(os.Stderr, "⚠  warning: failed to persist resume state: %v\n", saveErr)
			}
			return fmt.Errorf("step %q failed: %s", stepCfg.Name, result.Error)
		}
		fmt.Printf("✓ %s completed (run: %s)\n", stepCfg.Name, result.RunID)

		// Commit the feature's intermediate artifacts (design docs, reviews,
		// reports, …) as they are produced, scoped to docs/specs/<feature>/.
		// Best-effort: a failed commit is a warning, never a step failure.
		if err := steps.CommitFeatureArtifacts(e.RepoDir, featureDir, e.Feature, stepCfg.Name); err != nil {
			fmt.Fprintf(os.Stderr, "⚠  warning: failed to commit feature artifacts: %v\n", err)
		}

		completed = append(completed, stepCfg.Name)
		if saveErr := state.Save(e.RepoDir, state.WorkflowState{
			Workflow:       workflowName,
			Feature:        e.Feature,
			CurrentStep:    stepCfg.Name,
			CompletedSteps: completed,
			LastRunID:      result.RunID,
		}); saveErr != nil {
			fmt.Fprintf(os.Stderr, "⚠  warning: failed to persist resume state: %v\n", saveErr)
		}

		// Post-hook: after impl, archive the design trio to v<N>/ and rewrite
		// architecture.md to reflect the real, as-implemented architecture
		// (design §2.8). Archiving copies (root retains the trio) so the
		// rewrite overwrites only architecture.md, leaving the design-intent
		// snapshot frozen in v<N>/.
		if stepCfg.Name == "impl" && result.Success {
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

		// Status lifecycle post-hook (design §2.1): sync external task status.
		if status := stepStatus(stepCfg.Name); status != "" {
			e.emitStatus(featureDir, status)
		}
	}

	// All steps succeeded — clear any prior resume state so a subsequent
	// --resume does not re-run from a stale failed step.
	return state.Clear(e.RepoDir, workflowName, e.Feature)
}

// stepStatus maps a completed step to its external status label (design §2.1).
// Steps without a status (review_loop emits its own) return "".
func stepStatus(step string) string {
	switch step {
	case "spec":
		return "specified"
	case "design":
		return "designed"
	case "impl":
		return "implemented"
	case "pr":
		return "pr_opened"
	case "test":
		return "tested"
	default:
		return ""
	}
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
	featureDir := e.featureDir()
	prBridge := e.Adapters.ForAny("pr", "review")
	prRef := steps.ReadPRRef(featureDir).PRRef

	for round := 1; round <= maxRounds+1; round++ {
			reviewCfg := e.stepConfig(featureDir, e.Feature, "review")

		fmt.Printf("  review round %d...\n", round)
		result := reviewFn(reviewCfg, []string{fmt.Sprintf("r%d", round)})
		if !result.Success {
			return fmt.Errorf("review round %d failed: %s", round, result.Error)
		}

		reviewPath := filepath.Join(featureDir, "reviews", fmt.Sprintf("r%d-comments.md", round))
		comments, _ := os.ReadFile(reviewPath)

		// Post the round's review comments to the PR. Skip when the comments
		// file is empty to avoid a "Body cannot be blank" adapter error.
		if strings.TrimSpace(string(comments)) != "" {
			e.callAdapter(prBridge, adapter.CmdCommentPR, map[string]string{
				"pr_ref":   prRef,
				"comments": string(comments),
			})
		} else {
			fmt.Fprintf(os.Stderr, "⚠  skipping comment-pr: review comments are empty\n")
		}
		// Commit the review comments artifact (reviews/r<N>-comments.md).
		if err := steps.CommitFeatureArtifacts(e.RepoDir, featureDir, e.Feature, fmt.Sprintf("review-r%d", round)); err != nil {
			fmt.Fprintf(os.Stderr, "⚠  warning: failed to commit review artifacts: %v\n", err)
		}

		if !steps.HasBlockingIssues(reviewPath) {
			fmt.Printf("✓ review_loop converged in round %d\n", round)
			e.callAdapter(prBridge, adapter.CmdMarkPRReady, map[string]string{"pr_ref": prRef})
			e.emitStatus(featureDir, "reviewed")
			return nil
		}

		if round <= maxRounds {
			fmt.Printf("  → round %d found blocking issues, fixing...\n", round)
				implCfg := e.stepConfig(featureDir, e.Feature, "impl")
			fixResult := implFn(implCfg, []string{fmt.Sprintf("fix-r%d", round)})
			if !fixResult.Success {
				return fmt.Errorf("fix round %d failed: %s", round, fixResult.Error)
			}
			// Commit docs artifacts produced/changed by the fix round.
			if err := steps.CommitFeatureArtifacts(e.RepoDir, featureDir, e.Feature, fmt.Sprintf("fix-r%d", round)); err != nil {
				fmt.Fprintf(os.Stderr, "⚠  warning: failed to commit fix artifacts: %v\n", err)
			}
			// Commit source-code changes the fix round made (impl rewrites
			// source files under internal/, cmd/, etc.). pr already ran before
			// review_loop, so without this the fix code would stay uncommitted.
			if err := steps.CommitAllChanges(e.RepoDir, e.Feature, fmt.Sprintf("fix-r%d", round)); err != nil {
				fmt.Fprintf(os.Stderr, "⚠  warning: failed to commit fix code changes: %v\n", err)
			}
			e.callAdapter(prBridge, adapter.CmdUpdatePR, map[string]string{
				"pr_ref": prRef,
				"branch": steps.ReadPRRef(featureDir).Branch,
			})
		} else {
			e.callAdapter(prBridge, adapter.CmdMarkPRFailed, map[string]string{
				"pr_ref":   prRef,
				"comments": string(comments),
			})
			e.emitStatus(featureDir, "review_failed")
			fmt.Fprintf(os.Stderr, "✗ review_loop exhausted after %d rounds\n", round)
			fmt.Fprintf(os.Stderr, "  Final comments: %s\n", string(comments))

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

func (e *Engine) stepConfig(featureDir, feature, stepName string) *steps.StepConfig {
	modelID := e.resolveModel(stepName)
	provider := e.Config.ResolveProviderConfig(modelID)
	cfg := &steps.StepConfig{
		RepoDir:        e.RepoDir,
		FeatureDir:     featureDir,
		Feature:        feature,
		ModelName:      modelID,
		Adapters:       e.Adapters,
		AnthropicURL:   provider.BaseURL,
		AnthropicKey:   provider.AuthToken,
		AnthropicModel: provider.Model,
		BudgetTracker:  e.Tracker,
	}
	if e.LLMStream != nil {
		cfg.LLMStream = e.LLMStream
	} else {
		cfg.LLMStream = steps.NewLLMStream(cfg)
	}
	return cfg
}

// executeStep resolves a workflow step to either a registered built-in step or
// a user-defined custom step (design §3.3, Task 4). A step that is neither
// registered nor declared in custom_steps returns the original lookup error so
// callers surface a clear "unknown step" message.
func (e *Engine) executeStep(name string, cfg *steps.StepConfig) (*model.StepResult, error) {
	stepFn, err := steps.Get(name)
	if err == nil {
		return stepFn(cfg, nil), nil
	}
	if def, ok := e.Config.CustomSteps[name]; ok {
		return customstep.Run(name, def, customstep.Config{
			RepoDir:       cfg.RepoDir,
			ModelName:     cfg.ModelName,
			LLMStream:     cfg.LLMStream,
			BudgetTracker: cfg.BudgetTracker,
		}), nil
	}
	return nil, err
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

// callAdapter invokes a bridge subcommand with a JSON payload. PR/status
// bookkeeping is best-effort: a nil bridge (no adapter configured) is a no-op,
// and execution errors are warnings — the review verdict, not the PR
// bookkeeping, is the real signal.
func (e *Engine) callAdapter(br *adapter.Bridge, sub adapter.Subcommand, payload map[string]string) {
	if br == nil {
		return
	}
	data, _ := json.Marshal(payload)
	if _, err := br.Execute(sub, string(data), nil); err != nil {
		fmt.Fprintf(os.Stderr, "⚠  adapter %s failed: %v\n", sub, err)
	}
}

// emitStatus runs the §2.1 status lifecycle hook: it asks the task adapter to
// update the external task's status. It is best-effort — it reads the task_ref
// from task.json and skips silently when no task exists yet (the task step is
// last in the default workflow, so earlier steps have no task to update).
func (e *Engine) emitStatus(featureDir, status string) {
	br := e.Adapters.For("task")
	if br == nil {
		return
	}
	taskRef := steps.ReadTaskRef(featureDir).TaskRef
	if taskRef == "" {
		return // no task created yet; nothing to update
	}
	e.callAdapter(br, adapter.CmdUpdateStatus, map[string]string{
		"task_ref": taskRef,
		"status":   status,
	})
}

// rewriteArchitecture runs the §2.8 post-impl architecture rewrite, regenerating
// the root architecture.md from the actual implementation. Failures are
// non-fatal (the impl step itself already succeeded) and are surfaced as warnings.
func (e *Engine) rewriteArchitecture(featureDir string) error {
	cfg := e.stepConfig(featureDir, e.Feature, "design")

	result := steps.RewriteArchitecture(cfg)
	if !result.Success {
		return fmt.Errorf("%s", result.Error)
	}
	return nil
}
