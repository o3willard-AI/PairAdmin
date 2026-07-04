package keychain

import (
	"testing"

	"github.com/99designs/keyring"
)

// fakeKeyring is a test double implementing keyring.Keyring backed by a map.
type fakeKeyring struct {
	items map[string]keyring.Item
}

func newFakeKeyring() *fakeKeyring {
	return &fakeKeyring{items: make(map[string]keyring.Item)}
}

func (f *fakeKeyring) Get(key string) (keyring.Item, error) {
	item, ok := f.items[key]
	if !ok {
		return keyring.Item{}, keyring.ErrKeyNotFound
	}
	return item, nil
}

func (f *fakeKeyring) GetMetadata(key string) (keyring.Metadata, error) {
	return keyring.Metadata{}, nil
}

func (f *fakeKeyring) Set(item keyring.Item) error {
	f.items[item.Key] = item
	return nil
}

func (f *fakeKeyring) Remove(key string) error {
	delete(f.items, key)
	return nil
}

func (f *fakeKeyring) Keys() ([]string, error) {
	keys := make([]string, 0, len(f.items))
	for k := range f.items {
		keys = append(keys, k)
	}
	return keys, nil
}

// makeTestClient creates a Client injected with a fakeKeyring.
func makeTestClient(fake *fakeKeyring) *Client {
	return &Client{
		open: func(cfg keyring.Config) (keyring.Keyring, error) {
			return fake, nil
		},
	}
}

// TestClient_GetNotFound returns empty string for non-existent key.
func TestClient_GetNotFound(t *testing.T) {
	c := makeTestClient(newFakeKeyring())
	val, err := c.Get("openai")
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if val != "" {
		t.Errorf("Get() expected empty string, got %q", val)
	}
}

// TestClient_SetAndGet round-trips a stored value.
func TestClient_SetAndGet(t *testing.T) {
	c := makeTestClient(newFakeKeyring())
	if err := c.Set("openai", "sk-test-key-12345"); err != nil {
		t.Fatalf("Set() unexpected error: %v", err)
	}
	val, err := c.Get("openai")
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if val != "sk-test-key-12345" {
		t.Errorf("Get() expected 'sk-test-key-12345', got %q", val)
	}
}

// TestClient_RemoveThenGet returns empty string after removal.
func TestClient_RemoveThenGet(t *testing.T) {
	fake := newFakeKeyring()
	c := makeTestClient(fake)

	// Pre-populate
	if err := c.Set("anthropic", "sk-ant-test"); err != nil {
		t.Fatalf("Set() unexpected error: %v", err)
	}

	if err := c.Remove("anthropic"); err != nil {
		t.Fatalf("Remove() unexpected error: %v", err)
	}

	val, err := c.Get("anthropic")
	if err != nil {
		t.Fatalf("Get() after Remove() unexpected error: %v", err)
	}
	if val != "" {
		t.Errorf("Get() after Remove() expected empty string, got %q", val)
	}
}

// TestClient_InjectableMock verifies the open field allows injection of a mock keyring.
func TestClient_InjectableMock(t *testing.T) {
	fake := newFakeKeyring()
	// Manually inject a key into the fake
	fake.items["openai"] = keyring.Item{Key: "openai", Data: []byte("injected-key")}

	c := makeTestClient(fake)
	val, err := c.Get("openai")
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if val != "injected-key" {
		t.Errorf("Get() expected 'injected-key', got %q", val)
	}
}

// TestClient_SetAndGet_RealFileBackend_ColonKey is a regression test for a bug where
// composite keys containing colons (e.g. "remote:<uuid>:password") broke on Windows:
// the FileBackend keyring stores one file per key and only escapes forward slashes
// in the key (not colons), so os.WriteFile failed with "The filename, directory
// name, or volume label syntax is incorrect." The existing tests in this file all
// use fakeKeyring (an in-memory map), which never exercises real file writes and
// is exactly why this was never caught — this test uses the real keyring.Open via
// New(), pointed at an isolated temp directory, to actually hit the filesystem.
func TestClient_SetAndGet_RealFileBackend_ColonKey(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

	c := New()

	key := "remote:6bb44fd2-2489-42cf-812b-26618a544997:password"
	if err := c.Set(key, "hunter2"); err != nil {
		t.Fatalf("Set() with colon-containing key unexpected error: %v", err)
	}

	val, err := c.Get(key)
	if err != nil {
		t.Fatalf("Get() with colon-containing key unexpected error: %v", err)
	}
	if val != "hunter2" {
		t.Errorf("Get() expected 'hunter2', got %q", val)
	}

	if err := c.Remove(key); err != nil {
		t.Fatalf("Remove() with colon-containing key unexpected error: %v", err)
	}
	val, err = c.Get(key)
	if err != nil {
		t.Fatalf("Get() after Remove() unexpected error: %v", err)
	}
	if val != "" {
		t.Errorf("Get() after Remove() expected empty string, got %q", val)
	}
}

// TestSanitizeKey_LeavesSimpleKeysUnchanged ensures existing plain provider
// keys ("openai", "anthropic", etc.) are unaffected by the sanitization.
func TestSanitizeKey_LeavesSimpleKeysUnchanged(t *testing.T) {
	for _, k := range []string{"openai", "anthropic", "openrouter"} {
		if got := sanitizeKey(k); got != k {
			t.Errorf("sanitizeKey(%q) = %q, want unchanged", k, got)
		}
	}
}

// TestSanitizeKey_EscapesWindowsUnsafeChars verifies every Windows-invalid
// filename character gets percent-encoded, and that encoding round-trips to
// distinct outputs for distinct inputs (no collisions).
func TestSanitizeKey_EscapesWindowsUnsafeChars(t *testing.T) {
	got := sanitizeKey("remote:6bb44fd2:password")
	if got == "remote:6bb44fd2:password" {
		t.Fatal("expected colon-containing key to be transformed, got unchanged")
	}
	for _, c := range []string{"<", ">", ":", `"`, "/", `\`, "|", "?", "*"} {
		if sanitizeKey(c) == c {
			t.Errorf("expected %q to be escaped, got unchanged", c)
		}
	}

	a := sanitizeKey("remote_abc_password")
	b := sanitizeKey("remote:abc:password")
	if a == b {
		t.Error("expected different logical keys to sanitize to different strings")
	}
}

// TestNew_DefaultOpenFunction verifies New() creates a Client with a non-nil open field.
func TestNew_DefaultOpenFunction(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.open == nil {
		t.Error("New() Client.open field is nil")
	}
}
