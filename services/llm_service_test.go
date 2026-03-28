package services

import (
	"context"
	"os"
	"testing"
)

func TestLLMServiceSendMessageNilProviderNoError(t *testing.T) {
	// LLMService.SendMessage with nil provider returns error without panic
	svc := &LLMService{
		cfg:            Config{Provider: "openai", Model: "gpt-4"},
		activeProvider: nil,
	}
	svc.ctx = context.Background()

	err := svc.SendMessage("tab-1", "hello", "")
	if err == nil {
		t.Error("expected error when activeProvider is nil, got nil")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Config loaded from env vars maps provider/model correctly
	t.Setenv("PAIRADMIN_PROVIDER", "anthropic")
	t.Setenv("PAIRADMIN_MODEL", "claude-3-5-sonnet-20241022")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")
	t.Setenv("OLLAMA_HOST", "http://localhost:11434")

	cfg := LoadConfig()

	if cfg.Provider != "anthropic" {
		t.Errorf("expected Provider 'anthropic', got %q", cfg.Provider)
	}
	if cfg.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("expected Model 'claude-3-5-sonnet-20241022', got %q", cfg.Model)
	}
	if cfg.AnthropicKey != "test-anthropic-key" {
		t.Errorf("expected AnthropicKey 'test-anthropic-key', got %q", cfg.AnthropicKey)
	}
	if cfg.OpenAIKey != "test-openai-key" {
		t.Errorf("expected OpenAIKey 'test-openai-key', got %q", cfg.OpenAIKey)
	}
	if cfg.OpenRouterKey != "test-openrouter-key" {
		t.Errorf("expected OpenRouterKey 'test-openrouter-key', got %q", cfg.OpenRouterKey)
	}
	if cfg.OllamaHost != "http://localhost:11434" {
		t.Errorf("expected OllamaHost 'http://localhost:11434', got %q", cfg.OllamaHost)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	// Clear all related env vars
	for _, key := range []string{"PAIRADMIN_PROVIDER", "PAIRADMIN_MODEL", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "OPENROUTER_API_KEY", "OLLAMA_HOST"} {
		os.Unsetenv(key)
	}

	cfg := LoadConfig()

	// Should return empty/default values without panicking
	if cfg.Provider != "" {
		t.Errorf("expected empty Provider, got %q", cfg.Provider)
	}
	if cfg.Model != "" {
		t.Errorf("expected empty Model, got %q", cfg.Model)
	}
}

func TestNewLLMServiceCreation(t *testing.T) {
	cfg := Config{
		Provider: "openai",
		Model:    "gpt-4",
		OpenAIKey: "test-key",
	}
	svc := NewLLMService(cfg)
	if svc == nil {
		t.Fatal("expected non-nil LLMService")
	}
}

func TestLLMServiceStartup(t *testing.T) {
	cfg := Config{Provider: "openai", Model: "gpt-4"}
	svc := NewLLMService(cfg)
	ctx := context.Background()
	svc.Startup(ctx)
	if svc.ctx == nil {
		t.Error("expected ctx to be set after Startup")
	}
}
