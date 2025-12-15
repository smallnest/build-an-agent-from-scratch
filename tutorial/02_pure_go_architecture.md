# 纯粹的力量：Go 语言实现的 Deep Research 架构

> [!NOTE]
> 这一章我们将揭开 `agent-web` 的引擎盖，看看它是如何不依赖 LangChain 或 LangGraph，仅用 Go 语言构建出强大的智能体编排系统的。

## 1. 为什么选择 Go？

在 Python 统治 AI 的时代，选择 Go 似乎是一种"叛逆"。但对于 Agent Engineering 来说，这是一种**回归本质**的选择。

- **并发原生**: Deep Research 需要同时进行多个搜索、分析任务。Go 的 Goroutine 和 Channel 让并发控制变得轻而易举，无需像 Python 那样处理复杂的 AsyncIO 或多线程锁。
- **类型安全**: 复杂的 Agent 状态流转需要严谨的数据结构。Go 的静态类型系统在编译期就能捕获大量潜在错误。
- **部署简单**: 编译成单一二进制文件，无依赖地部署在任何服务器上。没有 `pip install` 的噩梦，没有虚拟环境的困扰。

## 1.1 秘密武器：go:embed 实现单文件部署

在第一章中我们提到，`agent-web` 最大的亮点之一是**零依赖单文件部署**。这背后的功臣就是 Go 1.16 引入的 `go:embed` 特性。

在传统的 Web 开发中，部署通常意味着要上传一个后端可执行文件，外加一个包含 HTML/CSS/JS 的 `static` 或 `dist` 文件夹。一旦路径配置错误或文件丢失，前端页面就会 404。

而 `agent-web` 利用 `go:embed` 将前端构建产物直接“打包”进了 Go 的二进制文件中：

```go
// cmd/agent-web/main.go

//go:embed dist/*
var distFS embed.FS

func main() {
    // ...
    // 将嵌入的文件系统作为静态资源服务
    fs := http.FileServer(http.FS(distFS))
    http.Handle("/", fs)
    // ...
}
```

**这样做的好处是巨大的：**
1.  **彻底告别 "Missing Assets"**：前端资源与后端逻辑融为一体，永远不会出现静态文件丢失的问题。
2.  **分发极其方便**：你只需要把编译好的 `agent-web` 文件 scp 到服务器，`chmod +x` 后直接运行即可。不需要 Nginx 反代静态文件，也不需要 Docker 挂载卷。
3.  **版本一致性**：前端和后端版本强制绑定，不会出现后端升级了但前端缓存还在旧版本导致的兼容性问题。

## 1.2 性能与简洁：Go vs Python

在 AI Agent 领域，Python 无疑是目前的霸主，拥有 LangChain、LlamaIndex 等丰富的生态。然而，当我们将视角从“实验原型”转向“生产级应用”时，Go 语言的优势便凸显出来。

### 极致性能

Deep Research 任务通常涉及大量的网络 I/O（搜索、抓取）和 CPU 密集型操作（文本处理、JSON 解析）。

*   **并发模型**：Python 的 `asyncio` 虽然能处理并发，但在 CPU 密集型任务上长期受限于 GIL（全局解释器锁）。虽然 Python 3.13+ 开始实验性支持无 GIL 模式（free-threaded），但这需要生态库的全面适配，目前尚不成熟。相比之下，Go 的 Goroutine 是成熟且真正的 M:N 调度，轻量且高效，能够轻松榨干多核性能，同时处理成千上万个并发请求。
*   **执行速度**：作为编译型语言，Go 的执行速度通常比解释型的 Python 快一个数量级。在处理大规模文本分析和 JSON 序列化时，这种性能差异会直接转化为更低的延迟和更少的资源消耗。

### 架构简洁

*   **代码可读性**：Python 的动态特性虽然灵活，但在大型项目中容易导致类型混乱，维护成本高昂。Go 强制的静态类型和统一的格式化（gofmt）保证了代码库的整洁和可维护性。
*   **依赖管理**：Python 项目往往深陷 "Dependency Hell"，不同库之间的版本冲突令人头秃。Go 的 Module 系统简洁明了，且编译后的二进制文件不依赖系统环境，极大地降低了运维复杂度。

选择 Go，意味着我们选择了一种**工程化**的思维方式：用更少的资源，更稳定的架构，去构建更可靠的智能体系统。

## 2. 核心数据结构

![Agent Workflow](../docs/images/agent_worflow.png)

`agent-web` 的核心在于 `PlanningAgent` 结构体（位于 `agent/agent.go`）。它极其精简：

```go
type PlanningAgent struct {
    client             *openai.Client
    config             AgentConfig
    messages           []openai.ChatCompletionMessage
    subagents          map[TaskType]Subagent
    interactionHandler InteractionHandler
}
```

没有复杂的 Graph 对象，没有隐式的 Context 传递。一切都是显式的。

### 任务类型 (TaskType)
系统定义了清晰的任务边界：
- `TaskTypeSearch`: 负责调用 Tavily API。
- `TaskTypeAnalyze`: 负责处理文本。
- `TaskTypeReport`: 负责生成最终产物。
- `TaskTypePPT` / `TaskTypePodcast`: 负责多模态生成。

## 2.1 核心大脑：PlanningAgent

`PlanningAgent` 是整个系统的指挥官，它负责理解用户意图、制定计划、调度子 Agent 并汇总结果。让我们深入了解它的核心逻辑：

### 1. 初始化 (NewPlanningAgent)
在 `NewPlanningAgent` 函数中，我们初始化了 OpenAI 客户端（OpenAI API 兼容的模型），并注册了所有的 Subagent（Search, Analyze, Report 等）。这就像组建了一个专家团队，每个成员都有特定的技能。

### 2. 规划 (Plan)
这是 Agent 的"思考"阶段。`Plan` 方法会构建一个精心设计的 System Prompt，告诉 LLM：
- 你有哪些可用的工具（Subagents）。
- 你的任务是将用户请求分解为有序的子任务列表。
- 输出必须是严格的 JSON 格式。

Prompt 的核心部分如下：
```text
你是一个规划 Agent...
你可以使用以下 Subagent：
- SEARCH: 执行网络搜索...
- ANALYZE: 分析和综合...
...
仅返回具有此结构的有效 JSON 对象：
{
  "tasks": [
    {"type": "SEARCH", "description": "...", "parameters": {...}},
    ...
  ]
}
```
**这里有一个关键的 Prompt Engineering 技巧**：我们并没有依赖 LLM 的 Function Calling 功能，而是通过在 Prompt 中严格定义 JSON Schema，强制 LLM 输出结构化数据。这种方法在不同模型（如 GPT-4, Claude 3, DeepSeek）之间具有更好的通用性和稳定性，确保了 Agent 能够准确理解并执行规划。

LLM 返回的 JSON 被解析为 `Plan` 结构体，这就形成了一个可执行的行动指南。

### 3. 执行 (Execute)
这是 Agent 的"行动"阶段。`Execute` 方法遍历 `Plan` 中的任务列表，其核心逻辑包含三个关键步骤：

**1. 上下文注入 (Context Injection)**
为了让每个子任务都能“看到”之前的成果，我们需要将历史信息注入到当前任务的参数中。

```go
// 注入全局对话历史 (User Request)
task.Parameters["global_context"] = globalContextBuilder.String()

// 注入上一步任务的输出 (Previous Outputs)
if len(contextData) > 0 {
    if task.Parameters == nil {
        task.Parameters = make(map[string]interface{})
    }
    // 如果 parameters 中已经存在 context，则追加；否则直接赋值
    if existingContext, ok := task.Parameters["context"].([]string); ok {
        task.Parameters["context"] = append(existingContext, contextData...)
    } else {
        task.Parameters["context"] = contextData
    }
}
```

**2. 分发任务 (Task Dispatch)**
根据任务类型（如 `SEARCH`, `ANALYZE`），从 `subagents` 映射表中找到对应的执行者。

```go
// 根据 TaskType 获取对应的 Subagent
subagent, ok := a.subagents[task.Type]
if !ok {
    return nil, fmt.Errorf("unknown task type: %s", task.Type)
}

// 执行任务
result, err := subagent.Execute(ctx, task)
```

**3. 动态调整 (Dynamic Adjustment)**
这是 Agent 智能的体现。如果 Subagent 发现当前信息不足，它可以返回 `NewTasks`，主循环会将这些新任务动态插入到执行队列中，实现自我修正。

```go
if result.Success {
    // 检查是否有动态生成的新任务
    if len(result.NewTasks) > 0 {
        // 将新任务插入到当前任务之后
        // 实现了 "Plan -> Execute -> Re-Plan" 的微循环
        rear := append([]Task{}, plan.Tasks[i+1:]...)
        plan.Tasks = append(plan.Tasks[:i+1], append(result.NewTasks, rear...)...)
    }
    
    // 收集当前任务的输出，供后续任务使用
    contextData = append(contextData, fmt.Sprintf("Output from %s task:\n%s", task.Type, result.Output))
}
```

### 4. 交互 (Run)

`PlanningAgent` 的主入口点是 `Run` 方法，它封装了完整的任务处理流程：

**Run (执行)**
1.  **Plan**: 调用 `Plan` 方法生成任务计划。
2.  **Execute**: 调用 `Execute` 方法执行计划并获取结果列表。
3.  **Output**: 智能提取最终结果。它会优先查找 `RENDER` 或 `REPORT` 类型的任务输出作为最终结果；如果没有，则将所有任务的输出拼接返回。

```go
func (a *PlanningAgent) Run(ctx context.Context, userRequest string) (string, error) {
    // 1. 规划
    plan, err := a.Plan(ctx, userRequest)
    // 2. 执行
    results, err := a.Execute(ctx, plan)
    // 3. 提取最终结果 (优先 RENDER/REPORT)
    // ...
}
```

## 3. 编排逻辑：Plan & Execute

`agent-web` 的编排逻辑非常直观，遵循 "Plan -> Execute -> Review" 的循环。

### 3.1 规划 (Plan)
`Plan` 方法通过精心设计的 System Prompt，让 LLM 将用户请求分解为 JSON 格式的任务列表：

```json
{
  "tasks": [
    {"type": "SEARCH", "description": "搜索关于...的信息"},
    {"type": "ANALYZE", "description": "分析搜索结果..."},
    {"type": "REPORT", "description": "撰写报告..."},
    {"type": "PPT", "description": "生成演示文稿..."},
    {"type": "PODCAST", "description": "生成播客脚本..."},
    {"type": "RENDER", "description": "渲染最终输出..."}
  ]
}
```

### 3.2 执行 (Execute)
执行阶段是一个简单的循环，遍历任务列表并分发给对应的 Subagent。

```go
for _, task := range plan.Tasks {
    subagent := agent.subagents[task.Type]
    result, err := subagent.Execute(ctx, task)
    // ... 处理结果和上下文传递 ...
}
```

**关键点：上下文注入 (Context Injection)**
每个任务执行完后，其输出会被收集并注入到后续任务的 `context` 参数中（注意：`global_context` 仅包含用户原始请求和对话历史）。这实现了任务间的信息流转，而不需要复杂的 Memory 模块。例如，`ANALYZE` 任务可以直接读取 `SEARCH` 任务的搜索结果，而 `REPORT` 任务则基于 `ANALYZE` 的分析结果进行写作。这种显式的参数传递使得数据流向非常清晰，易于调试。

## 4. 交互接口 (InteractionHandler)

为了实现 Web 端的实时反馈，`agent-web` 定义了 `InteractionHandler` 接口。

```go
type InteractionHandler interface {
    ReviewPlan(plan *Plan) (string, error)
    Log(message string)
    // ...
}
```

在 CLI 模式下，它打印到终端；在 Web 模式下（`cmd/agent-web`），它通过 SSE (Server-Sent Events) 将事件实时推送到前端。这种解耦设计使得核心逻辑可以复用于不同的界面。

比如下面的代码中，`interactionHandler` 的具体实现：
- 如果是 `WebInteractionHandler`，它会将日志通过 SSE 推送到前端。
- 如果是 `CLIInteractionHandler`，它会打印到终端。  
```go
		if s.interactionHandler != nil {
			s.interactionHandler.Log(fmt.Sprintf("  🔄 LLM 请求更多信息。新查询: %q", newQuery))
		}
```

这样我们就可以使用一个统一的输出接口，同时支持命令行程序和 web 应用程序。

---
**下一章预告**：我们将探讨 `agent-web` 的深度研究能力，看看它是如何像人类研究员一样，通过递归搜索和分析来解决复杂问题的。