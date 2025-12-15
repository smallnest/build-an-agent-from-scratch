# 第三章：全能专家团队：揭秘 Deep Research 的核心引擎

> [!NOTE]
> 在了解了架构之后，这一章我们将深入 `agent-web` 的业务核心：它是如何像人类专家一样进行深度研究的。我们将逐一拆解各个 Subagent 的实现。

## 1. 搜索子代理 (SearchSubagent)

`SearchSubagent` 是 Agent 的眼睛，负责从互联网获取实时信息。

### 核心能力
- **集成 Tavily API**: 使用专门为 AI 优化的搜索引擎 Tavily，能够获取高质量的文本内容，而非仅仅是链接。
- **智能查询生成**: 并非直接使用用户的问题，而是让 LLM 根据当前上下文生成最合适的搜索关键词。
- **反思循环 (Reflection Loop)**: Agent 会自我评估搜索结果的质量，如果发现信息不足，会自动生成新的查询词进行多轮迭代搜索，直到收集到足够的信息。这是 Deep Research "Deep" 的关键所在。
- **鲁棒的 Fallback 机制**: 考虑到 API 限流或网络问题，`SearchSubagent` 内置了多级降级策略。如果 Tavily 不可用，它会自动无缝切换到 **DuckDuckGo** 进行搜索；如果依然失败，还会尝试查询 **Wikipedia**。这种设计保证了 Agent 在极端情况下依然能获取基础信息，不会直接“罢工”。

### 代码实现 (`agent/subagents.go`)
```go
func (s *SearchSubagent) Execute(ctx context.Context, task Task) (Result, error) {
    // 1. 提取查询词
    query := task.Parameters["query"].(string)
    
    // 2. 尝试 Tavily 搜索
    searchResult, err := tool.TavilySearch(query)
    if err != nil {
        // 3. 降级：尝试 DuckDuckGo
        s.interactionHandler.Log(fmt.Sprintf("  ⚠️ Tavily 搜索失败: %v。回退到 DuckDuckGo。", err))
        searchResult, err = tool.DuckDuckGoSearch(query)
        if err != nil {
            return Result{Success: false, Error: err.Error()}, err
        }
    }
    
    // 4. 反思循环 (Reflection Loop)
    // 检查信息是否充足，如果不充足则生成新的查询词进行补充搜索
    // ... (省略反思逻辑代码)

    // 5. 补充维基百科 (Optional)
    // 即使前面的搜索成功了，也会尝试从维基百科获取权威定义
    wikiResult, wikiErr := tool.WikipediaSearch(query)
    if wikiErr == nil && wikiResult != "" {
        accumulatedResults = fmt.Sprintf("网络搜索结果:\n%s\n\n维基百科结果:\n%s", accumulatedResults, wikiResult)
    }
    
    return Result{Success: true, Output: accumulatedResults}, nil
}
```

 > ![TODO] 未来扩展
 > 未来可以集成Skill 和 MCP 等能力，实现更复杂的搜索逻辑，而不是现在固定的采用特定的工具进行搜索。

## 2. 分析子代理 (AnalysisSubagent)

`AnalysisSubagent` 是 Agent 的大脑，负责处理海量信息，提取关键洞察。

### 核心能力
- **信息综合**: 阅读多个搜索结果，提取与目标相关的事实。
- **缺失信息检测**: 在分析过程中，如果发现现有信息不足以回答问题，它会发出 `MISSING_INFO` 信号。
- **动态任务生成**: 接收到 `MISSING_INFO` 信号后，系统会自动生成新的 `SEARCH` 任务并插入到任务队列中，实现“分析 -> 发现缺失 -> 补充搜索 -> 再分析”的闭环。

### 动态规划逻辑 (`agent/subagents.go`)
`AnalysisSubagent` 通过特定的 Prompt 指令来检测信息缺失：

```go
// System Prompt 指令
systemPrompt := "你是一个分析助手...如果需要更多信息，请仅回复 'MISSING_INFO: <具体的搜索查询>'。"

// ... LLM 调用 ...

// 检查 LLM 是否请求更多信息
if strings.HasPrefix(analysis, "MISSING_INFO:") {
    newQuery := strings.TrimPrefix(analysis, "MISSING_INFO:")
    
    // 动态生成新任务
    newTasks := []Task{
        {
            Type:        TaskTypeSearch,
            Description: newQuery,
            Parameters:  map[string]interface{}{"query": newQuery},
        },
        // 关键点：将当前的分析任务重新放回队列，以便在搜索完成后再次分析
        task, 
    }
    
    return Result{
        Success:  true, 
        Output:   "正在请求更多信息...", 
        NewTasks: newTasks, // 返回新任务列表，主循环会自动调度
    }, nil
}
```

## 3. 报告子代理 (ReportSubagent)

`ReportSubagent` 是 Agent 的笔，负责将混乱的笔记整理成专业的研报。

### 核心能力
- **上下文感知**: 能够读取之前所有步骤（搜索、分析）积累的 `context` 信息。
- **结构化写作**: 遵循 Markdown 格式，包含标题、摘要、正文、结论。
- **智能配图**: 如果上下文信息中包含图片 URL（由 Tavily 搜索返回），它会智能选择最相关的图片并嵌入到报告中。

### 代码实现 (`agent/subagents.go`)
```go
func (r *ReportSubagent) Execute(ctx context.Context, task Task) (Result, error) {
    // 1. 获取上下文
    contextData, _ := task.Parameters["context"].([]string)
    
    // 2. 构建 Prompt
    systemPrompt := "你是一个报告写作助手...如果提供的信息包含带有 URL 和描述的图片，请选择最相关的图片，并使用标准 Markdown 图片语法 `![描述](URL)` 将其嵌入报告中..."
    
    prompt := fmt.Sprintf("基于以下信息，%s:\n\n%s", task.Description, strings.Join(contextData, "\n\n"))

    // 3. 调用 LLM 生成报告
    // ... (调用 OpenAI API)
    
    return Result{Success: true, Output: report}, nil
}
```

## 4. 多模态子代理 (PPT & Podcast)

为了让报告更具表现力，`agent-web` 引入了多模态生成能力。

### 4.1 PPT 子代理 (PPTSubagent)
它利用 LLM 生成 Slidev (基于 Vue 的 Markdown 演示文稿工具) 格式的 Markdown，然后通过 `npm` 构建生成现代化的 HTML 幻灯片。

**工作流程 (`agent/ppt_subagent.go`):**
1.  **结构化生成**: LLM 将文本内容转换为 JSON 格式的幻灯片数组 (`[]Slide`)，包含标题、内容点、布局建议和图片描述。
2.  **Slidev Markdown 转换**: 将 JSON 转换为带有 Slidev 特定 Frontmatter 和组件的 Markdown 文件。
    - **主题**: 默认使用 `default` 主题，代码高亮使用 `shiki`。
    - **视觉增强**: 自动注入 `bg-gradient-to-r` 渐变标题和 `glassmorphism` (毛玻璃) 效果的内容容器。
    - **动画**: 使用 `v-motion` 实现入场动画，`v-clicks` 实现列表项逐个显示。
    - **智能布局**: 根据 `Layout` 字段自动选择 `image-right` (右图左文), `title-center` (居中标题), `two-cols` (双栏) 或 `default` 布局。
    - **图片处理**: 如果未提供图片，会自动使用 `picsum.photos` 生成随机占位图。
3.  **构建**: 自动生成 `package.json`，在临时目录执行 `npm install` 和 `npm run build`，最终生成一个独立的静态 HTML 网站。

```go
// 自动生成 Slidev 项目结构
packageJson := `{
  "name": "slidev-project",
  "dependencies": {
    "@slidev/cli": "^0.48.0",
    "@slidev/theme-default": "latest",
    "vue": "^3.4.0"
  }
}`
// ... 写入文件并执行构建命令 ...
```

### 4.2 播客子代理 (PodcastSubagent)
它将书面报告转化为双人对话脚本（Host & Guest），然后可以调用 TTS (Text-to-Speech) 引擎生成音频。
- **角色扮演**: 模拟真实的对话语气，包含打断、感叹等语气词。
- **音频合成**: 使用 OpenAI TTS 或其他语音服务生成高质量音频。这里并没有真正实现调用 TTS 引擎生成音频，你可以补充实现。

## 5. 渲染子代理 (RenderSubagent)

`RenderSubagent` 是 Agent 的“排版师”，它负责将 Markdown 格式的报告转换为用户友好的最终展示形式。它支持两种渲染模式，分别适配 CLI 和 Web 环境。

### 核心能力
- **智能内容提取**: 如果未直接提供内容，它会自动从上下文 (`context`) 中查找 `REPORT` 任务的输出，确保渲染的是最终报告。
- **CLI 终端渲染**: 使用 `go-term-markdown` 库，将 Markdown 转换为带有 ANSI 颜色代码的文本，支持：
    - **语法高亮**: 对代码块进行彩色高亮。
    - **表格对齐**: 自动计算列宽，生成整齐的 ASCII 表格。
    - **自动换行**: 根据终端宽度（默认 80 列）自动换行，防止排版错乱。
- **HTML 渲染**: 使用 `gomarkdown` 库，将 Markdown 转换为标准的 HTML 页面，支持：
    - **完整页面**: 生成包含 `<head>` 和 `<body>` 的完整 HTML 文档。
    - **新标签页打开**: 自动为所有链接添加 `target="_blank"` 属性。

### 代码实现 (`agent/subagents.go`)
```go
func (r *RenderSubagent) Execute(ctx context.Context, task Task) (Result, error) {
    // ... (省略智能内容提取逻辑) ...

    var output string
    if r.renderHTML {
        // HTML 渲染模式 (用于 Web 界面)
        htmlFlags := html.CommonFlags | html.HrefTargetBlank | html.CompletePage
        opts := html.RendererOptions{Flags: htmlFlags, Title: "Agent Report"}
        renderer := html.NewRenderer(opts)
        output = string(gomarkdown.Render(doc, renderer))
    } else {
        // 终端渲染模式 (用于 CLI)
        output = string(markdown.Render(content, 80, 6))
    }

    return Result{Success: true, Output: output}, nil
}
```

通过这些分工明确的 Subagent，`agent-web` 实现了从信息获取到多模态输出的全流程自动化。每个 Subagent 既可以独立工作，又可以通过 `context` 无缝协作，共同完成复杂的深度研究任务。
