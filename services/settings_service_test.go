package services

import (
	"context"
	"errors"
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
	buildProviderFn = func(_ Config) llm.Provider {
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
	buildProviderFn = func(_ Config) llm.Provider {
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
	buildProviderFn = func(_ Config) llm.Provider {
		return nil
	}

	_, err := svc.TestConnection("unknown-provider", "")
	if err == nil {
		t.Fatal("TestConnection() expected error for nil provider, got nil")
	}
}
