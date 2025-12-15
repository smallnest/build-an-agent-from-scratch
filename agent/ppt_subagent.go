package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// PPTSubagent generates a modern HTML presentation from content.
type PPTSubagent struct {
	client             *openai.Client
	model              string
	verbose            bool
	interactionHandler InteractionHandler
	outputDir          string
}

// NewPPTSubagent creates a new PPTSubagent.
func NewPPTSubagent(client *openai.Client, model string, verbose bool, interactionHandler InteractionHandler, outputDir string) *PPTSubagent {
	return &PPTSubagent{
		client:             client,
		model:              model,
		verbose:            verbose,
		interactionHandler: interactionHandler,
		outputDir:          outputDir,
	}
}

// Type returns the task type this subagent handles.
func (p *PPTSubagent) Type() TaskType {
	return TaskTypePPT
}

// Slide represents a single slide in the presentation.
type Slide struct {
	Title   string   `json:"title"`
	Content []string `json:"content"`          // Bullet points or paragraphs
	Image   string   `json:"image,omitempty"`  // Image description or URL
	Layout  string   `json:"layout,omitempty"` // e.g., "title-center", "split-image-right", "bullets"
}

// Execute generates a PPT from the input content.
func (p *PPTSubagent) Execute(ctx context.Context, task Task) (Result, error) {
	if p.verbose {
		fmt.Println("📊 PPT  Subagent")
	}
	if p.interactionHandler != nil {
		p.interactionHandler.Log(fmt.Sprintf("> PPT  Subagent: %s", task.Description))
	}

	// Ensure output directory exists
	if err := os.MkdirAll(p.outputDir, 0755); err != nil {
		return Result{
			TaskType: TaskTypePPT,
			Success:  false,
			Error:    fmt.Sprintf("创建输出目录失败: %v", err),
		}, err
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

	// Extract images from content
	var images []string
	re := regexp.MustCompile(`!\[.*?\]\((.*?)\)`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			images = append(images, match[1])
		}
	}

	if p.verbose {
		fmt.Println("  正在生成幻灯片结构...")
		if len(images) > 0 {
			fmt.Printf("  在内容中发现 %d 张图片\n", len(images))
		}
	}

	// 1. Generate Slide Structure
	slides, err := p.generateSlides(ctx, content, images)
	if err != nil {
		return Result{
			TaskType: TaskTypePPT,
			Success:  false,
			Error:    fmt.Sprintf("生成幻灯片失败: %v", err),
		}, err
	}

	if p.verbose {
		fmt.Printf("  ✓ 已生成 %d 张幻灯片\n", len(slides))
	}

	// 2. Generate and Build
	url, err := p.GenerateAndBuild(ctx, slides)
	if err != nil {
		// Log detailed error to terminal/logs
		if p.verbose {
			fmt.Printf("❌ PPT 构建失败: %v\n", err)
		}
		if p.interactionHandler != nil {
			p.interactionHandler.Log("❌ PPT 构建失败。已跳过构建步骤。")
		}

		// Return success but with a warning message
		return Result{
			TaskType: TaskTypePPT,
			Success:  true,
			Output:   "PPT 内容已生成，但构建演示文稿失败 (可能是内存不足)。已跳过构建步骤，您可以查看生成的源文件。",
			Metadata: map[string]interface{}{
				"slides": slides,
				"error":  err.Error(),
			},
		}, nil
	}

	return Result{
		TaskType: TaskTypePPT,
		Success:  true,
		Output:   fmt.Sprintf("演示文稿生成成功。请访问: %s", url),
		Metadata: map[string]interface{}{
			"ppt_url": url,
			"slides":  slides,
		},
	}, nil
}

// GenerateAndBuild generates the markdown and builds the Slidev project.
func (p *PPTSubagent) GenerateAndBuild(ctx context.Context, slides []Slide) (string, error) {
	timestamp := time.Now().Unix()
	dirName := fmt.Sprintf("ppt_%d", timestamp)
	projectDir := filepath.Join(p.outputDir, dirName)

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return "", fmt.Errorf("创建项目目录失败: %v", err)
	}

	markdown := p.generateSlidevMarkdown(slides)
	if err := os.WriteFile(filepath.Join(projectDir, "slides.md"), []byte(markdown), 0644); err != nil {
		return "", fmt.Errorf("写入 slides.md 失败: %v", err)
	}

	if p.verbose {
		fmt.Printf("  ✓ 已在 %s 生成 slides.md\n", projectDir)
	}

	// Build with Slidev
	basePath := fmt.Sprintf("/generated/%s/dist/", dirName)

	// Create a simple package.json
	packageJson := `{
  "name": "slidev-project",
  "private": true,
  "scripts": {
    "build": "slidev build --out dist --base "
  },
  "dependencies": {
    "@slidev/cli": "^0.48.0",
    "@slidev/theme-default": "latest",
    "vue": "^3.4.0"
  }
}`
	packageJson = strings.Replace(packageJson, "--base ", "--base "+basePath, 1)

	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(packageJson), 0644); err != nil {
		return "", fmt.Errorf("写入 package.json 失败: %v", err)
	}

	// Run npm install
	if p.verbose {
		fmt.Println("  正在安装依赖 (npm install)...")
	}
	if p.interactionHandler != nil {
		p.interactionHandler.Log("正在安装依赖...")
	}

	// Create a context with timeout for npm install
	installCtx, installCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer installCancel()

	installCmd := exec.CommandContext(installCtx, "npm", "install")
	installCmd.Dir = projectDir
	if output, err := installCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("npm install 失败: %v\n输出: %s", err, string(output))
	}

	// Run npm run build
	if p.verbose {
		fmt.Println("  正在构建 Slidev 项目 (npm run build)...")
	}
	if p.interactionHandler != nil {
		p.interactionHandler.Log("正在构建演示文稿...")
	}

	// Create a context with timeout for npm run build
	buildCtx, buildCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer buildCancel()

	buildCmd := exec.CommandContext(buildCtx, "npm", "run", "build")
	buildCmd.Dir = projectDir
	if output, err := buildCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("slidev build 失败: %v\n输出: %s", err, string(output))
	}

	if p.verbose {
		fmt.Println("  ✓ 构建完成")
	}
	if p.interactionHandler != nil {
		p.interactionHandler.Log("✓ 演示文稿构建成功")
	}

	return fmt.Sprintf("%sindex.html", basePath), nil
}

func (p *PPTSubagent) generateSlides(ctx context.Context, content string, images []string) ([]Slide, error) {
	imagesContext := ""
	if len(images) > 0 {
		imagesContext = fmt.Sprintf("\n你可以使用以下来自源材料的图片：\n- %s\n\n在适当的时候，在幻灯片的 'image' 字段中使用这些确切的 URL。如果列表中没有相关的图片，请使用占位符或描述。", strings.Join(images, "\n- "))
	}

	systemPrompt := fmt.Sprintf(`你是一位专业的演示文稿设计师。你的目标是将提供的文本转换为结构化的幻灯片（5-20 张）。
设计应现代、简洁且引人入胜。
%s

仅输出一个 JSON 对象数组，其中每个对象代表一张幻灯片，包含：
- "title": 幻灯片标题。
- "content": 字符串数组（要点或短段落）。
- "image": 适合此幻灯片的图片描述（用于未来生成）或占位符 URL。
- "layout": 建议的布局 ("title-center", "split-image-right", "bullets", "quote")。

确保第一张幻灯片是标题幻灯片，最后一张是致谢/总结幻灯片。
保持文本简洁。尽可能使用要点。

Example:
[
  {"title": "The Future of AI", "content": ["AI is evolving rapidly", "Impact on all industries"], "layout": "title-center"},
  {"title": "Key Trends", "content": ["Generative Models", "Agentic Workflows"], "layout": "bullets"}
]`, imagesContext)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: fmt.Sprintf("根据此内容创建幻灯片（语言：中文）：\n\n%s", content),
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

	jsonContent := resp.Choices[0].Message.Content

	// Clean up markdown code blocks if present
	if idx := strings.Index(jsonContent, "```json"); idx != -1 {
		jsonContent = jsonContent[idx+7:]
	} else if idx := strings.Index(jsonContent, "```"); idx != -1 {
		jsonContent = jsonContent[idx+3:]
	}
	if idx := strings.LastIndex(jsonContent, "```"); idx != -1 {
		jsonContent = jsonContent[:idx]
	}
	jsonContent = strings.TrimSpace(jsonContent)

	var slides []Slide
	if err := json.Unmarshal([]byte(jsonContent), &slides); err != nil {
		return nil, fmt.Errorf("解析幻灯片 JSON 失败: %w", err)
	}

	return slides, nil
}

func (p *PPTSubagent) generateSlidevMarkdown(slides []Slide) string {
	var sb strings.Builder

	// 1. Global Frontmatter
	sb.WriteString("---\n")
	sb.WriteString("theme: default\n")
	sb.WriteString("highlighter: shiki\n")
	sb.WriteString("lineNumbers: false\n")
	sb.WriteString("info: | \n")
	sb.WriteString("  Generated by GoSkills Agent\n")
	sb.WriteString("drawings:\n")
	sb.WriteString("  enabled: false\n")
	sb.WriteString("transition: slide-left\n")
	sb.WriteString("mdc: true\n")
	// Dark theme background
	sb.WriteString("background: https://picsum.photos/1920/1080?blur=4\n")
	// sb.WriteString("class: text-white\n") // Removed global class to avoid duplicates

	// Inject first slide layout
	if len(slides) > 0 {
		s0 := slides[0]
		if s0.Layout == "split-image-right" {
			sb.WriteString("layout: image-right\n")
			img := s0.Image
			if img == "" || !strings.HasPrefix(img, "http") || strings.Contains(img, "source.unsplash.com") {
				img = "https://picsum.photos/800/600?random=0"
			}
			sb.WriteString(fmt.Sprintf("image: %s\n", img))
			sb.WriteString("class: text-white\n")
		} else if s0.Layout == "title-center" {
			sb.WriteString("layout: center\n")
			sb.WriteString("class: text-center text-white\n")
		} else if s0.Layout == "two-cols" {
			sb.WriteString("layout: two-cols\n")
			sb.WriteString("class: text-white\n")
		} else {
			sb.WriteString("layout: default\n")
			sb.WriteString("class: text-white\n")
		}
	} else {
		// Fallback if no slides
		sb.WriteString("class: text-white\n")
	}
	sb.WriteString("---\n\n")

	// 2. Generate Slides
	for i, slide := range slides {
		if i > 0 {
			sb.WriteString("\n---\n")

			if slide.Layout == "split-image-right" {
				sb.WriteString("layout: image-right\n")
				img := slide.Image
				if img == "" || !strings.HasPrefix(img, "http") || strings.Contains(img, "source.unsplash.com") {
					img = fmt.Sprintf("https://picsum.photos/800/600?random=%d", i)
				}
				sb.WriteString(fmt.Sprintf("image: %s\n", img))
				sb.WriteString("class: text-white\n")
			} else if slide.Layout == "title-center" {
				sb.WriteString("layout: center\n")
				sb.WriteString("class: text-center text-white\n")
			} else if slide.Layout == "two-cols" {
				sb.WriteString("layout: two-cols\n")
				sb.WriteString("class: text-white\n")
			} else {
				sb.WriteString("layout: default\n")
				sb.WriteString("class: text-white\n")
			}
			sb.WriteString("---\n\n")
		}

		// Title with Gradient
		sb.WriteString(fmt.Sprintf("# <span class=\"bg-gradient-to-r from-cyan-400 to-purple-500 bg-clip-text text-transparent\">%s</span>\n\n", slide.Title))

		// Content Wrapper with Glassmorphism and Animation
		sb.WriteString("<div class=\"bg-black/40 backdrop-blur-md p-6 rounded-xl border border-white/10 shadow-2xl mt-4\" v-motion :initial=\"{ y: 30, opacity: 0 }\" :enter=\"{ y: 0, opacity: 1, transition: { duration: 500 } }\">\n\n")

		if slide.Layout == "two-cols" && len(slide.Content) > 1 {
			half := len(slide.Content) / 2

			sb.WriteString("<v-clicks>\n\n")
			for _, item := range slide.Content[:half] {
				sb.WriteString(fmt.Sprintf("- %s\n", item))
			}
			sb.WriteString("\n</v-clicks>\n\n")

			sb.WriteString("</div>\n") // Close left wrapper
			sb.WriteString("::right::\n")
			sb.WriteString("<div class=\"bg-black/40 backdrop-blur-md p-6 rounded-xl border border-white/10 shadow-2xl mt-4\" v-motion :initial=\"{ y: 30, opacity: 0 }\" :enter=\"{ y: 0, opacity: 1, transition: { duration: 500, delay: 200 } }\">\n\n")

			sb.WriteString("<v-clicks>\n\n")
			for _, item := range slide.Content[half:] {
				sb.WriteString(fmt.Sprintf("- %s\n", item))
			}
			sb.WriteString("\n</v-clicks>\n")
		} else {
			if len(slide.Content) > 0 {
				sb.WriteString("<v-clicks>\n\n")
				for _, item := range slide.Content {
					sb.WriteString(fmt.Sprintf("- %s\n", item))
				}
				sb.WriteString("\n</v-clicks>\n")
			}
		}

		sb.WriteString("\n</div>\n") // Close main wrapper

		// Presenter Notes
		sb.WriteString("\n<!--\n")
		sb.WriteString(fmt.Sprintf("Presenter note for slide %d: %s\n", i+1, slide.Title))
		sb.WriteString("-->\n")
	}

	return sb.String()
}

// Unused but kept for interface compatibility if needed
func (p *PPTSubagent) generateHTML(slides []Slide, filepath string) error {
	return nil
}
