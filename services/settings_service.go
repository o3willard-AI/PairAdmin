package services

import (
	"context"
	"fmt"

	"pairadmin/services/config"
	"pairadmin/services/keychain"
	"pairadmin/services/llm"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// buildProviderFn is the function used to construct an LLM provider.
// Tests may replace this to inject a mock provider.
var buildProviderFn func(Config) llm.Provider = buildProvider

// SettingsService is the Wails-bound service for reading and writing application settings.
// It manages config persistence via Viper, API key storage via OS keychain, and LLM
// connection testing.
type SettingsService struct {
	ctx            context.Context
	keychainClient *keychain.Client
	llmService     *LLMService
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

// SaveSettings persists the given configuration to disk and emits a settings:changed event.
func (s *SettingsService) SaveSettings(cfg *config.AppConfig) error {
	if err := config.SaveAppConfig(cfg); err != nil {
		return err
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
func (s *SettingsService) SaveAPIKey(provider, key string) error {
	if key == "" {
		return s.keychainClient.Remove(provider)
	}
	return s.keychainClient.Set(provider, key)
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

	p := buildProviderFn(cfg)
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

