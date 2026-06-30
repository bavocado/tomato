# Tomato Top-Level Vision Design Document

- **Date**: 2026-06-18
- **Status**: Draft (pending user review)
- **Author**: tomato project team
- **Type**: Vision / Top-level product vision (not an implementation plan)

---

## TL;DR

> **tomato is a CLI-first AI software development workflow engine for individual developers and small teams. It turns "requirements → design → implementation → review → testing → task status" into a declarative, composable, adaptable pipeline.**
>
> Users compose 7 built-in steps into their own workflows in `tomato.yaml`; each workflow is automatically exposed as a top-level CLI command. tomato is a pure CLI tool — each invocation is a short-lived process, with no daemon, no background services. The backend LLM is multi-model BYOK; external systems are integrated through a user-provided driver CLI protocol.

---

## Current Implementation Notes (2026-06-30)

The codebase has evolved beyond the original draft in several important ways:

1. **Default workflow order** is now:

   ```yaml
   steps: [spec, task, design, impl, pr, review_loop, test]
   ```

   `task` runs immediately after `spec` so later steps can update external task status.

2. **Provider routing** uses Claude Code CLI for GLM / DeepSeek / Anthropic:

   ```yaml
   providers:
     glm:
       base_url: ...
       auth_token: ...
       model: glm-5.2
     deepseek:
       base_url: ...
       auth_token: ...
       model: deepseek-v4-pro
   ```

   tomato maps these values into `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, and `ANTHROPIC_MODEL` before launching:

   ```bash
   claude --print --permission-mode auto --effort high --model <provider.model>
   ```

3. **Claude CLI timeout** is implemented via `TOMATO_CLAUDE_TIMEOUT` (default `30m`). On timeout, tomato kills the whole Claude process group so child commands such as `make build` do not survive.

4. **`tomato pr` prepares a safe PR branch**. If the current branch is `main` or `master`, tomato switches to `tomato/<feature>`, commits generated changes, pushes the branch when `origin` exists, and asks the adapter to create a draft PR using `--head <branch>`.

5. **`impl` writes real source files** by extracting fenced code blocks from `impl-output.md` using the ` ```lang:path/to/file ` convention.

6. **`spec` input/output are separated**: `idea.txt` is the input, `prd.md` is the generated PRD.

These notes should be folded into a future rewrite of this vision document, but are recorded here so the design matches current behavior.

---

## 1. Background & Positioning

### 1.1 Industry Landscape Summary

| Category | Representatives | Main Stages Covered | Relationship to tomato |
|------|------|-------------|----------------|
| Autonomous coding agents | Devin, Factory.ai | Coding / Review / Testing | Competitor (different user base) |
| AI editors / pair programmers | Cursor, Claude Code, Aider, Cline | Coding | Downstream collaboration (artifacts can be fed to them) |
| Prompt-to-app generators | Lovable, v0, Bolt, Replit Agent | Coding | Different form, different audience |
| AI PRD tools | ChatPRD | Requirements | Partial functional overlap, different form |
| AI Code Review | CodeRabbit, Qodo | Review / Testing | Post-stage competitors |
| General AI workflow platforms | Dify, n8n, Coze | General | Lacks software development domain semantics |
| Sunset | GitHub Copilot Workspace (2025-05) | Full SDLC | Counter-example — validates the difficulty of "fully autonomous agent platforms" |

### 1.2 Core Theses

1. **Document-driven is more sustainable than conversation-driven**: Existing AI coding tools let users "converse to generate code", but lack intermediate design documents, making it unmaintainable 3 months later. tomato assumes that requirements and design documents are the most missing and most valuable hidden assets for individual developers.
2. **Individual developers want customizable workflows, not fully autonomous agents**: The Devin model (throwing tasks at an agent) suits enterprises; individual developers want "I design my own SOP, AI executes the mechanical steps for me".
3. **Domain semantics matter more than general-purpose engines**: General AI workflows feel blunt in software development scenarios; tomato has built-in first-class objects and nodes for PRD / design docs / PR / Test.
4. **The ecosystem is carried by "workflow files", not an app store**: A `tomato.yaml` shared on GitHub is the distribution unit, zero centralization.

### 1.3 Target Users

- **Primary users**: Independent developers / individual developers / 1–5 person small teams
- **Persona**: One person playing multiple roles, heavy context-switching across tools, prefers local-first and git-friendly, willing to customize workflows, reluctant to be locked into SaaS.
- **Non-target users**: Large enterprise R&D departments, teams needing multi-person collaboration + approval flows, pure product/design roles who don't write code.

### 1.4 Differentiation (in one sentence)

> tomato = a software-development-domain version of GitHub Actions, running on your local machine, with each node executed by AI.

| Dimension | Cursor / Claude Code | Lovable / v0 | Devin / Factory | Dify / n8n | **tomato** |
|------|----------------------|--------------|-----------------|------------|-----------|
| Primary interaction | IDE/CLI conversation | Browser prompt | Cloud tasks | Drag-and-drop nodes | **YAML + CLI** |
| Artifact form | Code | Code | Code + PR | Arbitrary | **Design docs + code + PR** |
| Document-driven | ❌ | ❌ | △ | ❌ | **✅ first-class** |
| Workflow customization | ❌ | ❌ | △ | ✅ | **✅** |
| Software dev domain semantics | △ | △ | ✅ | ❌ | **✅** |
| Deployment | Local + cloud | Cloud | Cloud | Self-hosted | **local-first** |
| Target users | All kinds of developers | Design/Frontend | Large enterprises | General business | **Individual / small teams** |

### 1.5 Non-Goals (explicitly not doing)

- ❌ Not building "prompt-to-SaaS in the browser" (v0/Lovable's track)
- ❌ Not building an IDE-internal coding experience to replace Cursor
- ❌ Not building cloud-hosted multi-tenant SaaS (v1 stage; may be considered later)
- ❌ Not building our own LLM or training our own model
- ❌ Not building a "general-purpose workflow engine" — rejecting nodes unrelated to software development
- ❌ Not building a plugin marketplace / central registry
- ❌ v1 does **not** implement remote agent execution — the protocol interface is designed in v1 for future-proofing, but actual cross-machine execution lands in v2 (see §5)

---

## 2. Domain Model & Workflow

### 2.1 Seven Built-in Steps (first-class citizens)

Each step is independently callable, independently runnable, and independently produces markdown/code artifacts. They communicate through **files** rather than in-memory objects — all artifacts are git-friendly.

| Step | CLI | Input | Default Artifacts | Notes |
|------|-----|------|----------|------|
| **spec** | `tomato spec` | User's rough idea / existing discussion notes | `prd.md` | Requirements analysis |
| **design** | `tomato design` | `prd.md` | `architecture.md`, `ui-spec.md`, `implementation.md` | Architecture / UI / Implementation — three documents |
| **impl** | `tomato impl` | design trio | Code diff / file changes | Code implementation |
| **pr** | `tomato pr` | git working tree state | `pr.md` (PR ref + URL) | Push branch + open/update PR via driver CLI adapter (draft) |
| **review** | `tomato review` | git diff / PR | `reviews/<round>-comments.md` with severity labels | Code review with structured output; embedded in `review_loop` (see §2.10) |
| **test** | `tomato test` | Code + design | Test files + test report | Automated testing |
| **task** | `tomato task` | spec/design artifacts | Task ID created/updated on external platform | Integrates with Jira/Linear/GitHub Issues etc. via driver CLI adapter |

Each step is idempotent, re-runnable, and force-overridable:

```bash
tomato design              # warns if artifacts already exist
tomato design --force      # force overwrite
tomato design --resume     # resume from interruption point
```

#### Execution location: `runs_on:` (v1 = local only, v2 = remote agents)

Every step has an implicit `runs_on:` location. In v1 this is hard-coded to `local`; the field is **reserved syntax** so that v1 workflows can be loaded by v2 engines without modification:

```yaml
workflows:
  default:
    steps:
      - spec
      - design
      - impl: { runs_on: local }       # v1: only "local" accepted
      - pr
      - review_loop:
          max_rounds: 2
          on_fail: stop
          # v2 will allow: runs_on: server-a
      - test
      - task
```

**v1 behavior**:
- `runs_on: local` (default, the only allowed value) — executes in the current `tomato` process
- Any other value raises a clear error: `runs_on: <X> is reserved for v2 remote agents (see §5)`
- The field's presence in v1 is purely forward-compatibility — see §5 for the v2 semantics

**Why reserve the field now**: it forces every v1 built-in step to be designed so that **all inputs and outputs flow through files**, never through in-process state. This is verifiable in v1 ("could this step run on a remote agent?") and makes v2 a true increment, not a rewrite.

#### Step Status Lifecycle

After each step completes, the external task status can be automatically updated via an adapter (not solely the responsibility of the `task` step):

| Step completed | Status label | Notes |
|----------|----------|------|
| `spec` completed | `specified` | Requirements locked |
| `design` completed | `designed` | Design docs produced |
| `impl` completed | `implemented` | Code implemented (**triggers architecture version archiving + real architecture rewrite**, see §2.8) |
| `pr` completed | `pr_opened` | PR created/updated in draft state |
| `review` passed | `reviewed` | review_loop converged (no blocking issues) |
| `review` failed | `review_failed` | review_loop exhausted max rounds; PR marked failed; subsequent steps blocked (see §2.10) |
| `test` completed | `tested` | Tests passed |

Status updates are a built-in post-hook for each step, configurable in `tomato.yaml` for whether to enable and which adapter to bind. The `task` step's responsibility remains **creating new tasks**.

### 2.2 Default Workflow

`tomato init` generates a default `tomato.yaml` in the repo, containing a `default` workflow:

```yaml
workflows:
  default:
    steps:
      - spec
      - task                 # create external task early for status post-hooks
      - design
      - impl
      - pr
      - review_loop:
          max_rounds: 2          # at most 2 fix iterations
          on_fail: stop          # stop pipeline on exhaustion
      - test
```

`tomato run` is equivalent to `tomato run default`, for users who "don't want to configure anything, just run the pipeline".

> `review_loop` is a **meta-step** (a built-in control-flow primitive). See §2.10 for its semantics. In flat workflows it may also be written as a plain `review` step (single-shot, no loop).

### 2.3 User-Defined Workflows (each = a top-level CLI command)

```yaml
workflows:
  default:
    steps:
      - spec
      - task
      - design
      - impl
      - pr
      - review_loop: { max_rounds: 2, on_fail: stop }
      - test

  hotfix:
    steps: [spec, impl, pr, review]      # single-shot review, no loop

  doc-only:
    steps: [spec, design]                 # docs only, no code changes
```

```bash
tomato hotfix       # = tomato run hotfix
tomato doc-only     # = tomato run doc-only
```

CLI dynamically registers subcommands by reading `tomato.yaml` at startup, erroring on conflicts with reserved commands.

**Reserved commands** (user workflow names cannot conflict with these):
`init / run / spec / design / impl / pr / review / test / task / config / version / help / history / cost`

### 2.4 Multi-Model Routing (BYOK)

Each step can be independently configured with a different model. Keys are provided via environment variables (not in git):

```yaml
models:
  default: deepseek/deepseek-4pro

  steps:
    spec:    openai/gpt-5               # strong model for requirement thinking
    design:  openai/gpt-5               # design quality is critical
    impl:    glm/glm-5.2                # code implementation
    review:  glm/glm-5.2                # code review
    test:    deepseek/deepseek-4pro     # test generation
```

**v1 supported providers**:
- **OpenAI** (GPT series) — OpenAI-compatible protocol
- **Zhipu** (GLM series) — OpenAI-compatible protocol
- **DeepSeek** — OpenAI-compatible protocol

> All three use the OpenAI-compatible protocol, so a single adapter covers them all; users only need to configure `base_url` + `api_key` + `model` to switch.

### 2.5 Custom Steps (a restrained extension point)

Users can define new steps in their own `tomato.yaml` and compose them into their own workflows:

```yaml
custom_steps:
  i18n-extract:
    prompt: prompts/i18n.md
    inputs:  [src/**/*.ts]
    outputs: [locales/*.json]
    model:   deepseek/deepseek-4pro

workflows:
  with_i18n:
    steps: [design, impl, i18n-extract, test]
```

**Key constraints**:
- ✅ Users can invoke custom steps in **their own workflows**
- ❌ tomato's 6 official steps **never** invoke user custom steps
- ❌ tomato does not maintain a "step marketplace" — distribution via GitHub/git URL, community-governed

→ **Unidirectional coupling**: users depend on tomato, tomato does not depend on users. This avoids the "two-sided ecosystem cold-start" trap.

### 2.6 Data Flow (artifacts as interfaces)

```
prd.md ──► architecture.md ──┐
                              │
prd.md ──► ui-spec.md ────────┼──► code diff ──► PR (draft) ──► review_loop ──► PR ready / failed
                              │                                       │                │
prd.md ──► implementation.md ─┘                                       │                │
                                                                      │                ↓
                                                                      │           test files
                                                                      │                │
prd.md / design artifacts ────────────────────────────────────────────┴────────────────┴──► external tasks (Jira/Linear...)
```

- Artifacts land in the repo's `docs/specs/<feature>/` (**in git**)
- Run logs land in the repo's `.tomato/runs/<run-id>/` (**not in git**)
- Steps communicate **only through files**, sharing no memory — so any step can be viewed offline, hand-edited, or rolled back
- If you modify an upstream artifact and re-run the downstream, the downstream regenerates based on the new artifact

### 2.7 Step Run Logs

Each step execution drops a run log to `.tomato/runs/<run-id>/`:
- Input file snapshots
- Output file snapshots
- Model invoked / token usage / duration
- Full prompt and LLM response

This is the data source for `tomato history` / `tomato cost`, and the carrier for post-hoc debugging / sharing reproducible runs.

### 2.8 Architecture Versioning & Rewrite

The same feature can go through multiple rounds of design → impl iteration. After each `impl` completes, tomato automatically:

1. **Archives**: moves the current root-level trio into a version directory `docs/specs/<feature>/v<N>/`
2. **Rewrites**: regenerates an `architecture.md` reflecting the real architecture based on the actually-implemented code, written back to the root directory

```
docs/specs/<feature>/
├── prd.md                  ← always latest (not involved in version archiving)
├── architecture.md         ← always latest (reflects real architecture after most recent impl)
├── ui-spec.md              ← always latest
├── implementation.md       ← always latest
├── v1/                     ← archive after round 1 of design+impl
│   ├── architecture.md
│   ├── ui-spec.md
│   └── implementation.md
├── v2/                     ← archive after round 2 of design+impl
│   ├── architecture.md
│   ├── ui-spec.md
│   └── implementation.md
└── reviews/
    └── 2026-06-18-pr-42.md
```

**Core promise**: the root `architecture.md` is always the "latest truth" — it's not the ideal architecture from the original design, but the real architecture reverse-engineered from the code after impl. Historical versions are fully preserved in `v<N>/`, traceable via git diff.

**Flow**:
1. `tomato design` → writes root-level trio (design intent)
2. `tomato impl` → reads root-level trio → produces code → after impl completes:
   - Archives the current root-level trio to `v<N>/` (N auto-increments)
   - Invokes LLM to analyze the actual code and generates a new `architecture.md` written back to root
   - Marks status as `implemented` via adapter post-hook

### 2.9 Token & Budget Control

Token cost is one of the biggest concerns for individual developers using tomato. tomato makes "saving" and "controlling" first-class citizens at the architectural level.

#### 2.9.1 Three Budget Presets

`tomato init` lets the user choose a budget tier, auto-generating the corresponding `models:` and `budget:` config:

| Tier | Per-run budget | Model strategy | Applicable scenarios |
|------|---------------|----------|----------|
| `frugal` | < 100K tokens | All cheap models (DeepSeek-4pro / GLM-5.2), strong model only for design | Students / side projects / experiments |
| **`balanced` (default)** | 100K–500K tokens | spec/design use GPT-5, impl/review/test use DeepSeek-4pro / GLM-5.2 | Main personal projects |
| `quality` | 500K+ tokens | All GPT-5 / GLM-5.2 | Critical projects / cost-insensitive |

**balanced tier default model routing** (v1 built-in):

```yaml
models:
  default: deepseek/deepseek-4pro
  steps:
    spec:    openai/gpt-5               # strong model for requirement thinking
    design:  openai/gpt-5               # design quality is critical
    impl:    glm/glm-5.2                # code implementation
    review:  glm/glm-5.2                # code review
    test:    deepseek/deepseek-4pro     # test generation
```

#### 2.9.2 Provider-native prompt caching (architecturally mandatory)

OpenAI / Zhipu / DeepSeek and other mainstream providers are gradually supporting prompt caching, **cached input tokens are ~10x cheaper**. tomato's prompt pattern is naturally suited (each step is "fixed template + varying input files").

**Architectural decision**: v1's prompt structure must be designed for caching from the start — explicit segmentation, marking templates + system prompts as `cache_control`, with input files as the variable segment. Retrofitting is costly, so v1 locks this in.

#### 2.9.3 Local Response Cache

`.tomato/cache/` (already planned in §3.5) refines the key design:

```
key = hash(prompt_template_version + input_files_content + model_id + params)
```

- Running design twice on the same PRD → second run is zero tokens
- Modifying one PRD section and re-running → only the actually-changed prompt segment triggers a new call (depends on the segmented structure in §2.9.2)
- On cache hit, meta.json still records "cache hit", keeping cost visible

#### 2.9.4 Budget Configuration

```yaml
budget:
  mode: balanced                      # frugal | balanced | quality
  global_per_run: 300000              # global per-run cap (tokens)
  per_step:
    spec:    50000
    design:  100000
    impl:    100000
    review:  30000
    test:    20000
  on_exceed: warn                     # fail | degrade | warn
  degrade_to: deepseek/deepseek-4pro  # model to switch to when on_exceed=degrade
```

- `on_exceed: fail` — stop immediately on hitting cap, wait for user decision
- `on_exceed: degrade` — auto-switch to the cheap model configured in `degrade_to` and continue
- `on_exceed: warn` — warn but continue (default)

#### 2.9.5 Cost Visibility

- `.tomato/runs/<id>/meta.json` records per-step token usage, cache hits, estimated cost
- `tomato history` — list past runs with per-step tokens / cost / cache hit ratio
- `tomato cost` — cumulative cost summary (overall / per day / per step)
- `tomato history show <run-id>` — drill into a single run (prompts, responses, artifacts)

#### 2.9.6 Toggleable Optional Items

Some "nice-to-have" LLM calls can be configured off to save tokens:

| Config item | Default | Description |
|--------|------|------|
| `impl.rewrite_arch` | `true` | §2.8's rewrite of real architecture after impl; turn off to save one large call |
| `design.generate_ui_spec` | `true` | Can be turned off for pure backend projects without UI |
| `review.generate_suggestions` | `true` | Turn off when you only want an issue list, no fix suggestions |

#### 2.9.7 v1 / v1.x Division

| Means | v1 must-do | v1.x | Reason |
|------|---------|------|------|
| prompt caching structure | ✅ | | Architectural, costly to retrofit |
| Local response cache | ✅ | | High hit rate, already planned in §3.5 |
| Three budget presets + config | ✅ | | Core individual-developer need |
| Model tier recommendation table | ✅ | | Zero dev cost, pure config |
| Cost visibility (meta + UI) | ✅ | | Already have meta foundation |
| Toggleable optional items | ✅ | | One config item each |
| Context trimming (relevant section retrieval) | | ✅ | Needs RAG; v1 uses whole files first |
| Incremental generation (diff-driven regen) | | ✅ | Depends on diff algorithm, complex |
| Dry-run estimation | | ✅ | Only accurate with historical data |
| Budget alerts + auto-degrade | | ✅ | Depends on budget config landing first |

#### 2.9.8 Model Selection Principles

**Choosing models by step nature**:

| Step | Key capability | balanced tier recommendation | Reason |
|------|---------|----------------|------|
| spec | Long-context understanding, reasoning, requirement mining | GPT-5 | Getting it wrong here cascades downstream; worth the spend |
| design | Structured output, domain knowledge, reasoning | GPT-5 | Design quality determines downstream quality |
| impl | Code generation, long output, spec adherence | GLM-5.2 | Code capability is commoditized; GLM-5.2 is cost-effective |
| review | Code understanding, defect detection | GLM-5.2 | No long output needed; same model as impl aids style consistency |
| test | Code generation + boundary enumeration | DeepSeek-4pro | Test generation needs strong code capability and low cost |
| task | Structured output (JSON) | DeepSeek-4pro / GLM-5.2 | Task system integration; cheap model is enough |

**Notes when switching models**:
- **Same step across models**: prompt templates may need tuning (different models have different sensitivities to system prompt style); v1 does not require adaptation, providing "per-model-versioned prompt templates" as a v1.x capability
- **Cross-step context consistency**: steps communicate via markdown files (§2.6), not relying on model-specific markers, so cross-model combinations are safe
- **Cache key includes model_id** (§2.9.3): switching models won't read stale cache, safe

**Anti-patterns (avoid)**:
- ❌ Using a cheap model for spec to save tokens → requirement misunderstanding, all downstream rework, more expensive
- ❌ Using the strongest model for impl → low marginal return, GLM-5.2 is already good enough
- ❌ Switching to a different model for every step → debugging complexity explodes, stay within 3 providers

### 2.10 Review Loop (built-in meta-step)

`review_loop` is the only **control-flow primitive** in v1. It wraps the `review` step with bounded iteration and automatic fix attempts. It exists as a first-class meta-step because "one-shot review" rarely catches everything and humans expect at least one round of "AI fixes its own bugs".

#### Semantics

```
review_loop(max_rounds=2, on_fail=stop):

  for round in 1..max_rounds+1:                       # at most 3 reviews → 2 fixes
    invoke `review` step
      → produces docs/specs/<feature>/reviews/r<round>-comments.md
      → calls adapter `comment-pr` with comments

    if no blocking issues:
      → call adapter `mark-pr-ready`
      → emit status `reviewed` and exit loop ✓

    if round < max_rounds + 1:
      invoke `impl` step in fix mode
        → reads previous reviews/r<round>-comments.md (blocking items only)
        → produces fixing diff, commits to branch
        → calls adapter `update-pr` to push commit
    else:
      → call adapter `mark-pr-failed` with final comments
      → emit status `review_failed`
      → print PR URL + blocking comments summary to stderr
      → apply `on_fail` policy
```

#### Severity classification

`review` outputs structured JSON; tomato parses it to decide whether the loop continues:

```json
{
  "comments": [
    { "file": "src/foo.go", "line": 42, "severity": "blocking", "message": "..." },
    { "file": "src/bar.go", "line": 17, "severity": "major",    "message": "..." },
    { "file": "src/baz.go", "line": 99, "severity": "minor",    "message": "..." }
  ],
  "summary": "..."
}
```

| Severity | Triggers fix loop? | Behavior |
|----------|---------------------|----------|
| `blocking` | ✅ yes | Must be addressed; otherwise review_loop fails |
| `major` | ❌ no (v1) | Posted to PR as comments; user decides |
| `minor` | ❌ no | Posted to PR as comments; user decides |

> Rationale: letting the LLM upgrade "rename a variable" to blocking would cause infinite loops. v1 hard-codes that **only `blocking` keeps the loop alive**. v1.x can revisit allowing user-configurable severity thresholds.

#### Configuration

```yaml
review_loop:
  max_rounds: 2                # 0 = no loop (pure single-shot review)
  on_fail: stop                # stop | continue | ask
  severity_threshold: blocking # what severity keeps the loop alive (v1: fixed at "blocking")
  fix_step: impl               # which step to invoke for fixes (default: impl)
```

`on_fail` semantics:
- `stop` (default) — exit the whole workflow with non-zero; subsequent steps (`test`, `task`) do not run
- `continue` — mark `review_failed` but proceed to the next step (e.g., still run `test`)
- `ask` — prompt the user on the terminal: retry / accept-as-is / abort

#### Artifacts

```
docs/specs/<feature>/
├── pr.md                     # PR ref + URL (written by `pr` step)
└── reviews/
    ├── r1-comments.md        # round 1 review output (structured + human-readable)
    ├── r2-comments.md        # round 2 review output
    └── final-comments.md     # on failure, copy of last round + failure summary
```

#### Recovery after failure

If `review_loop` fails (`review_failed`), the user has two paths to continue:

1. **Manual fix + `--force` rerun the loop**:
   ```bash
   # edit code by hand, then:
   tomato review --force         # re-runs the loop from scratch on current diff
   ```
2. **Resume the entire pipeline from review**:
   ```bash
   tomato run --resume           # picks up at review_loop with current state
   tomato run --from review      # restart from review onwards
   ```

Both paths reuse the existing PR (no new PR is opened on resume).

#### Why a meta-step instead of just looping in code?

- **Visibility**: `review_loop` appears explicitly in `tomato.yaml`, so behavior is declarative and reviewable in git.
- **Composability**: users can wrap other steps in `review_loop` later (v1.x), e.g., `review_loop` around `test` to re-run failing tests.
- **Off-switch**: writing the workflow as `[..., review, test, ...]` instead of `review_loop: {...}` opts out entirely — single-shot review, no fix attempts.

---

## 3. Technical Architecture

### 3.1 Process & Runtime Model

tomato is **a pure CLI tool** — every `tomato xxx` invocation is an independent short-lived process that exits when done. No daemon, no embedded server, no socket, no "always on".

```
$ tomato design
  ┌──────────────────────────────────────┐
  │ tomato process (short-lived)         │
  │  ├─ read tomato.yaml                 │
  │  ├─ render prompt                    │
  │  ├─ call LLM (streaming)             │
  │  ├─ write artifacts to docs/specs/   │
  │  ├─ drop run log to .tomato/runs/    │
  │  └─ exit                             │
  └──────────────────────────────────────┘
```

**Key design decisions**:

- **No daemon / no background process**: every CLI invocation is its own process, same mental model as `git` / `aider` / `claude-code`
- **All state on disk**: cross-invocation shared state (cache / run logs / config / locks) is all under `.tomato/`, read on demand at next invocation
- **Concurrency safety via file locks**: `.tomato/locks/` prevents the same step from being run concurrently
- **Streaming output**: each CLI streams directly to stdout/stderr; process death ends the stream naturally (Ctrl+C just works)
- **No external dependencies**: no Redis / Postgres / Docker; all runtime data lives in `.tomato/`
- **Cross-platform simplicity**: no daemon / socket / background process — Windows / Linux / macOS behave identically

### 3.2 Core Components

| Component | Responsibility | Talks to |
|------|------|----------|
| **Command Router** | Parses `tomato.yaml`, registers workflow names as CLI subcommands; dispatches to executors | User → engine |
| **Workflow Engine** | Schedules steps per workflow declaration, passes artifacts, handles failure/rerun/resume | Router → Step Runtime |
| **Step Runtime** | Executes a single step: renders prompt, calls LLM, writes artifacts, records run log | Engine → LLM Gateway / Adapter |
| **LLM Gateway** | Multi-provider adaptation, key management, token billing, retries, caching | Step Runtime → external LLM |
| **Adapter Bridge** | fork/exec user-provided adapter CLIs per the driver CLI protocol | Step → adapter subprocess |

### 3.3 Adapter Mechanism: driver CLI protocol

tomato does **not** build a plugin loader; all external system integration goes through **user-provided driver CLI subprocesses**.

#### Protocol Sketch (v1 locked)

```
$ <adapter-cli> <subcommand> < input.json > output.json
```

- **stdin**: JSON input (spec/design content, context, current step artifact paths)
- **stdout**: JSON output (task ref, URL, status, and other structured results)
- **stderr**: free-form logs (collected by tomato into the run log)
- **exit code**: 0 success, non-0 failure (error message read from stdout or stderr)
- **environment variables**: `TOMATO_RUN_ID`, `TOMATO_REPO_ROOT`, `TOMATO_DOCS_DIR`, `TOMATO_*` (context the adapter can read)

#### Subcommands required by built-in steps (v1 scope)

`task` step subcommands:

| Subcommand | Purpose |
|--------|------|
| `create-task` | Creates a task in the external system based on spec/design, returns a task reference |
| `update-status` | Modifies the external task's status (e.g., design done → task moves to in-progress) |
| `fetch-task` | Fetches existing tasks by query (for incremental workflows) |

`pr` step + `review_loop` subcommands:

| Subcommand | Purpose |
|--------|------|
| `create-pr` | Push current branch and open a PR in draft state; returns PR ref + URL |
| `update-pr` | Push new commits to an existing PR (used in review_loop fix rounds) |
| `comment-pr` | Post review comments to the PR (supports inline + summary comments) |
| `mark-pr-ready` | Transition draft PR to ready-for-review state (called after review_loop converges) |
| `mark-pr-failed` | Add a "review-failed" label + post final blocking comments (called when review_loop exhausts rounds) |

The input schema (JSON Schema) for each subcommand is published with the v1 docs and versioned (`tomato-adapter-protocol/v1`). A single adapter binary may implement any subset of these subcommands; tomato detects available subcommands via a `capabilities` subcommand.

#### Configuration

```yaml
adapters:
  # A single adapter can serve multiple roles
  github:
    bin: "github-tomato-adapter"   # executable on PATH
    env:
      GITHUB_TOKEN: "${GITHUB_TOKEN}"
      GITHUB_REPO: "owner/repo"

# Map roles to adapters; one adapter can be referenced by multiple roles
roles:
  task: github           # `task` step uses the "github" adapter
  pr:   github           # `pr` step + review_loop use the same adapter
```

#### Advantages

- **Zero SDK / zero runtime binding**: adapters can be Bash / Python / Go / Node / any program that reads stdin and writes stdout
- **Zero distribution channel**: distributed via GitHub / brew / apt / any means; install with `chmod +x` and put on PATH
- **MCP needs no special handling**: an MCP server can be exposed as a protocol-compliant driver CLI via a simple wrapper script
- **Testable**: adapters are ordinary CLIs, testable with shell and fixture JSON, consistent with other Unix tools

### 3.4 CLI Command List (v1)

**Reserved commands**:
```
tomato init             # initialize tomato.yaml in the current repo (with default workflow, .gitignore config)
tomato run [<flow>]     # run a workflow (default: default)
tomato spec             # run alone: requirements analysis
tomato design           # run alone: design
tomato impl             # run alone: code implementation
tomato pr               # run alone: push branch + open/update PR (draft)
tomato review           # run alone: single-shot review (no loop)
tomato test             # run alone: test generation
tomato task             # run alone: task sync
tomato history          # list past runs (per-step tokens, cost, status)
tomato history show <run-id>   # drill into a single run (prompts, responses, artifacts)
tomato cost             # cumulative cost summary
tomato config           # view/edit config (including API key status)
tomato version
tomato help
```

**Dynamic commands**: workflow names defined by the user in `tomato.yaml` (top-level), e.g., `tomato hotfix`, `tomato doc-only`.

### 3.5 Directory Conventions

```
my-project/
├── tomato.yaml                          # user config (in git)
├── .gitignore                           # already includes .tomato/
│
├── docs/                                ★ artifacts (in git)
│   └── specs/
│       └── <feature>/                   # grouped by feature
│           ├── prd.md                   # always latest
│           ├── architecture.md          # always latest (rewritten with real architecture after impl)
│           ├── ui-spec.md               # always latest
│           ├── implementation.md        # always latest
│           ├── pr.md                    # PR ref + URL (written by `pr` step, reused across rounds)
│           ├── v1/                      # round 1 design+impl archive
│           │   ├── architecture.md
│           │   ├── ui-spec.md
│           │   └── implementation.md
│           ├── v2/                      # round 2 archive (if any)
│           │   ├── architecture.md
│           │   ├── ui-spec.md
│           │   └── implementation.md
│           └── reviews/
│               ├── r1-comments.md       # review_loop round 1
│               ├── r2-comments.md       # review_loop round 2
│               └── final-comments.md    # on failure: copy of last round + summary
│
├── .tomato/                             ★ runtime data (not in git)
│   ├── runs/
│   │   └── <run-id>/
│   │       ├── meta.json                # steps, models, duration, tokens
│   │       ├── prompts.jsonl            # full prompts
│   │       ├── responses.jsonl          # full LLM responses
│   │       └── artifacts/               # snapshot of this run's artifacts
│   ├── cache/                           # prompt / response cache
│   └── locks/                           # prevent concurrent runs of the same step
│
└── (user's source code / tests / etc.)
```

The `docs/` path can be overridden in `tomato.yaml`, but defaults to this layout.

### 3.6 Observability via CLI

There is no GUI. All visibility into runs, costs, and prompts is exposed through CLI subcommands reading `.tomato/runs/`:

- `tomato history` — list past runs (timestamp, workflow, step, tokens, cost, status)
- `tomato history show <run-id>` — full prompts, responses, artifacts of one run
- `tomato history diff <run-id-a> <run-id-b> <artifact>` — compare an artifact across runs
- `tomato cost` — cumulative cost summary (overall / per day / per step / per model)

Output is plain text and pipe-friendly; users can grep / jq / awk it. The `.tomato/runs/` layout is documented as a stable contract for advanced users who want to write their own analysis tooling.

### 3.7 Tech Stack Selection

| Concern | Choice | Reason |
|--------|------|------|
| Main language | **Go** | Single binary / cross-platform / fast startup / strong subprocess orchestration |
| Auxiliary language | **Shell** | Adapter examples, build scripts, install scripts |
| Workflow definition language | **YAML** | Industry standard (Actions / Argo / Kubernetes), zero cognitive cost for users |
| LLM client | Mature Go-ecosystem clients (per provider) | Specific library to be finalized in implementation plan |
| Package distribution | **brew / scoop / direct binary download** | Native Go-ecosystem approach |

---

## 4. MVP, Roadmap, Risks

### 4.1 v1 Scope

**Includes**:

- ✅ Go single-binary CLI (macOS / Linux / Windows)
- ✅ Pure CLI architecture — every invocation is its own short-lived process; state on disk
- ✅ 7 built-in steps: `spec / design / impl / pr / review / test / task`
- ✅ `review_loop` meta-step (the only v1 control-flow primitive; max_rounds + severity-based exit)
- ✅ Workflow engine + `tomato.yaml` parsing + dynamic command registration
- ✅ Default `default` workflow (end-to-end example with `review_loop`)
- ✅ LLM Gateway: OpenAI (GPT) / Zhipu (GLM) / DeepSeek, all using OpenAI-compatible protocol
- ✅ Step-level model routing
- ✅ Token & budget control (three presets / prompt caching structure / local cache / budget config / cost visibility / toggleable optional items)
- ✅ Observability CLI subcommands (`tomato history` / `tomato cost`)
- ✅ Adapter driver CLI protocol (v1 version-locked, documented) covering `task` + `pr` + `review_loop` subcommands
- ✅ 1 officially maintained reference adapter (GitHub: implements `task` + `pr` + `comment-pr` + `mark-pr-*` subcommands)
- ✅ `docs/specs/<feature>/` per-feature directory convention (with `pr.md` and multi-round `reviews/`)
- ✅ `.tomato/` run logs, `tomato init` auto-writes `.gitignore`
- ✅ `runs_on:` field reserved in workflow YAML (only `local` accepted in v1); all built-in steps verified to be "file-in / file-out" so they can run on remote agents in v2 without rework (see §5)

**Excludes**:

- ❌ Local web UI / GUI / browser-based dashboard
- ❌ Long-lived daemon / background process (on the user's local machine)
- ❌ **Actual remote agent execution** (protocol designed in §5; implementation in v2)
- ❌ Bidirectional MCP support
- ❌ Multi-person shared state / team mode
- ❌ Workflow template distribution / community marketplace
- ❌ Cloud-hosted SaaS
- ❌ Visual UI mockups
- ❌ Built-in code sandbox / containerized execution

### 4.2 Roadmap

| Version | Theme | Key Deliverables |
|------|------|---------|
| **v0 PoC** | Validate thesis | CLI + engine + `design` single-step usable + 1 adapter example + user interviews validating H1 |
| **v1 Public Beta** | 7 steps all open + review_loop | See all "Includes" in 4.1 |
| **v1.x** | Protocol polish | More reference adapters (Linear / Jira / Tapd / ONES); local model (Ollama etc.) support |
| **v2** | Remote agent execution | Implement the §5 protocol: `runs_on: server-X` actually dispatches to a remote agent daemon; git as state bus; bearer-token auth; reference agent for self-hosted server |
| **v3** | MCP bridge | tomato can both call external MCP servers and act as an MCP server exposing steps to Cursor / Claude Code |
| **v4** | Workflow sharing | `tomato install <github-url>` to pull others' `tomato.yaml` |
| **v5 (?)** | Team / Cloud | Multi-user shared agents / hosted version (if OSS gains traction) |

### 4.3 Key Assumptions & Validation

| # | Assumption | Validation Method |
|---|------|----------|
| H1 | Individual developers are willing to write/read PRDs and design docs | **v0 PoC must do user interviews**: would you read/edit this generated design.md? |
| H2 | Pure-CLI observability (`tomato history` / `tomato cost`) is enough; users won't demand a GUI | v1 Beta: track GUI feature requests in issue tracker; if > 30% of feedback asks for GUI, reconsider |
| H3 | driver CLI protocol covers 80% of integration scenarios | v1.x: official + community write at least 5 adapters, see if an escape hatch is needed |
| H4 | The 7-step division is sufficient; users won't demand "6.5-step" cuts | v1 Beta: track naming distribution of user custom steps, look for patterns |
| H5 | YAML config + read-only UI is more accepted by engineers than pure visualization | Compare GitHub stars / forks with Dify, n8n over the same period |

### 4.4 Major Risks

| # | Risk | Mitigation Strategy |
|---|------|----------|
| R1 | Cursor / Claude Code adds "design doc generation" before v1 ships | Build moats across document-driven + workflow engine + adapter protocol; even if a single feature is co-opted, architectural differences remain |
| R2 | BYOK model cost is still too expensive for small teams | Built-in prompt/response cache + default cost-effective models (DeepSeek-4pro / GLM-5.2) + budget control (§2.9) |
| R3 | Adapter ecosystem fails to take off | v1.x: official writes 4–5 core adapters (GitHub / Linear / Jira / Tapd / ONES), lowering the bar to "users just follow the template" |
| R4 | "Document-driven" education cost is too high; users skip the design step | Make `tomato design` single-run extremely fast with crystal-clear artifacts; artifacts feed directly into Cursor to produce immediately visible downstream value |
| R5 | Per-invocation startup overhead (re-parsing yaml, re-loading cache) adds up in long workflows | Go startup is sub-100ms; lazy-load only what each step needs; benchmark in v0 PoC |

### 4.5 Shape of Success (qualitative dimensions)

Whether the vision succeeds is judged across these dimensions:

1. **An individual developer can complete the "idea → docs → code → PR" full process in one repo using tomato**
2. **Users' `tomato.yaml` files are found, forked, and modified on GitHub** — this is the true vitality of a workflow engine
3. **Someone in the community writes a driver CLI for non-English task systems (Tapd / ONES / Coding etc.)** — proves the value of the protocol
4. **An individual developer returns to their 6-month-old tomato project 3 months later and can still pick it up** — the essential test of the document-driven thesis

---

## 5. Distributed Execution (v2 Ahead-of-Time Design)

> **Status**: design only — NOT implemented in v1. v1 reserves the `runs_on:` field and verifies that all built-in steps are file-in / file-out, so v2 can ship this without rewriting steps.

### 5.1 Goal & Motivation

Allow each step in a workflow to execute on a designated **remote agent** rather than the user's local machine, so that:

- Heavy steps (e.g., `impl` on a 200k-line codebase, `test` requiring a GPU sandbox) run on a server with the right hardware
- Steps can call **server-local LLMs** (Ollama / vLLM / privately deployed models) that aren't reachable from the user's laptop
- A team can share a pool of compute without each developer configuring API keys

### 5.2 Topology

```
[user laptop]
   │
   │  tomato run
   │      ├── local steps  ──► current process (v1 behavior unchanged)
   │      │
   │      └── remote steps ──► HTTP/gRPC ──► [agent on server-A]
   │                                            │
   │                                            ├── git pull tomato/<run-id>
   │                                            ├── call server-local LLM
   │                                            ├── run step
   │                                            ├── git commit + push tomato/<run-id>
   │                                            └── return run-id + artifact paths
   │
   │  local: git pull tomato/<run-id> → next step
   ▼
docs/specs/<feature>/  (artifacts flow back via git)
```

**Roles**:
- **Local `tomato`** = orchestrator: parses yaml, decides `runs_on`, dispatches to local/remote, owns control flow (review_loop iteration, archiving, post-hooks)
- **Remote agent** = stateless executor: receives one step request, runs it, pushes artifacts via git, returns run-id. Knows nothing about workflows.
- **Git** = state bus: the only mechanism for cross-machine artifact transfer

### 5.3 Workflow Syntax

```yaml
agents:
  server-a:
    url: https://tomato-agent.example.com
    auth: ${TOMATO_AGENT_TOKEN_A}
    capabilities: [spec, design, impl]    # which steps this agent can run
    models:                                # which models this agent has access to
      - openai/gpt-5
      - ollama/qwen3-coder

  gpu-box:
    url: https://gpu-box.lan:9000
    auth: ${TOMATO_AGENT_TOKEN_GPU}
    capabilities: [test]

workflows:
  default:
    steps:
      - spec
      - design
      - impl:   { runs_on: server-a }
      - pr
      - review_loop:
          max_rounds: 2
          on_fail: stop
          fix_step: { runs_on: server-a }
      - test:   { runs_on: gpu-box }
      - task
```

**Validation rules**:
- Step's `runs_on` agent must declare the step in `capabilities`
- Step's model (from `models:` config) must be in the agent's `models` list
- Unknown agent name → hard error at workflow load time

### 5.4 Agent Protocol (v2 lock target)

Minimal HTTP API, JSON body, bearer-token auth:

| Endpoint | Purpose |
|----------|---------|
| `GET /healthz` | Liveness + reports capabilities, models, version |
| `POST /run` | Submit one step: `{ step, run_id, branch, inputs, env }` → `{ run_id, status: queued/running }` |
| `GET /run/<id>` | Poll status: `{ status, started_at, finished_at, exit_code, error? }` |
| `GET /run/<id>/stream` | SSE stream of stdout/stderr while running |
| `GET /run/<id>/meta` | Final run metadata (tokens, cost, cache hits, artifact paths) |

**Auth**: bearer token in `Authorization` header. Tokens managed per-agent in user's local config; never in `tomato.yaml` (which is in git).

**No agent-side state beyond the run**: agents do not store workflow context. Everything they need to know about prior steps comes from `git pull <branch>`. This keeps agents stateless and replaceable.

### 5.5 Git as State Bus

**Per-run branch convention**:

```
tomato/<run-id>           # e.g. tomato/2026-06-23-a1b2c3
```

- Each `tomato run` creates this branch from the user's current HEAD
- Every agent pushes its step's artifacts as one commit to this branch
- Local orchestrator `git pull --rebase` before dispatching the next step
- On run success (or failure with `--keep-branch`), user decides: squash-merge / cherry-pick / discard

**Tradeoffs accepted**:
- ⚠️ git push frequency is high (one per step) — acceptable because steps are not sub-second
- ⚠️ requires every agent to have git credentials + repo remote — explicit setup cost
- ⚠️ network partition between agent and git remote stalls the pipeline — v2 acceptable; user retries
- ✅ single state mechanism — no separate artifact store, no S3, no message queue
- ✅ full audit trail in git — every step's input/output is a commit, fully diff-able
- ✅ failure recovery is just `git checkout tomato/<run-id>` — no special protocol

### 5.6 Failure Semantics (v2 baseline)

v2 explicitly chooses **fail-fast over fault-tolerance**:

- Agent unreachable → step fails immediately, error printed with agent URL, pipeline stops
- Git push conflict → step fails, user resolves manually, then `tomato run --resume`
- Agent OOM / crash mid-step → step fails, partial artifacts on branch (visible in git), user can manually fix and resume
- Ctrl+C on local → local stops dispatching; in-flight remote step continues until done (its result is just discarded on next run unless reused via branch state)

**No auto-retry, no failover, no health-based load balancing in v2.** Those are v3+ if at all.

### 5.7 Local-Only Steps Stay First-Class

Steps without `runs_on:` (or `runs_on: local`) execute in the local `tomato` process exactly as in v1. Users who don't configure any agents see no behavior change — the v1 mental model is fully preserved as the default.

A workflow can mix local and remote steps freely; the orchestrator simply switches dispatch mode per step.

### 5.8 What v1 Does to Prepare for v2

The v1 design has already been shaped by anticipating §5:

| v1 constraint | Why it enables v2 |
|---------------|-------------------|
| All step inputs/outputs are files in `docs/specs/<feature>/` | Easy to ship via git diff to remote agents |
| `runs_on:` field already accepted in YAML parser (rejecting non-`local` values) | v2 just relaxes the validator |
| Steps are stateless — no in-process memory shared across steps | Steps can be picked up on any machine that has the branch checked out |
| `.tomato/runs/<id>/meta.json` records `model_id`, tokens, cost in a portable schema | Same schema works whether the step ran locally or on an agent |
| Adapter is already a separate executable (driver CLI protocol, §3.3) | Adapters can be installed per-agent without changes |

### 5.9 What v1 Explicitly Does NOT Do

- ❌ No agent reference implementation (not even a skeleton)
- ❌ No HTTP server in `tomato` binary
- ❌ No `runs_on:` value other than `local` accepted
- ❌ No agent / `runs_on` related CLI subcommands (no `tomato agent list`, etc.)
- ❌ No auth/token management infrastructure

The v1 binary contains zero networking code beyond LLM HTTP calls. This keeps v1 simple and secure.

---

## Appendix A · Glossary

| Term | Definition |
|------|------|
| **Step** | An atomic execution unit in a tomato workflow, corresponding to a class of domain operations (spec/design/impl/pr/review/test/task). 7 built-in; users can define new steps. |
| **Workflow** | A sequence of steps declared in `tomato.yaml`, automatically exposed as a top-level CLI command. |
| **Meta-step** | A built-in control-flow primitive (v1 has one: `review_loop`). Wraps regular steps with iteration / branching / exit conditions. Declared in `tomato.yaml` like a normal step but with structured parameters. |
| **review_loop** | The v1 meta-step that wraps `review` + `impl` (fix mode) in a bounded loop (default max_rounds=2). Exits on no blocking issues, or marks the PR as failed when rounds are exhausted. |
| **Agent** | (v2) A long-lived HTTP/gRPC server deployed on a remote machine that executes one tomato step on request. Stateless across runs; uses git as the state bus. v1 does not implement agents — see §5. |
| **`runs_on:`** | Workflow field declaring where a step executes. v1 accepts only `local`. v2 accepts agent names declared under `agents:`. Reserved in v1 syntax for forward-compatibility. |
| **Run Branch** | (v2) Git branch `tomato/<run-id>` created by `tomato run` and used as the state bus across agents. Each step's artifacts land as one commit on this branch. |
| **Artifact** | A file produced by a step, landing in `docs/specs/<feature>/`, in git. |
| **Run Log** | The complete context of a single step execution (prompt / response / metadata), landing in `.tomato/runs/`, not in git. |
| **Adapter** | A user-provided executable conforming to the tomato driver CLI protocol, used to bridge built-in steps to external systems (task platforms / messaging systems / doc systems). |
| **driver CLI protocol** | The stdin/stdout JSON over subprocess contract between tomato and adapters, version-locked in v1. |
| **BYOK** | Bring Your Own Key — users supply their own LLM API keys. |

---

## Appendix B · Mapping to Original Requirements

| Original requirement | Corresponding section | Notes |
|----------|----------|------|
| 1. AI-assisted requirements analysis | 2.1 `spec` step | |
| 2. AI-assisted design (architecture / UI / implementation) | 2.1 `design` step (produces three documents) | |
| 3. AI-assisted code implementation | 2.1 `impl` step | |
| 4. Task management platform plugin-ization | 2.1 `task` step + 3.3 driver CLI protocol | Plugin-ization via driver CLI, not traditional plugins |
| 5. Review code PRs | 2.1 `review` step + 2.1 `pr` step + 2.10 `review_loop` meta-step | Up to 2 fix rounds, auto-creates PR, posts comments back, marks failed on exhaustion |
| 6. Automated testing | 2.1 `test` step | |
