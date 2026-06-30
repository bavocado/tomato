# 🍅 tomato

**AI software development workflow engine** for individual developers and small teams.

[![CI](https://github.com/bavocado/tomato/actions/workflows/ci.yml/badge.svg)](https://github.com/bavocado/tomato/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/bavocado/tomato)](https://github.com/bavocado/tomato/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/bavocado/tomato)](https://goreportcard.com/report/github.com/bavocado/tomato)
[![License](https://img.shields.io/github/license/bavocado/tomato)](LICENSE)

---

Turn a rough idea into a PR — through specs, design docs, implementation, code review, testing, and task sync — all with AI, all in your terminal.

```
▶ tomato run

  spec ──► prd.md
  task ──► issue created on Linear/Jira (enables status updates)
  design ──► architecture.md · ui-spec.md · implementation.md
  impl ──► source code
  pr ──► draft PR on GitHub
  review_loop ──► review → fix → review → PR ready / failed
  test ──► test files + report
```

---

## Features

| Capability | Detail |
|------------|--------|
| 🧠 **7 built-in steps** | `spec` · `design` · `impl` · `pr` · `review` · `test` · `task` |
| 🔁 **review_loop meta-step** | Up to 2 automatic fix rounds with severity-classified comments |
| 📝 **Document-driven** | Every artifact is markdown in git — readable, editable, reviewable |
| 🔧 **Customizable workflows** | Compose steps in `tomato.yaml`; each auto-registers as a CLI command |
| 🎯 **Multi-model BYOK** | Mix GPT-5 / GLM-5.2 / DeepSeek-4pro / Claude per step |
| ⚡ **Claude CLI integration** | Anthropic models run via `claude --print --permission-mode auto --effort high` |
| 💰 **Token budget control** | Per-step & per-run caps, 3 budget presets, local response caching |
| 🔌 **Adapter protocol** | Integrate with GitHub, Jira, Linear etc. via simple CLI subprocesses |
| 🗂️ **Architecture versioning** | Each impl archives the design trio, rewrites real architecture from code |
| 🏗️ **Pure CLI** | No daemon, no GUI, no database, no Docker — single Go binary |

---

## Quick Start

### Prerequisites

- **Go 1.22+**
- **`claude` CLI** (if using Anthropic models):

  ```bash
  npm install -g @anthropic-ai/claude-code
  ```

### Install

#### One-line install (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/bavocado/tomato/main/install.sh | bash
```

This installs:

- `tomato`
- `github-tomato-adapter`

Supported targets:

| OS | Architecture |
|----|--------------|
| macOS | arm64 / amd64 |
| Linux | arm64 / amd64 |

Optional environment variables:

```bash
# Install a specific version/tag instead of latest
VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/bavocado/tomato/main/install.sh | bash

# Install somewhere else
INSTALL_DIR=$HOME/bin curl -fsSL https://raw.githubusercontent.com/bavocado/tomato/main/install.sh | bash

# Skip GitHub adapter
INSTALL_ADAPTER=0 curl -fsSL https://raw.githubusercontent.com/bavocado/tomato/main/install.sh | bash
```

#### Go install

```bash
go install github.com/bavocado/tomato@latest
```

> `go install` only installs the main `tomato` binary. Use `install.sh` if you also want `github-tomato-adapter`.

#### Build from source

```bash
git clone https://github.com/bavocado/tomato.git
cd tomato
go build -o tomato .
go build -o github-tomato-adapter ./adapters/github-tomato-adapter
```

### Initialize a Project

```bash
cd your-project
tomato init
```

This creates:

- **`tomato.yaml`** — workflow configuration with default settings
- **`.tomato/runs/`** — runtime data directory (auto-added to `.gitignore`)

### Configure

Edit `tomato.yaml` with your API keys:

```yaml
models:
  default: glm/glm-5.2
  steps:
    spec:   glm/glm-5.2
    design: glm/glm-5.2
    impl:   deepseek/deepseek-v4-pro
    review: glm/glm-5.2
    test:   glm/glm-5.2

providers:
  glm:
    base_url: https://your-glm-claude-compatible-endpoint
    auth_token: your-glm-token
    model: glm-5.2
  deepseek:
    base_url: https://your-deepseek-claude-compatible-endpoint
    auth_token: your-deepseek-token
    model: deepseek-v4-pro
```

> tomato runs GLM / DeepSeek / Anthropic through the `claude` CLI. For each step, tomato sets `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, and `ANTHROPIC_MODEL` from `providers.<provider>` before launching `claude --print --permission-mode auto --effort high`.

### Run Your First Workflow

```bash
# Full default pipeline (7 steps)
tomato run

# Run a single step
tomato spec
tomato design

# Custom workflow (defined in tomato.yaml)
tomato hotfix

# Target a specific feature directory (docs/specs/<feature>/).
# Precedence: --feature flag > tomato.yaml `feature:` > git branch > "current-feature".
tomato run --feature login
tomato design --feature login
```

Or pin the feature in `tomato.yaml` so you don't pass `--feature` each time:

```yaml
feature: login
```

---

## Configuration Reference

### Full `tomato.yaml`

```yaml
# ── Model Routing ──────────────────────────────────
models:
  default: glm/glm-5.2              # fallback when a step has no specific model
  steps:
    spec:   glm/glm-5.2
    design: glm/glm-5.2
    impl:   deepseek/deepseek-v4-pro
    review: glm/glm-5.2
    test:   glm/glm-5.2

# ── Claude Code Provider Routing ─────────────────────
# GLM / DeepSeek / Anthropic are all executed through the `claude` CLI.
# tomato maps the selected provider into ANTHROPIC_* env vars for that subprocess.
providers:
  glm:
    base_url: https://your-glm-claude-compatible-endpoint
    auth_token: ""
    model: glm-5.2

  deepseek:
    base_url: https://your-deepseek-claude-compatible-endpoint
    auth_token: ""
    model: deepseek-v4-pro

  anthropic:
    base_url: https://api.anthropic.com
    auth_token: ""
    model: claude-sonnet-4-20250514

# Legacy compatibility: still supported, equivalent to providers.anthropic.
anthropic:
  base_url: https://api.anthropic.com
  auth_token: ""
  model: claude-sonnet-4-20250514

# ── Token Budget ─────────────────────────────────────
budget:
  mode: balanced                         # frugal | balanced | quality
  global_per_run: 300000                 # tokens (not dollars)
  per_step:
    spec: 50000
    design: 100000
    impl: 100000
    review: 30000
    test: 20000
  on_exceed: warn                        # fail | degrade | warn
  degrade_to: deepseek/deepseek-4pro     # model to fall back to when budget hit

# ── Adapters ─────────────────────────────────────────
adapters:
  github:
    bin: github-tomato-adapter           # executable on PATH
    env:
      GITHUB_TOKEN: "${GITHUB_TOKEN}"

roles:
  task: github        # `tomato task` uses the "github" adapter
  pr:   github        # `tomato pr` uses the same adapter
  review: github      # review_loop posts comments via this adapter

# ── Workflows ────────────────────────────────────────
workflows:
  default:
    steps:
      - spec
      - task          # create task early so status lifecycle can update it
      - design
      - impl
      - pr
      - review_loop: { max_rounds: 2, on_fail: stop }
      - test

  hotfix:
    steps: [spec, impl, pr, review]

  doc-only:
    steps: [spec, design]
```

### Model Providers

Model format: `provider/model`

| Provider | Example | How tomato runs it | Config |
|----------|---------|--------------------|--------|
| OpenAI | `openai/gpt-5` | Direct OpenAI-compatible HTTP | `OPENAI_API_KEY` |
| Zhipu (GLM) | `glm/glm-5.2` | `claude` CLI subprocess | `providers.glm.{base_url,auth_token,model}` → `ANTHROPIC_*` |
| DeepSeek | `deepseek/deepseek-v4-pro` | `claude` CLI subprocess | `providers.deepseek.{base_url,auth_token,model}` → `ANTHROPIC_*` |
| Anthropic | `anthropic/claude-sonnet-4` | `claude` CLI subprocess | `providers.anthropic` or legacy `anthropic` |

For Claude-CLI providers, tomato launches:

```bash
ANTHROPIC_BASE_URL=<provider.base_url> \
ANTHROPIC_AUTH_TOKEN=<provider.auth_token> \
ANTHROPIC_MODEL=<provider.model> \
claude --print --permission-mode auto --effort high --model <provider.model>
```

---

## Steps Reference

| Step | Command | Input | Output | API Key |
|------|---------|-------|--------|---------|
| **spec** | `tomato spec` | User's rough idea | `prd.md` | Per model config |
| **design** | `tomato design` | `prd.md` | `architecture.md` + `ui-spec.md` + `implementation.md` | Per model config |
| **impl** | `tomato impl` | design trio | Source code diff | Per model config |
| **pr** | `tomato pr` | Git working tree | Draft PR on GitHub/GitLab | Adapter (`GITHUB_TOKEN`) |
| **review** | `tomato review` | Code diff + design | `reviews/r<n>-comments.md` with severity labels | Per model config |
| **test** | `tomato test` | Code + design | Test files + report | Per model config |
| **task** | `tomato task` | spec/design | Issue/task on Linear/Jira/GitHub | Adapter (`GITHUB_TOKEN`) |

### review_loop Meta-Step

The `review_loop` wraps `review` and `impl` into a bounded convergence loop.

```
review_loop(max_rounds=2, on_fail=stop):
  ┌─ round 1: review → blocking issues?
  │    ├─ no  → mark-pr-ready ✓ PASSED
  │    └─ yes → impl --fix + update-pr
  │
  └─ round 2: review → blocking issues?
       ├─ no  → mark-pr-ready ✓ PASSED
       └─ yes → mark-pr-failed ❌ FAILED (pipeline stops)
```

- **`blocking`** severity → triggers fix round
- **`major` / `minor`** → posted to PR as comments, no fix triggered
- **`on_fail: stop`** (default) → pipeline exits with error
- **`on_fail: continue`** → marks failed but continues to `test`/`task`
- **`on_fail: ask`** → prompts user at terminal (accept / retry / abort)

---

## Artifact Layout

```
my-project/
├── tomato.yaml                        # configuration (in git)
├── .gitignore                         # created by init, includes .tomato/
│
├── docs/specs/<feature>/              ★ design artifacts (in git)
│   ├── prd.md                         ← always latest
│   ├── architecture.md                ← always latest (rewritten after impl with real arch)
│   ├── ui-spec.md                     ← always latest
│   ├── implementation.md              ← always latest
│   ├── pr.md                          ← PR ref + URL
│   ├── v1/                            ← archive after round 1 of design+impl
│   │   ├── architecture.md
│   │   ├── ui-spec.md
│   │   └── implementation.md
│   ├── v2/                            ← round 2 archive
│   └── reviews/
│       ├── r1-comments.md
│       ├── r2-comments.md
│       └── final-comments.md
│
├── .tomato/                           ★ runtime data (NOT in git)
│   ├── runs/<id>/meta.json            # step metadata, tokens, model, duration
│   ├── cache/                         # prompt/response cache
│   └── locks/                         # concurrency locks
│
└── (your source code / tests ...)
```

---

## Adapter Protocol

External systems (GitHub Issues, Jira, Linear, ONES, Tapd) are integrated via **driver CLI subprocesses**.

Write an adapter in **any language** — tomato forks it with stdin/stdout JSON:

```
$ <adapter-cli> <subcommand> < input.json > output.json
```

| Environment Variable | Description |
|----------------------|-------------|
| `TOMATO_RUN_ID` | Current run ID |
| `TOMATO_REPO_ROOT` | Absolute path to project root |
| `TOMATO_DOCS_DIR` | Absolute path to `docs/` directory |

### Subcommands

| Subcommand | Required By | Input (stdin) | Output (stdout) |
|-----------|-------------|---------------|-----------------|
| `create-task` | `task` step | `{ title, description, status }` | `{ task_ref, url }` |
| `update-status` | post-hook | `{ task_ref, status }` | `{ task_ref, status }` |
| `fetch-task` | `task` step | `{ query }` | `[ { number, title, url } ]` |
| `create-pr` | `pr` step | `{ branch, repo, title, draft }` | `{ pr_ref, url }` |
| `update-pr` | review_loop | `{ pr_ref }` | `{ pr_ref, status }` |
| `comment-pr` | review_loop | `{ pr_ref, comments }` | `{ pr_ref, status }` |
| `mark-pr-ready` | review_loop | `{ pr_ref }` | `{ pr_ref, status }` |
| `mark-pr-failed` | review_loop | `{ pr_ref }` | `{ pr_ref, status }` |

### Reference Adapter (GitHub)

A fully functional reference adapter is included:

```bash
# Go binary (recommended)
go build -o github-tomato-adapter ./adapters/github-tomato-adapter
export TOMATO_ADAPTER_BIN=./github-tomato-adapter

# Or use the shell version (requires gh CLI + jq)
export TOMATO_ADAPTER_BIN=./adapters/github-tomato-adapter.sh
```

Configure in `tomato.yaml`:

```yaml
adapters:
  github:
    bin: github-tomato-adapter
    env:
      GITHUB_TOKEN: "${GITHUB_TOKEN}"

roles:
  task: github
  pr: github
  review: github
```

---

## Token Budget

Three presets (`tomato init` defaults to `balanced`):

| Preset | Per-Run Budget | Strategy |
|--------|----------------|----------|
| `frugal` | < 100K tokens | Cheap models everywhere; strong model only for design |
| `balanced` | 100K–500K | GPT-5 for spec/design; DeepSeek-4pro / GLM-5.2 for impl/review/test |
| `quality` | 500K+ | Strong models for every step |

### Cost-Saving Mechanisms

| Mechanism | How it works | Typical savings |
|-----------|-------------|-----------------|
| **Provider prompt caching** | Prompt templates are segmented; templates + system prompts use `cache_control` tags | ~10x on input tokens |
| **Local response cache** | Cache key = `hash(template + input + model + params)`; same prompt → zero LLM calls | 100% on repeat runs |
| **Per-step model routing** | Assign different models per step (e.g., GPT-5 for design, DeepSeek-4pro for test) | 2–5x vs one-model-fits-all |
| **Toggleable features** | `impl.rewrite_arch`, `design.generate_ui_spec`, `review.generate_suggestions` | Save 1 LLM call each |

### On-Exceed Strategies

```yaml
budget:
  on_exceed: warn     # (default) warn on terminal, continue running
  on_exceed: degrade  # auto-switch to degrade_to model for remaining steps
  on_exceed: fail     # stop the pipeline immediately
```

---

## CLI Reference

### Commands

| Command | Description |
|---------|-------------|
| `tomato init` | Initialize `tomato.yaml` and `.tomato/runs/` in the current directory |
| `tomato run [name]` | Run a workflow (defaults to `default`) |
| `tomato spec` | Run requirements analysis (generate PRD) |
| `tomato design` | Run design (architecture + UI + implementation docs) |
| `tomato impl` | Run code implementation |
| `tomato pr` | Push branch and create/update a draft PR |
| `tomato review` | Single-shot code review (no loop — use `run` for review_loop) |
| `tomato test` | Generate and run tests |
| `tomato task` | Sync external tasks via configured adapter |
| `tomato history` | List past runs |
| `tomato history show <id>` | Show full details of a single run (prompts, responses, artifacts) |
| `tomato cost` | Cumulative token usage and estimated cost |
| `tomato config` | View current configuration and API key status |
| `tomato help` | Show help |
| `tomato version` | Show version |

### Flags (all commands)

| Flag | Description |
|------|-------------|
| `--help`, `-h` | Show help for the command |
| `--version`, `-v` | Show version |

> `--force` / `--resume` / `--from` flags are planned for a future release.

### Custom Workflow Commands

Every workflow defined in `tomato.yaml` (except `default`) becomes its own CLI command:

```bash
tomato hotfix       # runs the "hotfix" workflow
tomato doc-only     # runs the "doc-only" workflow
```

---

## Observability

```bash
# List all past runs
tomato history

# Example output:
# ✓ 2026-06-24-a1b2c3d4   design       gpt-5         150   ✓
# ✗ 2026-06-24-e5f6g7h8   review       claude-4       80   ✗

# Inspect a specific run (prompts, responses, artifacts)
tomato history show 2026-06-24-a1b2c3d4

# Cumulative token usage
tomato cost

# Example output:
# Token Usage Summary
# ===================
# Total runs:  12
# Tokens in:   154200
# Tokens out:  43200
#
# By step:
#   spec      2 runs   34000 in   12000 out
#   design    2 runs   72000 in   18000 out
#   impl      3 runs   38100 in   10200 out
#   review    3 runs    5100 in    1500 out
#   test      2 runs    5000 in    1500 out
```

---

## Development

```bash
# Clone and build
git clone https://github.com/bavocado/tomato.git
cd tomato
go build -o tomato .

# Install git hooks (one-time per clone) — adds a Tomato signature recording
# the parent commit hash to every commit message.
scripts/install-hooks.sh

# Run tests
go test ./... -count=1

# CI runs on every push (GitHub Actions)
#   - ubuntu-latest + macos-latest
#   - go vet ./...
#   - go test ./... -count=1 -v
```

> **Tomato signature**: commits and tomato-created PRs carry a chain-of-provenance
> footer (`Tomato-Parent: <hash>`). The commit footer is added automatically by
> the `prepare-commit-msg` hook installed via `scripts/install-hooks.sh`; the
> `pr` step stamps the same footer into PR descriptions.

### Project Structure

```
tomato/
├── main.go                         # entry point
├── cmd/                            # CLI commands
│   ├── root.go                     # cobra root + subcommand registration
│   ├── commands.go                 # all command implementations
│   └── helpers.go                  # withFeatureAndModel, resolveModel, etc.
├── pkg/
│   ├── config/                     # tomato.yaml parsing + validation + defaults
│   ├── model/                      # Step, StepResult, RunMeta types
│   ├── runner/                     # step executor (prompt → LLM → artifacts → run log)
│   ├── llm/                        # multi-provider LLM gateway
│   │   ├── gateway.go              # OpenAI-compatible providers (OpenAI/GLM/DeepSeek)
│   │   ├── anthropic.go            # Claude CLI subprocess provider
│   │   └── cache.go                # local response cache on disk
│   ├── steps/                      # 7 built-in steps + registry
│   │   ├── registry.go             # step lookup + LLM stream factory
│   │   ├── {spec,design,impl,review,test,pr,task}.go
│   ├── adapter/                    # driver CLI protocol
│   │   ├── protocol.go             # subcommand constants
│   │   └── bridge.go               # subprocess executor
│   ├── engine/                     # workflow engine + review_loop
│   ├── archive/                    # architecture versioning (v<N>/)
│   ├── budget/                     # per-step + per-run token budget tracker
│   ├── history/                    # run log reader
│   └── cost/                       # cumulative token usage summary
└── adapters/github-tomato-adapter/ # Go reference adapter (independent module)
```

### Built-in Prompts

Each step uses a Go template prompt that reads from input files. The template variables are the filenames of the input artifacts:

| Step | Template Variables |
|------|--------------------|
| `spec` | `{{.prd.md}}` (user's rough idea) |
| `design` | `{{.prd.md}}` |
| `impl` | `{{.architecture.md}}`, `{{.ui-spec.md}}`, `{{.implementation.md}}` |
| `review` | `{{.architecture.md}}`, `{{.implementation.md}}`, `{{.diff}}` |
| `test` | `{{.architecture.md}}`, `{{.implementation.md}}`, `{{.impl_code}}` |

### Adding a Custom Step

Custom steps are defined in `tomato.yaml`:

```yaml
custom_steps:
  i18n-extract:
    prompt: prompts/i18n.md
    inputs:  [src/**/*.ts]
    outputs: [locales/*.json]
    model:   openai/gpt-5

workflows:
  with_i18n:
    steps: [design, impl, i18n-extract, test]
```

Custom steps can only be used in user-defined workflows. The 7 built-in steps never depend on custom steps (unidirectional coupling).

---

## Architecture Concepts

### "Document-Driven" Philosophy

Existing AI coding tools let you "converse to generate code" — but the output is ephemeral. tomato takes a different approach: **every step produces a markdown document in git**. Six months later, the design decisions, architecture rationale, and implementation notes are still there, diffable, and editable.

```
Conversation-driven (Cursor/Claude Code):
  prompt → code ← done, nothing to review later

Document-driven (tomato):
  idea → prd.md → architecture.md → implementation.md → code
         ↑ all archived, git-versioned, human-reviewable ↑
```

### Why Files, Not Memory?

Steps communicate through files under `docs/specs/<feature>/` — never through in-process memory. This means:

- Any step can be inspected or hand-edited between runs
- Steps can be reordered, skipped, or re-run independently
- The entire design history is a `git log` away
- In the future (v2), steps can run on remote machines via git push/pull

---

## Roadmap

| Version | Theme | Key Features |
|---------|-------|-------------|
| **v0.1.0** | Initial Beta | Single-binary CLI, 7 steps, review_loop, 4 LLM providers, adapter protocol, token budget, architecture versioning |
| **v1.x** | Polish | More reference adapters (Linear, Jira, Tapd, ONES); local model support (Ollama); `--force`/`--resume` flags |
| **v2** | Remote Agents | `runs_on: server-X` dispatch; git-based state bus; self-hosted agent daemon |
| **v3** | MCP Bridge | tomato as MCP server/client; interoperate with Cursor, Claude Code, etc. |
| **v4** | Workflow Sharing | `tomato install <github-url>` to share workflow templates |
| **v5** | Team/Cloud | Multi-user; hosted version (if OSS gains traction) |

---

## FAQ

**Q: How is this different from Cursor / Claude Code / Aider?**

They're AI pair programmers that help you write code faster. tomato is a workflow engine that orchestrates the entire SDLC — from spec to PR — with AI at every stage. The output includes design documents, not just code. You can use tomato alongside Cursor (for the coding step) if you prefer.

**Q: Do I need a specific LLM provider?**

No — tomato supports OpenAI (GPT), Zhipu (GLM), DeepSeek, and Anthropic (Claude). Mix and match per step. Keys go in env vars or config.

**Q: Can I use tomato without Anthropic?**

Yes — just use `openai/gpt-5`, `deepseek/deepseek-4pro`, or `glm/glm-5.2` for all steps. The `anthropic:` section is only needed for `anthropic/*` models.

**Q: Does tomato run my code?**

No — `tomato impl` generates code output (as text), but it doesn't execute it. `tomato test` generates test files but doesn't run test suites (planned for a future release).

**Q: Is there a cloud version?**

No. tomato is a purely local tool — no data leaves your machine except LLM API calls and git pushes.

---

## Contributing

Contributions are welcome! This is early-stage software, so the best way to contribute is:

1. **Use it** — run `tomato run` on a real project and open issues for bugs
2. **Write adapters** — Linear, Jira, Tapd, ONES, Coding.net adapters would be valuable
3. **Improve prompts** — the step templates in `pkg/steps/*.go` are basic; better prompts = better results
4. **Add tests** — see `pkg/history/history_test.go` or `pkg/cost/cost_test.go` for examples

---

## License

MIT