# RTK + Caveman Implementation Design

This document turns `docs/rtk-caveman.md` into a concrete tomato implementation plan.

## Goal

Add token-saving compression for heavy Claude Code sessions while keeping raw debugging data recoverable.

The first implementation should be boring:

- RTK compresses tool and command output.
- Caveman compresses final prose summaries.
- Raw artifacts stay unchanged.
- Raw run data remains available under `.tomato/runs/<run-id>/`.

## Non-Goals

- No new external dependency.
- No semantic LLM-based compression.
- No compression for source patches or generated code.
- No change to model routing.

## Current Touch Points

`pkg/llm/anthropic.go`

- Reads Claude `stream-json`.
- Logs `tool_use`, `tool_result`, and `TOMATO_STEP` progress.
- This is the best place for live RTK log display.

`pkg/runner/runner.go`

- Builds prompts.
- Receives final LLM text.
- Writes artifacts and `.tomato/runs/<run-id>/meta.json`.
- This is the best place to record compressed/raw size metadata.

`pkg/budget/tracker.go`

- Estimates tokens with the existing rough `chars / 4` heuristic.
- Reuse this for compression savings estimates.

## Data Flow

```text
Claude stream-json
  -> parse event
  -> raw event kept in memory for final parsing
  -> RTK formats/compresses console log
  -> final LLM result
  -> Caveman optionally compresses summary copy
  -> runner writes artifacts and run metadata
```

## RTK Design

RTK is a small deterministic string compressor for tool output.

Input:

- tool name
- tool input
- tool result text
- status / error flag

Output:

- formatted console block
- compressed text
- original byte count
- compressed byte count

Rules:

- Always keep command name.
- Always keep exit/error status.
- Always keep first error line.
- Always keep file paths that appear near errors.
- For successful long output, keep head and tail.
- Collapse repeated identical lines.
- Limit per tool result to a small default, for example 120 lines or 12 KB.

Suggested package:

```text
pkg/compress/
  rtk.go
  caveman.go
```

Minimal API:

```go
type Result struct {
    Text       string
    RawBytes   int
    KeptBytes  int
    Truncated  bool
}

func RTKToolOutput(tool, text string, isError bool) Result
func Caveman(text string) Result
```

## Caveman Design

Caveman is deterministic prose trimming.

Input:

- final agent summary
- review/fix/test report snippets

Output:

- compact prose suitable for logs or future prompt context

Rules:

- Keep bullets with file paths.
- Keep test command and pass/fail result.
- Keep blockers and risks.
- Drop filler phrases.
- Collapse blank lines.
- Trim long paragraphs to one or two sentences.

Do not apply Caveman to:

- code blocks
- diffs
- JSON that downstream tools parse
- user-facing artifacts unless explicitly requested

## Storage

Keep raw output recoverable.

Proposed run layout:

```text
.tomato/runs/<run-id>/
  meta.json
  artifacts/
  logs/
    claude.raw.jsonl
    claude.compact.log
```

First version can skip `claude.raw.jsonl` if wiring raw stream capture is too invasive, because stdout is already buffered in `ClaudeCLIProvider.Stream`. The important first step is `claude.compact.log`.

Add optional metadata fields later:

```json
{
  "compression": {
    "rtk_raw_bytes": 100000,
    "rtk_kept_bytes": 18000,
    "caveman_raw_bytes": 8000,
    "caveman_kept_bytes": 4300
  }
}
```

## CLI Behavior

Default:

- Console shows compact RTK logs.
- Artifacts remain raw.
- History commands continue to work.

Optional future flag:

```text
tomato run --compact-logs=false
```

Do not add this flag until someone needs it. Default compact logs are enough.

## Implementation Steps

1. Add `pkg/compress` with `RTKToolOutput` and `Caveman`.
2. Unit test RTK against:
   - successful long output
   - failing output
   - repeated lines
   - file path near error
3. Unit test Caveman against:
   - filler prose
   - bullets with file paths
   - code blocks preserved
4. Use RTK in `logClaudeUser` for `tool_result` blocks.
5. Write compact Claude log to `.tomato/runs/<run-id>/logs/claude.compact.log`.
6. Add byte-saving fields to run metadata only after compact logs are stable.

## Tests

Minimum tests:

- `pkg/compress` unit tests.
- Existing `TestClaudeCLIProviderPrintsClaudeLogs` updated to assert compressed multiline output.
- Runner test that a run can write compact log data without changing artifacts.

No integration test with real Claude is required.

## Risks

- Over-compression can hide useful context.
- Tool output may contain secrets; compression must not duplicate or expand them.
- JSON and code blocks must not be Caveman-compressed.

Mitigation:

- Keep raw output recoverable.
- Start with deterministic rules only.
- Prefer under-compression over misleading output.
