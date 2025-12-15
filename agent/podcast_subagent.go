package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// PodcastSubagent generates a podcast from a report.
type PodcastSubagent struct {
	client             *openai.Client
	model              string
	verbose            bool
	interactionHandler InteractionHandler
}

// NewPodcastSubagent creates a new PodcastSubagent.
func NewPodcastSubagent(client *openai.Client, model string, verbose bool, interactionHandler InteractionHandler) *PodcastSubagent {
	return &PodcastSubagent{
		client:             client,
		model:              model,
		verbose:            verbose,
		interactionHandler: interactionHandler,
	}
}

// Type returns the task type this subagent handles.
func (p *PodcastSubagent) Type() TaskType {
	return TaskTypePodcast
}

// DialogueLine represents a single line of dialogue in the podcast.
type DialogueLine struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

// Execute generates a podcast from the input content.
func (p *PodcastSubagent) Execute(ctx context.Context, task Task) (Result, error) {
	if p.verbose {
		fmt.Println("🎙️ 播客 Subagent")
	}
	if p.interactionHandler != nil {
		p.interactionHandler.Log(fmt.Sprintf("> 播客 Subagent: %s", task.Description))
	}

	// Get content from parameters or description
	content, ok := task.Parameters["content"].(string)
	if !ok || content == "Use the content from the previous REPORT task." {
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
		} else if !ok {
			content = task.Description
		}
	}

	if p.verbose {
		fmt.Println("  正在生成对话脚本...")
	}

	// 1. Generate Dialogue Script
	script, err := p.generateScript(ctx, content)
	if err != nil {
		return Result{
			TaskType: TaskTypePodcast,
			Success:  false,
			Error:    fmt.Sprintf("生成脚本失败: %v", err),
		}, err
	}

	if p.verbose {
		fmt.Printf("  ✓ 脚本已生成 (%d 行)\n", len(script))
	}
	if p.interactionHandler != nil {
		p.interactionHandler.Log(fmt.Sprintf("✓ 脚本已生成 (%d 行)", len(script)))
	}

	// Convert script to JSON string for output
	scriptJSON, err := json.MarshalIndent(script, "", "  ")
	if err != nil {
		return Result{
			TaskType: TaskTypePodcast,
			Success:  false,
			Error:    fmt.Sprintf("序列化脚本失败: %v", err),
		}, err
	}

	outputMsg := fmt.Sprintf("播客脚本生成成功！\n\n请将以下脚本提交到 https://listenhub.ai/zh 以生成音频：\n\n%s", string(scriptJSON))

	return Result{
		TaskType: TaskTypePodcast,
		Success:  true,
		Output:   outputMsg,
		Metadata: map[string]interface{}{
			"script": script,
		},
	}, nil
}

func (p *PodcastSubagent) generateScript(ctx context.Context, content string) ([]DialogueLine, error) {
	systemPrompt := `你是一位播客制作人。你的目标是将提供的输入文本（报告或文章）转换为两位主持人之间引人入胜的对话：
- 主持人 1 (男): 热情、好奇，负责提问和引入话题。
- 主持人 2 (女): 知识渊博、冷静，负责解释细节和提供见解。

对话应自然、口语化且易于收听。它应涵盖输入文本的要点。
仅输出一个 JSON 对象数组，其中每个对象包含 "speaker" ("Host 1" 或 "Host 2") 和 "text" (口语台词)。
Example:
[
  {"speaker": "Host 1", "text": "Welcome back to the show! Today we're discussing..."},
  {"speaker": "Host 2", "text": "That's right. It's a fascinating topic..."}
]`

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: fmt.Sprintf("将此文本转换为播客对话 (输出中文):\n\n%s", content),
		},
	}

	req := openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: 0.7,
	}

	resp, err := p.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	scriptContent := resp.Choices[0].Message.Content

	// Clean up markdown code blocks if present
	if idx := strings.Index(scriptContent, "```json"); idx != -1 {
		scriptContent = scriptContent[idx+7:]
	} else if idx := strings.Index(scriptContent, "```"); idx != -1 {
		scriptContent = scriptContent[idx+3:]
	}
	if idx := strings.LastIndex(scriptContent, "```"); idx != -1 {
		scriptContent = scriptContent[:idx]
	}
	scriptContent = strings.TrimSpace(scriptContent)

	var script []DialogueLine
	if err := json.Unmarshal([]byte(scriptContent), &script); err != nil {
		return nil, fmt.Errorf("解析脚本 JSON 失败: %w", err)
	}

	return script, nil
}
