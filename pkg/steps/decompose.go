package steps

import (
	"path/filepath"

	"github.com/bavocado/tomato/pkg/model"
	"github.com/bavocado/tomato/pkg/runner"
)

var DecomposePrompt = `You are tomato's architecture decomposition analyst.

Your job is to break the attached design document into N independently-implementable
sub-features. Each sub-feature must be small enough to run through tomato's
spec -> design -> impl -> pr -> review -> test workflow on its own, without being
blocked by other sub-features except via explicit dependencies.

Source design document:
{{.source-design.md}}

Methodology constraints (follow strictly):
1. Vertical slice first: each sub-feature must deliver end-to-end observable value,
   crossing necessary architecture layers. NEVER produce pure horizontal-layer slices
   ("just the DB schema", "just the UI") unless explicitly tagged enabler or spike,
   with a note on which business feature it supports.
2. Each sub-feature satisfies INVEST: Independent, Valuable, Estimable, Small, Testable.
3. depends_on MUST form a DAG (no cycles); only reference already-defined, earlier ids.
   After producing the list, self-check for cycles and report dag_check.
4. If implementation is uncertain, split out a spike first (is_spike: true + timebox +
   questions to answer) and make it a dependency of the relevant slices. Spike is the
   last resort.
5. Freeze cross-boundary interfaces as independent "contract" sub-features; consumers
   depend on them.
6. Split along domain/responsibility boundaries (C4 container level) first, then
   interfaces, then performance.
7. Tag each sub-feature with slice_type, c4_level, priority (MoSCoW).
8. For each sub-feature write goal, user_value, acceptance_criteria (testable),
   out_of_scope.
9. Meta-pattern: find the core complexity -> list variants -> reduce each slice to one
   variant. Prefer splits that let you drop low-value slices.

Two-stage output (inside this single decomposition.md):
- Stage 1: thin table for quick DAG review, plus dag_check, critical_path, spikes.
- Stage 2: details per id (goal, user_value, acceptance_criteria, complexity, out_of_scope).

Reflection self-review (apply revisions before finalizing):
- Is each slice independently verifiable (not a pseudo-story)?
- Is the dependency graph acyclic, with no redundant edges?
- Are there low-value slices to drop?
- Are all spikes correctly placed before their dependents?
- Any untagged horizontal slices?
- Is granularity balanced (no slice significantly larger than the others)?

Output format - produce this exact markdown structure:

# Decomposition: <source doc title>

## 0. Meta
- source: source-design.md
- total_features: <N>
- dag_check: passed
- critical_path: [<ids along the longest dependency chain>]
- spikes: [<ids with is_spike: true>]

## 1. Thin table
| id | title | slice_type | c4_level | priority | depends_on | is_spike |
|----|-------|-----------|----------|----------|------------|----------|
| F-001 | ... | ... | ... | ... | ... | ... |

## 2. Details
### F-001 <title>
- goal: ...
- user_value: ...
- acceptance_criteria: ...
- complexity: ...
- out_of_scope: ...

## 3. Machine-readable manifest (--apply parses ONLY this block)
` + "```yaml" + `
source: source-design.md
total_features: <N>
dag_check: passed
critical_path: [<ids>]
spikes: [<ids>]
features:
  - id: F-001
    title: ...
    goal: ...
    user_value: ...
    slice_type: <workflow|operations_crud|business_rule|data_variation|data_entry|major_effort|simple_complex|defer_performance|spike|contract|enabler>
    c4_level: <context|container|component|code>
    priority: <must|should|could|won't>
    depends_on: []
    acceptance_criteria:
      - ...
    complexity: <S|M|L>
    is_spike: false
    timebox: ""
    out_of_scope:
      - ...
    open_questions:
      - ...
` + "```" + `

Rules:
- The yaml block in section 3 is authoritative; --apply parses only it.
- ids are stable: F-001, F-002, ... in definition order.
- depends_on references must point to earlier ids (DAG, no cycles).
- Every sub-feature must have at least one acceptance_criterion.
- Spike sub-features (is_spike: true) must have a non-empty timebox.
- Section 1 table and section 2 details are human-readable summaries of section 3;
  keep them consistent with the yaml block.`

func init() {
	Register("decompose", runDecompose)
}

func runDecompose(cfg *StepConfig, args []string) *model.StepResult {
	// Input: source design doc (source-design.md); Output: decomposition.md.
	// Single output file -> no ---TOMATO-ARTIFACT--- marker needed.
	sourcePath := filepath.Join(cfg.FeatureDir, "source-design.md")
	outPath := filepath.Join(cfg.FeatureDir, "decomposition.md")
	return runner.Execute(
		"decompose",
		DecomposePrompt,
		[]string{sourcePath},
		[]string{outPath},
		cfg.RepoDir,
		cfg.ModelName,
		cfg.LLMStream,
		cfg.PromptVersion,
		cfg.BudgetTracker,
	)
}