package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strings"
	"testing"

	"pairadmin/services/config"
	"pairadmin/services/keychain"
	"pairadmin/services/llm"
	"pairadmin/services/version"

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
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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

// TestSettingsService_SaveSettings_PreservesRemoteHosts is a regression test for a bug
// where every Settings-dialog tab (LLM Config, Prompts, Terminals, Hotkeys, Appearance)
// sends a partial AppConfig — e.g. LLMConfigTab sends only
// {Provider, Model, OllamaHost, LMStudioHost} — and SaveSettings previously wrote that
// partial struct straight to disk, silently wiping RemoteHosts (all saved remote
// terminal connections) back to empty on every unrelated settings save.
func TestSettingsService_SaveSettings_PreservesRemoteHosts(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.emitFn = nil
	svc.ctx = context.Background()

	remoteSvc := NewRemoteService(makeTestKeychainClient(mem))
	saved, err := remoteSvc.SaveRemoteHost(config.RemoteHost{Kind: "ssh", Host: "10.0.1.5", Username: "ubuntu"}, "hunter2", "")
	if err != nil {
		t.Fatalf("SaveRemoteHost() unexpected error: %v", err)
	}

	// Simulate a settings tab (e.g. LLMConfigTab) sending only the handful of fields
	// it manages — RemoteHosts is deliberately left as its Go zero value (nil), exactly
	// as every real Settings tab component does today.
	partialCfg := &config.AppConfig{Provider: "openai", Model: "gpt-4"}
	if err := svc.SaveSettings(partialCfg); err != nil {
		t.Fatalf("SaveSettings() unexpected error: %v", err)
	}

	hosts, err := remoteSvc.ListRemoteHosts()
	if err != nil {
		t.Fatalf("ListRemoteHosts() unexpected error: %v", err)
	}
	if len(hosts) != 1 || hosts[0].ID != saved.ID {
		t.Errorf("expected saved remote host to survive an unrelated SaveSettings call, got %+v", hosts)
	}

	loaded, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() unexpected error: %v", err)
	}
	if loaded.Provider != "openai" || loaded.Model != "gpt-4" {
		t.Errorf("expected the partial save's own fields to still apply, got Provider=%q Model=%q", loaded.Provider, loaded.Model)
	}
}

// TestSettingsService_SaveSettings_PreservesPinnedCommands mirrors
// TestSettingsService_SaveSettings_PreservesRemoteHosts for PinnedCommands:
// an unrelated Settings-tab save must never wipe previously-saved pinned commands.
func TestSettingsService_SaveSettings_PreservesPinnedCommands(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	svc := NewSettingsService(makeTestKeychainClient(newInMemoryKeyring()))
	svc.emitFn = nil
	svc.ctx = context.Background()

	if err := svc.SavePinnedCommands([]config.PinnedCommand{
		{Command: "tmux set -g mouse on"},
	}); err != nil {
		t.Fatalf("SavePinnedCommands() unexpected error: %v", err)
	}

	// Simulate a settings tab sending only the fields it manages — PinnedCommands
	// deliberately left as its Go zero value (nil), exactly as every real
	// Settings tab component does today.
	partialCfg := &config.AppConfig{Provider: "openai", Model: "gpt-4"}
	if err := svc.SaveSettings(partialCfg); err != nil {
		t.Fatalf("SaveSettings() unexpected error: %v", err)
	}

	loaded, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() unexpected error: %v", err)
	}
	if len(loaded.PinnedCommands) != 1 || loaded.PinnedCommands[0].Command != "tmux set -g mouse on" {
		t.Errorf("expected saved pinned command to survive an unrelated SaveSettings call, got %+v", loaded.PinnedCommands)
	}
}

// TestSettingsService_SavePinnedCommands_RoundTrip verifies pinned commands
// can be saved and read back via GetSettings, including replacing a previous save.
func TestSettingsService_SavePinnedCommands_RoundTrip(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	svc := NewSettingsService(makeTestKeychainClient(newInMemoryKeyring()))
	svc.emitFn = nil
	svc.ctx = context.Background()

	if err := svc.SavePinnedCommands([]config.PinnedCommand{
		{Command: "df -h", OriginalQuestion: "how much space is left?"},
		{Command: "tmux set -g mouse on"},
	}); err != nil {
		t.Fatalf("SavePinnedCommands() unexpected error: %v", err)
	}

	loaded, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() unexpected error: %v", err)
	}
	if len(loaded.PinnedCommands) != 2 {
		t.Fatalf("expected 2 pinned commands, got %d", len(loaded.PinnedCommands))
	}
	if loaded.PinnedCommands[0].Command != "df -h" || loaded.PinnedCommands[0].OriginalQuestion != "how much space is left?" {
		t.Errorf("PinnedCommands[0] mismatch: %+v", loaded.PinnedCommands[0])
	}

	// A second save fully replaces the first, rather than appending.
	if err := svc.SavePinnedCommands([]config.PinnedCommand{
		{Command: "kubectl get pods"},
	}); err != nil {
		t.Fatalf("second SavePinnedCommands() unexpected error: %v", err)
	}
	reloaded, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() unexpected error: %v", err)
	}
	if len(reloaded.PinnedCommands) != 1 || reloaded.PinnedCommands[0].Command != "kubectl get pods" {
		t.Errorf("expected second save to replace the first, got %+v", reloaded.PinnedCommands)
	}
}

// TestSettingsService_GetAPIKeyStatus_Stored returns "stored" when key exists.
func TestSettingsService_GetAPIKeyStatus_Stored(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.ctx = context.Background()

	// Swap buildProvider for test with a mock that succeeds.
	orig := buildProviderFn
	defer func() { buildProviderFn = orig }()
	buildProviderFn = func(_ Config, _ func(string) string) llm.Provider {
		return &mockProvider{name: "mock", connErr: nil}
	}

	result, err := svc.TestConnection("mock", "mock-model", "", "")
	if err != nil {
		t.Fatalf("TestConnection() unexpected error: %v", err)
	}
	if result != "Connected" {
		t.Errorf("TestConnection() expected 'Connected', got %q", result)
	}
}

// TestSettingsService_TestConnection_InjectsOllamaKeyFromKeychain verifies
// that TestConnection resolves the "ollama" keychain entry into the keyFn
// handed to buildProvider, so authenticated remote Ollama servers (which carry
// an optional bearer token) still receive their key.
//
// Mutation check: restoring the pre-PA-TC per-provider switch — or passing a
// nil keyFn — makes keyFn("ollama") return "" and this test fails. It asserts
// on the keyFn argument rather than cfg.OllamaKey because the key no longer
// travels through per-provider Config fields (they only dropped it for the six
// providers the switch did not list).
func TestSettingsService_TestConnection_InjectsOllamaKeyFromKeychain(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.ctx = context.Background()

	// The UI stores the Ollama key under the plain "ollama" keychain entry.
	if err := mem.Set(keyring.Item{Key: "ollama", Data: []byte("sk-remote-ollama")}); err != nil {
		t.Fatalf("keyring Set(ollama): %v", err)
	}

	orig := buildProviderFn
	defer func() { buildProviderFn = orig }()
	buildProviderFn = func(_ Config, keyFn func(string) string) llm.Provider {
		if keyFn == nil {
			t.Error("buildProvider received a nil keyFn, so the keychain key is unreachable")
		} else if got := keyFn("ollama"); got != "sk-remote-ollama" {
			t.Errorf("expected keyFn(\"ollama\") to return the keychain key, got %q", got)
		}
		return &mockProvider{name: "ollama", connErr: nil}
	}

	if _, err := svc.TestConnection("ollama", "llama3", "", ""); err != nil {
		t.Fatalf("TestConnection() unexpected error: %v", err)
	}
}

// TestSettingsService_TestConnection_Failure returns error for failing provider.
func TestSettingsService_TestConnection_Failure(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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

	_, err := svc.TestConnection("mock", "mock-model", "", "")
	if err == nil {
		t.Fatal("TestConnection() expected error for failing provider, got nil")
	}
}

// TestSettingsService_TestConnection_NilProvider returns error for nil provider.
func TestSettingsService_TestConnection_NilProvider(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.ctx = context.Background()

	// Swap buildProvider for test with a function that returns nil.
	orig := buildProviderFn
	defer func() { buildProviderFn = orig }()
	buildProviderFn = func(_ Config, _ func(string) string) llm.Provider {
		return nil
	}

	_, err := svc.TestConnection("unknown-provider", "", "", "")
	if err == nil {
		t.Fatal("TestConnection() expected error for nil provider, got nil")
	}
}

// --- TestConnection: typed key, provider coverage, redaction (PA-TC) ---

// captureKeyFn installs a buildProviderFn that records the keyFn it is handed
// (and the Config, for tests that assert on hosts) and returns a provider whose
// TestConnection succeeds.
func captureKeyFn(t *testing.T, seen *string) {
	t.Helper()
	orig := buildProviderFn
	t.Cleanup(func() { buildProviderFn = orig })
	buildProviderFn = func(_ Config, keyFn func(string) string) llm.Provider {
		if keyFn == nil {
			t.Error("buildProvider received a nil keyFn — the resolved key is unreachable")
			return &mockProvider{name: "mock"}
		}
		*seen = keyFn("probe")
		return &mockProvider{name: "mock"}
	}
}

// TestSettingsService_TestConnection_TypedKeyWins pins defect #1: the key the
// user typed in the API Key field is the key that gets tested, not the stored
// one. Testing the stored key while a different key sits in the field is what
// made a good OpenRouter key report "unauthorized" in UAT.
//
// Mutation check: ignoring the typedKey parameter (or restoring the keychain
// Get as the unconditional source) makes keyFn return "sk-stored" and this
// test fails.
func TestSettingsService_TestConnection_TypedKeyWins(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.ctx = context.Background()
	if err := mem.Set(keyring.Item{Key: "openai", Data: []byte("sk-stored")}); err != nil {
		t.Fatalf("keyring Set(openai): %v", err)
	}

	var seen string
	captureKeyFn(t, &seen)

	if _, err := svc.TestConnection("openai", "gpt-4o-mini", "", "sk-typed"); err != nil {
		t.Fatalf("TestConnection() unexpected error: %v", err)
	}
	if seen != "sk-typed" {
		t.Errorf("expected the typed key to be tested, got %q", seen)
	}
}

// TestSettingsService_TestConnection_EmptyTypedKeyFallsBackToStored pins the
// other half of defect #1's contract, and the behavior the app-mount probe
// depends on: an empty field tests the saved key.
//
// Mutation check: dropping the `if apiKey == ""` fallback (so an empty typed
// key is passed through as "") makes this yield "" instead of "sk-stored".
func TestSettingsService_TestConnection_EmptyTypedKeyFallsBackToStored(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.ctx = context.Background()
	if err := mem.Set(keyring.Item{Key: "openai", Data: []byte("sk-stored")}); err != nil {
		t.Fatalf("keyring Set(openai): %v", err)
	}

	var seen string
	captureKeyFn(t, &seen)

	if _, err := svc.TestConnection("openai", "gpt-4o-mini", "", ""); err != nil {
		t.Fatalf("TestConnection() unexpected error: %v", err)
	}
	if seen != "sk-stored" {
		t.Errorf("expected the stored key to be tested, got %q", seen)
	}
}

// TestSettingsService_TestConnection_NeverPersistsTheTypedKey pins the security
// property that makes forwarding the typed key safe: Test must not write it to
// the keychain. Persisting is Save's job, on an explicit click.
//
// Mutation check: adding any keychainClient.Set (or SaveAPIKey) call to
// TestConnection makes the stored value become "sk-typed" and this test fails.
func TestSettingsService_TestConnection_NeverPersistsTheTypedKey(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.ctx = context.Background()
	if err := mem.Set(keyring.Item{Key: "openai", Data: []byte("sk-stored")}); err != nil {
		t.Fatalf("keyring Set(openai): %v", err)
	}

	var seen string
	captureKeyFn(t, &seen)

	if _, err := svc.TestConnection("openai", "gpt-4o-mini", "", "sk-typed"); err != nil {
		t.Fatalf("TestConnection() unexpected error: %v", err)
	}

	item, ok := mem.items["openai"]
	if !ok {
		t.Fatal("expected the originally stored key to still be present")
	}
	if string(item.Data) != "sk-stored" {
		t.Errorf("TestConnection persisted the typed key: expected %q, got %q", "sk-stored", string(item.Data))
	}
}

// TestSettingsService_TestConnection_EveryKeyedProviderGetsTheKey pins defect
// #2. The per-provider switch only injected the keychain key for openai,
// anthropic, openrouter and ollama; the other six were handed a nil keyFn and
// fell through to os.Getenv, so Test Connection failed for them whenever the
// matching env var was unset — even though chat worked off the in-memory key.
// Invisible until someone tried one of those six.
//
// Each env var is explicitly cleared, so a pass cannot come from the fallback.
//
// Mutation check: restoring the four-provider switch makes the six new rows
// fail (nil keyFn, or a keyFn that returns ""). This is the test that pins #2.
func TestSettingsService_TestConnection_EveryKeyedProviderGetsTheKey(t *testing.T) {
	cases := []struct{ provider, envKey string }{
		{"openai", "OPENAI_API_KEY"},
		{"anthropic", "ANTHROPIC_API_KEY"},
		{"openrouter", "OPENROUTER_API_KEY"},
		{"ollama", ""}, // local: no env key; the keychain key is an optional bearer token
		{"google", "GOOGLE_API_KEY"},
		{"deepseek", "DEEPSEEK_API_KEY"},
		{"xai", "XAI_API_KEY"},
		{"mistral", "MISTRAL_API_KEY"},
		{"groq", "GROQ_API_KEY"},
		{"glm", "ZAI_API_KEY"},
	}

	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			homeDir := t.TempDir()
			t.Setenv("HOME", homeDir)
			t.Setenv("USERPROFILE", homeDir)
			if tc.envKey != "" {
				t.Setenv(tc.envKey, "") // cleared: the stored key is the only source
			}

			mem := newInMemoryKeyring()
			svc := NewSettingsService(makeTestKeychainClient(mem))
			svc.ctx = context.Background()

			want := "sk-stored-" + tc.provider
			if err := mem.Set(keyring.Item{Key: tc.provider, Data: []byte(want)}); err != nil {
				t.Fatalf("keyring Set(%s): %v", tc.provider, err)
			}

			var seen string
			orig := buildProviderFn
			t.Cleanup(func() { buildProviderFn = orig })
			buildProviderFn = func(_ Config, keyFn func(string) string) llm.Provider {
				if keyFn == nil {
					t.Errorf("%s: buildProvider received a nil keyFn — the stored key was dropped", tc.provider)
					return &mockProvider{name: tc.provider}
				}
				seen = keyFn(tc.provider)
				return &mockProvider{name: tc.provider}
			}

			if _, err := svc.TestConnection(tc.provider, "some-model", "", ""); err != nil {
				t.Fatalf("%s: TestConnection() unexpected error: %v", tc.provider, err)
			}
			if seen != want {
				t.Errorf("%s: expected the stored key %q to reach keyFn, got %q", tc.provider, want, seen)
			}
		})
	}
}

// TestSettingsService_TestConnection_RedactsKeyFromErrors pins the §3/§6
// guarantee that a key can never reach the UI inside an error string. The mock
// echoes the key back the way a badly-worded provider error could.
//
// Mutation check: returning err unwrapped (no redactAPIKey) makes this test
// find the raw key in the message.
func TestSettingsService_TestConnection_RedactsKeyFromErrors(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.ctx = context.Background()

	const typed = "sk-super-secret-typed-key"

	orig := buildProviderFn
	defer func() { buildProviderFn = orig }()
	buildProviderFn = func(_ Config, keyFn func(string) string) llm.Provider {
		// A provider error that carelessly includes the credential.
		return &mockProvider{
			name:    "openai",
			connErr: fmt.Errorf("401 unauthorized: the key %s was rejected", keyFn("openai")),
		}
	}

	_, err := svc.TestConnection("openai", "gpt-4o-mini", "", typed)
	if err == nil {
		t.Fatal("expected an error from the failing provider")
	}
	if strings.Contains(err.Error(), typed) {
		t.Errorf("the API key leaked into the error string: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "[redacted]") {
		t.Errorf("expected the key to be replaced with [redacted], got %q", err.Error())
	}
}

// TestSettingsService_TestConnection_KeyedProviderWithNoKeyAnywhere pins the
// "No API key saved for <provider>" message: a keyed provider with no key
// typed, none stored and no env var cannot authenticate, so it should say so
// instead of surfacing a provider-specific 401.
//
// Mutation check: removing the polish branch makes this return the mock
// provider's success/"Connected" instead of the message.
func TestSettingsService_TestConnection_KeyedProviderWithNoKeyAnywhere(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("DEEPSEEK_API_KEY", "")

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.ctx = context.Background()

	orig := buildProviderFn
	defer func() { buildProviderFn = orig }()
	buildProviderFn = func(_ Config, _ func(string) string) llm.Provider {
		t.Error("no provider should be built when there is no key to test with")
		return &mockProvider{name: "deepseek"}
	}

	_, err := svc.TestConnection("deepseek", "deepseek-v4-flash", "", "")
	if err == nil {
		t.Fatal("expected an error when no key is available")
	}
	if !strings.Contains(err.Error(), "No API key saved for deepseek") {
		t.Errorf("expected the actionable no-key message, got %q", err.Error())
	}
}

// TestSettingsService_TestConnection_EmptyKeyKeepsEnvFallback pins that the
// no-key polish does NOT swallow the documented environment fallback: with
// DEEPSEEK_API_KEY set, an empty field must still reach the provider so
// resolveKey can pick the env var up. Without the os.Getenv guard on the
// polish branch, this would fail closed and break a configuration that
// previously worked.
//
// Mutation check: dropping the `os.Getenv(cat.EnvKey) == ""` condition from the
// polish branch makes this return the no-key error instead of connecting.
func TestSettingsService_TestConnection_EmptyKeyKeepsEnvFallback(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("DEEPSEEK_API_KEY", "sk-from-env")

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.ctx = context.Background()

	var seen string
	var built bool
	orig := buildProviderFn
	defer func() { buildProviderFn = orig }()
	buildProviderFn = func(_ Config, keyFn func(string) string) llm.Provider {
		built = true
		if keyFn != nil {
			seen = keyFn("deepseek")
		}
		return &mockProvider{name: "deepseek"}
	}

	if _, err := svc.TestConnection("deepseek", "deepseek-v4-flash", "", ""); err != nil {
		t.Fatalf("env-var fallback must be unchanged: %v", err)
	}
	if !built {
		t.Fatal("expected a provider to be built so the env fallback can resolve the key")
	}
	if seen != "" {
		t.Errorf("expected no key from keyFn (the env var supplies it), got %q", seen)
	}
}

// TestSettingsService_TestConnection_KeylessProvidersStillTest pins that the
// no-key message is gated on catalog NeedsKey, so the local providers — which
// legitimately need no key — still test successfully with an empty field.
//
// Mutation check: keying the polish branch on anything broader than NeedsKey
// (e.g. "not ollama") makes lmstudio fail here.
func TestSettingsService_TestConnection_KeylessProvidersStillTest(t *testing.T) {
	for _, provider := range []string{"ollama", "lmstudio"} {
		t.Run(provider, func(t *testing.T) {
			homeDir := t.TempDir()
			t.Setenv("HOME", homeDir)
			t.Setenv("USERPROFILE", homeDir)

			mem := newInMemoryKeyring()
			svc := NewSettingsService(makeTestKeychainClient(mem))
			svc.ctx = context.Background()

			orig := buildProviderFn
			t.Cleanup(func() { buildProviderFn = orig })
			buildProviderFn = func(_ Config, _ func(string) string) llm.Provider {
				return &mockProvider{name: provider}
			}

			result, err := svc.TestConnection(provider, "", "", "")
			if err != nil {
				t.Fatalf("%s: keyless test must still work, got error: %v", provider, err)
			}
			if result != "Connected" {
				t.Errorf("%s: expected 'Connected', got %q", provider, result)
			}
		})
	}
}

// --- SetModel tests ---

// TestSettingsService_SetModel_OpenAI saves provider/model for "openai:gpt-4".
func TestSettingsService_SetModel_OpenAI(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))

	_, err := svc.SetContextLines(0)
	if err == nil {
		t.Fatal("SetContextLines(0) expected error, got nil")
	}
}

// TestSettingsService_SetContextLines_Negative returns error for negative.
func TestSettingsService_SetContextLines_Negative(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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

// TestSettingsService_ForceRefresh_WithManager calls ForceCapture and returns success
// for each legacy CaptureManager-backed tab prefix.
func TestSettingsService_ForceRefresh_WithManager(t *testing.T) {
	for _, tabId := range []string{"tmux:%3", "atspi:42", "windows:1234"} {
		t.Run(tabId, func(t *testing.T) {
			homeDir := t.TempDir()
			t.Setenv("HOME", homeDir)
			t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

			mem := newInMemoryKeyring()
			svc := NewSettingsService(makeTestKeychainClient(mem))
			svc.emitFn = nil

			mock := &mockCaptureManager{}
			svc.SetCaptureManager(mock)

			result, err := svc.ForceRefresh(tabId)
			if err != nil {
				t.Fatalf("ForceRefresh(%q) unexpected error: %v", tabId, err)
			}
			if result != "Terminal content refreshed" {
				t.Errorf("ForceRefresh(%q) expected 'Terminal content refreshed', got %q", tabId, result)
			}
			if mock.forceCaptureCount != 1 {
				t.Errorf("ForceCapture() expected 1 call, got %d", mock.forceCaptureCount)
			}
		})
	}
}

// TestSettingsService_ForceRefresh_NoManager returns error when captureManager is nil,
// for a legacy-capture tab that would otherwise need one.
func TestSettingsService_ForceRefresh_NoManager(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.emitFn = nil

	_, err := svc.ForceRefresh("tmux:%3")
	if err == nil {
		t.Fatal("ForceRefresh() with nil manager expected error, got nil")
	}
}

// TestSettingsService_ForceRefresh_NativePTYTab_NoOpNoManagerNeeded is a
// regression test for PRE_INSTALLER_TASKS.md item 3: /refresh previously
// reported "Terminal content refreshed" for Local/SSH/WinRM tabs even though
// nothing was captured and nothing changed — misleading, since those tabs
// stream live via pty:output and have no capture step at all. Covers every
// native PTY tabId shape: bare (Local), "ssh:", and "winrm:".
func TestSettingsService_ForceRefresh_NativePTYTab_NoOpNoManagerNeeded(t *testing.T) {
	for _, tabId := range []string{"a1b2c3d4-local-uuid", "ssh:a1b2c3d4", "winrm:a1b2c3d4"} {
		t.Run(tabId, func(t *testing.T) {
			homeDir := t.TempDir()
			t.Setenv("HOME", homeDir)
			t.Setenv("USERPROFILE", homeDir)

			mem := newInMemoryKeyring()
			svc := NewSettingsService(makeTestKeychainClient(mem))
			svc.emitFn = nil
			// Deliberately no SetCaptureManager call — a native PTY tab must
			// not require one, since it never reaches ForceCapture().

			result, err := svc.ForceRefresh(tabId)
			if err != nil {
				t.Fatalf("ForceRefresh(%q) unexpected error: %v", tabId, err)
			}
			if result != "This terminal is already live — nothing to refresh." {
				t.Errorf("ForceRefresh(%q) = %q, want the native-PTY no-op message", tabId, result)
			}
		})
	}
}

// TestSettingsService_ForceRefresh_NativePTYTab_DoesNotCallCaptureManager
// verifies a native PTY tab's refresh doesn't trigger ForceCapture() even
// when a CaptureManager happens to be wired (it always is in the real app).
func TestSettingsService_ForceRefresh_NativePTYTab_DoesNotCallCaptureManager(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.emitFn = nil

	mock := &mockCaptureManager{}
	svc.SetCaptureManager(mock)

	if _, err := svc.ForceRefresh("ssh:a1b2c3d4"); err != nil {
		t.Fatalf("ForceRefresh() unexpected error: %v", err)
	}
	if mock.forceCaptureCount != 0 {
		t.Errorf("expected ForceCapture() not to be called for a native PTY tab, got %d calls", mock.forceCaptureCount)
	}
}

// --- ExportChat tests ---

// TestSettingsService_ExportChat_JSON writes JSON file and returns path.
func TestSettingsService_ExportChat_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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
	t.Setenv("USERPROFILE", tmpDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

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

// TestSettingsService_GetCurrentUsername verifies GetCurrentUsername resolves
// the logged-in OS account to a non-empty short/login name that matches the
// host's resolved username (os/user.Current().Username, or the USER/LOGNAME
// env fallback).
// Mutation check: stubbing the resolver to a constant empty string, or
// removing the os/user.Current() / env-fallback path, makes this test red
// (the returned value would be empty or not equal the host's resolved name).
func TestSettingsService_GetCurrentUsername(t *testing.T) {
	svc := NewSettingsService(nil)

	got, err := svc.GetCurrentUsername()
	if err != nil {
		t.Fatalf("GetCurrentUsername() unexpected error: %v", err)
	}
	if got == "" {
		t.Fatal("GetCurrentUsername() returned an empty username")
	}

	expected := ""
	if u, uerr := user.Current(); uerr == nil && u.Username != "" {
		expected = u.Username
	} else {
		for _, name := range []string{"USER", "LOGNAME"} {
			if v := os.Getenv(name); v != "" {
				expected = v
				break
			}
		}
	}
	if expected == "" {
		t.Skip("cannot determine expected username on this host")
	}
	if got != expected {
		t.Errorf("GetCurrentUsername() = %q, want %q", got, expected)
	}
}

// TestGetLLMCatalog_ReturnsAllElevenInCatalogOrder verifies GetLLMCatalog
// exposes every catalog provider, in catalog order, with the picker fields.
// Mutation check: dropping GetLLMCatalog's mapping loop (or trimming the
// provider list) makes this test fail — a stale/partial catalog would omit a
// provider from the Settings picker.
func TestGetLLMCatalog_ReturnsAllElevenInCatalogOrder(t *testing.T) {
	svc := NewSettingsService(nil)
	views := svc.GetLLMCatalog()
	if len(views) != 11 {
		t.Fatalf("expected 11 catalog providers, got %d", len(views))
	}
	if views[0].ID != "openai" || views[0].Name == "" || views[0].Adapter == "" {
		t.Errorf("first provider should be openai with a name+adapter, got %q/%q/%q",
			views[0].ID, views[0].Name, views[0].Adapter)
	}
	last := views[len(views)-1]
	if last.ID != "lmstudio" {
		t.Errorf("last provider should be lmstudio (catalog order), got %q", last.ID)
	}
	for i := 0; i < len(views); i++ {
		if views[i].ID == "" {
			t.Errorf("provider %d has an empty id", i)
		}
	}
}

// TestGetLLMCatalog_MapsModelsPinned verifies the model view carries the
// picker fields (id/name/context/reasoning/tool_call) and NEVER EnvKey/costs.
// Mutation check: eliding the model-mapping loop (or leaking cost/EnvKey
// fields into the view) makes this test fail — openai must expose its 5
// catalog models with their capability flags, nothing more.
func TestGetLLMCatalog_MapsModelsPinned(t *testing.T) {
	svc := NewSettingsService(nil)
	views := svc.GetLLMCatalog()
	openai := views[0]
	if len(openai.Models) != 5 {
		t.Fatalf("openai should expose 5 catalog models, got %d", len(openai.Models))
	}
	m := openai.Models[0]
	if m.ID != "gpt-5.6-luna" || m.Name != "gpt-5.6-luna" {
		t.Errorf("first openai model mismatch: %q/%q", m.ID, m.Name)
	}
	if !m.Reasoning || !m.ToolCall {
		t.Errorf("gpt-5.6-luna should flag Reasoning+ToolCall, got %v/%v", m.Reasoning, m.ToolCall)
	}
	// Context window is unknown (0) for gpt-5.6-luna in the catalog.
	if m.Context != 0 {
		t.Errorf("gpt-5.6-luna context should be 0 (unknown), got %d", m.Context)
	}
}

// TestGetLLMCatalog_LocalProvidersExposeNoModels verifies ollama/lmstudio
// surface an EMPTY models list (their models are discovered at runtime, not
// cataloged) — so the picker falls back to free-text for them.
func TestGetLLMCatalog_LocalProvidersExposeNoModels(t *testing.T) {
	svc := NewSettingsService(nil)
	views := svc.GetLLMCatalog()

	indexOf := func(id string) int {
		for i := 0; i < len(views); i++ {
			if views[i].ID == id {
				return i
			}
		}
		return -1
	}

	if oi := indexOf("ollama"); oi >= 0 && len(views[oi].Models) != 0 {
		t.Errorf("ollama should expose no catalog models, got %d", len(views[oi].Models))
	}
	if oi := indexOf("lmstudio"); oi >= 0 && len(views[oi].Models) != 0 {
		t.Errorf("lmstudio should expose no catalog models, got %d", len(views[oi].Models))
	}
}

func TestSettingsServiceGetVersion(t *testing.T) {
	// The About tab's Wails binding: SettingsService.GetVersion must delegate
	// to services/version (which release.yml overrides via -ldflags on every
	// tagged release). Mutation check: hardcoding a stale constant here (or
	// returning "") drifts from services/version.GetVersion and fails the
	// equality assertion.
	s := NewSettingsService(nil)
	got := s.GetVersion()
	if got == "" {
		t.Fatal("GetVersion() must never return an empty string")
	}
	if got != version.GetVersion() {
		t.Errorf("SettingsService.GetVersion() = %q, want services/version.GetVersion() = %q", got, version.GetVersion())
	}
	if version.GetVersion() != "dev" {
		t.Errorf("default must be %q (release builds override it via -ldflags), got %q", "dev", version.GetVersion())
	}
}
