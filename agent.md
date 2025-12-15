# 震撼发布：GoSkills Agent —— 纯 Go 打造的 Deep Research 深度研究智能体

**零框架依赖，极致轻量，让 Deep Research 的力量触手可及！**

项目代码位于：https://github.com/smallnest/goskills/

> **百度 "智能体研究社" 荣誉出品。**
> 百度 "智能体研究社" 是百度厂内自发组织的一个对AI应用和智能体进行研究的社团。跟进业界研究动态，分析AI应用和智能体的最新进展，组织和开发 AI 应用开源项目等。
> 欢迎朋友们交流和合作。

你是否曾被复杂的 Agent 框架（如 LangChain, AutoGen）搞得晕头转向？你是否渴望拥有一个像字节跳动 [Deer Flow](https://github.com/bytedance/deer-flow) 那样强大的深度研究助手，却苦于没有合适的 Go 语言实现？

今天，我们隆重介绍 **GoSkills Agent** —— 一个完全使用 Go 语言原生标准库和轻量级工具打造的 Deep Research 智能体！它不依赖任何臃肿的第三方 Agent 框架，以最纯粹的代码展示了 Agentic AI 的核心逻辑。

它不仅是一个工具，更是一次对 Agent 本质的回归与探索。准备好，让我们一起揭开它的神秘面纱！

## 0. 背景和功能介绍

在 AI 2.0 时代，**Deep Research（深度研究）** 已成为 Agent 领域皇冠上的明珠。它要求 AI 不仅仅是回答问题，而是像人类研究员一样：**拆解目标 -> 搜集信息 -> 阅读分析 -> 综合报告**。

虽然 Python 生态中已有不少实现，但在 Go 语言的世界里，我们一直渴望一个高性能、易部署、逻辑清晰的 Deep Research 解决方案。

**GoSkills Agent** 应运而生。它具备以下核心能力：
*   **自主规划 (Autonomous Planning)**：根据用户问题，自动拆解为搜索、分析、撰写报告等子任务。
*   **深度搜索 (Deep Search)**：集成 Tavily 搜索工具，能够实时获取互联网上的最新信息。
*   **智能分析 (Intelligent Analysis)**：对海量搜索结果进行去噪、提炼和深度思考。
*   **专业报告 (Professional Reporting)**：最终生成结构清晰、内容详实的 Markdown 研报。
*   **原生 TUI (Terminal UI)**：提供类似 Gemini CLI 的极客风交互界面，体验丝滑。

## 1. 编译

GoSkills Agent 的编译过程极其简单，得益于 Go 语言强大的工具链。

确保你已安装 Go 1.25+ 环境。

```bash
# 克隆项目
git clone https://github.com/smallnest/goskills.git
cd goskills

# 使用 Makefile 一键编译
make agent

# 或者直接使用 go build
go build -o agent-cli ./cmd/agent-cli
```

编译完成后，你将在当前目录下看到 `agent-cli` 可执行文件。

## 2. 安装

由于 Go 编译产物是静态链接的二进制文件，你无需安装任何 Python 依赖或配置复杂的虚拟环境。

只需将 `agent-cli` 移动到你的系统 PATH 中即可：

```bash
mv agent-cli /usr/local/bin/
```

现在，你可以在任何地方通过 `agent-cli` 命令唤醒你的深度研究助手了！

## 3. 使用

启动 Agent，你将看到一个设计精美的 TUI 界面：

```bash
agent-cli
```

**交互指令：**
*   直接输入你的研究课题，例如：“分析 2024 年量子计算的最新突破”
*   `\help`：查看帮助信息
*   `\clear`：清除上下文历史
*   `\exit` 或 `\quit`：退出程序

**示例：**

### 3.1 启动 agent-cli
首先设置几个环境变量：
```
# 这里我使用的是百度智能云的deepseek服务，你可以修改为你的服务信息
export OPENAI_API_KEY=YOUR_KEY
export OPENAI_API_BASE=https://qianfan.baidubce.com/v2
export OPENAI_MODEL=deepseek-v3

# 到 https://www.tavily.com/ 申请key, 有免费额度。 需要使用它搜索网页资源
export TAVILY_API_KEY=tvly-dev-xxxxxxxxxxxxxxxx
```

然后启动程序,建议加`-v`，显示调试信息，方便你观察智能体处理流程：
```bash
agent-cli -v
```

- 退出程序使用 `\exit` 命令
- 寻求帮助使用 `\help` 命令  或者 `\quit` 命令
- 清除上下文历史使用 `\clear` 命令


### 3.2 输入请求，智能体开始指定计划
![](docs/images/agent_plan.png)

这个示例中我们让智能体输出“请用浅显的术语解释量子计算技术。报告请使用中文。“的分析报告。

### 3.3 搜索和分析
![](docs/images/agent_search.png)
然后按照计划，进行资料的搜索，然后将搜索的结果给LLM进行分析。

### 3.4 生成报告
![](docs/images/agent_report.png)
将分析的结果提供LLM, 按照要求生成分析报告(markdown 格式)。

### 3.5 渲染和输出报告
![](docs/images/agent_output.png)
因为生成的报告是markdown 格式，在终端中不好看，所以这个子代理将markdown 格式渲染成更漂亮的格式，输出出来。

这样一个完整的流程就完成了。

如果你意犹未尽，可以在这个session 继续提问，或者使用 `\exit` 退出程序。


## 4. 具体实现原理

GoSkills Agent 的核心魅力在于其**去框架化 (Framework-less)** 的设计。我们没有使用任何黑盒 SDK，而是通过清晰的**规划器-执行器-子代理 (Planner-Executor-SubAgents)** 模式实现了复杂的 Agent 行为。



### 4.1 流程 (Workflow)

整个 Agent 的运行流程是一个闭环的反馈系统：

![](docs/images/agent_worflow.png)

1.  **感知 (Perception)**：接收用户输入。
2.  **规划 (Planning)**：Planning Agent 利用 LLM 的推理能力，将模糊的需求转化为结构化的 `Plan`（包含一系列有序的 `Task`）。
3.  **执行 (Execution)**：主循环依次调度相应的 Subagent 执行任务。
4.  **记忆 (Memory)**：所有 Subagent 的输出都会汇聚到 Shared Context 中，供后续步骤使用。
5.  **表达 (Expression)**：Render Subagent 将最终结果渲染为终端友好的格式。

同时引入 `human-in-the-loop`, 允许用户修改计划和参与到subagent 的执行中。

### 4.2 组件介绍

代码结构清晰，模块化程度极高：

*   **Planning Agent (`agent.go`)**：系统的指挥官。它负责理解意图、生成 JSON 格式的计划，并根据执行结果动态调整策略。它维护着全局的上下文记忆。
*   **Search Subagent (`subagents.go`)**：信息搜集者。集成了 Tavily API，能够执行高质量的互联网搜索，并支持自动翻页和结果预览。
*   **Analysis Subagent (`subagents.go`)**：逻辑思考者。它阅读搜索到的原始文本，提取关键信息，识别矛盾，并进行逻辑推理。
*   **Report Subagent (`subagents.go`)**：内容创作者。基于分析结果，它能够撰写结构严谨、引用规范的 Markdown 报告。
*   **Render Subagent (`subagents.go`)**：视觉设计师。利用 `go-term-markdown` 库，将枯燥的文本转化为带有颜色、表格和代码高亮的终端输出。
*   **TUI (`cmd/agent-cli/tui.go`)**：交互层。使用 `bubbletea` 框架构建的现代化命令行界面，支持动态调整大小、颜色高亮和丝滑的输入体验。实现类似 `Claude Code`、`Gemini CLI` 的极客风交互界面。

### 4.3 未来规划

GoSkills Agent 只是一个开始，我们有着宏大的愿景：
1.  **长短期记忆 (Long-term Memory)**：引入向量数据库，让 Agent 拥有“过目不忘”的能力。
2.  **检查点 (Checkpoints)**：增加对检查点的管理能力，让 Agent 可以恢复执行。
3.  **多模态能力 (Multimodal)**：可以生成podcast、PPT 的能力。
4.  **工具生态**：增加中间件的支持，扩展生态圈。

--- 

**GoSkills Agent** —— 用最纯粹的 Go 代码，致敬人类探索未知的精神。现在就 Clone 代码，开启你的 Deep Research 之旅吧！
