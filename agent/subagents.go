package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/smallnest/goskills/tool"

	markdown "github.com/MichaelMure/go-term-markdown"
	gomarkdown "github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	openai "github.com/sashabaranov/go-openai"
)

// SearchSubagent performs web searches.
type SearchSubagent struct {
	client             *openai.Client
	model              string
	verbose            bool
	interactionHandler InteractionHandler
}

// NewSearchSubagent creates a new SearchSubagent.
func NewSearchSubagent(client *openai.Client, model string, verbose bool, interactionHandler InteractionHandler) *SearchSubagent {
	return &SearchSubagent{
		client:             client,
		model:              model,
		verbose:            verbose,
		interactionHandler: interactionHandler,
	}
}

// Type returns the task type this subagent handles.
func (s *SearchSubagent) Type() TaskType {
	return TaskTypeSearch
}

// Execute performs a web search based on the task.
func (s *SearchSubagent) Execute(ctx context.Context, task Task) (Result, error) {
	if s.verbose {
		fmt.Println("🌐 网络搜索 Subagent")
	}
	if s.interactionHandler != nil {
		s.interactionHandler.Log(fmt.Sprintf("> 网络搜索 Subagent: %s", task.Description))
	}

	// Extract query from parameters
	query, ok := task.Parameters["query"].(string)
	if !ok {
		query = task.Description
	}

	if s.verbose {
		fmt.Printf("  查询: %q\n", query)
	}
	if s.interactionHandler != nil {
		s.interactionHandler.Log(fmt.Sprintf("  查询: %q", query))
	}

	// Perform Tavily search
	searchResult, err := tool.TavilySearch(query)
	if err != nil {
		// Fallback to DuckDuckGo if Tavily fails (e.g. missing key)
		if s.verbose {
			fmt.Printf("  ⚠️ Tavily 搜索失败: %v。回退到 DuckDuckGo。\n", err)
		}
		if s.interactionHandler != nil {
			s.interactionHandler.Log(fmt.Sprintf("  ⚠️ Tavily 搜索失败: %v。回退到 DuckDuckGo。", err))
		}
		searchResult, err = tool.DuckDuckGoSearch(query)
		if err != nil {
			return Result{
				TaskType: TaskTypeSearch,
				Success:  false,
				Error:    err.Error(),
			}, err
		}
	}

	// Reflection Loop
	maxIterations := 3
	accumulatedResults := searchResult

	for i := 0; i < maxIterations; i++ {
		// Prepare prompt for reflection
		reflectionPrompt := fmt.Sprintf(`用户查询: %s
当前搜索结果:
%s

信息是否足以回答用户的查询？
如果是，请仅回复 "SUFFICIENT"。
如果否，请回复一个新的、更精细的搜索查询以查找缺失的信息。不要添加任何其他文本。`, query, accumulatedResults)

		// Truncate if too long to avoid context limit issues
		if len(reflectionPrompt) > 80000 {
			reflectionPrompt = reflectionPrompt[:80000] + "\n...(truncated)"
		}

		resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model: s.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "你是一个搜索优化助手。你评估搜索结果并决定是否需要更多信息。",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: reflectionPrompt,
				},
			},
			Temperature: 0.1, // Low temp for decision making
		})

		if err != nil {
			if s.verbose {
				fmt.Printf("  ⚠️ 反思失败: %v\n", err)
			}
			if s.interactionHandler != nil {
				s.interactionHandler.Log(fmt.Sprintf("  ⚠️ 反思失败: %v", err))
			}
			break // Stop reflection if LLM fails
		}

		decision := strings.TrimSpace(resp.Choices[0].Message.Content)

		// Check if sufficient (case-insensitive check for robustness)
		if strings.Contains(strings.ToUpper(decision), "SUFFICIENT") {
			if s.verbose {
				fmt.Println("  ✓ LLM 认为信息已充足。")
			}
			if s.interactionHandler != nil {
				s.interactionHandler.Log("  ✓ LLM 认为信息已充足。")
			}
			break
		}

		// It's a new query
		newQuery := decision
		// Clean up quotes if present
		newQuery = strings.Trim(newQuery, "\"'")

		if s.verbose {
			fmt.Printf("  🔄 LLM 请求更多信息。新查询: %q\n", newQuery)
		}
		if s.interactionHandler != nil {
			s.interactionHandler.Log(fmt.Sprintf("  🔄 LLM 请求更多信息。新查询: %q", newQuery))
		}
		if s.interactionHandler != nil {
			s.interactionHandler.Log(fmt.Sprintf("🔄 补充搜索: %s", newQuery))
		}

		// Execute new search
		newResults, err := tool.TavilySearch(newQuery)
		if err != nil {
			// Try DDG fallback
			newResults, err = tool.DuckDuckGoSearch(newQuery)
		}

		if err == nil {
			accumulatedResults += "\n\n--- Additional Search Results ---\n" + newResults
		}
	}

	// Also try Wikipedia if results are sparse (optional, keeping existing logic)
	wikiResult, wikiErr := tool.WikipediaSearch(query)
	if wikiErr == nil && wikiResult != "" {
		accumulatedResults = fmt.Sprintf("网络搜索结果:\n%s\n\n维基百科结果:\n%s", accumulatedResults, wikiResult)
	}

	// Parse and log simplified results
	var resultLog strings.Builder
	resultLog.WriteString("已检索信息:\n")

	// Simple parsing of the text format returned by TavilySearch
	// Format: Title: ...\nURL: ...\nContent: ...\n\n
	entries := strings.Split(accumulatedResults, "\n\n")
	for _, entry := range entries {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		lines := strings.Split(entry, "\n")
		var title, url string
		for _, line := range lines {
			if strings.HasPrefix(line, "Title: ") {
				title = strings.TrimPrefix(line, "Title: ")
			} else if strings.HasPrefix(line, "URL: ") {
				url = strings.TrimPrefix(line, "URL: ")
			}
		}
		if title != "" && url != "" {
			resultLog.WriteString(fmt.Sprintf("- [%s](%s)\n", title, url))
		}
	}

	logContent := resultLog.String()
	if len([]rune(logContent)) > 200 {
		logContent = string([]rune(logContent)[:200]) + "..."
	}

	if s.verbose {
		fmt.Printf("\n  ✓ %s\n", logContent)
	}
	if s.interactionHandler != nil {
		s.interactionHandler.Log(fmt.Sprintf("✓ %s", logContent))
	}

	return Result{
		TaskType: TaskTypeSearch,
		Success:  true,
		Output:   accumulatedResults,
		Metadata: map[string]interface{}{
			"query": query,
		},
	}, nil
}

// AnalysisSubagent analyzes and synthesizes information.
type AnalysisSubagent struct {
	client             *openai.Client
	model              string
	verbose            bool
	interactionHandler InteractionHandler
}

// NewAnalysisSubagent creates a new AnalysisSubagent.
func NewAnalysisSubagent(client *openai.Client, model string, verbose bool, interactionHandler InteractionHandler) *AnalysisSubagent {
	return &AnalysisSubagent{
		client:             client,
		model:              model,
		verbose:            verbose,
		interactionHandler: interactionHandler,
	}
}

// Type returns the task type this subagent handles.
func (a *AnalysisSubagent) Type() TaskType {
	return TaskTypeAnalyze
}

// Execute analyzes information using the LLM.
func (a *AnalysisSubagent) Execute(ctx context.Context, task Task) (Result, error) {
	if a.verbose {
		fmt.Println("🔬 分析 Subagent")
	}
	if a.interactionHandler != nil {
		a.interactionHandler.Log(fmt.Sprintf("> 分析 Subagent: %s", task.Description))
	}

	// Get context from parameters if available
	contextData, hasContext := task.Parameters["context"].([]string)

	var prompt string
	if hasContext && len(contextData) > 0 {
		prompt = fmt.Sprintf("分析以下信息并 %s:\n\n%s", task.Description, strings.Join(contextData, "\n\n"))
	} else {
		prompt = task.Description
	}

	// Check for global context
	globalContext, _ := task.Parameters["global_context"].(string)
	systemPrompt := "你是一个分析助手，负责综合和分析信息。请提供清晰、结构化的分析。\n" +
		"如果提供的信息不足以完成分析，你可以请求更多信息。\n" +
		"如果需要更多信息，请仅回复 'MISSING_INFO: <具体的搜索查询>'。\n" +
		"例如: 'MISSING_INFO: 2024年Q3特斯拉财报数据'"

	if globalContext != "" {
		systemPrompt += "\n\n来自用户的重要上下文/指令：\n" + globalContext
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: prompt,
		},
	}

	req := openai.ChatCompletionRequest{
		Model:       a.model,
		Messages:    messages,
		Temperature: 0.3,
	}

	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return Result{
			TaskType: TaskTypeAnalyze,
			Success:  false,
			Error:    err.Error(),
		}, err
	}

	analysis := resp.Choices[0].Message.Content

	// Check for MISSING_INFO signal
	if strings.HasPrefix(strings.TrimSpace(analysis), "MISSING_INFO:") {
		newQuery := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(analysis), "MISSING_INFO:"))

		if a.verbose {
			fmt.Printf("  🔄 分析发现信息缺失，请求新搜索: %q\n", newQuery)
		}
		if a.interactionHandler != nil {
			a.interactionHandler.Log(fmt.Sprintf("🔄 分析发现信息缺失，请求新搜索: %q", newQuery))
		}

		// Create new tasks
		newTasks := []Task{
			{
				Type:        TaskTypeSearch,
				Description: newQuery,
				Parameters: map[string]interface{}{
					"query": newQuery,
				},
			},
			// Re-queue the current analysis task to run after the search
			task,
		}

		return Result{
			TaskType: TaskTypeAnalyze,
			Success:  true, // Step succeeded in identifying need
			Output:   fmt.Sprintf("正在请求更多信息: %s", newQuery),
			NewTasks: newTasks,
		}, nil
	}

	if a.verbose {
		fmt.Printf("  ✓ 信息这已足够，分析完成 (%d 字节)\n", len(analysis))
	}
	if a.interactionHandler != nil {
		a.interactionHandler.Log(fmt.Sprintf("✓ 信息这已足够，分析完成 (%d 字节)", len(analysis)))
	}

	return Result{
		TaskType: TaskTypeAnalyze,
		Success:  true,
		Output:   analysis,
	}, nil
}

// ReportSubagent generates formatted reports.
type ReportSubagent struct {
	client             *openai.Client
	model              string
	verbose            bool
	interactionHandler InteractionHandler
}

// NewReportSubagent creates a new ReportSubagent.
func NewReportSubagent(client *openai.Client, model string, verbose bool, interactionHandler InteractionHandler) *ReportSubagent {
	return &ReportSubagent{
		client:             client,
		model:              model,
		verbose:            verbose,
		interactionHandler: interactionHandler,
	}
}

// Type returns the task type this subagent handles.
func (r *ReportSubagent) Type() TaskType {
	return TaskTypeReport
}

// Execute generates a formatted report.
func (r *ReportSubagent) Execute(ctx context.Context, task Task) (Result, error) {
	if r.verbose {
		fmt.Println("📝 报告 Subagent")
	}
	if r.interactionHandler != nil {
		r.interactionHandler.Log(fmt.Sprintf("> 报告 Subagent: %s", task.Description))
	}

	// Get context from parameters if available
	contextData, hasContext := task.Parameters["context"].([]string)

	var prompt string
	if hasContext && len(contextData) > 0 {
		prompt = fmt.Sprintf("基于以下信息，%s:\n\n%s", task.Description, strings.Join(contextData, "\n\n"))
	} else {
		prompt = task.Description
	}

	// Check for global context
	globalContext, _ := task.Parameters["global_context"].(string)
	systemPrompt := "你是一个报告写作助手，负责创建格式良好、清晰且全面的 Markdown 格式报告。使用适当的标题、列表和格式使报告易于阅读。如果提供的信息包含带有 URL 和描述的图片，请选择最相关的图片，并使用标准 Markdown 图片语法 `![描述](URL)` 将其嵌入报告中。将图片放置在相关文本部分附近。"
	if globalContext != "" {
		systemPrompt += "\n\n来自用户的重要上下文/指令：\n" + globalContext
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: prompt,
		},
	}

	req := openai.ChatCompletionRequest{
		Model:       r.model,
		Messages:    messages,
		Temperature: 0.5,
	}

	resp, err := r.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return Result{
			TaskType: TaskTypeReport,
			Success:  false,
			Error:    err.Error(),
		}, err
	}

	report := resp.Choices[0].Message.Content

	if r.verbose {
		fmt.Printf("  ✓ 报告已生成 (%d 字节)\n", len(report))
	}
	if r.interactionHandler != nil {
		r.interactionHandler.Log(fmt.Sprintf("✓ 报告已生成 (%d 字节)", len(report)))
	}

	return Result{
		TaskType: TaskTypeReport,
		Success:  true,
		Output:   report,
	}, nil
}

// RenderSubagent renders markdown to terminal-friendly format.
type RenderSubagent struct {
	verbose            bool
	renderHTML         bool
	interactionHandler InteractionHandler
}

// NewRenderSubagent creates a new RenderSubagent.
func NewRenderSubagent(verbose bool, renderHTML bool, interactionHandler InteractionHandler) *RenderSubagent {
	return &RenderSubagent{
		verbose:            verbose,
		renderHTML:         renderHTML,
		interactionHandler: interactionHandler,
	}
}

// Type returns the task type this subagent handles.
func (r *RenderSubagent) Type() TaskType {
	return TaskTypeRender
}

// Execute renders markdown content.
func (r *RenderSubagent) Execute(ctx context.Context, task Task) (Result, error) {
	if r.verbose {
		fmt.Println("🎨 渲染 Subagent")
	}
	if r.interactionHandler != nil {
		r.interactionHandler.Log(fmt.Sprintf("> 渲染 Subagent: %s", task.Description))
	}

	// Get content from parameters or description
	content, ok := task.Parameters["content"].(string)
	if !ok {
		// Try to get from context (passed from previous task)
		if ctxContent, ok := task.Parameters["context"].([]string); ok && len(ctxContent) > 0 {
			// Try to find the output from the REPORT task
			var foundReport bool
			for i := len(ctxContent) - 1; i >= 0; i-- {
				if strings.Contains(ctxContent[i], "Output from REPORT task:") {
					content = ctxContent[i]
					// Extract the content after the header
					if idx := strings.Index(content, "\n"); idx != -1 {
						content = content[idx+1:]
					}
					foundReport = true
					break
				}
			}

			if !foundReport {
				// If no REPORT output found, use the last task's output
				content = ctxContent[len(ctxContent)-1]
				// Extract the content after the header if present
				if idx := strings.Index(content, "Output from "); idx != -1 {
					if newlineIdx := strings.Index(content[idx:], "\n"); newlineIdx != -1 {
						content = content[idx+newlineIdx+1:]
					}
				}
			}
			content = strings.TrimSpace(content)
		} else {
			content = task.Description
		}
	}

	if r.verbose {
		fmt.Printf("  正在渲染 %d 字节的内容\n", len(content))
	}
	if r.interactionHandler != nil {
		r.interactionHandler.Log(fmt.Sprintf("正在渲染 %d 字节的内容", len(content)))
	}

	// Render markdown
	var output string
	if r.renderHTML {
		extensions := parser.CommonExtensions | parser.AutoHeadingIDs
		p := parser.NewWithExtensions(extensions)
		doc := p.Parse([]byte(content))

		htmlFlags := html.CommonFlags | html.HrefTargetBlank | html.CompletePage
		opts := html.RendererOptions{Flags: htmlFlags, Title: "Agent Report"}
		renderer := html.NewRenderer(opts)

		output = string(gomarkdown.Render(doc, renderer))
	} else {
		output = string(markdown.Render(content, 80, 6))
	}

	return Result{
		TaskType: TaskTypeRender,
		Success:  true,
		Output:   output,
	}, nil
}
