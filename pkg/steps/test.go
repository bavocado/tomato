package steps

import (
	"path/filepath"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

var TestPrompt = `You are a QA engineer. Generate test cases and test code for the following implementation.

Design:
{{.architecture.md}}
{{.implementation.md}}

Source code:
{{.impl_code}}

Output test files covering:
- Unit tests for core functions
- Edge cases and boundary conditions
- Integration test for the main flow`

func init() {
	Register("test", runTest)
}

func runTest(cfg *StepConfig, args []string) *model.StepResult {
	inputFiles := []string{
		filepath.Join(cfg.FeatureDir, "architecture.md"),
		filepath.Join(cfg.FeatureDir, "implementation.md"),
		filepath.Join(cfg.FeatureDir, "impl-output.md"),
	}
	return runner.Execute(
		"test",
		TestPrompt,
		inputFiles,
		[]string{filepath.Join(cfg.FeatureDir, "test-report.md")},
		cfg.RepoDir,
		cfg.ModelName,
		cfg.LLMStream,
		cfg.PromptVersion,
	)
}