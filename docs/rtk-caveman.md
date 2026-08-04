# RTK + Caveman Compression

RTK + Caveman is a two-stage compression pipeline inspired by OmniRoute's stacked compression mode.

Source reference: https://github.com/diegosouzapw/OmniRoute

## Goal

Reduce token use in heavy tool sessions without hiding the information needed to debug, review, or resume work.

For tomato, this mainly targets Claude Code runs that produce lots of shell output, tool events, test logs, and final summaries.

## Pipeline

```text
raw tool/session output
  -> RTK compression
  -> Caveman compression
  -> compact agent context/log
```

## Stage 1: RTK

RTK compresses structured and semi-structured tool output.

It should run first because command output is usually the largest and noisiest part of a coding session.

Good candidates:

- build logs
- test output
- git diffs and status
- grep/search results
- tool call JSON
- long stack traces

Keep:

- command name and exit code
- failing test names
- error lines
- changed file names
- stack trace top/bottom frames
- enough nearby context to act

Drop or fold:

- repeated progress lines
- long successful test listings
- duplicate stack frames
- unchanged diff context
- verbose dependency/install noise

## Stage 2: Caveman

Caveman compresses prose after RTK has reduced tool output.

It should shorten natural-language agent text while preserving decisions and next actions.

Examples:

```text
Before:
I inspected the repository and found that the fast workflow is currently using the normal cache path, which means it can skip Claude.

After:
Found fast using cache; can skip Claude.
```

Keep:

- decisions
- blockers
- test commands/results
- files changed
- risks
- next action

Drop:

- apologies
- filler
- repeated rationale
- vague status text

## Stacked Savings

Savings compound, they do not add.

```text
combined = 1 - (1 - rtk_savings) * (1 - caveman_savings)
```

Example:

```text
RTK saves 80%
Caveman saves 46% of what remains

combined = 1 - (1 - 0.80) * (1 - 0.46)
         = 89.2%
```

That is why a heavy tool session can report about 89% total token reduction even though no single stage saves 89% by itself.

## Tomato Fit

Minimal tomato version:

1. Format Claude stream logs into blocks.
2. Compress tool results before storing or feeding them back into prompts.
3. Compress final prose reports before reuse.
4. Keep raw output recoverable from `.tomato/runs/...` when possible.

Suggested default:

```text
tool output: RTK
agent summaries: Caveman
stored artifacts: uncompressed unless explicitly marked compact
```

## Safety Rules

- Never drop failing command exit codes.
- Never drop the first error line.
- Never drop file paths from failures.
- Never compress source patches unless raw diff is still available.
- Prefer no compression over misleading compression.
