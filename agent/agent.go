package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// PlanningAgent orchestrates task planning and subagent execution.
type PlanningAgent struct {
	client             *openai.Client
	config             AgentConfig
	messages           []openai.ChatCompletionMessage
	subagents          map[TaskType]Subagent
	interactionHandler InteractionHandler
}

// AgentConfig holds the configuration for the planning agent.
type AgentConfig struct {
	APIKey     string
	APIBase    string
	Model      string
	Verbose    bool
	RenderHTML bool
	OutputDir  string
}

// NewPlanningAgent creates and initializes a new PlanningAgent.
func NewPlanningAgent(config AgentConfig, interactionHandler InteractionHandler) (*PlanningAgent, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if config.Model == "" {
		config.Model = "gpt-4o" // Default model
	}
	if config.OutputDir == "" {
		config.OutputDir = "generated" // Default output directory
	}

	openaiConfig := openai.DefaultConfig(config.APIKey)
	if config.APIBase != "" {
		openaiConfig.BaseURL = config.APIBase
	}
	client := openai.NewClientWithConfig(openaiConfig)

	agent := &PlanningAgent{
		client:             client,
		config:             config,
		messages:           []openai.ChatCompletionMessage{},
		subagents:          make(map[TaskType]Subagent),
		interactionHandler: interactionHandler,
	}

	// Initialize subagents
	agent.subagents[TaskTypeSearch] = NewSearchSubagent(client, config.Model, config.Verbose, interactionHandler)
	agent.subagents[TaskTypeAnalyze] = NewAnalysisSubagent(client, config.Model, config.Verbose, interactionHandler)
	agent.subagents[TaskTypeReport] = NewReportSubagent(client, config.Model, config.Verbose, interactionHandler)
	agent.subagents[TaskTypeRender] = NewRenderSubagent(config.Verbose, config.RenderHTML, interactionHandler)
	agent.subagents[TaskTypePodcast] = NewPodcastSubagent(client, config.Model, config.Verbose, interactionHandler)
	agent.subagents[TaskTypePPT] = NewPPTSubagent(client, config.Model, config.Verbose, interactionHandler, config.OutputDir)

	return agent, nil
}

// Plan decomposes a user request into subtasks.
func (a *PlanningAgent) Plan(ctx context.Context, userRequest string) (*Plan, error) {
	if a.config.Verbose {
		fmt.Println("🧠 规划 Agent")
	}
	if a.interactionHandler != nil {
		a.interactionHandler.Log("🧠 正在规划...")
	}

	systemPrompt := `你是一个规划 Agent，负责将用户请求分解为子任务。
你可以使用以下 Subagent：
- SEARCH: 执行网络搜索以收集信息
- ANALYZE: 分析和综合收集到的信息
- REPORT: 根据分析数据生成格式化报告
- PODCAST: 根据报告生成播客脚本 (TaskType: PODCAST)
- PPT: 根据报告生成幻灯片 (HTML) (TaskType: PPT)
- RENDER: 将 Markdown 内容渲染为终端友好的格式

对于给定的用户请求，创建一个包含任务序列的计划。
每个任务应包含：
- type: SEARCH, ANALYZE, REPORT, PODCAST, PPT, 或 RENDER 之一
- description:  Subagent 应该做什么
- parameters: 任务的可选参数 (例如: {"query": "搜索词"})

重要提示：
- 仅在用户明确请求播客时包含 PODCAST 任务。
- 仅在用户明确请求幻灯片或演示文稿时包含 PPT 任务。
- 在 REPORT 任务之后始终包含 RENDER 任务，以生成最终的文本报告。

仅返回具有此结构的有效 JSON 对象：
{
  "description": "总体计划描述",
  "tasks": [
    {"type": "SEARCH", "description": "...", "parameters": {"query": "..."}},
    {"type": "ANALYZE", "description": "..."},
    {"type": "REPORT", "description": "..."},
    {"type": "PPT", "description": "根据报告生成幻灯片"},
    {"type": "RENDER", "description": "渲染报告"}
  ]
}

保持计划简单且重点突出。通常 3-5 个任务就足够了。`

	// Inject global context from history
	var globalContextBuilder strings.Builder
	for _, msg := range a.messages {
		if msg.Role == openai.ChatMessageRoleDeveloper {
			globalContextBuilder.WriteString(fmt.Sprintf("User: %s\n", msg.Content))
		}
	}

	if globalContextBuilder.Len() > 0 {
		systemPrompt += "\n\n来自用户的重要上下文/指令：\n" + globalContextBuilder.String()
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: fmt.Sprintf("为该请求创建计划：%s", userRequest),
	})

	req := openai.ChatCompletionRequest{
		Model:       a.config.Model,
		Messages:    messages,
		Temperature: 0,
	}

	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create plan: %w", err)
	}

	content := resp.Choices[0].Message.Content

	// Clean up the content if it contains markdown code blocks
	if len(content) > 0 {
		// Remove ```json prefix if present
		if idx := strings.Index(content, "```json"); idx != -1 {
			content = content[idx+7:]
		} else if idx := strings.Index(content, "```"); idx != -1 {
			content = content[idx+3:]
		}

		// Remove closing ``` if present
		if idx := strings.LastIndex(content, "```"); idx != -1 {
			content = content[:idx]
		}

		content = strings.TrimSpace(content)
	}

	// Parse the JSON response
	var plan Plan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w\nResponse: %s", err, content)
	}

	if a.config.Verbose {
		fmt.Printf("📋 计划: %s\n", plan.Description)
		for i, task := range plan.Tasks {
			fmt.Printf("  %d. [%s] %s\n", i+1, task.Type, task.Description)
		}
		fmt.Println()
	}
	if a.interactionHandler != nil {
		a.interactionHandler.Log(fmt.Sprintf("📋 计划已生成: %s", plan.Description))
	}

	return &plan, nil
}

// PlanWithReview creates a plan and optionally allows the user to review and modify it.
func (a *PlanningAgent) PlanWithReview(ctx context.Context, userRequest string) (*Plan, error) {
	// Create initial plan
	plan, err := a.Plan(ctx, userRequest)
	if err != nil {
		return nil, err
	}

	// If no interaction handler, return the plan as-is
	if a.interactionHandler == nil {
		return plan, nil
	}

	// Allow user to review and modify the plan
	for {
		modification, err := a.interactionHandler.ReviewPlan(plan)
		if err != nil {
			return nil, fmt.Errorf("plan review failed: %w", err)
		}

		// If no modification requested, use the current plan
		if modification == "" {
			break
		}

		// Re-plan with the user's modification
		if a.config.Verbose {
			fmt.Printf("🔄 根据用户反馈重新规划: %s\n\n", modification)
		}
		a.interactionHandler.Log(fmt.Sprintf("🔄 根据用户反馈重新规划: %s", modification))

		plan, err = a.Plan(ctx, modification)
		if err != nil {
			return nil, fmt.Errorf("re-planning failed: %w", err)
		}
	}

	return plan, nil
}

// Execute runs the plan by executing each task with the appropriate subagent.
func (a *PlanningAgent) Execute(ctx context.Context, plan *Plan) ([]Result, error) {
	if a.config.Verbose {
		fmt.Println("🔍 正在执行计划...")
		fmt.Println()
	}

	results := make([]Result, 0, len(plan.Tasks))

	var contextData []string

	// Use a loop index that can be modified to support dynamic task insertion
	for i := 0; i < len(plan.Tasks); i++ {
		task := plan.Tasks[i]

		if a.config.Verbose {
			fmt.Printf("📍 步骤 %d/%d: [%s] %s\n", i+1, len(plan.Tasks), task.Type, task.Description)
		}
		if a.interactionHandler != nil {
			a.interactionHandler.Log(fmt.Sprintf("📍 步骤 %d/%d: [%s] %s", i+1, len(plan.Tasks), task.Type, task.Description))
		}

		// Inject global context from history
		if task.Parameters == nil {
			task.Parameters = make(map[string]interface{})
		}
		var globalContextBuilder strings.Builder
		for _, msg := range a.messages {
			if msg.Role == openai.ChatMessageRoleUser {
				globalContextBuilder.WriteString(fmt.Sprintf("User: %s\n", msg.Content))
			}
		}
		task.Parameters["global_context"] = globalContextBuilder.String()

		// Inject context from previous tasks
		if len(contextData) > 0 {
			if task.Parameters == nil {
				task.Parameters = make(map[string]interface{})
			}
			// If context already exists in parameters, append to it
			if existingContext, ok := task.Parameters["context"].([]string); ok {
				task.Parameters["context"] = append(existingContext, contextData...)
			} else {
				task.Parameters["context"] = contextData
			}
		}

		subagent, ok := a.subagents[task.Type]
		if !ok {
			return nil, fmt.Errorf("unknown task type: %s", task.Type)
		}

		result, err := subagent.Execute(ctx, task)
		if err != nil {
			return nil, fmt.Errorf("task %d failed: %w", i+1, err)
		}

		results = append(results, result)

		if result.Success {
			// Check for dynamic tasks
			if len(result.NewTasks) > 0 {
				if a.config.Verbose {
					fmt.Printf("  🔄 动态规划更新: 插入 %d 个新任务\n", len(result.NewTasks))
				}
				if a.interactionHandler != nil {
					a.interactionHandler.Log(fmt.Sprintf("🔄 动态规划更新: 插入 %d 个新任务", len(result.NewTasks)))
				}

				// Insert new tasks at the current position + 1
				// We need to create a new slice to avoid modifying the original plan array in place if it was smaller
				// But here plan.Tasks is a slice, so we can use append tricks
				rear := append([]Task{}, plan.Tasks[i+1:]...)
				plan.Tasks = append(plan.Tasks[:i+1], append(result.NewTasks, rear...)...)
			}

			// Accumulate output for next tasks
			contextData = append(contextData, fmt.Sprintf("Output from %s task:\n%s", task.Type, result.Output))

			if a.config.Verbose {
				fmt.Printf("  ✓ 完成\n\n")
			}
			if a.interactionHandler != nil {
				a.interactionHandler.Log("  ✓ 完成")
			}
		} else {
			if a.config.Verbose {
				fmt.Printf("  ✗ 失败: %s\n\n", result.Error)
			}
			if a.interactionHandler != nil {
				a.interactionHandler.Log(fmt.Sprintf("  ✗ 失败: %s", result.Error))
			}
		}
	}

	return results, nil
}

// Run is the main entry point that plans and executes a user request.
func (a *PlanningAgent) Run(ctx context.Context, userRequest string) (string, error) {
	// Create a plan
	plan, err := a.Plan(ctx, userRequest)
	if err != nil {
		return "", err
	}

	// Execute the plan
	results, err := a.Execute(ctx, plan)
	if err != nil {
		return "", err
	}

	// Extract the final output (typically from the RENDER or REPORT task)
	var finalOutput string
	for i := len(results) - 1; i >= 0; i-- {
		if (results[i].TaskType == TaskTypeRender || results[i].TaskType == TaskTypeReport) && results[i].Success {
			finalOutput = results[i].Output
			break
		}
	}

	// If no report was generated, concatenate all outputs
	if finalOutput == "" {
		for _, result := range results {
			if result.Success {
				finalOutput += result.Output + "\n\n"
			}
		}
	}

	return finalOutput, nil
}

// AddUserMessage adds a user message to the conversation history.
func (a *PlanningAgent) AddUserMessage(content string) {
	a.messages = append(a.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: content,
	})
}

// AddDeveloperMessage adds a developer message to the conversation history.
func (a *PlanningAgent) AddDeveloperMessage(content string) {
	a.messages = append(a.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleDeveloper,
		Content: content,
	})
}

// AddAssistantMessage adds an assistant message to the conversation history.
func (a *PlanningAgent) AddAssistantMessage(content string) {
	a.messages = append(a.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: content,
	})
}

// ClearHistory clears the conversation history.
func (a *PlanningAgent) ClearHistory() {
	a.messages = []openai.ChatCompletionMessage{}
}

// Chat performs a simple chat interaction without planning.
func (a *PlanningAgent) Chat(ctx context.Context, userRequest string) (string, error) {
	// Add user message
	a.AddUserMessage(userRequest)

	// Inject global context from history
	var globalContextBuilder strings.Builder
	for _, msg := range a.messages {
		if msg.Role == openai.ChatMessageRoleUser {
			globalContextBuilder.WriteString(fmt.Sprintf("User: %s\n", msg.Content))
		}
	}

	systemPrompt := "你是一个乐于助人的助手。"
	if globalContextBuilder.Len() > 0 {
		systemPrompt += "\n\n来自用户的重要上下文/指令：\n" + globalContextBuilder.String()
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}
	messages = append(messages, a.messages...)

	req := openai.ChatCompletionRequest{
		Model:    a.config.Model,
		Messages: messages,
	}

	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	content := resp.Choices[0].Message.Content
	a.AddAssistantMessage(content)

	return content, nil
}
