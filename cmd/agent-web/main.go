package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/smallnest/goskills/agent"
	"github.com/spf13/cobra"
)

//go:embed ui/*
var uiAssets embed.FS

var (
	apiKey  string
	apiBase string
	model   string
	addr    string
	verbose bool
	ppt     bool
	podcast bool
)

// WebInteractionHandler implements agent.InteractionHandler for the web interface.
type WebInteractionHandler struct {
	eventChan    chan Event
	responseChan chan string
	events       []Event
	mu           sync.Mutex
	sessionID    string
	userRequest  string
}

type Event struct {
	Type      string      `json:"type"`
	Content   string      `json:"content,omitempty"`
	Plan      *agent.Plan `json:"plan,omitempty"`
	Podcast   interface{} `json:"podcast,omitempty"`
	PPT       string      `json:"ppt,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

func NewWebInteractionHandler(sessionID, userRequest string) *WebInteractionHandler {
	return &WebInteractionHandler{
		eventChan:    make(chan Event, 100),
		responseChan: make(chan string),
		events:       make([]Event, 0),
		sessionID:    sessionID,
		userRequest:  userRequest,
	}
}

func (h *WebInteractionHandler) ReviewPlan(plan *agent.Plan) (string, error) {
	event := Event{
		Type:      "plan_review",
		Plan:      plan,
		Timestamp: time.Now(),
	}
	h.Broadcast(event)
	// Wait for user response
	response := <-h.responseChan
	return response, nil
}

func (h *WebInteractionHandler) ConfirmPodcastGeneration(report string) (bool, error) {
	// Auto-approve for web interface
	return true, nil
}

func (h *WebInteractionHandler) Log(message string) {
	h.Broadcast(Event{
		Type:      "log",
		Content:   message,
		Timestamp: time.Now(),
	})
}

func (h *WebInteractionHandler) Broadcast(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	h.mu.Lock()
	h.events = append(h.events, event)
	h.mu.Unlock()

	h.eventChan <- event

	if event.Type == "done" {
		h.SaveSession()
	}
}

func (h *WebInteractionHandler) SaveSession() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.events) == 0 {
		return
	}

	// Do not save session if request is /clear
	if strings.TrimSpace(h.userRequest) == "/clear" {
		return
	}

	// Create sessions directory if not exists
	if err := os.MkdirAll("sessions", 0755); err != nil {
		log.Printf("Failed to create sessions directory: %v", err)
		return
	}

	// Sanitize user request for filename
	safeRequest := sanitizeFilename(h.userRequest)

	// Truncate to first 50 chars (rune-aware)
	runes := []rune(safeRequest)
	if len(runes) > 50 {
		safeRequest = string(runes[:50])
	}

	// Ensure filename is not empty
	if safeRequest == "" {
		safeRequest = h.sessionID
	}

	// Append session ID to ensure uniqueness
	filename := fmt.Sprintf("sessions/%s-%s.json", safeRequest, h.sessionID)

	file, err := os.Create(filename)
	if err != nil {
		log.Printf("Failed to create session file: %v", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(h.events); err != nil {
		log.Printf("Failed to save session: %v", err)
	}
}

func sanitizeFilename(name string) string {
	// Replace invalid characters with underscore
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\n", "\r", "\t"}
	for _, char := range invalid {
		name = strings.ReplaceAll(name, char, "_")
	}
	return strings.TrimSpace(name)
}

// Session represents a user session
type Session struct {
	ID        string
	Agent     *agent.PlanningAgent
	Handler   *WebInteractionHandler
	CreatedAt time.Time
}

// SessionManager manages user sessions
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

func (sm *SessionManager) GetSession(id string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[id]
}

func (sm *SessionManager) CreateSession(id string, config agent.AgentConfig) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if session already exists
	if session, ok := sm.sessions[id]; ok {
		return session, nil
	}

	handler := NewWebInteractionHandler(id, "")
	planningAgent, err := agent.NewPlanningAgent(config, handler)
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:        id,
		Agent:     planningAgent,
		Handler:   handler,
		CreatedAt: time.Now(),
	}

	sm.sessions[id] = session
	return session, nil
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "agent-web",
		Short: "Start the agent web interface",
		Run:   runServer,
	}

	rootCmd.Flags().StringVar(&apiKey, "api-key", os.Getenv("OPENAI_API_KEY"), "OpenAI API Key")
	rootCmd.Flags().StringVar(&apiBase, "api-base", os.Getenv("OPENAI_API_BASE"), "OpenAI API Base URL")
	rootCmd.Flags().StringVar(&model, "model", os.Getenv("OPENAI_MODEL"), "OpenAI Model")
	rootCmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "Address to listen on")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.Flags().BoolVar(&ppt, "ppt", false, "Enable PPT generation")
	rootCmd.Flags().BoolVar(&podcast, "podcast", true, "Enable Podcast generation")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command, args []string) {
	if apiKey == "" {
		log.Fatal("API key is required")
	}

	// Initialize agent config template
	configTemplate := agent.AgentConfig{
		APIKey:     apiKey,
		APIBase:    apiBase,
		Model:      model,
		Verbose:    verbose,
		RenderHTML: true,
	}

	sessionManager := NewSessionManager()

	// Serve static files
	uiFS, err := fs.Sub(uiAssets, "ui")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", http.FileServer(http.FS(uiFS)))

	// Serve generated files
	os.MkdirAll("generated", 0755)
	http.Handle("/generated/", http.StripPrefix("/generated/", http.FileServer(http.Dir("generated"))))

	// API endpoints
	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.URL.Query().Get("session_id")
		if sessionID == "" {
			http.Error(w, "Session ID required", http.StatusBadRequest)
			return
		}

		// Create session if it doesn't exist (on connection)
		session, err := sessionManager.CreateSession(sessionID, configTemplate)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		handler := session.Handler

		for {
			select {
			case event := <-handler.eventChan:
				data, err := json.Marshal(event)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	http.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Message   string `json:"message"`
			SessionID string `json:"session_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.SessionID == "" {
			http.Error(w, "Session ID required", http.StatusBadRequest)
			return
		}

		session := sessionManager.GetSession(req.SessionID)
		if session == nil {
			// Try to create it if missing (e.g. server restart)
			var err error
			session, err = sessionManager.CreateSession(req.SessionID, configTemplate)
			if err != nil {
				http.Error(w, "Failed to create session", http.StatusInternalServerError)
				return
			}
		}

		planningAgent := session.Agent
		handler := session.Handler

		// Update user request in handler for filename generation
		session.Handler.mu.Lock()
		session.Handler.userRequest = req.Message
		session.Handler.mu.Unlock()

		// Run agent in a goroutine
		go func() {
			defer func() {
				if r := recover(); r != nil {
					handler.Broadcast(Event{
						Type:    "error",
						Content: fmt.Sprintf("Panic: %v", r),
					})
				}
			}()

			// Check for direct chat
			if strings.HasPrefix(req.Message, "\\") {
				msg := strings.TrimPrefix(req.Message, "\\")

				planningAgent.AddDeveloperMessage(msg)

				// Log user request
				handler.Broadcast(Event{
					Type:    "log",
					Content: fmt.Sprintf("> User Request: %s", msg),
				})

				handler.Broadcast(Event{
					Type: "done",
				})
				return
			}

			// Add user message to history
			planningAgent.AddUserMessage(req.Message)

			// Plan with review
			plan, err := planningAgent.PlanWithReview(context.Background(), req.Message)
			if err != nil {
				handler.Broadcast(Event{
					Type:    "error",
					Content: err.Error(),
				})
				return
			}

			// Ensure PODCAST task exists if REPORT task is present - REMOVED logic to force podcast
			// The user must explicitly request a podcast for it to be included.

			// Execute
			results, err := planningAgent.Execute(context.Background(), plan)
			if err != nil {
				handler.Broadcast(Event{
					Type:    "error",
					Content: err.Error(),
				})
				return
			}

			// Extract final output and podcast script
			var finalOutput string
			var podcastScript interface{}
			var pptURL string

			for i := len(results) - 1; i >= 0; i-- {
				if (results[i].TaskType == agent.TaskTypeRender || results[i].TaskType == agent.TaskTypeReport) && results[i].Success {
					if finalOutput == "" {
						finalOutput = results[i].Output
					}
				}
				if results[i].TaskType == agent.TaskTypePodcast && results[i].Success {
					podcastScript = results[i].Metadata["script"]
				}
				if results[i].TaskType == agent.TaskTypePPT && results[i].Success {
					if url, ok := results[i].Metadata["ppt_url"].(string); ok {
						pptURL = url
					}
				}
			}

			if finalOutput == "" {
				for _, result := range results {
					if result.Success {
						finalOutput += result.Output + "\n\n"
					}
				}
			}

			// Add assistant message
			planningAgent.AddAssistantMessage(finalOutput)

			handler.Broadcast(Event{
				Type:    "response",
				Content: finalOutput,
				Podcast: podcastScript,
				PPT:     pptURL,
			})

			handler.Broadcast(Event{
				Type: "done",
			})
		}()

		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/respond", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Response  string `json:"response"`
			SessionID string `json:"session_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.SessionID == "" {
			http.Error(w, "Session ID required", http.StatusBadRequest)
			return
		}

		session := sessionManager.GetSession(req.SessionID)
		if session == nil {
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}

		// Send response to the waiting channel
		select {
		case session.Handler.responseChan <- req.Response:
		default:
			// No one waiting
		}

		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{
			"ppt":     ppt,
			"podcast": podcast,
		})
	})

	http.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		entries, err := os.ReadDir("sessions")
		if err != nil {
			// If directory doesn't exist, return empty list
			if os.IsNotExist(err) {
				json.NewEncoder(w).Encode([]string{})
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var sessions []map[string]interface{}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				info, err := entry.Info()
				if err != nil {
					continue
				}
				sessions = append(sessions, map[string]interface{}{
					"id":        strings.TrimSuffix(entry.Name(), ".json"),
					"timestamp": info.ModTime(),
				})
			}
		}

		// Sort by timestamp desc
		// (Simple bubble sort or just leave it to frontend, but let's do it here for convenience if needed,
		// actually let's just return the list and let frontend sort or just return as is)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
	})

	http.HandleFunc("/api/replay", func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.URL.Query().Get("session_id")
		if sessionID == "" {
			http.Error(w, "Session ID required", http.StatusBadRequest)
			return
		}

		filename := fmt.Sprintf("sessions/%s.json", sessionID)
		data, err := os.ReadFile(filename)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Session not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	fmt.Printf("Starting server on http://%s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
