package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/smallnest/goskills/agent"
	"github.com/smallnest/goskills/config"
	"github.com/spf13/cobra"
)

// CLIInteractionHandler implements agent.InteractionHandler for the CLI.
type CLIInteractionHandler struct {
	scanner *bufio.Scanner
}

func NewCLIInteractionHandler(scanner *bufio.Scanner) *CLIInteractionHandler {
	return &CLIInteractionHandler{scanner: scanner}
}

func (h *CLIInteractionHandler) ReviewPlan(plan *agent.Plan) (string, error) {
	fmt.Println("\n📋 Proposed Plan:")
	fmt.Printf("Description: %s\n", plan.Description)
	for i, task := range plan.Tasks {
		fmt.Printf("  %d. [%s] %s\n", i+1, task.Type, task.Description)
	}
	fmt.Println()

	fmt.Print("\033[1;33mDo you want to approve this plan? (y/N/modification):\033[0m ")
	if !h.scanner.Scan() {
		return "", h.scanner.Err()
	}
	input := strings.TrimSpace(h.scanner.Text())

	if input == "" || strings.EqualFold(input, "y") || strings.EqualFold(input, "yes") {
		return "", nil
	}

	if strings.EqualFold(input, "n") || strings.EqualFold(input, "no") {
		return "", fmt.Errorf("plan rejected by user")
	}

	// Treat other input as modification request
	return input, nil
}

func (h *CLIInteractionHandler) ConfirmPodcastGeneration(report string) (bool, error) {
	fmt.Print("\n\033[1;33mDo you want to generate a podcast from this report? (y/N):\033[0m ")
	if !h.scanner.Scan() {
		return false, h.scanner.Err()
	}
	input := strings.TrimSpace(h.scanner.Text())

	return strings.EqualFold(input, "y") || strings.EqualFold(input, "yes"), nil
}

func (h *CLIInteractionHandler) Log(message string) {
	fmt.Println(message)
}

var rootCmd = &cobra.Command{
	Use:   "agent-cli",
	Short: "A deep agents CLI tool with planning and specialized subagents.",
	Long: `agent-cli is a command-line interface that implements a deep research agent architecture.
It uses a planning agent to decompose tasks and coordinate specialized subagents for:
- Web search (DuckDuckGo, Wikipedia)
- Information analysis
- Report generation
- Render markdown to content in terminal

In interactive mode, you can have multi-turn conversations with the agent.
The agent maintains conversation history across messages.

Special commands:
  /help   - Show available commands
  /clear  - Clear conversation history
  /exit   - Exit the chat session
  /quit   - Exit the chat session`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig(cmd)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		agentConfig := agent.AgentConfig{
			APIKey:  cfg.APIKey,
			APIBase: cfg.APIBase,
			Model:   cfg.Model,
			Verbose: cfg.Verbose,
		}

		ctx := context.Background()
		scanner := bufio.NewScanner(os.Stdin)
		interactionHandler := NewCLIInteractionHandler(scanner)

		planningAgent, err := agent.NewPlanningAgent(agentConfig, interactionHandler)
		if err != nil {
			return fmt.Errorf("failed to create planning agent: %w", err)
		}

		logo := "\033[38;2;255;8;68m╱\033[38;2;255;12;70m╭\033[38;2;255;15;72m━\033[38;2;255;19;74m━\033[38;2;255;23;75m━\033[38;2;255;26;77m╮\033[38;2;255;30;79m╱\033[38;2;255;34;81m╭\033[38;2;255;37;83m━\033[38;2;255;41;85m━\033[38;2;255;45;86m━\033[38;2;255;48;88m╮\033[38;2;255;52;90m╱\033[38;2;255;56;92m╭\033[38;2;255;59;94m━\033[38;2;255;63;96m━\033[38;2;255;67;98m━\033[38;2;255;70;99m╮\033[38;2;255;74;101m╱\033[38;2;255;78;103m╭\033[38;2;255;81;105m╮\033[38;2;255;85;107m╭\033[38;2;255;89;109m━\033[38;2;255;93;111m╮\033[38;2;255;96;112m╱\033[38;2;255;100;114m╭\033[38;2;255;104;116m━\033[38;2;255;107;118m━\033[38;2;255;111;120m╮\033[38;2;255;115;122m╱\033[38;2;255;118;123m╭\033[38;2;255;122;125m╮\033[38;2;255;126;127m╱\033[38;2;255;129;129m╱\033[38;2;255;133;131m╱\033[38;2;255;137;133m╱\033[38;2;255;140;135m╭\033[38;2;255;144;136m╮\033[38;2;255;148;138m╱\033[38;2;255;151;140m╱\033[38;2;255;155;142m╱\033[38;2;255;159;144m╱\033[38;2;255;162;146m╭\033[38;2;255;166;147m━\033[38;2;255;170;149m━\033[38;2;255;173;151m━\033[38;2;255;177;153m╮\033[39m\n" +
			"\033[38;2;255;8;68m╱\033[38;2;255;12;70m┃\033[38;2;255;15;72m╭\033[38;2;255;19;74m━\033[38;2;255;23;75m╮\033[38;2;255;26;77m┃\033[38;2;255;30;79m╱\033[38;2;255;34;81m┃\033[38;2;255;37;83m╭\033[38;2;255;41;85m━\033[38;2;255;45;86m╮\033[38;2;255;48;88m┃\033[38;2;255;52;90m╱\033[38;2;255;56;92m┃\033[38;2;255;59;94m╭\033[38;2;255;63;96m━\033[38;2;255;67;98m╮\033[38;2;255;70;99m┃\033[38;2;255;74;101m╱\033[38;2;255;78;103m┃\033[38;2;255;81;105m┃\033[38;2;255;85;107m┃\033[38;2;255;89;109m╭\033[38;2;255;93;111m╯\033[38;2;255;96;112m╱\033[38;2;255;100;114m╰\033[38;2;255;104;116m┫\033[38;2;255;107;118m┣\033[38;2;255;111;120m╯\033[38;2;255;115;122m╱\033[38;2;255;118;123m┃\033[38;2;255;122;125m┃\033[38;2;255;126;127m╱\033[38;2;255;129;129m╱\033[38;2;255;133;131m╱\033[38;2;255;137;133m╱\033[38;2;255;140;135m┃\033[38;2;255;144;136m┃\033[38;2;255;148;138m╱\033[38;2;255;151;140m╱\033[38;2;255;155;142m╱\033[38;2;255;159;144m╱\033[38;2;255;162;146m┃\033[38;2;255;166;147m╭\033[38;2;255;170;149m━\033[38;2;255;173;151m╮\033[38;2;255;177;153m┃\033[39m\n" +
			"\033[38;2;255;8;68m╱\033[38;2;255;12;70m┃\033[38;2;255;15;72m┃\033[38;2;255;19;74m╱\033[38;2;255;23;75m╰\033[38;2;255;26;77m╯\033[38;2;255;30;79m╱\033[38;2;255;34;81m┃\033[38;2;255;37;83m┃\033[38;2;255;41;85m╱\033[38;2;255;45;86m┃\033[38;2;255;48;88m┃\033[38;2;255;52;90m╱\033[38;2;255;56;92m┃\033[38;2;255;59;94m╰\033[38;2;255;63;96m━\033[38;2;255;67;98m━\033[38;2;255;70;99m╮\033[38;2;255;74;101m╱\033[38;2;255;78;103m┃\033[38;2;255;81;105m╰\033[38;2;255;85;107m╯\033[38;2;255;89;109m╯\033[38;2;255;93;111m╱\033[38;2;255;96;112m╱\033[38;2;255;100;114m╱\033[38;2;255;104;116m┃\033[38;2;255;107;118m┃\033[38;2;255;111;120m╱\033[38;2;255;115;122m╱\033[38;2;255;118;123m┃\033[38;2;255;122;125m┃\033[38;2;255;126;127m╱\033[38;2;255;129;129m╱\033[38;2;255;133;131m╱\033[38;2;255;137;133m╱\033[38;2;255;140;135m┃\033[38;2;255;144;136m┃\033[38;2;255;148;138m╱\033[38;2;255;151;140m╱\033[38;2;255;155;142m╱\033[38;2;255;159;144m╱\033[38;2;255;162;146m┃\033[38;2;255;166;147m╰\033[38;2;255;170;149m━\033[38;2;255;173;151m━\033[38;2;255;177;153m╮\033[39m\n" +
			"\033[38;2;255;8;68m╱\033[38;2;255;12;70m┃\033[38;2;255;15;72m┃\033[38;2;255;19;74m╭\033[38;2;255;23;75m━\033[38;2;255;26;77m╮\033[38;2;255;30;79m╱\033[38;2;255;34;81m┃\033[38;2;255;37;83m┃\033[38;2;255;41;85m╱\033[38;2;255;45;86m┃\033[38;2;255;48;88m┃\033[38;2;255;52;90m╱\033[38;2;255;56;92m╰\033[38;2;255;59;94m━\033[38;2;255;63;96m━\033[38;2;255;67;98m╮\033[38;2;255;70;99m┃\033[38;2;255;74;101m╱\033[38;2;255;78;103m┃\033[38;2;255;81;105m╭\033[38;2;255;85;107m╮\033[38;2;255;89;109m┃\033[38;2;255;93;111m╱\033[38;2;255;96;112m╱\033[38;2;255;100;114m╱\033[38;2;255;104;116m┃\033[38;2;255;107;118m┃\033[38;2;255;111;120m╱\033[38;2;255;115;122m╱\033[38;2;255;118;123m┃\033[38;2;255;122;125m┃\033[38;2;255;126;127m╱\033[38;2;255;129;129m╭\033[38;2;255;133;131m╮\033[38;2;255;137;133m╱\033[38;2;255;140;135m┃\033[38;2;255;144;136m┃\033[38;2;255;148;138m╱\033[38;2;255;151;140m╭\033[38;2;255;155;142m╮\033[38;2;255;159;144m╱\033[38;2;255;162;146m╰\033[38;2;255;166;147m━\033[38;2;255;170;149m━\033[38;2;255;173;151m╮\033[38;2;255;177;153m┃\033[39m\n" +
			"\033[38;2;255;8;68m╱\033[38;2;255;12;70m┃\033[38;2;255;15;72m╰\033[38;2;255;19;74m┻\033[38;2;255;23;75m━\033[38;2;255;26;77m┃\033[38;2;255;30;79m╱\033[38;2;255;34;81m┃\033[38;2;255;37;83m╰\033[38;2;255;41;85m━\033[38;2;255;45;86m╯\033[38;2;255;48;88m┃\033[38;2;255;52;90m╱\033[38;2;255;56;92m┃\033[38;2;255;59;94m╰\033[38;2;255;63;96m━\033[38;2;255;67;98m╯\033[38;2;255;70;99m┃\033[38;2;255;74;101m╱\033[38;2;255;78;103m┃\033[38;2;255;81;105m┃\033[38;2;255;85;107m┃\033[38;2;255;89;109m╰\033[38;2;255;93;111m╮\033[38;2;255;96;112m╱\033[38;2;255;100;114m╭\033[38;2;255;104;116m┫\033[38;2;255;107;118m┣\033[38;2;255;111;120m╮\033[38;2;255;115;122m╱\033[38;2;255;118;123m┃\033[38;2;255;122;125m╰\033[38;2;255;126;127m━\033[38;2;255;129;129m╯\033[38;2;255;133;131m┃\033[38;2;255;137;133m╱\033[38;2;255;140;135m┃\033[38;2;255;144;136m╰\033[38;2;255;148;138m━\033[38;2;255;151;140m╯\033[38;2;255;155;142m┃\033[38;2;255;159;144m╱\033[38;2;255;162;146m┃\033[38;2;255;166;147m╰\033[38;2;255;170;149m━\033[38;2;255;173;151m╯\033[38;2;255;177;153m┃\033[39m\n\033[0m" +
			"\033[38;2;255;8;68m╱\033[38;2;255;12;70m╰\033[38;2;255;15;72m━\033[38;2;255;19;74m━\033[38;2;255;23;75m━\033[38;2;255;26;77m╯\033[38;2;255;30;79m╱\033[38;2;255;34;81m╰\033[38;2;255;37;83m━\033[38;2;255;41;85m━\033[38;2;255;45;86m━\033[38;2;255;48;88m╯\033[38;2;255;52;90m╱\033[38;2;255;56;92m╰\033[38;2;255;59;94m━\033[38;2;255;63;96m━\033[38;2;255;67;98m━\033[38;2;255;70;99m╯\033[38;2;255;74;101m╱\033[38;2;255;78;103m╰\033[38;2;255;81;105m╯\033[38;2;255;85;107m╰\033[38;2;255;89;109m━\033[38;2;255;93;111m╯\033[38;2;255;96;112m╱\033[38;2;255;100;114m╰\033[38;2;255;104;116m━\033[38;2;255;107;118m━\033[38;2;255;111;120m╯\033[38;2;255;115;122m╱\033[38;2;255;118;123m╰\033[38;2;255;122;125m━\033[38;2;255;126;127m━\033[38;2;255;129;129m━\033[38;2;255;133;131m╯\033[38;2;255;137;133m╱\033[38;2;255;140;135m╰\033[38;2;255;144;136m━\033[38;2;255;148;138m━\033[38;2;255;151;140m━\033[38;2;255;155;142m╯\033[38;2;255;159;144m╱\033[38;2;255;162;146m╰\033[38;2;255;166;147m━\033[38;2;255;170;149m━\033[38;2;255;173;151m━\033[38;2;255;177;153m╯\033[39m"

		fmt.Print(logo)
		fmt.Print("\n\n")
		fmt.Println("\033[1;36mGoSkills Agent CLI - Interactive Chat\033[0m")
		fmt.Println("Type \033[1;33m\\help\033[0m for available commands, \033[1;33m\\exit\033[0m to quit")
		fmt.Println(strings.Repeat("-", 60))

		var lastReport string

		for {
			// Use TUI for input
			input, err := GetInput("> ")
			if err != nil {
				fmt.Printf("Error reading input: %v\n", err)
				break
			}

			input = strings.TrimSpace(input)
			if input == "" {
				continue
			}

			// Handle special commands
			switch input {
			case "\\help":
				fmt.Println("\n📚 Available Commands:")
				fmt.Println("  \\help    - Show this help message")
				fmt.Println("  \\clear   - Clear conversation history")
				fmt.Println("  \\podcast - Generate a podcast script from the last report")
				fmt.Println("  \\exit    - Exit the chat session")
				fmt.Println("  \\quit    - Exit the chat session")
				continue
			case "\\clear":
				planningAgent.ClearHistory()
				fmt.Println("✨ Conversation history cleared")
				continue
			case "\\podcast":
				if lastReport == "" {
					fmt.Println("❌ No report available to convert to podcast. Please generate a report first.")
					continue
				}
				fmt.Println("🎙️ Generating podcast script...")

				// Create a plan for podcast generation
				podcastPlan := &agent.Plan{
					Description: "Generate podcast script",
					Tasks: []agent.Task{
						{
							Type:        agent.TaskTypePodcast,
							Description: "Generate podcast script from the report",
							Parameters: map[string]interface{}{
								"content": lastReport,
							},
						},
					},
				}

				results, err := planningAgent.Execute(ctx, podcastPlan)
				if err != nil {
					fmt.Printf("\n❌ Error: %v\n", err)
					continue
				}

				for _, result := range results {
					if result.Success {
						fmt.Println("\n" + result.Output)
					}
				}
				continue
			case "\\exit", "\\quit":
				fmt.Println("👋 Goodbye!")
				return nil
			}

			// Add user message to history
			planningAgent.AddUserMessage(input)

			plan, err := planningAgent.PlanWithReview(ctx, input)
			if err != nil {
				fmt.Printf("\n❌ Error: %v\n", err)
				continue
			}

			results, err := planningAgent.Execute(ctx, plan)
			if err != nil {
				fmt.Printf("\n❌ Error: %v\n", err)
				continue
			}

			// Extract final output
			var finalOutput string
			for i := len(results) - 1; i >= 0; i-- {
				if (results[i].TaskType == agent.TaskTypeRender || results[i].TaskType == agent.TaskTypeReport) && results[i].Success {
					finalOutput = results[i].Output
					break
				}
			}
			if finalOutput == "" {
				for _, result := range results {
					if result.Success {
						finalOutput += result.Output + "\n\n"
					}
				}
			}

			// Update lastReport if we have a valid output
			if finalOutput != "" {
				lastReport = finalOutput
			}

			// Add assistant response to history
			planningAgent.AddAssistantMessage(finalOutput)

			fmt.Println("\n📄 Final Report:")
			if cfg.Verbose {
				fmt.Println(strings.Repeat("-", 60))
			}
			fmt.Println(finalOutput)

			// Podcast generation is now handled by the planner based on user request.
			// We no longer automatically prompt for it here.
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	// Disable the default completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	config.SetupFlags(rootCmd)
}
