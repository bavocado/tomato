package config

import (
	"os"
	"strings"
)

// KarpathyGuidelines is the working-protocols content written into CLAUDE.md
// during `tomato init`. Derived from
// https://github.com/multica-ai/andrej-karpathy-skills (MIT). Embedded as a
// constant so tomato never depends on a network fetch at init time.
//
// The content is the guidelines body (frontmatter stripped) of the
// karpathy-guidelines skill. The leading "# Karpathy Guidelines" header also
// serves as the idempotency marker: init skips appending when it is already
// present in CLAUDE.md.
const KarpathyGuidelines = `# Karpathy Guidelines

Behavioral guidelines to reduce common LLM coding mistakes, derived from [Andrej Karpathy's observations](https://x.com/karpathy/status/2015883857489522876) on LLM coding pitfalls.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
` + "```" + `
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
` + "```" + `

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.
`

// karpathyMarker is the substring used to detect whether the guidelines are
// already present in an existing CLAUDE.md so init does not append them twice.
const karpathyMarker = "# Karpathy Guidelines"

// WriteCLAUDEMD writes the Karpathy guidelines into CLAUDE.md at the given path:
//   - File does not exist → write the guidelines as a new file.
//   - File exists and already contains the marker → skip (idempotent).
//   - File exists without the marker → append the guidelines with a separator.
//
// Returns the action taken: "created", "appended", or "skipped".
func WriteCLAUDEMD(path string) (string, error) {
	existing, err := os.ReadFile(path)
	if err == nil {
		if strings.Contains(string(existing), karpathyMarker) {
			return "skipped", nil
		}
		content := strings.TrimRight(string(existing), "\n") + "\n\n---\n\n" + KarpathyGuidelines
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return "", err
		}
		return "appended", nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.WriteFile(path, []byte(KarpathyGuidelines), 0644); err != nil {
		return "", err
	}
	return "created", nil
}
