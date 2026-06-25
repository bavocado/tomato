# tomato 🍅

**AI software development workflow engine** for individual developers and small teams.

Turn a rough idea into a PR — through specs, design docs, implementation, code review, testing, and task sync — all with AI assistance, all in your terminal.

```
idea → spec → design → impl → PR → review_loop → test → task
```

---

## Features

- **7 built-in steps**: `spec` · `design` · `impl` · `pr` · `review` · `test` · `task`
- **`review_loop` meta-step**: up to 2 automatic fix iterations, severity-classified comments
- **Document-driven architecture**: every artifact is markdown in git — readable, editable, reviewable
- **Customizable workflows**: compose steps in `tomato.yaml`; each workflow auto-registers as a CLI command
- **Multi-model BYOK**: mix GPT-5, GLM-5.2, DeepSeek-4pro and others per step
- **`claude` CLI integration**: Anthropic models run via `claude` CLI tool for full auth/context management
- **Token budget control**: per-step and per-run caps, caching, three budget presets
- **Adapter protocol**: integrate with GitHub Issues, Jira, Linear, etc. via simple driver CLI subprocesses
- **Architecture versioning**: each impl round archives the design trio, rewrites real architecture from code
- **Pure CLI**: no daemon, no GUI, no external dependencies

---

## Quick Start

### Prerequisites

- **Go 1.22+** (for building from source)
- **`claude` CLI** (for Anthropic models):

```bash
npm install -g @anthropic-ai/claude-code
```

### Install

```bash
brew install tomato
```

Or build from source:

```bash
git clone https://github.com/bavocado/tomato.git
cd tomato
go build -o tomato .
mv tomato /usr/local/bin/
```

### Initialize

```bash
cd your-project
tomato init
```

This creates `tomato.yaml` and `.tomato/runs/`.

### Configure

Edit `tomato.yaml` with your API keys:

```yaml
models:
  default: openai/gpt-5
  steps:
    spec:   anthropic/claude-sonnet-4-20250514
    design: anthropic/claude-sonnet-4-20250514
    impl:   anthropic/claude-sonnet-4-20250514
    review: anthropic/claude-sonnet-4-20250514
    test:   openai/gpt-5

anthropic:
  base_url: https://api.anthropic.com
  auth_token: sk-ant-xxxxx
  model: claude-sonnet-4-20250514
```

> **Security note**: `auth_token` is written in `tomato.yaml`. Add `tomato.yaml` to `.gitignore` or use environment variable overrides in CI.

### Run

```bash
# Full default workflow
tomato run

# Single step
tomato spec
tomato design

# Custom workflow
tomato hotfix     # if defined in tomato.yaml
```

---

## Configuration Reference

### `tomato.yaml`

```yaml
models:
  default: openai/gpt-5              # fallback model
  steps:
    spec:   anthropic/claude-sonnet-4
    design: anthropic/claude-sonnet-4
    impl:   anthropic/claude-sonnet-4
    review: anthropic/claude-sonnet-4
    test:   openai/gpt-5

anthropic:
  base_url: https://api.anthropic.com   # optional, default
  auth_token: ""                         # required for anthropic models
  model: claude-sonnet-4-20250514        # optional

budget:
  mode: balanced                         # frugal | balanced | quality
  global_per_run: 300000
  per_step:
    spec: 50000
    design: 100000
    impl: 100000
    review: 30000
    test: 20000
  on_exceed: warn                        # fail | degrade | warn
  degrade_to: deepseek/deepseek-4pro

adapters:
  github:
    bin: github-tomato-adapter
    env:
      GITHUB_TOKEN: "${GITHUB_TOKEN}"

roles:
  task: github
  pr: github
  review: github

workflows:
  default:
    steps:
      - spec
      - design
      - impl
      - pr
      - review_loop: { max_rounds: 2, on_fail: stop }
      - test
      - task

  hotfix:
    steps: [spec, impl, pr, review]

  doc-only:
    steps: [spec, design]
```

### Supported Models

Model format: `provider/model`

| Provider | Format | API Key Env Var | Base URL (default) |
|----------|--------|----------------|-------------------|
| OpenAI | `openai/gpt-5` | `OPENAI_API_KEY` | `https://api.openai.com/v1` |
| Zhipu (GLM) | `glm/glm-5.2` | `GLM_API_KEY` | `https://open.bigmodel.cn/api/paas/v4` |
| DeepSeek | `deepseek/deepseek-4pro` | `DEEPSEEK_API_KEY` | `https://api.deepseek.com` |
| Anthropic | `anthropic/claude-sonnet-4` | (uses `auth_token` from `anthropic:` config) | `claude` CLI via subprocess |

---

## Steps Reference

| Step | Command | Input | Output |
|------|---------|-------|--------|
| **spec** | `tomato spec` | user's rough idea | `prd.md` |
| **design** | `tomato design` | `prd.md` | `architecture.md` + `ui-spec.md` + `implementation.md` |
| **impl** | `tomato impl` | design trio | source code diff |
| **pr** | `tomato pr` | git working tree | PR on GitHub/GitLab (via adapter) |
| **review** | `tomato review` | code diff + design | `reviews/r<n>-comments.md` |
| **test** | `tomato test` | code + design | test files + report |
| **task** | `tomato task` | spec/design | task on Jira/Linear/GitHub Issues (via adapter) |

### review_loop Meta-Step

```
review_loop(max_rounds=2, on_fail=stop):
  round 1: review → blocking issues? → yes → fix → round 2 → blocking issues? → yes → FAILED
                                              → no  → PASSED                    → no  → PASSED
```

- `blocking` severity issues keep the loop alive
- `major` / `minor` are posted to the PR but don't trigger fixes
- On exhaustion: adapter marks PR as failed, pipeline stops

---

## Artifact Layout

```
docs/specs/<feature>/
├── prd.md                  ← always latest
├── architecture.md         ← always latest (rewritten after impl with real arch)
├── ui-spec.md              ← always latest
├── implementation.md       ← always latest
├── pr.md                   ← PR ref + URL
├── v1/                     ← archive after round 1 of design+impl
│   ├── architecture.md
│   ├── ui-spec.md
│   └── implementation.md
├── v2/                     ← round 2 archive
└── reviews/
    ├── r1-comments.md
    ├── r2-comments.md
    └── final-comments.md
```

---

## Adapter Protocol

External system integration is done via **driver CLI subprocesses**. Write an adapter in any language; tomato forks it with stdin/stdout JSON.

### Subcommands

| Subcommand | Step | Purpose |
|-----------|------|---------|
| `create-task` | task | Create an issue/task |
| `update-status` | task (post-hook) | Update task status |
| `fetch-task` | task | Query existing tasks |
| `create-pr` | pr | Create draft PR |
| `update-pr` | review_loop | Push fix commits |
| `comment-pr` | review_loop | Post review comments |
| `mark-pr-ready` | review_loop | Mark PR as ready |
| `mark-pr-failed` | review_loop | Mark PR as failed |

### Reference Adapter

A fully functional reference adapter for GitHub is included at [`adapters/github-tomato-adapter.sh`](./adapters/github-tomato-adapter.sh).

---

## Token Budget

Three presets (`tomato init` chooses `balanced`):

- **frugal**: < 100K tokens/run — cheap models everywhere
- **balanced** (default): 100K–500K — strong models for spec/design, cheap for rest
- **quality**: 500K+ — strong models everywhere

Built-in cost saving:
- Provider-native prompt caching (architectural — prompt templates are segmented for `cache_control`)
- Local response cache on disk (`.tomato/cache/`)
- Toggleable optional steps (`impl.rewrite_arch`, `design.generate_ui_spec`, `review.generate_suggestions`)

---

## Observability

```bash
# List past runs
tomato history

# Show details of one run
tomato history show <run-id>

# Cumulative token usage
tomato cost
```

---

## Development

```bash
go test ./...
go build -o tomato .
```

### Planned (v2+)

- Remote agent execution (`runs_on: server-X`)
- MCP bridge
- Workflow sharing (`tomato install <github-url>`)

---

## License

MIT