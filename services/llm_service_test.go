package services

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"pairadmin/services/audit"
	"pairadmin/services/llm"

	"github.com/awnumar/memguard"
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

// --- Audit + memguard tests ---

// streamingMockProvider returns a single text chunk followed by a Done chunk.
type streamingMockProvider struct {
	text string
}

func (m *streamingMockProvider) Name() string { return "streaming-mock" }

func (m *streamingMockProvider) Stream(_ context.Context, _ []llm.Message) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 2)
	if m.text != "" {
		ch <- llm.StreamChunk{Text: m.text}
	}
	ch <- llm.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *streamingMockProvider) TestConnection(_ context.Context) error { return nil }

// readAuditLog reads and returns the contents of the audit log file in tmpDir.
func readAuditLog(t *testing.T, logDir string) string {
	t.Helper()
	entries, err := os.ReadDir(logDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("audit log directory empty or unreadable: %v", err)
	}
	data, err := os.ReadFile(logDir + "/" + entries[0].Name())
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}
	return string(data)
}

// TestSendMessageAuditUserMessage verifies that SendMessage writes a user_message audit entry
// containing only userInput (not terminalContext).
func TestSendMessageAuditUserMessage(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := audit.NewAuditLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewAuditLogger: %v", err)
	}

	svc := &LLMService{
		cfg:            Config{Provider: "mock"},
		activeProvider: &streamingMockProvider{text: "hello"},
		emitFn:         func(_ context.Context, _ string, _ ...interface{}) {},
	}
	svc.ctx = context.Background()
	svc.SetAuditLogger(logger, "test-session")

	err = svc.SendMessage("tmux:%3", "how to list ports", "terminal context here")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	// Wait for goroutine to complete.
	time.Sleep(300 * time.Millisecond)

	contents := readAuditLog(t, tmpDir)
	if !strings.Contains(contents, `"user_message"`) {
		t.Errorf("expected user_message event in audit log, got:\n%s", contents)
	}
	if !strings.Contains(contents, "how to list ports") {
		t.Errorf("expected userInput in audit log content, got:\n%s", contents)
	}
	if strings.Contains(contents, "terminal context here") {
		t.Errorf("expected terminalContext NOT in audit log, but it was found:\n%s", contents)
	}
}

// TestSendMessageAuditAIResponse verifies that SendMessage writes an ai_response audit entry
// with credential-filtered content after the stream completes.
func TestSendMessageAuditAIResponse(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := audit.NewAuditLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewAuditLogger: %v", err)
	}

	// Build a key that matches the anthropic-api-key pattern: sk-ant- + 95 chars
	credentialText := "your key is sk-ant-" + strings.Repeat("a", 95)

	svc := &LLMService{
		cfg:            Config{Provider: "mock"},
		activeProvider: &streamingMockProvider{text: credentialText},
		emitFn:         func(_ context.Context, _ string, _ ...interface{}) {},
	}
	svc.ctx = context.Background()
	svc.SetAuditLogger(logger, "test-session")

	err = svc.SendMessage("tmux:%3", "show key", "")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	// Wait for goroutine to complete.
	time.Sleep(300 * time.Millisecond)

	contents := readAuditLog(t, tmpDir)
	if !strings.Contains(contents, `"ai_response"`) {
		t.Errorf("expected ai_response event in audit log, got:\n%s", contents)
	}
	if !strings.Contains(contents, "[REDACTED:anthropic-api-key]") {
		t.Errorf("expected credential to be redacted in audit log, got:\n%s", contents)
	}
}

// TestGetAPIKeyStringFromEnclave verifies that getAPIKeyString returns the original key from an Enclave.
func TestGetAPIKeyStringFromEnclave(t *testing.T) {
	t.Cleanup(func() { memguard.Purge() })

	svc := &LLMService{}
	buf := memguard.NewBufferFromBytes([]byte("test-secret-key"))
	enc := buf.Seal()
	svc.SetAPIKeyEnclave("openai", enc)

	result := svc.getAPIKeyString("openai")
	if result != "test-secret-key" {
		t.Errorf("expected 'test-secret-key', got %q", result)
	}
}

// TestGetAPIKeyStringNilEnclave verifies that getAPIKeyString returns "" when no Enclave is set.
func TestGetAPIKeyStringNilEnclave(t *testing.T) {
	svc := &LLMService{}
	result := svc.getAPIKeyString("openai")
	if result != "" {
		t.Errorf("expected empty string with nil enclaves map, got %q", result)
	}
}

// TestAuditLoggerNilNoOpLLMService verifies that SendMessage works without panic when auditLogger is nil.
func TestAuditLoggerNilNoOpLLMService(t *testing.T) {
	svc := &LLMService{
		cfg:            Config{Provider: "mock"},
		activeProvider: &streamingMockProvider{text: "hello"},
		emitFn:         func(_ context.Context, _ string, _ ...interface{}) {},
	}
	svc.ctx = context.Background()
	// auditLogger deliberately not set (nil)

	err := svc.SendMessage("tab-1", "test", "")
	if err != nil {
		t.Errorf("SendMessage with nil auditLogger should not return error, got: %v", err)
	}
	// Wait for goroutine
	time.Sleep(200 * time.Millisecond)
}
