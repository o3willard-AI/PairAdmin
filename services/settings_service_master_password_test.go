package services

import (
	"errors"
	"strings"
	"testing"

	"pairadmin/services/keychain"

	"github.com/99designs/keyring"
)

// failingKeyring is an in-memory keyring whose Get always fails with the
// given error — used to test LoadAPIKeys' per-provider error collection.
type failingKeyring struct {
	getErr error
}

func (f *failingKeyring) Get(string) (keyring.Item, error) {
	return keyring.Item{}, f.getErr
}
func (f *failingKeyring) GetMetadata(string) (keyring.Metadata, error) {
	return keyring.Metadata{}, nil
}
func (f *failingKeyring) Set(keyring.Item) error  { return nil }
func (f *failingKeyring) Remove(string) error     { return nil }
func (f *failingKeyring) Keys() ([]string, error) { return nil, nil }

// TestSettingsService_LoadAPIKeys_SealsStoredKeys covers the happy path:
// keys saved through the keychain are sealed into LLMService Enclaves by
// LoadAPIKeys, and an empty keychain is not an error.
func TestSettingsService_LoadAPIKeys_SealsStoredKeys(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	mem := newInMemoryKeyring()
	svc := NewSettingsService(makeTestKeychainClient(mem))
	svc.llmService = NewLLMService(Config{})

	// Empty keychain: no error, nothing sealed.
	if err := svc.LoadAPIKeys(); err != nil {
		t.Fatalf("LoadAPIKeys() on empty keychain: unexpected error: %v", err)
	}

	// Store two keys (openrouter deliberately left unset).
	if err := svc.SaveAPIKey("openai", "sk-openai-test"); err != nil {
		t.Fatalf("SaveAPIKey(openai): %v", err)
	}
	if err := svc.SaveAPIKey("anthropic", "sk-ant-test"); err != nil {
		t.Fatalf("SaveAPIKey(anthropic): %v", err)
	}

	if err := svc.LoadAPIKeys(); err != nil {
		t.Fatalf("LoadAPIKeys() unexpected error: %v", err)
	}
	if got := svc.llmService.getAPIKeyString("openai"); got != "sk-openai-test" {
		t.Errorf("openai enclave = %q, want %q", got, "sk-openai-test")
	}
	if got := svc.llmService.getAPIKeyString("anthropic"); got != "sk-ant-test" {
		t.Errorf("anthropic enclave = %q, want %q", got, "sk-ant-test")
	}
	if got := svc.llmService.getAPIKeyString("openrouter"); got != "" {
		t.Errorf("openrouter enclave = %q, want empty (key was never stored)", got)
	}
}

// TestSettingsService_LoadAPIKeys_CollectsErrors verifies that a keychain
// read failure for one provider does not stop the others, and that the
// joined error names the failing provider.
func TestSettingsService_LoadAPIKeys_CollectsErrors(t *testing.T) {
	svc := NewSettingsService(keychain.NewWithOpenFunc(func(keyring.Config) (keyring.Keyring, error) {
		return &failingKeyring{getErr: errors.New("keychain exploded")}, nil
	}))
	svc.llmService = NewLLMService(Config{})

	err := svc.LoadAPIKeys()
	if err == nil {
		t.Fatal("LoadAPIKeys() expected joined error when every Get fails, got nil")
	}
	for _, provider := range apiKeysProviders {
		if !strings.Contains(err.Error(), provider) {
			t.Errorf("joined error %q does not mention provider %q", err.Error(), provider)
		}
	}
}

// TestSettingsService_MasterPasswordWrappers exercises the thin delegation
// wrappers end-to-end against the real keychain.Client (real file backend,
// isolated HOME): first-set, verify (right and wrong), change, and the
// NeedsMasterPassword/HasMasterPassword probes.
func TestSettingsService_MasterPasswordWrappers(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	svc := NewSettingsService(keychain.NewWithOpenFunc(func(cfg keyring.Config) (keyring.Keyring, error) {
		// No OS backend in tests; allow the file backend through to the
		// real keyring.Open like the keychain package's own tests do.
		for _, b := range cfg.AllowedBackends {
			if b == keyring.FileBackend {
				return keyring.Open(cfg)
			}
		}
		return nil, keyring.ErrNoAvailImpl
	}))

	needs, err := svc.NeedsMasterPassword()
	if err != nil {
		t.Fatalf("NeedsMasterPassword(): %v", err)
	}
	if !needs {
		t.Fatal("NeedsMasterPassword() = false with no OS backend, want true")
	}
	if svc.HasMasterPassword() {
		t.Fatal("HasMasterPassword() = true before any set, want false")
	}

	if err := svc.SetMasterPassword("first-pw"); err != nil {
		t.Fatalf("SetMasterPassword(): %v", err)
	}
	if !svc.HasMasterPassword() {
		t.Fatal("HasMasterPassword() = false after SetMasterPassword, want true")
	}

	ok, err := svc.VerifyMasterPassword("wrong-pw")
	if err != nil {
		t.Fatalf("VerifyMasterPassword(wrong): %v", err)
	}
	if ok {
		t.Error("VerifyMasterPassword(wrong) = true, want false")
	}
	ok, err = svc.VerifyMasterPassword("first-pw")
	if err != nil {
		t.Fatalf("VerifyMasterPassword(correct): %v", err)
	}
	if !ok {
		t.Error("VerifyMasterPassword(correct) = false, want true")
	}

	// Wrong current password must be rejected without re-encrypting.
	if err := svc.ChangeMasterPassword("not-it", "second-pw"); !errors.Is(err, keychain.ErrWrongMasterPassword) {
		t.Errorf("ChangeMasterPassword(wrong old) err = %v, want ErrWrongMasterPassword", err)
	}
	if err := svc.ChangeMasterPassword("first-pw", "second-pw"); err != nil {
		t.Fatalf("ChangeMasterPassword(): %v", err)
	}
	ok, err = svc.VerifyMasterPassword("second-pw")
	if err != nil || !ok {
		t.Errorf("VerifyMasterPassword(second-pw) = %v, %v; want true, nil", ok, err)
	}
}
