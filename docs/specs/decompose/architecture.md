# Decompose 功能设计文档

> 对一份设计文档做任务拆解,拆出多个可独立实现的子 feature,后续各子 feature 用 tomato 独立实现。
> 状态:设计待审阅 | 日期:2026-07-28 | 方案:A(新增 `decompose` step + `tomato decompose` 子命令)

## 概述

`tomato decompose` 把一份设计文档(任意 `.md`,通常是 architecture.md / PRD)拆解成多个可独立实现的子 feature。采用**两阶段**形态:

1. **生成清单**:`tomato decompose --input <设计文档>` -> 产出 `decomposition.md`(子 feature 清单 + 依赖图 + 验收标准)。
2. **落地**:`tomato decompose --apply` -> 读取清单,为每个子 feature 创建 `docs/specs/<子>/` 目录(写 `idea.md` + `parent-context.md`),并生成 `orchestration.yaml`(按依赖拓扑序 × MoSCoW 排序的执行计划)。

落地后,用户按 `orchestration.yaml` 的 `order` 手动逐个 `tomato run --feature <子>` 实现各子 feature。本期不内置自动执行引擎(给自动化留口子,但不做)。

## 方法论依据

本设计参考互联网成熟的「设计文档 -> 可独立交付单元」方法论(用户要求)。核心来源:

- **层级粒度**(Atlassian / Roman Pichler):整份设计文档视为 Epic,拆到「能独立走一次 spec->design->impl->PR、不被其他子 feature 阻塞」的 Feature/Story 粒度(典型 1~3 天)。https://www.atlassian.com/agile/project-management/epics-stories-themes
- **垂直切片 + INVEST + 9 种拆分法**(Humanizing Work / Bill Wake):每个子 feature 必须端到端产生可观测价值,禁止纯水平层切片(除非 enabler/spike)。https://www.humanizingwork.com/the-humanizing-work-guide-to-splitting-user-stories/
- **DAG + MoSCoW 排序**:`depends_on` 必须无环(拓扑排序),优先级用 MoSCoW(must/should/could/won't)。https://www.agilebusiness.org/resource/what-is-moscow-prioritization/
- **Spike 显式前置**:实现不确定先拆 spike(时间盒 + 要回答的问题),作为相关实现片的依赖。https://skillifysolutions.com/blogs/agile/what-is-a-spike-in-agile/
- **接口契约先行**:跨边界接口先冻结为独立子 feature,消费方依赖它。https://kpavlov.me/blog/contract-first-vs-contract-last/
- **C4 模型粒度分层**:拆分主要发生在 C4 L2 Container 层。https://c4model.com/
- **Definition of Ready**(Scrum.org):一个子 feature「可被独立实现」的判定 = 边界清晰、无未解依赖、有可测试验收标准、垂直切片。这是 `--apply` 落地前 DoR 机器校验的理论根基。https://www.scrum.org/resources/blog/what-difference-between-definition-done-dod-and-definition-ready-dor
- **AI 辅助拆解**(Andrew Ng Planning pattern):两阶段产出(先薄表 review DAG,再细化验收标准)+ reflection 自审。https://learn.deeplearning.ai/courses/agentic-ai/lesson/rm9bg7/agentic-design-patterns

这些约束直接写进 `DecomposePrompt`(见第 2 节)。

## 1. CLI 接口

新增子命令 `tomato decompose`,注册于 `cmd/root.go` 的 `NewRootCmd`,实现于 `cmd/commands.go` 的 `NewDecomposeCmd`(参考 `NewSpecCmd` 的 `--force`/`--feature` flag 模式与 `withFeatureAndModel` 包装器)。

**阶段 1 - 生成清单**
```
tomato decompose --input <设计文档路径> [--feature <父feature名>] [--force]
```
- `--input`(必填):设计文档路径,任意 `.md`(不要求是 tomato 风格的 architecture.md)。
- `--feature`(可选):父 feature 名,决定清单落点。默认 = 当前 git 分支(复用 `steps.ResolveFeature`)。
- `--force`:覆盖已存在的清单。
- 产物:`docs/specs/<父feature>/decomposition.md` + `docs/specs/<父feature>/source-design.md`(`--input` 内容的留档拷贝)。

**阶段 2 - 落地**
```
tomato decompose --apply [--feature <父feature名>] [--force]
```
- `--apply`:读取 `docs/specs/<父feature>/decomposition.md`,落地子 feature。
- 不需要 `--input`(读已有清单)。
- `--force`:覆盖已存在的子 feature 目录。
- 产物:每个子 feature 的 `docs/specs/<子feature>/` 目录 + `docs/specs/<父feature>/orchestration.yaml`。

**约束**:`--input` 与 `--apply` 互斥;两者都没传则报错提示用法。父子 feature 平级存放(`docs/specs/<父>/` 与 `docs/specs/<子>/` 都在 specs 下),因为子 feature 后续要独立 `tomato run --feature <子>`,`FeatureDir` 本就是 `docs/specs/<feature>/`;父目录的 `decomposition.md` + `orchestration.yaml` 充当索引/编排。

## 2. decompose step 行为 + prompt

### step 结构(照 `spec.go`/`design.go` 模式)

新建 `pkg/steps/decompose.go`:
- `Register("decompose", runDecompose)`(`init()` 中)。
- 输入文件:`docs/specs/<父feature>/source-design.md`。
- 输出文件:`docs/specs/<父feature>/decomposition.md`(单文件)。

### 输入占位符决策

`--input` 是任意路径,文件名不固定;但 `runner.Execute` 的 `buildMessages` 用 `filepath.Base(inPath)` 作为占位符 key(`{{.basename}}`),文件名不固定就无法在 prompt 里写死占位符。**解法**:命令层先把 `--input` 内容写到固定名 `source-design.md`,prompt 用固定占位符 `{{.source-design.md}}`。好处:① 复用现有 `runner.Execute`,不改 runner;② `source-design.md` 留档有溯源价值。

命令层 `tomato decompose --input X` 职责:把 X 写到 `source-design.md` -> 跑 decompose step。

### 输出:单文件,避开 splitArtifacts 冲突

`decomposition.md` 是单文件输出,不需要 `---TOMATO-ARTIFACT---` marker(该 marker 用于多文件分割)。`runner.Execute` 对单输出文件天然把整段 response 写入(`splitArtifacts` 无 marker 时返回 whole text)。这规避了「多个子 feature 同名 idea.txt basename 冲突」--拆解阶段只产出一个清单文件,子 feature 文件由 `--apply` 的 Go 代码直接写,不走 LLM/splitArtifacts。

### DecomposePrompt(全文)

```
You are tomato's architecture decomposition analyst.

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
```yaml
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
```

Rules:
- The yaml block in section 3 is authoritative; --apply parses only it.
- ids are stable: F-001, F-002, ... in definition order.
- depends_on references must point to earlier ids (DAG, no cycles).
- Every sub-feature must have at least one acceptance_criterion.
- Spike sub-features (is_spike: true) must have a non-empty timebox.
- Section 1 table and section 2 details are human-readable summaries of section 3;
  keep them consistent with the yaml block.
```

## 3. 清单 schema(decomposition.md 结构)

### 文件结构:markdown 外壳 + 权威 yaml 块

`decomposition.md` = 人读 markdown(§0 元信息、§1 薄表、§2 详情)+ §3 的 ```yaml 代码块作为机器可读权威。`--apply` 只解析 §3 的 yaml 块(`yaml.Unmarshal`,tomato 已用 yaml 库解析 tomato.yaml,复用)。这样人可读、机器稳健,不依赖脆弱的 markdown 列表解析。

### 字段 enum

| 字段 | 取值 |
|------|------|
| `slice_type` | workflow / operations_crud / business_rule / data_variation / data_entry / major_effort / simple_complex / defer_performance / spike / contract / enabler |
| `c4_level` | context / container / component / code |
| `priority` | must / should / could / won't |
| `complexity` | S / M / L |

### Go 侧 struct(新 `pkg/decompose`)

```go
type Decomposition struct {
    Source        string       `yaml:"source"`
    TotalFeatures int          `yaml:"total_features"`
    DagCheck      string       `yaml:"dag_check"`
    CriticalPath  []string     `yaml:"critical_path"`
    Spikes        []string     `yaml:"spikes"`
    Features      []SubFeature `yaml:"features"`
}

type SubFeature struct {
    ID                 string   `yaml:"id"`
    Title              string   `yaml:"title"`
    Goal               string   `yaml:"goal"`
    UserValue          string   `yaml:"user_value"`
    SliceType          string   `yaml:"slice_type"`
    C4Level            string   `yaml:"c4_level"`
    Priority           string   `yaml:"priority"`
    DependsOn          []string `yaml:"depends_on"`
    AcceptanceCriteria []string `yaml:"acceptance_criteria"`
    Complexity         string   `yaml:"complexity"`
    IsSpike            bool     `yaml:"is_spike"`
    Timebox            string   `yaml:"timebox,omitempty"`
    OutOfScope         []string `yaml:"out_of_scope"`
    OpenQuestions      []string `yaml:"open_questions,omitempty"`
}
```

## 4. --apply 落地逻辑 + orchestration.yaml

### --apply 流程(纯 Go,不跑 LLM)

1. 读 `docs/specs/<父>/decomposition.md`,提取 §3 的 ```yaml 块,`yaml.Unmarshal` 到 `Decomposition`。
2. **DoR 机器校验**(全部通过才进入落地):
   - id 唯一
   - `depends_on` 引用的 id 存在
   - **DAG 无环**(拓扑排序时检测,有环报错并列出环链)
   - spike 必有非空 timebox
   - 必填字段(id/title/goal/user_value/slice_type/c4_level/priority/acceptance_criteria/complexity/is_spike/out_of_scope)非空
3. 按拓扑序 × MoSCoW 排序,对每个子 feature:
   - 子 feature 名 = `<父feature>-<小写id>`(如父=main、id=F-001 -> `main-f001`),保证唯一且归属清晰。
   - 建 `docs/specs/<子>/` 目录(`--force` 覆盖;否则已存在则报错)。
   - 写 `idea.md`:从清单的 `goal/user_value/acceptance_criteria/out_of_scope/priority/depends_on` 拼成结构化需求(markdown)。
   - 写 `parent-context.md`:本子 feature 边界(同 idea.md 的结构化信息)+ **完整 `source-design.md`** 作为父架构上下文。
   - 写 `parent.json`:`{parent_feature, id, source: decomposition.md}`(溯源)。
4. 生成 `docs/specs/<父>/orchestration.yaml`。

### parent-context.md 的语义策略

`--apply` 是纯 Go,做不了语义抽取。策略:提供「本子 feature 的明确 goal/scope + 父架构全文」,让后续 design step 的 LLM 自己聚焦相关部分。语义工作交给 design 的 LLM(它擅长),`--apply` 只做确定性拼接。

### idea.md 结构

```markdown
# <title>

## 目标
<goal>

## 用户价值
<user_value>

## 验收标准
- <ac1>
- <ac2>

## 不在范围内
- <oos1>

## 优先级
<priority>

## 依赖
<depends_on>
```

### parent-context.md 结构

```markdown
# 子 feature 上下文: <title> (<id>)

## 本子 feature 的边界(来自父拆解清单)
<同 idea.md 的结构化信息>

## 父 feature 设计文档(完整)
<source-design.md 全文>
```

### orchestration.yaml 结构

```yaml
parent_feature: main
source: docs/specs/main/decomposition.md
# 按依赖拓扑序 × MoSCoW 排序;spike 优先(去风险)
order:
  - feature: main-f002   # spike 优先
    id: F-002
    title: 调研 X 可行性
    priority: must
    depends_on: []
  - feature: main-f001
    id: F-001
    title: 用户登录
    priority: must
    depends_on: [F-002]
  - feature: main-f003
    id: F-003
    title: ...
    priority: should
    depends_on: [F-001]
```

排序规则:拓扑序(依赖在前)为骨架,同层按 MoSCoW(must > should > could;`won't` 排除),spike 在同优先级内前置。用法:用户按 `order` 手动逐个 `tomato run --feature <feature>`。未来可加 `tomato run-orchestration` 消费它,本期不做。

### 连带改动(让「idea.md + 架构上下文片段」真正生效)

1. **spec step 支持 idea.md**:`runSpec`(`pkg/steps/spec.go`)优先读 `idea.md`,回退 `idea.txt`(向后兼容现有 feature)。空值守卫调整为「两者皆无/皆空才报错」。子 feature 用 idea.md,老 feature 的 idea.txt 不受影响。
2. **design step 读 parent-context.md(可选)**:`runDesign`(`pkg/steps/design.go`)输入加 `parent-context.md`(存在则读入,prompt 加 `{{.parent-context.md}}` 占位符 + 约束「若存在父架构,子 feature 设计须与其一致、不重复定义父已定的边界」)。子 feature 的 design 因此继承父架构;独立 feature 无此文件,行为不变。

## 5. 错误处理

原则:**阶段1(生成清单)对 LLM 输出宽容,阶段2(--apply)严格**。先校验后落地,降低中途失败。

| 场景 | 处理 | 依据 |
|------|------|------|
| `--input` 不存在或为空 | 报错退出 | 类比 spec 空值守卫 |
| `--input` 与 `--apply` 同时传 | 报错(互斥) | 一个生成、一个消费 |
| 两者都没传 | 报错提示用法 | 无默认动作 |
| `decomposition.md` 已存在且无 `--force` | 报错 | 类比 spec `outputsExist` |
| 阶段1 LLM 失败/空响应 | `runner.Execute` 已有 failure 透传 | 复用现有 |
| 阶段1 生成后 yaml 自校验失败 | **警告(非致命)**,提示检查或重跑 | LLM 输出不稳,人读部分可能仍有用;严格校验留给 --apply |
| `--apply` 时 `decomposition.md` 不存在 | 报错 | 没跑过阶段1 |
| `--apply` 时 yaml 块缺失/无法解析 | 报错,指向 yaml 块,建议手改或重跑阶段1 | 清单损坏 |
| DoR 校验失败(重复 id / 悬空依赖 / DAG 有环 / spike 无 timebox / 必填空) | **报错列出具体问题,不落地** | DoR 是「可被独立实现」的硬门槛 |
| `--apply` 子 feature 目录已存在且无 `--force` | 报错停住(安全优先) | 避免覆盖用户已开始的子 feature 工作 |
| `--apply` 时 `source-design.md` 缺失 | 警告(父架构上下文缺失),仍落地 | `idea.md` 不依赖它,`parent-context.md` 边界部分仍可用 |
| budget 超限 | `runner.Execute` 的 `on_exceed: warn` 继续 | 复用现有 |

**DoR 校验前置**:所有 DoR 检查在落地前一次性完成,全部通过才开始写文件,把「中途失败」概率压到最低(落地阶段只剩确定性的文件写)。

**落地中途文件写失败的策略**:若落地中途某个子 feature 写失败,报错并列出「已落地 / 未落地」,**不回滚**(事务性回滚复杂且可能误删用户已有内容)。用户可对未落地的部分重跑(`--force` 覆盖已落地的,或手改清单)。

**DAG 环检测**:拓扑排序时检测,有环则报错并列出参与环的 id 链(如 `F-003 -> F-005 -> F-003`),便于定位。

## 6. 测试计划

遵循 TDD(先写测试再实现)。代码组织:`pkg/decompose/`(新,放 --apply 的纯逻辑:DoR 校验/拓扑排序/落地/orchestration,易单测),`pkg/steps/decompose.go`(LLM step),`cmd/commands.go`(命令)。

### A. decompose step(`pkg/steps/decompose_test.go`)

- `TestDecomposePrompt`:prompt 含方法论关键词(垂直切片/INVEST/DAG/spike/契约/MoSCoW/C4/out_of_scope)。
- `TestDecomposeStepRegistered`:`Get("decompose")` 成功。
- `TestRunDecomposeWritesDecomposition`:有 `source-design.md`,mock LLM 返回清单文本 -> 验证 `decomposition.md` 写入(类比 `TestRunFastBypassesResponseCache` 风格)。

### B. DoR 校验(`pkg/decompose/dor_test.go`)

- id 重复 -> 报错
- `depends_on` 引用不存在的 id -> 报错
- DAG 有环 -> 报错并列出环链
- spike 缺 timebox -> 报错
- 必填字段空 -> 报错
- 合法清单 -> 通过

### C. 拓扑排序 + orchestration(`pkg/decompose/order_test.go`)

- 依赖在前(拓扑序)
- 同层 MoSCoW(must > should > could;`won't` 排除)
- spike 同优先级前置
- `orchestration.yaml` 生成且 order 正确

### D. 落地(`pkg/decompose/apply_test.go`)

- 子 feature 目录创建,`idea.md`/`parent-context.md`/`parent.json` 内容正确(idea.md 含 goal/AC;parent-context.md 含边界 + source-design.md 全文)
- 目录已存在无 `--force` -> 报错;有 `--force` -> 覆盖
- yaml 块从 decomposition.md 提取 + Unmarshal 成功

### E. 连带改动测试

- `spec_test.go` 扩展:idea.md 存在读它;仅 idea.txt 存在读它(兼容);两者皆空报错(守卫)。
- `design_test.go` 扩展:parent-context.md 存在则作为输入;不存在行为不变。

### F. 命令层(`cmd/commands_test.go`)

- `--input` 与 `--apply` 互斥
- `decomposition.md` 已存在无 `--force` 报错

## 代码组织总结

| 位置 | 职责 |
|------|------|
| `pkg/steps/decompose.go` | `DecomposePrompt` + `runDecompose`(LLM step,读 source-design.md,输出 decomposition.md) |
| `pkg/steps/spec.go` | 连带改动:支持 idea.md 优先 / idea.txt 回退 |
| `pkg/steps/design.go` | 连带改动:可选读 parent-context.md |
| `pkg/decompose/`(新) | DoR 校验、拓扑排序、落地、orchestration.yaml 生成、yaml 块解析(纯逻辑,可单测) |
| `cmd/commands.go` | `NewDecomposeCmd`:参数解析、写 source-design.md、调 step / 调 pkg/decompose 落地 |
| `cmd/root.go` | 注册 `NewDecomposeCmd` |

## 范围边界(本期不做)

- 不内置自动执行引擎(`orchestration.yaml` 只生成不执行;未来 `tomato run-orchestration` 另做)。
- 不做子 feature 之间的代码级冲突检测(各子 feature 独立分支实现,冲突由 git/PR 流程处理)。
- 不做拆解清单的版本管理/增量更新(重新 `tomato decompose --force` 整体覆盖)。
