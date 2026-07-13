package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bavocado/tomato/pkg/adapter"
	"github.com/bavocado/tomato/pkg/config"
	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
	"github.com/bavocado/tomato/pkg/state"
	"github.com/bavocado/tomato/pkg/steps"
)

// fakeAdapterScript writes a bash adapter that appends its argv+stdin to logPath
// and prints fixed JSON. Returns the script path.
func fakeAdapterScript(t *testing.T, dir, logPath string) string {
	t.Helper()
	script := filepath.Join(dir, "fake.sh")
	body := "#!/bin/sh\necho \"argv: $@\" >> '" + logPath + "'\ncat >> '" + logPath + "'\necho >> '" + logPath + "'\necho '{}'\n"
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return script
}

func newEngineForTest(t *testing.T, reg *adapter.Registry) *Engine {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	config.Save(cfg, filepath.Join(dir, "tomato.yaml"))
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)
	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	if reg != nil {
		eng.Adapters = reg
	}
	return eng
}

// TestEmitStatusSkipsWithoutTaskRef verifies the status hook is a no-op when no
// task.json exists (the task step is last in the default workflow, so earlier
// steps must not error or block on a missing task).
func TestEmitStatusSkipsWithoutTaskRef(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "adapter.log")
	reg := adapter.NewRegistry()
	reg.Set("task", &adapter.Bridge{Bin: fakeAdapterScript(t, t.TempDir(), logPath)})

	eng := newEngineForTest(t, reg)
	featureDir := t.TempDir() // no task.json here

	eng.emitStatus(featureDir, "specified")

	if data, _ := os.ReadFile(logPath); len(data) != 0 {
		t.Errorf("adapter should NOT be called without a task_ref; log:\n%s", string(data))
	}
}

// TestEmitStatusCallsUpdateStatus verifies the hook calls update-status with the
// task_ref and status once task.json exists.
func TestEmitStatusCallsUpdateStatus(t *testing.T) {
	scriptDir := t.TempDir()
	logPath := filepath.Join(scriptDir, "adapter.log")
	reg := adapter.NewRegistry()
	reg.Set("task", &adapter.Bridge{Bin: fakeAdapterScript(t, scriptDir, logPath)})

	eng := newEngineForTest(t, reg)
	featureDir := t.TempDir()
	if err := steps.WriteTaskRef(featureDir, steps.TaskRef{TaskRef: "ISSUE-9"}); err != nil {
		t.Fatal(err)
	}

	eng.emitStatus(featureDir, "implemented")

	log := func() string { d, _ := os.ReadFile(logPath); return string(d) }()
	if !strings.Contains(log, "argv: update-status") {
		t.Errorf("expected update-status call; log:\n%s", log)
	}
	if !strings.Contains(log, "ISSUE-9") || !strings.Contains(log, "implemented") {
		t.Errorf("payload missing task_ref/status; log:\n%s", log)
	}
}

// TestBuildRegistryExpandsEnv verifies adapter env values are expanded against
// the process environment (so "${VAR}" resolves).
func TestBuildRegistryExpandsEnv(t *testing.T) {
	t.Setenv("MY_TOKEN", "secret-123")
	cfg := &config.Config{
		Adapters: map[string]config.AdapterDef{
			"gh": {Bin: "gh-adapter", Env: map[string]string{"TOKEN": "${MY_TOKEN}"}},
		},
		Roles: map[string]string{"pr": "gh"},
	}
	reg := BuildRegistry(cfg)
	br := reg.For("pr")
	if br == nil {
		t.Fatal("expected a bridge for the pr role")
	}
	if br.Env["TOKEN"] != "secret-123" {
		t.Errorf("env not expanded: TOKEN=%q, want secret-123", br.Env["TOKEN"])
	}
}

// TestBuildRegistryFallback verifies TOMATO_ADAPTER_BIN serves the built-in
// roles when no roles are configured.
func TestBuildRegistryFallback(t *testing.T) {
	t.Setenv("TOMATO_ADAPTER_BIN", "/usr/bin/myadapter")
	cfg := &config.Config{} // no roles
	reg := BuildRegistry(cfg)
	for _, role := range []string{"pr", "task", "review"} {
		br := reg.For(role)
		if br == nil || br.Bin != "/usr/bin/myadapter" {
			t.Errorf("role %q should fall back to TOMATO_ADAPTER_BIN, got %v", role, br)
		}
	}
}

func TestEngineLoadsDefaultWorkflow(t *testing.T) {
	dir := t.TempDir()

	cfg := config.Default()
	config.Save(cfg, filepath.Join(dir, "tomato.yaml"))
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(eng.Workflows) != 1 {
		t.Errorf("expected 1 workflow, got %d", len(eng.Workflows))
	}

	wf := eng.Workflows["default"]
	if len(wf.Steps) != 7 {
		t.Errorf("expected 7 steps in default workflow, got %d", len(wf.Steps))
	}
}

func TestEngineHasWorkflow(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
workflows:
  default:
    steps: [spec, design, impl, pr, review, test, task]
  hotfix:
    steps: [spec, impl, review]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	if !eng.HasWorkflow("hotfix") {
		t.Error("engine should have hotfix workflow")
	}
	if eng.HasWorkflow("nonexistent") {
		t.Error("engine should not have nonexistent workflow")
	}
}

func TestGetSteps(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
workflows:
  quick:
    steps: [spec, impl, task]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	steps := eng.GetSteps("quick")
	if len(steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(steps))
	}
	if steps[0] != "spec" {
		t.Errorf("expected first step 'spec', got '%s'", steps[0])
	}
}

func TestGetStepsNonexistent(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `workflows: { default: { steps: [spec] } }`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, _ := NewEngine(dir)
	steps := eng.GetSteps("ghost")
	if steps != nil {
		t.Error("expected nil for nonexistent workflow")
	}
}

func TestResolveModel(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
models:
  default: openai/gpt-5
  steps:
    spec: anthropic/claude-sonnet-4
    impl: deepseek/deepseek-4pro

workflows:
  default:
    steps: [spec, impl]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	model := eng.resolveModel("spec")
	if model != "anthropic/claude-sonnet-4" {
		t.Errorf("expected anthropic/claude-sonnet-4, got %s", model)
	}

	model = eng.resolveModel("nonexistent-step")
	if model != "openai/gpt-5" {
		t.Errorf("expected fallback to openai/gpt-5, got %s", model)
	}
}

func TestStepConfigUsesRoutedProviderModelOverAnthropicEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "https://glm.example.com")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "glm-token")
	t.Setenv("ANTHROPIC_MODEL", "glm-5.2")

	dir := t.TempDir()
	cfg := config.Default()
	cfg.Models.Steps["design"] = "deepseek/deepseek-v4-pro"
	cfg.Providers["deepseek"] = config.ProviderConnectionConfig{
		BaseURL:   "https://deepseek.example.com",
		AuthToken: "deepseek-token",
		Model:     "deepseek-v4-pro",
	}
	config.Save(cfg, filepath.Join(dir, "tomato.yaml"))
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	stepCfg := eng.stepConfig(filepath.Join(dir, "docs", "specs", "f"), "f", "design")

	if stepCfg.ModelName != "deepseek/deepseek-v4-pro" {
		t.Fatalf("expected routed model deepseek/deepseek-v4-pro, got %s", stepCfg.ModelName)
	}
	if stepCfg.AnthropicURL != "https://deepseek.example.com" {
		t.Errorf("expected deepseek base url, got %s", stepCfg.AnthropicURL)
	}
	if stepCfg.AnthropicKey != "deepseek-token" {
		t.Errorf("expected deepseek token, got %s", stepCfg.AnthropicKey)
	}
	if stepCfg.AnthropicModel != "deepseek-v4-pro" {
		t.Errorf("expected deepseek provider model, got %s", stepCfg.AnthropicModel)
	}
}

func TestEngineBudgetTrackerInitialized(t *testing.T) {
	dir := t.TempDir()

	cfg := config.Default()
	config.Save(cfg, filepath.Join(dir, "tomato.yaml"))
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	if eng.Tracker == nil {
		t.Fatal("expected budget tracker to be initialized")
	}

	// Verify budget was loaded from config
	if eng.Tracker.OnExceed() != "warn" {
		t.Errorf("expected on_exceed 'warn', got '%s'", eng.Tracker.OnExceed())
	}
}

func TestEngineRunFailsOnNonexistentWorkflow(t *testing.T) {
	dir := t.TempDir()

	cfg := config.Default()
	config.Save(cfg, filepath.Join(dir, "tomato.yaml"))
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	err = eng.Run("nonexistent-workflow")
	if err == nil {
		t.Error("expected error for nonexistent workflow")
	}
}

// TestAskOnFailNonInteractiveAborts verifies that on_fail=ask does NOT block on
// stdin in a non-interactive context (e.g. CI) — it aborts with a clear error
// instead of hanging or silently accepting.
func TestAskOnFailNonInteractiveAborts(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	config.Save(cfg, filepath.Join(dir, "tomato.yaml"))
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)
	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	// A regular file is not a character device, so it models non-interactive
	// input. The call must return promptly with an error, never block.
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	done := make(chan error, 1)
	go func() { done <- eng.askOnFail(f) }()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error in non-interactive ask mode, got nil (would silently accept)")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("askOnFail blocked on non-interactive stdin (should fail fast)")
	}
}

func TestRunOptionsFromSkipsEarlierSteps(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
workflows:
  default:
    steps: [spec, design, impl]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	steps := eng.planSteps("default", RunOptions{From: "design"})
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].Name != "design" || steps[1].Name != "impl" {
		t.Fatalf("unexpected planned steps: %#v", steps)
	}
}

func TestRunOptionsFastUsesSingleFastStep(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
workflows:
  default:
    steps:
      - design
      - pr
      - review_loop: { max_rounds: 3, on_fail: stop }
      - test
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	steps := eng.planSteps("default", RunOptions{Fast: true})
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %#v", steps)
	}
	if steps[0].Name != "fast" || steps[1].Name != "pr" {
		t.Fatalf("unexpected fast plan: %#v", steps)
	}
}

func TestRunOptionsFromUnknownStep(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `workflows: { default: { steps: [spec, design] } }`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = eng.planStepsChecked("default", RunOptions{From: "missing"})
	if err == nil {
		t.Fatal("expected unknown --from step error")
	}
}

func TestRunOptionsResumeStartsAtFailedStep(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `workflows: { default: { steps: [spec, design, impl, test] } }`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := state.Save(dir, state.WorkflowState{
		Workflow:       "default",
		Feature:        eng.Feature,
		FailedStep:     "impl",
		CompletedSteps: []string{"spec", "design"},
	}); err != nil {
		t.Fatal(err)
	}

	planned, err := eng.planStepsChecked("default", RunOptions{Resume: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(planned) != 2 || planned[0].Name != "impl" || planned[1].Name != "test" {
		t.Fatalf("expected [impl, test], got %#v", planned)
	}
}

func TestRunOptionsResumeNoFailedStepErrors(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `workflows: { default: { steps: [spec, design] } }`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	// State exists but records no failed step — nothing to resume from.
	if err := state.Save(dir, state.WorkflowState{
		Workflow:       "default",
		Feature:        eng.Feature,
		CompletedSteps: []string{"spec", "design"},
	}); err != nil {
		t.Fatal(err)
	}

	_, err = eng.planStepsChecked("default", RunOptions{Resume: true})
	if err == nil {
		t.Fatal("expected error when resuming with no failed step recorded")
	}
}

func TestRunOptionsFromAndResumeMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `workflows: { default: { steps: [spec, design] } }`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = eng.planStepsChecked("default", RunOptions{From: "design", Resume: true})
	if err == nil {
		t.Fatal("expected error when --from and --resume are used together")
	}
}

func TestEngineCustomBudgetInConfig(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
budget:
  mode: quality
  global_per_run: 999999
  per_step:
    spec: 50000
  on_exceed: fail
  degrade_to: openai/gpt-5

workflows:
  default:
    steps: [spec]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	if eng.Config.Budget.Mode != "quality" {
		t.Errorf("expected budget mode 'quality', got '%s'", eng.Config.Budget.Mode)
	}
	if eng.Config.Budget.GlobalPerRun != 999999 {
		t.Errorf("expected global_per_run 999999, got %d", eng.Config.Budget.GlobalPerRun)
	}
	if eng.Config.Budget.OnExceed != "fail" {
		t.Errorf("expected on_exceed 'fail', got '%s'", eng.Config.Budget.OnExceed)
	}
}

// fakeStep returns a StepFunc that always yields the given result. Registered
// under names not used by any real step (alpha/beta/gamma) so it never clashes
// with the built-in registry.
func fakeStep(result *model.StepResult) steps.StepFunc {
	return func(*steps.StepConfig, []string) *model.StepResult {
		return result
	}
}

func TestRunWithOptionsPersistsStateOnFailure(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `workflows: { default: { steps: [alpha, beta, gamma] } }`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	steps.Register("alpha", fakeStep(&model.StepResult{StepName: "alpha", Success: true, RunID: "r-alpha"}))
	steps.Register("beta", fakeStep(&model.StepResult{StepName: "beta", Success: false, Error: "boom"}))
	gammaRan := false
	steps.Register("gamma", func(*steps.StepConfig, []string) *model.StepResult {
		gammaRan = true
		return &model.StepResult{StepName: "gamma", Success: true}
	})

	err = eng.RunWithOptions("default", RunOptions{})
	if err == nil {
		t.Fatal("expected workflow to fail when beta fails")
	}
	if gammaRan {
		t.Error("gamma should not run after beta fails")
	}

	st, err := state.Load(dir, "default", eng.Feature)
	if err != nil {
		t.Fatalf("expected state persisted on failure: %v", err)
	}
	if st.FailedStep != "beta" {
		t.Errorf("expected FailedStep=beta, got %q", st.FailedStep)
	}
	if len(st.CompletedSteps) != 1 || st.CompletedSteps[0] != "alpha" {
		t.Errorf("expected CompletedSteps=[alpha], got %#v", st.CompletedSteps)
	}
}

func TestRunWithOptionsClearsStateOnCompletion(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `workflows: { default: { steps: [alpha, beta] } }`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Pre-existing state from a prior failed run must be cleared once the
	// workflow completes successfully — otherwise a later --resume would
	// re-run from a stale failed step.
	if err := state.Save(dir, state.WorkflowState{
		Workflow:   "default",
		Feature:    eng.Feature,
		FailedStep: "alpha",
	}); err != nil {
		t.Fatal(err)
	}

	steps.Register("alpha", fakeStep(&model.StepResult{StepName: "alpha", Success: true, RunID: "r-alpha"}))
	steps.Register("beta", fakeStep(&model.StepResult{StepName: "beta", Success: true, RunID: "r-beta"}))

	if err := eng.RunWithOptions("default", RunOptions{}); err != nil {
		t.Fatal(err)
	}

	if _, err := state.Load(dir, "default", eng.Feature); err == nil {
		t.Fatal("expected state to be cleared after successful completion")
	}
}

// TestRunWithOptionsDispatchesCustomStep verifies that a workflow step that is
// not a registered built-in but IS declared in custom_steps is executed via
// customstep.Run (writing its declared output) rather than erroring as unknown.
// The LLM stream is injected so no real provider is contacted.
func TestRunWithOptionsDispatchesCustomStep(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "prompts"), 0755)
	os.WriteFile(filepath.Join(dir, "prompts", "echo.md"), []byte("say hello"), 0644)

	yamlContent := `
custom_steps:
  myecho:
    prompt: prompts/echo.md
    outputs: [out/echo.txt]
workflows:
  default:
    steps: [myecho]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	eng.LLMStream = func(_ []runner.Message, onChunk func(string)) error {
		onChunk("echo-output")
		return nil
	}

	if err := eng.RunWithOptions("default", RunOptions{}); err != nil {
		t.Fatalf("expected custom step workflow to succeed: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(dir, "out", "echo.txt"))
	if err != nil {
		t.Fatalf("expected custom step output written: %v", err)
	}
	if string(out) != "echo-output" {
		t.Errorf("expected output 'echo-output', got %q", string(out))
	}
}

// TestRunWithOptionsUnknownStepStillErrors is a regression guard: a step that
// is neither a registered built-in nor a custom step must still surface the
// "unknown step" error rather than silently passing.
func TestRunWithOptionsUnknownStepStillErrors(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `workflows: { default: { steps: [ghoststep] } }`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	err = eng.RunWithOptions("default", RunOptions{})
	if err == nil {
		t.Fatal("expected error for step that is neither built-in nor custom")
	}
}

// initGitRepoForEngine creates a git repo in dir with one initial commit so
// HEAD resolves and branch operations work. Mirrors the production layout.
func initGitRepoForEngine(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@t.com"},
		{"config", "user.name", "T"},
		{"checkout", "-b", "main"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %s", strings.Join(args, " "), string(out))
		}
	}
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".tomato/\n"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0644)
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %s", strings.Join(args, " "), string(out))
		}
	}
}

// TestRunWithOptionsCommitsFeatureArtifacts verifies that after a step writes
// artifacts under docs/specs/<feature>/, those artifacts are committed to git
// (only the feature dir, not unrelated working-tree changes). The .tomato/
// runtime dir must NOT be committed.
func TestRunWithOptionsCommitsFeatureArtifacts(t *testing.T) {
	dir := t.TempDir()
	initGitRepoForEngine(t, dir)

	yamlContent := `
workflows:
  default:
    steps: [alpha]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	// alpha writes a docs artifact into the feature dir, plus an unrelated
	// working-tree file that must NOT be swept into the artifact commit.
	featureDir := eng.featureDir()
	steps.Register("alpha", func(cfg *steps.StepConfig, _ []string) *model.StepResult {
		os.MkdirAll(cfg.FeatureDir, 0755)
		os.WriteFile(filepath.Join(cfg.FeatureDir, "architecture.md"), []byte("# arch"), 0644)
		// Unrelated working-tree change — should remain untracked.
		os.WriteFile(filepath.Join(cfg.RepoDir, "unrelated.txt"), []byte("nope"), 0644)
		return &model.StepResult{StepName: "alpha", Success: true, RunID: "r-alpha"}
	})

	if err := eng.RunWithOptions("default", RunOptions{}); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	// After the workflow, tomato switches back to main. The artifact commit
	// lives on the tomato/<feature> branch. Verify it's tracked there.
	branches, _ := exec.Command("git", "-C", dir, "branch", "--list", "tomato/*").Output()
	branchLine := strings.TrimSpace(string(branches))
	if branchLine == "" {
		t.Fatal("expected a tomato/ feature branch to exist")
	}
	featureBranch := strings.TrimPrefix(branchLine, "* ")
	featureBranch = strings.TrimSpace(featureBranch)

	// Check the artifact is tracked on the feature branch.
	out, err := exec.Command("git", "-C", dir, "ls-tree", "-r", "--name-only", featureBranch).CombinedOutput()
	if err != nil {
		t.Fatalf("listing feature branch tree: %s", string(out))
	}
	archRel, _ := filepath.Rel(dir, filepath.Join(featureDir, "architecture.md"))
	if !strings.Contains(string(out), archRel) {
		t.Errorf("expected %s tracked on feature branch %s, tree:\n%s", archRel, featureBranch, string(out))
	}

	// The unrelated file must NOT be tracked on the feature branch.
	if strings.Contains(string(out), "unrelated.txt") {
		t.Errorf("unrelated.txt should NOT be committed on feature branch")
	}

	// Local main must NOT carry the artifact (it's on the feature branch).
	mainTree, _ := exec.Command("git", "-C", dir, "ls-tree", "-r", "--name-only", "main").CombinedOutput()
	if strings.Contains(string(mainTree), archRel) {
		t.Errorf("artifact should NOT be on main, but it is:\n%s", string(mainTree))
	}

	// .tomato/ must never be committed on either branch.
	if strings.Contains(string(out), ".tomato/") {
		t.Errorf(".tomato/ should NOT be committed on feature branch")
	}
}

// TestRunWithOptionsSwitchesToFeatureBranchAndBack verifies that:
//   - At the start of a workflow run, tomato switches to a tomato/<feature>
//     branch based on origin/main (not committing on local main).
//   - After the workflow completes, tomato switches back to main and syncs
//     to origin/main so local main stays clean.
//   - The feature branch carries the committed artifacts.
func TestRunWithOptionsSwitchesToFeatureBranchAndBack(t *testing.T) {
	dir := t.TempDir()
	initGitRepoForEngine(t, dir)

	// Add a bare remote so origin/main exists and push works.
	bare := t.TempDir()
	runGitCmd2(t, bare, "init", "--bare")
	runGitCmd2(t, dir, "remote", "add", "origin", bare)
	runGitCmd2(t, dir, "push", "-u", "origin", "main")

	yamlContent := `
workflows:
  default:
    steps: [alpha]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	steps.Register("alpha", func(cfg *steps.StepConfig, _ []string) *model.StepResult {
		// Must be on a tomato/ branch, NOT on main.
		branch := currentGitBranch(t, cfg.RepoDir)
		if !strings.HasPrefix(branch, "tomato/") {
			t.Errorf("expected to be on tomato/ branch during workflow, got %q", branch)
		}
		os.MkdirAll(cfg.FeatureDir, 0755)
		os.WriteFile(filepath.Join(cfg.FeatureDir, "architecture.md"), []byte("# arch"), 0644)
		return &model.StepResult{StepName: "alpha", Success: true, RunID: "r-alpha"}
	})

	if err := eng.RunWithOptions("default", RunOptions{}); err != nil {
		t.Fatalf("workflow failed: %v", err)
	}

	// After workflow: must be back on main.
	branch := currentGitBranch(t, dir)
	if branch != "main" {
		t.Errorf("expected to be back on main after workflow, got %q", branch)
	}

	// Local main must be in sync with origin/main (no divergence).
	localMain, _ := exec.Command("git", "-C", dir, "rev-parse", "main").Output()
	originMain, _ := exec.Command("git", "-C", dir, "rev-parse", "origin/main").Output()
	if strings.TrimSpace(string(localMain)) != strings.TrimSpace(string(originMain)) {
		t.Errorf("local main (%s) should match origin/main (%s) after workflow",
			strings.TrimSpace(string(localMain)), strings.TrimSpace(string(originMain)))
	}

	// The feature branch must exist and carry the artifact commit.
	out, _ := exec.Command("git", "-C", dir, "branch", "--list", "tomato/*").Output()
	if !strings.Contains(string(out), "tomato/") {
		t.Errorf("expected a tomato/ feature branch to exist, got %q", string(out))
	}
}

// TestRunWithOptionsStaysOnFeatureBranchOnFailure verifies that when a workflow
// step fails, tomato does NOT switch back to main — the user stays on the
// feature branch so they can inspect/fix.
func TestRunWithOptionsStaysOnFeatureBranchOnFailure(t *testing.T) {
	dir := t.TempDir()
	initGitRepoForEngine(t, dir)

	bare := t.TempDir()
	runGitCmd2(t, bare, "init", "--bare")
	runGitCmd2(t, dir, "remote", "add", "origin", bare)
	runGitCmd2(t, dir, "push", "-u", "origin", "main")

	yamlContent := `
workflows:
  default:
    steps: [alpha, beta]
`
	os.WriteFile(filepath.Join(dir, "tomato.yaml"), []byte(yamlContent), 0644)
	os.MkdirAll(filepath.Join(dir, ".tomato", "runs"), 0755)

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}

	steps.Register("alpha", fakeStep(&model.StepResult{StepName: "alpha", Success: true, RunID: "r-alpha"}))
	steps.Register("beta", fakeStep(&model.StepResult{StepName: "beta", Success: false, Error: "boom"}))

	_ = eng.RunWithOptions("default", RunOptions{})

	// Must stay on the feature branch, not main.
	branch := currentGitBranch(t, dir)
	if branch == "main" {
		t.Error("expected to stay on feature branch after failure, but on main")
	}
}

func runGitCmd2(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %s", strings.Join(args, " "), string(out))
	}
}

func currentGitBranch(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatalf("getting current branch: %v", err)
	}
	return strings.TrimSpace(string(out))
}
