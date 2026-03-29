package services

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"pairadmin/services/config"
	"pairadmin/services/llm"
	"pairadmin/services/llm/filter"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Config holds the LLM configuration sourced from environment variables.
type Config struct {
	Provider      string // PAIRADMIN_PROVIDER: "openai"|"anthropic"|"ollama"|"openrouter"|"lmstudio"
	Model         string // PAIRADMIN_MODEL: model name string
	OpenAIKey     string // OPENAI_API_KEY
	AnthropicKey  string // ANTHROPIC_API_KEY
	OpenRouterKey string // OPENROUTER_API_KEY (alternative key for OpenRouter)
	OllamaHost    string // OLLAMA_HOST: optional, defaults to localhost
	LMStudioHost  string // LMSTUDIO_HOST: optional, defaults to http://localhost:1234/v1
}

// LoadConfig reads LLM configuration from environment variables.
func LoadConfig() Config {
	return Config{
		Provider:      os.Getenv("PAIRADMIN_PROVIDER"),
		Model:         os.Getenv("PAIRADMIN_MODEL"),
		OpenAIKey:     os.Getenv("OPENAI_API_KEY"),
		AnthropicKey:  os.Getenv("ANTHROPIC_API_KEY"),
		OpenRouterKey: os.Getenv("OPENROUTER_API_KEY"),
		OllamaHost:    os.Getenv("OLLAMA_HOST"),
		LMStudioHost:  os.Getenv("LMSTUDIO_HOST"),
	}
}

// ChatTokenEvent is the payload emitted on "llm:chunk" and "llm:error" events.
type ChatTokenEvent struct {
	Seq   int    `json:"seq"`
	Text  string `json:"text"`
	Done  bool   `json:"done"`
	Error string `json:"error,omitempty"`
}

// UsageEvent is the payload emitted on "llm:usage" events.
type UsageEvent struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

// filterPipelineRebuilder is implemented by CaptureManager to rebuild its filter pipeline.
type filterPipelineRebuilder interface {
	RebuildFilterPipeline()
}

// LLMService is the Wails-bound service that streams LLM responses to the frontend.
// It follows the same lifecycle pattern as CommandService (Startup + ctx).
type LLMService struct {
	ctx            context.Context
	cfg            Config
	activeProvider llm.Provider
	captureManager filterPipelineRebuilder
}

// SetCaptureManager wires the CaptureManager so FilterCommand can trigger pipeline rebuilds.
func (s *LLMService) SetCaptureManager(mgr filterPipelineRebuilder) {
	s.captureManager = mgr
}

// NewLLMService creates a new LLMService and initializes the active provider based on cfg.
func NewLLMService(cfg Config) *LLMService {
	svc := &LLMService{cfg: cfg}
	svc.activeProvider = buildProvider(cfg)
	return svc
}

// Startup is called by Wails after the application context is available.
func (s *LLMService) Startup(ctx context.Context) {
	s.ctx = ctx
}

// SendMessage sends a user message and streams the LLM response via Wails events.
// Events emitted: "llm:chunk" (with sequence numbers), "llm:done", "llm:error", "llm:usage".
// Returns immediately; response tokens arrive asynchronously via events.
func (s *LLMService) SendMessage(tabId, userInput, terminalContext string) error {
	if s.activeProvider == nil {
		return fmt.Errorf("no LLM provider configured; set PAIRADMIN_PROVIDER environment variable")
	}

	// Apply filter pipeline: ANSI stripping + credential redaction before LLM
	credFilter, err := filter.NewCredentialFilter()
	if err != nil {
		return fmt.Errorf("failed to initialize credential filter: %w", err)
	}
	pipeline := filter.NewPipeline(filter.NewANSIFilter(), credFilter)
	filteredContext, _ := pipeline.Apply(terminalContext)

	messages := llm.BuildMessages(llm.SystemPrompt, filteredContext, userInput)

	go func() {
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
		defer cancel()

		ch, err := s.activeProvider.Stream(ctx, messages)
		if err != nil {
			runtime.EventsEmit(s.ctx, "llm:error", ChatTokenEvent{
				Error: err.Error(), Done: true,
			})
			return
		}

		seq := 0
		var batch []string
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		flush := func() {
			if len(batch) == 0 {
				return
			}
			runtime.EventsEmit(s.ctx, "llm:chunk", ChatTokenEvent{
				Seq:  seq,
				Text: strings.Join(batch, ""),
			})
			seq++
			batch = batch[:0]
		}

		for {
			select {
			case chunk, ok := <-ch:
				if !ok {
					// Channel closed — stream ended without explicit Done
					flush()
					runtime.EventsEmit(s.ctx, "llm:done", ChatTokenEvent{Seq: seq, Done: true})
					return
				}
				if chunk.Error != nil {
					flush()
					runtime.EventsEmit(s.ctx, "llm:error", ChatTokenEvent{
						Seq: seq, Error: chunk.Error.Error(), Done: true,
					})
					return
				}
				if chunk.Done {
					flush()
					runtime.EventsEmit(s.ctx, "llm:done", ChatTokenEvent{Seq: seq, Done: true})
					return
				}
				batch = append(batch, chunk.Text)
			case <-ticker.C:
				flush()
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// FilterCommand handles /filter add|list|remove commands.
// Returns a human-readable string to display as a system message in the chat pane.
func (s *LLMService) FilterCommand(command string) (string, error) {
	parts := strings.Fields(command)
	// parts[0] is "/filter"
	if len(parts) < 2 {
		return "Usage: /filter add <name> <regex> <action> | /filter list | /filter remove <name>", nil
	}

	cfg, err := config.LoadAppConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	switch parts[1] {
	case "list":
		if len(cfg.CustomPatterns) == 0 {
			return "No custom filter patterns configured.", nil
		}
		var sb strings.Builder
		sb.WriteString("Custom filter patterns:\n")
		for _, p := range cfg.CustomPatterns {
			sb.WriteString(fmt.Sprintf("  - %s: /%s/ (%s)\n", p.Name, p.Regex, p.Action))
		}
		return sb.String(), nil

	case "add":
		if len(parts) < 5 {
			return "Usage: /filter add <name> <regex> <action>\nAction: redact | remove", nil
		}
		name := parts[2]
		regex := parts[3]
		action := parts[4]
		if action != "redact" && action != "remove" {
			return fmt.Sprintf("Invalid action %q. Use 'redact' or 'remove'.", action), nil
		}
		// Validate regex compiles
		if _, err := regexp.Compile(regex); err != nil {
			return fmt.Sprintf("Invalid regex %q: %v", regex, err), nil
		}
		// Check for duplicate name
		for _, p := range cfg.CustomPatterns {
			if p.Name == name {
				return fmt.Sprintf("Pattern %q already exists. Remove it first.", name), nil
			}
		}
		cfg.CustomPatterns = append(cfg.CustomPatterns, config.CustomPattern{
			Name: name, Regex: regex, Action: action,
		})
		if err := config.SaveAppConfig(cfg); err != nil {
			return "", fmt.Errorf("failed to save config: %w", err)
		}
		if s.captureManager != nil {
			s.captureManager.RebuildFilterPipeline()
		}
		return fmt.Sprintf("Added filter pattern %q (/%s/ %s).", name, regex, action), nil

	case "remove":
		if len(parts) < 3 {
			return "Usage: /filter remove <name>", nil
		}
		name := parts[2]
		found := false
		filtered := make([]config.CustomPattern, 0, len(cfg.CustomPatterns))
		for _, p := range cfg.CustomPatterns {
			if p.Name == name {
				found = true
				continue
			}
			filtered = append(filtered, p)
		}
		if !found {
			return fmt.Sprintf("Pattern %q not found.", name), nil
		}
		cfg.CustomPatterns = filtered
		if err := config.SaveAppConfig(cfg); err != nil {
			return "", fmt.Errorf("failed to save config: %w", err)
		}
		if s.captureManager != nil {
			s.captureManager.RebuildFilterPipeline()
		}
		return fmt.Sprintf("Removed filter pattern %q.", name), nil

	default:
		return "Unknown subcommand. Use: /filter add | /filter list | /filter remove", nil
	}
}

// buildProvider creates the appropriate LLM provider based on the config.
// Returns nil for unknown or empty providers rather than panicking.
func buildProvider(cfg Config) llm.Provider {
	switch cfg.Provider {
	case "openai":
		return llm.NewOpenAIProvider(cfg.OpenAIKey, "", cfg.Model)
	case "openrouter":
		key := cfg.OpenRouterKey
		if key == "" {
			key = cfg.OpenAIKey // fallback
		}
		return llm.NewOpenAIProvider(key, "https://openrouter.ai/api/v1", cfg.Model)
	case "lmstudio":
		baseURL := cfg.LMStudioHost
		if baseURL == "" {
			baseURL = "http://localhost:1234/v1"
		}
		return llm.NewOpenAIProvider("", baseURL, cfg.Model)
	case "anthropic":
		return llm.NewAnthropicProvider(cfg.AnthropicKey, cfg.Model)
	case "ollama":
		p, err := llm.NewOllamaProvider(cfg.OllamaHost, cfg.Model)
		if err != nil {
			// Log as runtime issue; return nil so SendMessage returns descriptive error
			return nil
		}
		return p
	default:
		return nil
	}
}
