package steps

import (
	"path/filepath"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

var TestPrompt = `You are tomato's test engineer.

Generate a concrete test plan and test code suggestions based on the design and implementation output.

Architecture:
{{.architecture.md}}

Implementation Plan:
{{.implementation.md}}

Implementation Output / Source Summary:
{{.impl-output.md}}

Output markdown with this exact structure:

# Test Plan

## 1. Scope
What behavior is covered by these tests?

## 2. Unit Tests
For each unit test, provide:
- Test name
- Given / When / Then
- Expected assertion
- Exact file where the test should live

## 3. Integration Tests
Include end-to-end or multi-component flows.

## 4. Error and Edge Cases
Cover invalid input, missing files, auth failures, network failures, and partial success.

## 5. Regression Tests
List tests that protect previously working behavior.

## 6. Test Code
Provide complete test code blocks where possible.

## 7. Commands
List exact commands to run, for example:
- go test ./...
- go test ./pkg/foo -run TestName -v

## 8. Coverage Gaps
List behavior that cannot be tested automatically yet and why.

Rules:
- Tests must be deterministic.
- Prefer small unit tests plus one integration test over a single broad test.
- Do not mock everything; only mock external services, clocks, network, or filesystem when necessary.
- Every blocking requirement from the PRD/design should have at least one test.
- If source code is unavailable, produce a test plan rather than pretending to know exact APIs.`

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
		cfg.BudgetTracker,
	)
}