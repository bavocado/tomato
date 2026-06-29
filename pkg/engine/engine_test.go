package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bavocado/tomato/pkg/adapter"
	"github.com/bavocado/tomato/pkg/config"
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
