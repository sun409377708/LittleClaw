# LittleClaw 路线图与阶段规划

## 1. 目标定义

本项目目标不是一次性复刻 OpenClaw，而是按阶段实现一个可运行、可调试、可扩展的 AI Agent Runtime：

1. 先做最小可运行版本 `LittleClaw v0`
2. 再逐步补齐 tool、memory、workflow、scheduler、skill 等能力
3. 最终达到一个接近 OpenClaw 的通用 autonomous runtime

设计原则：

- 先保证 agent loop 稳定，再扩展工具和编排能力
- 先保证运行可观测、可重放、可调试，再追求自治程度
- 先做单任务闭环，再做长期运行和复杂调度

---

## 2. 最小版本边界

### 2.1 LittleClaw v0 要解决的问题

给定一个任务，runtime 能完成下面的闭环：

```text
Task
-> 构建上下文
-> LLM 决策
-> 选择工具
-> 执行工具
-> 记录 observation
-> 判断是否继续
-> 输出最终结果
```

### 2.2 v0 必须具备的能力

- CLI 入口
- 单任务 agent loop
- 基础 planner
- Tool 注册和执行框架
- 2 到 4 个基础工具
- step 级 memory
- 超时、最大步数、失败退出
- 运行日志输出

### 2.3 v0 明确不做的内容

- 多 agent 协作
- 浏览器自动化
- 向量数据库
- YAML workflow 编排
- cron 定时调度
- 远程 worker
- 沙箱隔离体系
- 权限审批流

---

## 3. 推荐技术结构

### 3.1 技术栈

- Go
- Cobra
- YAML
- Cron
- SQLite 或 JSONL

### 3.2 推荐目录

```text
cmd/littleclaw/
internal/runtime/
internal/planner/
internal/llm/
internal/tools/
internal/memory/
internal/skills/
internal/workflow/
internal/scheduler/
internal/types/
docs/
```

### 3.3 核心对象建议

`Task`

- 用户输入任务
- 运行参数
- 约束条件

`Step`

- 第几步
- LLM 输出
- 工具调用
- observation
- error
- 时间戳

`Run`

- run id
- task id
- 当前状态
- memory
- 最终结果

`Tool`

- name
- description
- schema
- execute

---

## 4. 分阶段实施方案

## Phase 0: 项目骨架

### 目标

把工程跑起来，先建立后续演进不会推翻的目录和核心接口。

### 交付物

- Go module 初始化
- Cobra CLI 初始化
- 基础目录结构
- 日志模块
- 配置加载
- 统一 types 定义

### 成功标准

命令可以运行：

```bash
littleclaw run "hello"
```

即使只是打印输入，也说明入口链路打通。

### 风险

- 过早抽象 plugin system
- 过早引入复杂配置系统

---

## Phase 1: 最小 Agent Loop

### 目标

完成真正可运行的最小 agent runtime。

### 范围

- 单任务执行
- 单轮或多轮决策
- planner 输出固定 schema
- tool call 执行
- 结果回写 memory
- 满足停止条件后退出

### 最小工具集合

- `shell`
- `file.read`
- `file.write`

如果想再加一个工具，优先：

- `http`

### 建议停止条件

- 达到最终答案
- 达到 `max_steps`
- 连续工具失败超过阈值
- 超过总超时时间

### 交付物

- `runtime.Run(task)`
- `planner.Decide(state)`
- `tool.Executor`
- `memory.Store`
- 终端 step 输出

### 成功标准

可以完成类似任务：

```bash
littleclaw run "列出当前目录最大的10个文件并总结"
```

---

## Phase 2: 可观测性与稳定性

### 目标

让 runtime 可调试、可重放、可定位问题。

### 范围

- step 日志持久化
- run summary
- replay 模式
- verbose 模式
- tool 输入输出记录
- 错误分类

### 交付物

- `runs/<run-id>.jsonl` 或 SQLite 记录
- `littleclaw replay <run-id>`
- `littleclaw inspect <run-id>`

### 成功标准

任意一次 agent 执行失败后，可以回答：

- 哪一步失败
- 调用了哪个工具
- 输入是什么
- observation 是什么
- 为什么没有继续

---

## Phase 3: Skill 机制

### 目标

让 runtime 能按任务类型切换不同能力配置，而不是只用一个通用 prompt。

### 推荐实现方式

第一版 skill 不做复杂插件，先做静态声明：

```yaml
name: coder
system_prompt: ...
allowed_tools:
  - shell
  - file.read
  - file.write
stop_policy:
  max_steps: 12
```

### 范围

- skill registry
- skill loader
- 按 task 选择 skill
- skill 限制可用工具
- skill 覆盖 planner/system prompt

### 示例 skill

- `default`
- `coder`
- `researcher`
- `ops`

### 成功标准

不同任务能自动或显式选择 skill，并看到明显不同的行为。

---

## Phase 4: Tool 扩展

### 目标

把 LittleClaw 从“本地命令执行器”升级为“通用任务执行器”。

### 新增工具建议顺序

1. `http`
2. `python`
3. `search`
4. `browser`

### 说明

`shell` 已经很强，但不可控。更好的方向是：

- 能用 `http` 解决的，不必进 shell
- 能用 `file` 解决的，不必进 shell
- 能用结构化工具解决的，不必让模型拼命令

### 交付物

- tool schema 校验
- tool timeout
- tool 权限分类
- 结构化结果返回

### 成功标准

Agent 可以处理：

- 文件分析
- 数据抓取
- 简单脚本执行
- API 调用

---

## Phase 5: Workflow Engine

### 目标

在 agent loop 之外增加可声明式流程，让 runtime 支持半自动、强约束任务。

### 为什么放在这里

如果过早做 workflow，系统会变成流程编排器，而不是 agent runtime。

### 范围

- YAML workflow 定义
- step 依赖
- 条件分支
- tool step
- agent step
- manual approval step

### 示例

```yaml
name: daily_report
steps:
  - type: tool
    tool: http
  - type: agent
    skill: researcher
  - type: tool
    tool: file.write
```

### 成功标准

固定流程任务可以不用自由 agent loop，直接按 workflow 稳定执行。

---

## Phase 6: Scheduler

### 目标

支持长期运行任务和周期性任务。

### 范围

- cron 任务定义
- task source
- workflow 调度
- 失败重试
- 运行历史

### 典型场景

- 每天定时抓取数据
- 每小时巡检服务
- 每周生成报告

### 成功标准

runtime 可以长期运行，按计划触发任务，保留记录，并支持失败恢复。

---

## Phase 7: Advanced Runtime

### 目标

逐步逼近 OpenClaw 类 runtime 的真实能力。

### 能力扩展方向

- 长期记忆
- 任务队列
- 多 worker 并发
- 浏览器自动化
- 审批机制
- 权限和沙箱
- 远程工具调用
- 事件驱动触发
- 多 agent 协作

### 说明

这一阶段已经不是 LittleClaw，而是面向生产可用的 Agent Runtime。

---

## 5. 每个阶段的建议交付节奏

### Sprint 1

- 完成 Phase 0
- 完成 Phase 1

结果：

有一个真正可执行的最小 agent loop。

### Sprint 2

- 完成 Phase 2
- 补强 Phase 1 稳定性

结果：

runtime 可调试、可回放、可排查问题。

### Sprint 3

- 完成 Phase 3
- 完成 Phase 4 的前两个工具扩展

结果：

runtime 从“一个 agent”变成“多 skill、多工具”的通用执行框架。

### Sprint 4

- 完成 Phase 5
- 完成 Phase 6

结果：

支持半自动流程和周期性任务。

### Sprint 5+

- 逐步推进 Phase 7

结果：

朝接近 OpenClaw 的 runtime 演进。

---

## 6. 最终能达到什么程度

如果按上面的路线持续实现，最终可以达到的效果大致是：

### 6.1 可以做到的能力

- 接受自然语言任务并自主拆解步骤
- 根据 skill 选择工具和执行策略
- 调用本地 shell、文件、HTTP、Python、浏览器等工具
- 保留完整运行轨迹和 memory
- 执行固定 workflow 和自由 agent loop
- 通过 scheduler 长期运行
- 支持常见自动化任务和研发辅助任务

### 6.2 接近 OpenClaw 的能力范围

- 本地 autonomous runtime
- 多工具编排
- skill 驱动行为切换
- 长时间任务执行
- 可观测和可回放
- 基础的 workflow + schedule 联动

### 6.3 与完整 OpenClaw 仍可能存在的差距

- 更成熟的沙箱和权限体系
- 更强的浏览器与桌面自动化
- 更复杂的多 agent 协作
- 更完善的云端任务分发
- 更强的安全审计与生产级运维能力

也就是说，最终完全有机会做到：

- 对个人开发者和小团队足够实用
- 对本地自动化、研发流程、数据任务非常有价值
- 在架构层面接近 OpenClaw

但如果要达到真正大规模、多租户、生产安全级别的平台，还需要额外投入。

---

## 7. 当前最推荐的起步范围

现在最推荐直接开工的范围只有这些：

1. `cmd/littleclaw run`
2. `runtime` 主循环
3. `planner` 固定 schema
4. `shell` 和 `file` 工具
5. step memory 持久化
6. 最大步数和超时控制

这套做完之后，再进入下一阶段。不要一开始就同时做：

- workflow
- cron
- plugin system
- 多 agent
- 向量检索

否则项目复杂度会上升很快，但真实可用性不会同步提升。

---

## 8. 一句话总结

最合理的路径是：

先做一个稳定、可观测、可执行的最小 agent runtime，再逐步把 skill、tool、workflow、scheduler 和长期运行能力加上去，最终形成一个接近 OpenClaw 的通用 AI Agent Runtime。
