package services

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"pairadmin/services/keychain"
	"pairadmin/services/llm"

	"github.com/99designs/keyring"
)

// --- Test helpers ---

// inMemoryKeyring is a simple in-memory keyring for tests.
type inMemoryKeyring struct {
	items map[string]keyring.Item
}

func newInMemoryKeyring() *inMemoryKeyring {
	return &inMemoryKeyring{items: make(map[string]keyring.Item)}
}

func (k *inMemoryKeyring) Get(key string) (keyring.Item, error) {
	item, ok := k.items[key]
	if !ok {
		return keyring.Item{}, keyring.ErrKeyNotFound
	}
	return item, nil
}

func (k *inMemoryKeyring) GetMetadata(key string) (keyring.Metadata, error) {
	return keyring.Metadata{}, nil
}

func (k *inMemoryKeyring) Set(item keyring.Item) error {
	k.items[item.Key] = item
	return nil
}

func (k *inMemoryKeyring) Remove(key string) error {
	delete(k.items, key)
	return nil
}

func (k *inMemoryKeyring) Keys() ([]string, error) {
	keys := make([]string, 0, len(k.items))
	for k := range k.items {
		keys = append(keys, k)
	}
	return keys, nil
}

// makeTestKeychainClient returns a keychain.Client backed by an in-memory keyring.
func makeTestKeychainClient(mem *inMemoryKeyring) *keychain.Client {
	return keychain.NewWithOpenFunc(func(_ keyring.Config) (keyring.Keyring, error) {
		return mem, nil
	})
}

// mockProvider is a test double implementing llm.Provider.
type mockProvider struct {
	name    string
	connErr error
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Stream(_ context.Context, _ []llm.Message) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockProvider) TestConnection(_ context.Context) error {
	return m.connErr
}

// --- Tests ---

// TestSettingsService_GetSettings returns current AppConfig without error.
func TestSettingsService_GetSettings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))

	cfg, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("GetSettings() returned nil config")
	}
}

// TestSettingsService_SaveSettings persists config and GetSettings reflects changes.
func TestSettingsService_SaveSettings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	// Use nil emitFn to avoid calling Wails runtime.EventsEmit with a non-Wails context.
	svc.emitFn = nil
	svc.ctx = context.Background()

	cfg, _ := svc.GetSettings()
	cfg.Provider = "openai"
	cfg.Model = "gpt-4"

	if err := svc.SaveSettings(cfg); err != nil {
		t.Fatalf("SaveSettings() unexpected error: %v", err)
	}

	loaded, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() after save unexpected error: %v", err)
	}
	if loaded.Provider != "openai" {
		t.Errorf("Provider: expected 'openai', got %q", loaded.Provider)
	}
	if loaded.Model != "gpt-4" {
		t.Errorf("Model: expected 'gpt-4', got %q", loaded.Model)
	}
}

// TestSettingsService_GetAPIKeyStatus_Stored returns "stored" when key exists.
func TestSettingsService_GetAPIKeyStatus_Stored(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	mem.items["openai"] = keyring.Item{Key: "openai", Data: []byte("sk-test-key")}

	svc := NewSettingsService(makeTestKeychainClient(mem))

	status, err := svc.GetAPIKeyStatus("openai")
	if err != nil {
		t.Fatalf("GetAPIKeyStatus() unexpected error: %v", err)
	}
	if status != "stored" {
		t.Errorf("GetAPIKeyStatus() expected 'stored', got %q", status)
	}
}

// TestSettingsService_GetAPIKeyStatus_NotStored returns "" when key absent.
func TestSettingsService_GetAPIKeyStatus_NotStored(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))

	status, err := svc.GetAPIKeyStatus("openai")
	if err != nil {
		t.Fatalf("GetAPIKeyStatus() unexpected error: %v", err)
	}
	if status != "" {
		t.Errorf("GetAPIKeyStatus() expected '', got %q", status)
	}
}

// TestSettingsService_SaveAPIKey writes key to keychain.
func TestSettingsService_SaveAPIKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))

	if err := svc.SaveAPIKey("openai", "sk-secret"); err != nil {
		t.Fatalf("SaveAPIKey() unexpected error: %v", err)
	}

	item, ok := mem.items["openai"]
	if !ok {
		t.Fatal("expected key 'openai' to be stored in keychain")
	}
	if string(item.Data) != "sk-secret" {
		t.Errorf("expected stored data 'sk-secret', got %q", string(item.Data))
	}
}

// TestSettingsService_SaveAPIKey_EmptyRemoves removes key when empty string passed.
func TestSettingsService_SaveAPIKey_EmptyRemoves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	mem.items["openai"] = keyring.Item{Key: "openai", Data: []byte("sk-old-key")}

	svc := NewSettingsService(makeTestKeychainClient(mem))

	if err := svc.SaveAPIKey("openai", ""); err != nil {
		t.Fatalf("SaveAPIKey('', '') unexpected error: %v", err)
	}

	if _, ok := mem.items["openai"]; ok {
		t.Error("expected key 'openai' to be removed from keychain")
	}
}

// TestSettingsService_TestConnection_Success returns "Connected" for working provider.
func TestSettingsService_TestConnection_Success(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.ctx = context.Background()

	// Swap buildProvider for test with a mock that succeeds.
	orig := buildProviderFn
	defer func() { buildProviderFn = orig }()
	buildProviderFn = func(_ Config, _ func(string) string) llm.Provider {
		return &mockProvider{name: "mock", connErr: nil}
	}

	result, err := svc.TestConnection("mock", "mock-model")
	if err != nil {
		t.Fatalf("TestConnection() unexpected error: %v", err)
	}
	if result != "Connected" {
		t.Errorf("TestConnection() expected 'Connected', got %q", result)
	}
}

// TestSettingsService_TestConnection_Failure returns error for failing provider.
func TestSettingsService_TestConnection_Failure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.ctx = context.Background()

	// Swap buildProvider for test with a mock that fails.
	orig := buildProviderFn
	defer func() { buildProviderFn = orig }()
	connErr := errors.New("connection refused")
	buildProviderFn = func(_ Config, _ func(string) string) llm.Provider {
		return &mockProvider{name: "mock", connErr: connErr}
	}

	_, err := svc.TestConnection("mock", "mock-model")
	if err == nil {
		t.Fatal("TestConnection() expected error for failing provider, got nil")
	}
}

// TestSettingsService_TestConnection_NilProvider returns error for nil provider.
func TestSettingsService_TestConnection_NilProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.ctx = context.Background()

	// Swap buildProvider for test with a function that returns nil.
	orig := buildProviderFn
	defer func() { buildProviderFn = orig }()
	buildProviderFn = func(_ Config, _ func(string) string) llm.Provider {
		return nil
	}

	_, err := svc.TestConnection("unknown-provider", "")
	if err == nil {
		t.Fatal("TestConnection() expected error for nil provider, got nil")
	}
}

// --- SetModel tests ---

// TestSettingsService_SetModel_OpenAI saves provider/model for "openai:gpt-4".
func TestSettingsService_SetModel_OpenAI(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.emitFn = nil
	svc.ctx = context.Background()

	result, err := svc.SetModel("openai:gpt-4")
	if err != nil {
		t.Fatalf("SetModel() unexpected error: %v", err)
	}
	if result != "Model set to openai:gpt-4" {
		t.Errorf("SetModel() expected 'Model set to openai:gpt-4', got %q", result)
	}

	cfg, _ := svc.GetSettings()
	if cfg.Provider != "openai" {
		t.Errorf("Provider: expected 'openai', got %q", cfg.Provider)
	}
	if cfg.Model != "gpt-4" {
		t.Errorf("Model: expected 'gpt-4', got %q", cfg.Model)
	}
}

// TestSettingsService_SetModel_Ollama saves provider/model for "ollama:llama3".
func TestSettingsService_SetModel_Ollama(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.emitFn = nil
	svc.ctx = context.Background()

	result, err := svc.SetModel("ollama:llama3")
	if err != nil {
		t.Fatalf("SetModel() unexpected error: %v", err)
	}
	if result != "Model set to ollama:llama3" {
		t.Errorf("SetModel() expected 'Model set to ollama:llama3', got %q", result)
	}

	cfg, _ := svc.GetSettings()
	if cfg.Provider != "ollama" {
		t.Errorf("Provider: expected 'ollama', got %q", cfg.Provider)
	}
	if cfg.Model != "llama3" {
		t.Errorf("Model: expected 'llama3', got %q", cfg.Model)
	}
}

// TestSettingsService_SetModel_InvalidFormat returns error when no colon.
func TestSettingsService_SetModel_InvalidFormat(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.emitFn = nil

	_, err := svc.SetModel("invalid")
	if err == nil {
		t.Fatal("SetModel('invalid') expected error, got nil")
	}
}

// --- SetContextLines tests ---

// TestSettingsService_SetContextLines_Valid saves ContextLines=300.
func TestSettingsService_SetContextLines_Valid(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.emitFn = nil
	svc.ctx = context.Background()

	result, err := svc.SetContextLines(300)
	if err != nil {
		t.Fatalf("SetContextLines() unexpected error: %v", err)
	}
	if result != "Context set to 300 lines" {
		t.Errorf("SetContextLines() expected 'Context set to 300 lines', got %q", result)
	}

	cfg, _ := svc.GetSettings()
	if cfg.ContextLines != 300 {
		t.Errorf("ContextLines: expected 300, got %d", cfg.ContextLines)
	}
}

// TestSettingsService_SetContextLines_Zero returns error for zero.
func TestSettingsService_SetContextLines_Zero(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))

	_, err := svc.SetContextLines(0)
	if err == nil {
		t.Fatal("SetContextLines(0) expected error, got nil")
	}
}

// TestSettingsService_SetContextLines_Negative returns error for negative.
func TestSettingsService_SetContextLines_Negative(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))

	_, err := svc.SetContextLines(-1)
	if err == nil {
		t.Fatal("SetContextLines(-1) expected error, got nil")
	}
}

// --- ForceRefresh tests ---

// mockCaptureManager is a test double for the captureManagerForceCapture interface.
type mockCaptureManager struct {
	forceCaptureCount int
}

func (m *mockCaptureManager) ForceCapture() {
	m.forceCaptureCount++
}

// TestSettingsService_ForceRefresh_WithManager calls ForceCapture and returns success.
func TestSettingsService_ForceRefresh_WithManager(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.emitFn = nil

	mock := &mockCaptureManager{}
	svc.SetCaptureManager(mock)

	result, err := svc.ForceRefresh()
	if err != nil {
		t.Fatalf("ForceRefresh() unexpected error: %v", err)
	}
	if result != "Terminal content refreshed" {
		t.Errorf("ForceRefresh() expected 'Terminal content refreshed', got %q", result)
	}
	if mock.forceCaptureCount != 1 {
		t.Errorf("ForceCapture() expected 1 call, got %d", mock.forceCaptureCount)
	}
}

// TestSettingsService_ForceRefresh_NoManager returns error when captureManager is nil.
func TestSettingsService_ForceRefresh_NoManager(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.emitFn = nil

	_, err := svc.ForceRefresh()
	if err == nil {
		t.Fatal("ForceRefresh() with nil manager expected error, got nil")
	}
}

// --- ExportChat tests ---

// TestSettingsService_ExportChat_JSON writes JSON file and returns path.
func TestSettingsService_ExportChat_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.emitFn = nil

	msgs := []ExportMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
	}

	result, err := svc.ExportChat("tab1", "json", msgs)
	if err != nil {
		t.Fatalf("ExportChat() unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("ExportChat() returned empty path")
	}

	// Verify file exists and contains JSON
	data, err := os.ReadFile(result)
	if err != nil {
		t.Fatalf("ExportChat() file not found at %q: %v", result, err)
	}
	if len(data) == 0 {
		t.Error("ExportChat() wrote empty file")
	}
}

// TestSettingsService_ExportChat_TXT writes plain text file and returns path.
func TestSettingsService_ExportChat_TXT(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.emitFn = nil

	msgs := []ExportMessage{
		{Role: "user", Content: "hello"},
	}

	result, err := svc.ExportChat("tab1", "txt", msgs)
	if err != nil {
		t.Fatalf("ExportChat() unexpected error: %v", err)
	}

	data, err := os.ReadFile(result)
	if err != nil {
		t.Fatalf("ExportChat() txt file not found at %q: %v", result, err)
	}
	content := string(data)
	if !strings.Contains(content, "[user]:") {
		t.Errorf("ExportChat() txt missing '[user]:' in %q", content)
	}
}

// --- RenameTab tests ---

// TestSettingsService_RenameTab emits terminal:rename event and returns success message.
func TestSettingsService_RenameTab(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.ctx = context.Background()

	var emittedEvent string
	var emittedData interface{}
	svc.emitFn = func(_ context.Context, event string, data ...interface{}) {
		emittedEvent = event
		if len(data) > 0 {
			emittedData = data[0]
		}
	}

	result, err := svc.RenameTab("tab1", "myterm")
	if err != nil {
		t.Fatalf("RenameTab() unexpected error: %v", err)
	}
	if result != "Tab renamed to myterm" {
		t.Errorf("RenameTab() expected 'Tab renamed to myterm', got %q", result)
	}
	if emittedEvent != "terminal:rename" {
		t.Errorf("RenameTab() expected event 'terminal:rename', got %q", emittedEvent)
	}
	if emittedData == nil {
		t.Error("RenameTab() expected emitted data, got nil")
	}
}
