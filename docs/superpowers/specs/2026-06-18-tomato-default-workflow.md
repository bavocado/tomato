# Tomato Default Development Workflow Diagrams

- **Date**: 2026-06-18
- **Status**: Draft
- **Companion document**: [2026-06-18-tomato-vision-design.md](./2026-06-18-tomato-vision-design.md)
- **Type**: Diagram collection for the Vision document

> This file visually presents the default workflow described in §2 of the vision document using Mermaid diagrams. All diagrams are consistent with the vision document; in case of conflict, the vision document prevails.

---

## Diagram 1 · Default Development Workflow Main Diagram (`tomato run` full process)

`tomato run` is equivalent to `tomato run default`, executing the 7 built-in steps in order: `spec → task → design → impl → pr → review_loop → test`. The `task` step runs early so subsequent step post-hooks can update the external task status. The `review_loop` is the only built-in meta-step (control-flow primitive), allowing up to 2 fix iterations before failing the pipeline.

```mermaid
flowchart TD
    classDef cmd fill:#fef3c7,stroke:#f59e0b,color:#000
    classDef art fill:#dbeafe,stroke:#3b82f6,color:#000
    classDef ext fill:#fce7f3,stroke:#ec4899,color:#000
    classDef ep  fill:#d1fae5,stroke:#10b981,color:#000
    classDef fail fill:#fecaca,stroke:#dc2626,color:#000
    classDef loop fill:#e9d5ff,stroke:#9333ea,color:#000

    Start([💡 User input<br/>rough idea / feature name]):::ep

    Start --> C1["▶ tomato spec<br/>requirements analysis"]:::cmd
    C1 --> A1[/"📄 prd.md"/]:::art

    A1 --> CTask["▶ tomato task<br/>create external task"]:::cmd
    CTask --> ATask[("🌐 task platform<br/>task created")]
    ATask --> C2["▶ tomato design<br/>architecture + UI + implementation"]:::cmd
    C2 --> A2[/"📄 architecture.md<br/>📄 ui-spec.md<br/>📄 implementation.md"/]:::art

    A2 --> C3["▶ tomato impl<br/>code implementation"]:::cmd
    C3 --> A3[/"💾 source diff"/]:::art
    A3 -->|post-hook| S1[/"📤 status → implemented<br/>trio archived to v&lt;N&gt;/<br/>real architecture.md rewritten"/]:::art

    A3 --> C4["▶ tomato pr<br/>push branch + open draft PR"]:::cmd
    C4 -->|adapter create-pr| Ext1[("🌐 GitHub/GitLab<br/>draft PR opened")]:::ext
    Ext1 --> A4[/"📄 pr.md<br/>(ref + URL)"/]:::art

    A4 --> RL{{"🔁 review_loop<br/>max_rounds=2"}}:::loop
    RL --> RV1["▶ round 1: tomato review<br/>+ adapter comment-pr"]:::cmd
    RV1 --> AR1[/"📝 reviews/r1-comments.md<br/>(severity-classified)"/]:::art
    AR1 --> D1{blocking issues?}:::loop
    D1 -- no --> READY["✓ adapter mark-pr-ready<br/>status → reviewed"]:::ep
    D1 -- yes --> FIX1["▶ tomato impl --fix r1<br/>+ adapter update-pr"]:::cmd
    FIX1 --> RV2["▶ round 2: tomato review<br/>+ adapter comment-pr"]:::cmd
    RV2 --> AR2[/"📝 reviews/r2-comments.md"/]:::art
    AR2 --> D2{blocking issues?}:::loop
    D2 -- no --> READY
    D2 -- yes --> FAIL["❌ adapter mark-pr-failed<br/>status → review_failed<br/>print PR URL + final comments"]:::fail
    FAIL --> FAILEnd([🛑 pipeline stops]):::fail

    READY --> C5["▶ tomato test<br/>test generation + run"]:::cmd
    A2 -.supplementary context.-> C5
    A3 -.supplementary context.-> C5
    C5 --> A5[/"🧪 test files + report"/]:::art

    A5 --> Done([✅ tomato run complete]):::ep
```

### Reading notes

- **Solid thick arrows** = default sequential flow
- **Dashed "supplementary context"** = the step also reads upstream artifacts when executing (not a trigger relationship)
- **Colors**:
  - 🟡 Yellow = CLI commands
  - 🔵 Blue = git-tracked artifacts (in `docs/`)
  - 🟣 Pink = external systems (PRs / task platforms)
  - 🟢 Green = flow start / end points
  - 🟪 Purple = `review_loop` meta-step control flow
  - 🔴 Red = failure path

### Dependency summary between steps

| Step | Main input | Supplementary context | Main artifact | post-hook |
|------|--------|------------|--------|------------|
| spec | User's rough idea | — | `prd.md` | status → `specified` |
| task | `prd.md` | — | external task created (`task.json`) | enables later status post-hooks |
| design | `prd.md` | — | `architecture.md` / `ui-spec.md` / `implementation.md` | status → `designed` |
| impl | design trio | — | source diff | status → `implemented`; trio archived to `v<N>/`; real `architecture.md` rewritten |
| pr | git working tree | — | `pr.md` (PR ref + URL) | status → `pr_opened` (draft) |
| review_loop | source diff + `pr.md` | — | `reviews/r<n>-comments.md`, PR comments posted | status → `reviewed` (pass) OR `review_failed` (pipeline stops) |
| test | source diff | design trio | test files + report | status → `tested` |

---

## Diagram 2 · Single-Step Internal Interaction

Each step at runtime involves three parties: user terminal, tomato CLI (single short-lived process), LLM provider. The `task` step additionally invokes the user-provided driver CLI adapter via subprocess.

### 2.1 General Step (using `tomato design` as example)

```mermaid
sequenceDiagram
    actor U as User terminal
    participant T as tomato process<br/>(short-lived)
    participant L as LLM Provider<br/>(routed per yaml)
    participant FS as File system

    U->>T: tomato design
    T->>FS: read tomato.yaml + docs/specs/<feature>/prd.md
    FS-->>T: config + PRD content
    T->>T: render prompt template<br/>(architecture + UI + implementation — 3 segments)
    T->>L: call LLM (streaming)
    L-->>T: streaming response
    T-->>U: stream tokens to stdout in real time
    T->>FS: write architecture.md
    T->>FS: write ui-spec.md
    T->>FS: write implementation.md
    T->>FS: drop .tomato/runs/&lt;id&gt;/ run log
    T-->>U: completion notice + run-id
    Note over U: inspect later via<br/>tomato history show <run-id>
```

### 2.2 Extra step for `task` (driver CLI adapter)

```mermaid
sequenceDiagram
    participant T as tomato process
    participant A as driver CLI adapter<br/>(user-provided executable)
    participant Ext as External task system<br/>(Linear/Jira/...)
    participant FS as File system

    T->>FS: read prd.md / design artifacts
    T->>A: fork/exec: <adapter> create-task<br/>stdin: JSON context
    activate A
    A->>Ext: platform API call
    Ext-->>A: task creation result
    A-->>T: stdout: JSON { task_ref, url }
    deactivate A
    T->>FS: drop run log (including task ref)
```

For detailed adapter protocol specs, see vision document §3.3.

---

## Diagram 3 · Iteration / Re-run Model

`tomato run` is not a one-shot pipeline — all artifacts are in git, so **at any point you can hand-edit any artifact and then re-run from any middle step**; downstream regenerates based on the new artifact.

```mermaid
flowchart LR
    classDef art fill:#dbeafe,stroke:#3b82f6,color:#000
    classDef cmd fill:#fef3c7,stroke:#f59e0b,color:#000
    classDef edit fill:#fde68a,stroke:#d97706,color:#000
    classDef ver fill:#ede9fe,stroke:#8b5cf6,color:#000

    A1[/"prd.md"/]:::art --> A2[/"design trio"/]:::art
    A2 --> A3[/"code diff"/]:::art
    A3 --> AP[/"pr.md"/]:::art
    AP --> A4[/"reviews/r&lt;n&gt;-comments.md"/]:::art
    A3 --> A5[/"tests"/]:::art
    A2 --> A6[/"external tasks"/]:::art

    H((✍ User edits<br/>prd.md)):::edit -.overwrites.-> A1
    A1 -.can trigger rerun.-> R2["▶ tomato design --force"]:::cmd
    R2 -.regenerates.-> A2

    H2((✍ User edits<br/>architecture.md)):::edit -.overwrites.-> A2
    A2 -.can trigger rerun.-> R3["▶ tomato impl --force"]:::cmd
    R3 -.regenerates.-> A3
    R3 -.archive + rewrite.-> V1[/"v&lt;N&gt;/ archive<br/>+ rewrite architecture"/]:::ver

    H3((✍ User fixes code<br/>after review_failed)):::edit -.overwrites.-> A3
    A3 -.can resume.-> R4["▶ tomato run --resume<br/>or tomato review --force"]:::cmd
    R4 -.re-enters review_loop.-> A4

    Re["▶ tomato run --from impl"]:::cmd -.re-runs whole chain from middle step.-> A3
```

### Core promises

Every artifact is markdown / text, fully tracked by git, so these are all first-class operations:

- ✏️ AI made a mistake → hand-edit the artifact
- 🔄 Want to regenerate downstream → run `tomato <step> --force` or `tomato run --from <step>`
- ⏪ Roll back an experiment → just git-revert, no context lost
- 🔍 Debug why the LLM wrote it this way → look at `.tomato/runs/<id>/prompts.jsonl`
- 📦 Auto-archive old design after impl → `docs/specs/<feature>/v<N>/` preserves full history
- 🏗️ Architecture doc is always the truth → `architecture.md` reflects the real code structure after the most recent impl

---

## Mapping to vision document

| This file's section | Vision document section |
|------------|---------------------|
| Diagram 1 main flow | §2.1 seven built-in steps, §2.2 default workflow, §2.6 data flow |
| Diagram 1 post-hook | §2.1 step status lifecycle, §2.8 architecture versioning & rewrite |
| Diagram 1 `review_loop` subgraph | §2.10 review loop (max_rounds / severity / on_fail) |
| Diagram 1 `pr` step + adapter calls | §2.1 `pr` step, §3.3 driver CLI protocol (create-pr / update-pr / comment-pr / mark-pr-*) |
| Diagram 2.1 general single-step | §3.1 process model, §3.2 core components, §2.7 run logs |
| Diagram 2.2 task step | §3.3 driver CLI protocol |
| Diagram 3 iteration re-run | §2.1 step idempotency, §2.6 artifacts as interfaces, §2.8 architecture versioning, §2.10 review_loop recovery |

---

## Diagram 4 · Token & Budget Control Flow

tomato controls tokens at three levels: **saving (caching + tiered routing), visibility (meta + CLI), caps (budget + on-exceed strategy)**.

```mermaid
flowchart TD
    classDef cmd fill:#fef3c7,stroke:#f59e0b,color:#000
    classDef cache fill:#d1fae5,stroke:#10b981,color:#000
    classDef llm fill:#dbeafe,stroke:#3b82f6,color:#000
    classDef budget fill:#fce7f3,stroke:#ec4899,color:#000
    classDef ui fill:#ede9fe,stroke:#8b5cf6,color:#000

    Start([▶ tomato run]):::cmd

    Start --> Check[/"read budget.mode<br/>(frugal/balanced/quality)"/]:::budget
    Check --> Route[/"route per-step model by tier<br/>spec/design strong, rest cost-effective"/]:::budget

    Route --> Step["execute single step"]:::cmd
    Step --> Cache{local cache hit?}:::cache
    Cache -- hit --> Hit["zero tokens<br/>meta records cache hit"]:::cache
    Cache -- miss --> Caching["build prompt<br/>segment-mark cache_control"]:::cmd
    Caching --> LLM["call LLM<br/>(provider-native caching)"]:::llm
    LLM --> Count[/"accumulate tokens<br/>check per_step cap"/]:::budget

    Count -- under cap --> Record["write meta.json<br/>tokens / cost / cache hits"]:::ui
    Count -- cap hit --> Exceed{on_exceed?}:::budget
    Exceed -- warn --> Record
    Exceed -- degrade --> Degrade["switch to degrade_to model<br/>continue running"]:::budget
    Exceed -- fail --> Fail([❌ stop run]):::budget

    Hit --> Record
    Record --> Global{global_per_run<br/>cap hit?}:::budget
    Global -- no --> Next["next step"]:::cmd
    Global -- yes --> Fail
    Next --> Step

    Record -.metadata.-> UI[/"CLI cost views<br/>tomato history / tomato cost"/]:::ui
```

### Reading notes

- **Green** = cache path (zero or low tokens)
- **Blue** = actual LLM call (via provider-native caching)
- **Pink** = budget control points (per_step and global two-level checks)
- **Purple** = cost visibility (meta + CLI subcommands)
- **Three presets** are read at run start, determining which model each step uses
- **Two-level caps**: per-step budget hit → `on_exceed` strategy; global budget hit → stop directly

### Key savings points

| Means | When effective | Savings magnitude |
|------|----------|----------|
| Provider-native prompt caching | Every actual LLM call | Input tokens ~10x |
| Local response cache | Same input re-run (common in iteration scenarios) | 100% (zero calls) |
| Tiered model routing | Per-step independent model selection | strong model for design, cheap for the rest |
| Toggleable optional items | User proactively turns off non-essential calls | Each saves one full call |

### Mapping to vision document (supplementary)

| This file's section | Vision document section |
|------------|---------------------|
| Diagram 4 token control flow | §2.9 Token & Budget Control |
| Diagram 5 distributed execution (v2 design) | §5 Distributed Execution |

---

## Diagram 5 · Distributed Execution (v2 Ahead-of-Time Design)

> ⚠️ **NOT implemented in v1.** This diagram visualizes the v2 target topology described in vision §5. v1's `runs_on:` field is reserved syntax accepting only `local`; agents and remote dispatch are not built. This diagram exists so that v1 design decisions can be validated against the v2 target.

```mermaid
flowchart TD
    classDef local fill:#fef3c7,stroke:#f59e0b,color:#000
    classDef remote fill:#dbeafe,stroke:#3b82f6,color:#000
    classDef git fill:#d1fae5,stroke:#10b981,color:#000
    classDef llm fill:#ede9fe,stroke:#8b5cf6,color:#000
    classDef out fill:#fce7f3,stroke:#ec4899,color:#000

    subgraph User["👤 User Laptop"]
        Orch["tomato (orchestrator)<br/>parses yaml<br/>resolves runs_on<br/>controls review_loop"]:::local
    end

    subgraph Git["📦 Git Remote (state bus)"]
        Branch[("tomato/&lt;run-id&gt;<br/>commits per step")]:::git
    end

    subgraph ServerA["🖥️ Server A (agent)"]
        AgentA["tomato-agent daemon<br/>HTTP/gRPC"]:::remote
        LLMA["server-local LLM<br/>Ollama / vLLM / GPT-5"]:::llm
        AgentA -.calls.-> LLMA
    end

    subgraph GPUBox["🖥️ GPU Box (agent)"]
        AgentB["tomato-agent daemon"]:::remote
        LLMB["GPU sandbox<br/>+ test runners"]:::llm
        AgentB -.calls.-> LLMB
    end

    Orch -->|1. local step:<br/>spec / design / pr / task| LocalRun["run in current process<br/>(v1 behavior)"]:::local
    Orch -->|2. remote step impl<br/>POST /run + branch ref| AgentA
    Orch -->|3. remote step test<br/>POST /run + branch ref| AgentB

    LocalRun -->|commit + push| Branch
    AgentA -->|git pull tomato/&lt;run-id&gt;<br/>execute<br/>git push| Branch
    AgentB -->|git pull<br/>execute<br/>git push| Branch

    Branch -.|local: git pull<br/>before next step|.-> Orch

    Orch -->|after all steps| Out[("docs/specs/&lt;feature&gt;/<br/>artifacts in working tree")]:::out
```

### Reading notes

- 🟡 Yellow = local execution (v1 behavior unchanged)
- 🔵 Blue = remote agent daemons (v2 only)
- 🟢 Green = git as the single state bus
- 🟪 Purple = LLM instances reachable only from a specific agent
- 🟣 Pink = final artifacts in the user's working tree

### Sequence: one remote step end-to-end (v2)

```mermaid
sequenceDiagram
    actor U as User
    participant T as tomato (local)
    participant G as Git Remote
    participant A as Agent (server-A)
    participant L as Server-local LLM

    U->>T: tomato run
    T->>T: parse yaml, see `impl: { runs_on: server-a }`
    T->>G: create branch tomato/<run-id>, push current state
    T->>A: POST /run { step: impl, branch: tomato/<run-id> } + bearer token
    activate A
    A->>G: git clone / pull tomato/<run-id>
    A->>A: read design trio from working tree
    A->>L: call local LLM (streaming)
    L-->>A: streaming response
    A->>A: write code diff, run impl post-hook (archive + rewrite architecture)
    A->>G: git commit + push tomato/<run-id>
    A-->>T: { run_id, status: done, artifact_paths: [...] }
    deactivate A
    T-->>U: stream progress (relayed from agent's SSE)
    T->>G: git pull tomato/<run-id>
    T->>T: next step (could be local again)
```

### v1 → v2 invariants preserved

| v1 design choice | Why it stays correct under v2 |
|------------------|------------------------------|
| Steps communicate via files in `docs/specs/<feature>/` | Files travel via git — no change to step internals |
| `tomato.yaml` is source of truth | Same yaml runs locally or distributed; only `runs_on:` changes |
| Per-run logs in `.tomato/runs/<id>/` | Agent's run logs stay agent-side; local just stores its own |
| Control flow (review_loop, archiving, post-hooks) is in the orchestrator | Never delegated to agents — agents stay stateless and replaceable |
| Adapters are separate executables | Installed per-agent; no protocol change needed |

### Failure modes (v2 baseline)

| Failure | v2 behavior |
|---------|-------------|
| Agent unreachable | Step fails immediately, agent URL printed, pipeline stops |
| Agent OOM mid-step | Partial state visible on branch; user manually fixes + `tomato run --resume` |
| Git push conflict | Step fails; user resolves; resume |
| Local Ctrl+C | Local stops dispatching; in-flight remote step continues to completion |
| Auth token rejected | Step fails at submission; config error |

**No auto-retry, no failover, no health-based load balancing in v2.** Those are v3+ if at all.
