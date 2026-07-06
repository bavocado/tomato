---
name: tomato
description: >
  AI 驱动的软件开发工作流引擎。当用户提到 tomato、需求分析、PRD、技术方案、
  架构设计、实现代码、代码审查、测试生成，或者想要「从想法到 PR」的端到端
  开发流程时，使用此 skill。触发词包括：tomato、spec、PRD、需求文档、设计方案、
  架构文档、实现方案、代码审查、review loop、tomato run、tomato init。
  即使用户没有明确说出「tomato」，只要他们在描述一个从需求到代码的完整开发
  工作流，就应该考虑使用此 skill。
---

# Tomato — AI 开发工作流引擎

在 Claude Code 内直接执行 tomato 的完整开发工作流：从原始需求到可合并的 PR。

---

## run — 执行工作流

当用户说出以下任一表达时，执行此命令：

- `tomato run`
- `tomato run <workflow>`
- `tomato run --from <step>`
- `tomato run --resume`
- `tomato run --feature <name>`
- 「执行 tomato 工作流」「跑 tomato」「开始 tomato pipeline」

### 执行流程

#### Phase 0 — 加载配置

1. 读取当前目录的 `tomato.yaml`
2. 如果文件不存在：询问用户是否初始化。若同意，创建 `tomato.yaml` 和 `docs/specs/` 目录；若拒绝，终止
3. 从 `tomato.yaml` 提取：
   - `workflows.<name>.steps` — 步骤列表
   - `feature` — 默认 feature 名（可选）
   - `custom_steps` — 自定义步骤（可选）

#### Phase 1 — 解析参数

| 参数 | 来源 |
|------|------|
| **workflow** | 用户指定 → `tomato.yaml` 第一个 key（通常是 `default`） |
| **feature** | 用户指定 `--feature` → `tomato.yaml` 的 `feature` → git 分支名 → `"current-feature"` |
| **--from** | 从指定 step 开始，跳过之前的步骤 |
| **--resume** | 从上次失败的 step 继续 |

`--from` 和 `--resume` 互斥。

#### Phase 2 — 准备环境

1. 确保 `docs/specs/<feature>/` 目录存在
2. 创建/切换到 feature 分支 `tomato/<feature>`：
   ```bash
   git checkout main && git pull origin main
   git checkout -b tomato/<feature>
   ```
   分支已存在（resume）则直接切换

#### Phase 3 — 按序执行步骤

遍历工作流的每个 step。如果指定了 `--from <step>`，从该 step 开始。

**普通 step**（spec、design、impl、review、test、pr、task）：

1. 按下方「单步执行参考」执行
2. 完成后 git commit 制品
3. 对于 impl，代码变更也一并提交
4. 记录状态到 `.tomato/runs/state.json`
5. 如果失败：保存状态，停止，告知用户

**meta-step `review_loop: {max_rounds: N, on_fail: stop|continue|ask}`**：

```
round = 1
while round <= max_rounds + 1:
    1. 执行 review (r<N>)
    2. 输出审查结果到 reviews/r<N>-comments.md + .json
    3. 检查 has_blocking:
       - false → 审查通过，循环结束
       - true 且 round <= max_rounds:
         → 执行 impl fix-r<N>（只修复 review 指出的问题）
         → commit，round++
       - true 且 round > max_rounds:
         → stop: 终止，报错
         → continue: 警告，继续后续步骤
         → ask: 询问用户
```

**自定义 step**（`custom_steps.<name>`）：

1. 读取 `prompt` 指向的 prompt 文件
2. 读取 `inputs` 列出的输入文件
3. 执行任务，输出到 `outputs` 列出的文件

#### Phase 4 — 收尾

1. 清除 `.tomato/runs/state.json`
2. 切回 main：`git checkout main && git pull origin main`
3. 汇总结果：每个 step 的状态、token 消耗、产出文件

---

## 单步执行参考

### spec — 需求分析

**输入**：用户需求描述
**输出**：`docs/specs/<feature>/prd.md`

1. 理解用户需求，阅读现有代码
2. 生成 PRD，包含：问题陈述、目标与非目标、目标用户、用户故事、功能需求 (FR-001...)、验收标准 (Given/When/Then)、边界情况、数据与状态、依赖项、开放问题

### design — 方案设计

**输入**：`prd.md`
**输出**：`architecture.md`、`ui-spec.md`、`implementation.md`

1. 阅读 PRD 和现有代码
2. 生成三个文档

architecture.md：系统概述、组件划分、数据流 (Mermaid)、接口定义、持久化、错误处理、安全、测试策略、权衡决策

ui-spec.md（CLI/库可简化）：交互界面、用户流程、状态管理、文案

implementation.md：文件变更计划、公开签名、关键算法、数据结构、迁移步骤、测试计划

### impl — 代码实现

**输入**：`architecture.md`、`ui-spec.md`、`implementation.md`
**输出**：代码变更 + `impl-output.md`

1. 阅读设计文档和现有代码
2. 实际编写代码，遵循 Ponytail 原则
3. 生成 `impl-output.md`，commit 所有变更

**fix 模式**（review_loop 修复时）：只改 review 指出的问题，最小化改动。

### review — 代码审查

**输入**：`architecture.md`、`implementation.md`、git diff
**输出**：`reviews/r<N>-comments.md` + `.json`

1. `git diff origin/main...HEAD`
2. 三个级别：blocking (安全/数据/逻辑)、major (性能/可维护性)、minor (命名/风格)
3. 输出 JSON `{"comments": [...], "summary": "...", "has_blocking": bool}` + markdown

### test — 测试

**输入**：`architecture.md`、`implementation.md`、`impl-output.md`
**输出**：`test-report.md`

生成测试方案（范围、单元/集成/边界测试、代码、运行命令），可执行时实际运行。

### pr — 创建 Pull Request

**输入**：feature 分支、feature 名
**输出**：`pr.md` + `pr.json`

`gh pr create --draft --title "feat(<feature>): <描述>"`。Body 含 PRD 摘要、变更、测试、`Tomato-Parent: <HEAD hash>`。

### task — 任务同步

**输入**：`prd.md`
**输出**：`task.json`

如果配置了外部任务系统，创建对应 task。

---

## 配置约定

### Feature 分支

- 分支名：`tomato/<feature>`
- 开始前创建 → 结束后切回 main

### 状态文件 `.tomato/runs/state.json`

```json
{
  "run_id": "<uuid>",
  "feature": "<feature>",
  "workflow": "default",
  "completed": ["spec", "design"],
  "failed_step": "impl"
}
```

resume 时从 `failed_step` 继续。

### Ponytail 原则

编写代码时：YAGNI → 复用 → 标准库 → 原生平台 → 已有依赖 → 最少代码。

### 错误处理

- 输入缺失：告知用户，建议先执行前置 step
- 输出已存在：询问是否覆盖（--force 跳过）
- Git 冲突：stash 后继续
- 执行失败：重试一次，仍失败则停止并保存状态
