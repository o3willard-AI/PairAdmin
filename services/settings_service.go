package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pairadmin/services/config"
	"pairadmin/services/keychain"
	"pairadmin/services/llm"

	"github.com/awnumar/memguard"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// buildProviderFn is the function used to construct an LLM provider.
// Tests may replace this to inject a mock provider.
var buildProviderFn func(Config, func(string) string) llm.Provider = buildProvider

// captureManagerForceCapture is implemented by CaptureManager to allow forcing an immediate capture.
type captureManagerForceCapture interface {
	ForceCapture()
}

// ExportMessage is a single chat message for export.
type ExportMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SettingsService is the Wails-bound service for reading and writing application settings.
// It manages config persistence via Viper, API key storage via OS keychain, and LLM
// connection testing.
type SettingsService struct {
	ctx            context.Context
	keychainClient *keychain.Client
	llmService     *LLMService
	captureManager captureManagerForceCapture
	// emitFn is the Wails events emitter; injectable for test isolation.
	emitFn func(ctx context.Context, event string, optionalData ...interface{})
}

// NewSettingsService creates a new SettingsService with the given keychain client.
func NewSettingsService(kc *keychain.Client) *SettingsService {
	return &SettingsService{
		keychainClient: kc,
		emitFn:         runtime.EventsEmit,
	}
}

// Startup is called by Wails after the application context is available.
func (s *SettingsService) Startup(ctx context.Context) {
	s.ctx = ctx
}

// SetLLMService wires the LLMService reference so TestConnection can call buildProvider.
func (s *SettingsService) SetLLMService(svc *LLMService) {
	s.llmService = svc
}

// GetSettings returns the current application configuration from disk.
func (s *SettingsService) GetSettings() (*config.AppConfig, error) {
	return config.LoadAppConfig()
}

// SaveSettings persists the given configuration to disk, rebuilds the LLM provider,
// and emits a settings:changed event.
func (s *SettingsService) SaveSettings(cfg *config.AppConfig) error {
	if err := config.SaveAppConfig(cfg); err != nil {
		return err
	}
	// Rebuild so provider/model changes take effect without requiring an app restart.
	// RebuildProvider re-reads config from disk so it sees the values just written.
	if s.llmService != nil {
		s.llmService.RebuildProvider()
	}
	if s.ctx != nil && s.emitFn != nil {
		s.emitFn(s.ctx, "settings:changed", cfg)
	}
	return nil
}

// GetAPIKeyStatus returns "stored" if a key exists for the given provider, "" otherwise.
// NEVER returns the actual key to the frontend.
func (s *SettingsService) GetAPIKeyStatus(provider string) (string, error) {
	val, err := s.keychainClient.Get(provider)
	if err != nil {
		return "", err
	}
	if val != "" {
		return "stored", nil
	}
	return "", nil
}

// SaveAPIKey writes the API key for the given provider to the OS keychain.
// Passing an empty key removes the stored key.
// After a successful keychain write, the Enclave on LLMService is updated and provider rebuilt.
func (s *SettingsService) SaveAPIKey(provider, key string) error {
	if key == "" {
		return s.keychainClient.Remove(provider)
	}
	if err := s.keychainClient.Set(provider, key); err != nil {
		return err
	}
	// If LLMService is wired, update its Enclave and rebuild provider.
	if s.llmService != nil && key != "" {
		buf := memguard.NewBufferFromBytes([]byte(key))
		s.llmService.SetAPIKeyEnclave(provider, buf.Seal())
		s.llmService.RebuildProvider()
	}
	return nil
}

// TestConnection tests the LLM connection for the given provider and model.
// Returns "Connected" on success, or a descriptive error string on failure.
func (s *SettingsService) TestConnection(provider, model string) (string, error) {
	// Load keychain key for the given provider.
	apiKey, err := s.keychainClient.Get(provider)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve API key: %w", err)
	}

	// Build a temporary config using the keychain key and env vars for other fields.
	envCfg := LoadConfig()
	cfg := Config{
		Provider:      provider,
		Model:         model,
		OpenAIKey:     envCfg.OpenAIKey,
		AnthropicKey:  envCfg.AnthropicKey,
		OpenRouterKey: envCfg.OpenRouterKey,
		OllamaHost:    envCfg.OllamaHost,
		LMStudioHost:  envCfg.LMStudioHost,
	}

	// Inject the keychain key for the specified provider.
	switch provider {
	case "openai":
		if apiKey != "" {
			cfg.OpenAIKey = apiKey
		}
	case "anthropic":
		if apiKey != "" {
			cfg.AnthropicKey = apiKey
		}
	case "openrouter":
		if apiKey != "" {
			cfg.OpenRouterKey = apiKey
		}
	}

	p := buildProviderFn(cfg, nil)
	if p == nil {
		return "", fmt.Errorf("unsupported or unconfigured provider: %s", provider)
	}

	ctx := s.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	if err := p.TestConnection(ctx); err != nil {
		return "", err
	}
	return "Connected", nil
}

// SetCaptureManager wires the CaptureManager so ForceRefresh can trigger an immediate capture.
func (s *SettingsService) SetCaptureManager(cm captureManagerForceCapture) {
	s.captureManager = cm
}

// SetModel parses a "provider:model" string, saves to AppConfig, and emits settings:model-changed.
// Returns an error if the format is invalid (no colon separator).
func (s *SettingsService) SetModel(providerModel string) (string, error) {
	idx := strings.Index(providerModel, ":")
	if idx < 0 {
		return "", fmt.Errorf("Invalid format: use provider:model (e.g., openai:gpt-4)")
	}
	provider := providerModel[:idx]
	model := providerModel[idx+1:]
	if strings.Contains(model, `\`) {
		return "", fmt.Errorf("Invalid model ID %q: use a forward slash, not a backslash (e.g. google/gemma-3-27b-it)", model)
	}

	cfg, err := config.LoadAppConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	cfg.Provider = provider
	cfg.Model = model
	if err := config.SaveAppConfig(cfg); err != nil {
		return "", fmt.Errorf("failed to save config: %w", err)
	}

	if s.llmService != nil {
		s.llmService.RebuildProvider()
	}
	if s.ctx != nil && s.emitFn != nil {
		s.emitFn(s.ctx, "settings:model-changed", providerModel)
	}
	return fmt.Sprintf("Model set to %s", providerModel), nil
}

// SetContextLines saves the terminal context line count to AppConfig.
// Returns an error if lines is not positive or exceeds 10000.
func (s *SettingsService) SetContextLines(lines int) (string, error) {
	if lines <= 0 {
		return "", fmt.Errorf("Context lines must be positive")
	}
	if lines > 10000 {
		return "", fmt.Errorf("Context lines must be <= 10000")
	}

	cfg, err := config.LoadAppConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	cfg.ContextLines = lines
	if err := config.SaveAppConfig(cfg); err != nil {
		return "", fmt.Errorf("failed to save config: %w", err)
	}
	return fmt.Sprintf("Context set to %d lines", lines), nil
}

// ForceRefresh triggers an immediate terminal capture via the CaptureManager.
func (s *SettingsService) ForceRefresh() (string, error) {
	if s.captureManager == nil {
		return "", fmt.Errorf("No capture manager available")
	}
	s.captureManager.ForceCapture()
	return "Terminal content refreshed", nil
}

// ExportChat exports the given chat messages to a file in the home directory.
// format must be "json" or "txt". Returns the absolute file path.
func (s *SettingsService) ExportChat(tabId, format string, messages []ExportMessage) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02-150405")
	filename := fmt.Sprintf("pairadmin-export-%s.%s", timestamp, format)
	path := filepath.Join(home, filename)

	var data []byte
	switch format {
	case "json":
		data, err = json.MarshalIndent(messages, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal messages: %w", err)
		}
	case "txt":
		var sb strings.Builder
		for _, m := range messages {
			sb.WriteString(fmt.Sprintf("[%s]: %s\n\n", m.Role, m.Content))
		}
		data = []byte(sb.String())
	default:
		return "", fmt.Errorf("unsupported format: %q (use json or txt)", format)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	return path, nil
}

// RenameTab emits a terminal:rename event so the frontend can update the tab label.
func (s *SettingsService) RenameTab(tabId, label string) (string, error) {
	if s.ctx != nil && s.emitFn != nil {
		s.emitFn(s.ctx, "terminal:rename", map[string]string{"tabId": tabId, "label": label})
	}
	return fmt.Sprintf("Tab renamed to %s", label), nil
}

// LoadConfigWithViper reads provider/model from AppConfig (Viper/YAML), falling back to
// environment variables when those fields are empty. This implements D-04 config priority.
func LoadConfigWithViper() Config {
	appCfg, _ := config.LoadAppConfig()
	envCfg := LoadConfig()

	provider := envCfg.Provider
	if appCfg != nil && appCfg.Provider != "" {
		provider = appCfg.Provider
	}

	model := envCfg.Model
	if appCfg != nil && appCfg.Model != "" {
		model = appCfg.Model
	}

	return Config{
		Provider:      provider,
		Model:         model,
		OpenAIKey:     envCfg.OpenAIKey,
		AnthropicKey:  envCfg.AnthropicKey,
		OpenRouterKey: envCfg.OpenRouterKey,
		OllamaHost:    envCfg.OllamaHost,
		LMStudioHost:  envCfg.LMStudioHost,
	}
}

