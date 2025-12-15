package agent

import "context"

// TaskType represents the type of task to be executed by a subagent.
type TaskType string

const (
	TaskTypeSearch  TaskType = "SEARCH"
	TaskTypeAnalyze TaskType = "ANALYZE"
	TaskTypeReport  TaskType = "REPORT"
	TaskTypeRender  TaskType = "RENDER"
	TaskTypePodcast TaskType = "PODCAST"
	TaskTypePPT     TaskType = "PPT"
)

// Task represents a subtask to be executed by a subagent.
type Task struct {
	Type        TaskType               `json:"type"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// Result contains the output from a subagent execution.
type Result struct {
	TaskType TaskType               `json:"task_type"`
	Success  bool                   `json:"success"`
	Output   string                 `json:"output"`
	Error    string                 `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	NewTasks []Task                 `json:"new_tasks,omitempty"`
}

// Plan represents a collection of tasks with dependencies.
type Plan struct {
	Tasks       []Task `json:"tasks"`
	Description string `json:"description"`
}

// Subagent interface for all subagent implementations.
type Subagent interface {
	Execute(ctx context.Context, task Task) (Result, error)
	Type() TaskType
}

// InteractionHandler defines methods for human-in-the-loop interaction.
type InteractionHandler interface {
	// ReviewPlan asks the user to review and potentially modify the plan.
	// Returns the modified plan description (if changed) or empty string if approved.
	ReviewPlan(plan *Plan) (string, error)

	// ConfirmPodcastGeneration asks the user if they want to generate a podcast from the report.
	// Returns true if confirmed.
	ConfirmPodcastGeneration(report string) (bool, error)

	// Log sends a log message to the user interface.
	Log(message string)
}
