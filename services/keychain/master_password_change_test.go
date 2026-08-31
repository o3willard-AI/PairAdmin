package keychain

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/99designs/keyring"
)

// TestMasterPassword_ChangeMasterPassword_ReEncrypts covers the full change
// flow: wrong old password rejected, correct old password re-encrypts every
// file-backend item under the new password and rewrites the hash, and the
// updated file backend round-trips with the new password.
func TestMasterPassword_ChangeMasterPassword_ReEncrypts(t *testing.T) {
	home := isolateHome(t)
	c := newFileBackendTestClient()

	if err := c.SetMasterPassword("old-password"); err != nil {
		t.Fatalf("SetMasterPassword() unexpected error: %v", err)
	}

	// Populate the file backend via the Client (uses masterPW) with
	// multiple items, including a colon-containing key (the
	// percent-encoded filename case).
	if err := c.Set("openai", "sk-test-12345"); err != nil {
		t.Fatalf("Set(openai) unexpected error: %v", err)
	}
	if err := c.Set("anthropic", "sk-ant-67890"); err != nil {
		t.Fatalf("Set(anthropic) unexpected error: %v", err)
	}
	colonKey := "remote:6bb44fd2-2489-42cf-812b-26618a544997:password"
	if err := c.Set(colonKey, "hunter2"); err != nil {
		t.Fatalf("Set(colon key) unexpected error: %v", err)
	}

	// Wrong old password must be rejected without touching anything.
	if err := c.ChangeMasterPassword("not-the-password", "new-password"); !errors.Is(err, ErrWrongMasterPassword) {
		t.Fatalf("ChangeMasterPassword(wrong old) err = %v, want ErrWrongMasterPassword", err)
	}

	// Correct old password must succeed.
	if err := c.ChangeMasterPassword("old-password", "new-password"); err != nil {
		t.Fatalf("ChangeMasterPassword() unexpected error: %v", err)
	}

	// Hash file must now verify the NEW password and reject the OLD one.
	hashPath := filepath.Join(home, ".pairadmin", "master_password.hash")
	if _, err := os.Stat(hashPath); err != nil {
		t.Fatalf("hash file missing after change: %v", err)
	}
	newClient := newFileBackendTestClient()
	ok, err := newClient.VerifyMasterPassword("new-password")
	if err != nil {
		t.Fatalf("VerifyMasterPassword(new) unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("hash file does not verify new password after change")
	}
	oldClient := newFileBackendTestClient()
	ok, err = oldClient.VerifyMasterPassword("old-password")
	if err != nil {
		t.Fatalf("VerifyMasterPassword(old) unexpected error: %v", err)
	}
	if ok {
		t.Fatal("hash file still verifies old password after change")
	}

	// Every item must decrypt under the new password and hold its original
	// value — with a fresh client process (fresh keyring state).
	fresh := newFileBackendTestClient()
	if ok, err := fresh.VerifyMasterPassword("new-password"); err != nil || !ok {
		t.Fatalf("VerifyMasterPassword(new) on fresh client: ok=%v err=%v", ok, err)
	}
	for _, tt := range []struct {
		key  string
		want string
	}{
		{"openai", "sk-test-12345"},
		{"anthropic", "sk-ant-67890"},
		{colonKey, "hunter2"},
	} {
		got, err := fresh.Get(tt.key)
		if err != nil {
			t.Errorf("Get(%q) after change: unexpected error: %v", tt.key, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Get(%q) = %q, want %q (item not re-encrypted correctly?)", tt.key, got, tt.want)
		}
	}
}

// halfDone marks the expected outcome of the partial re-encryption helper
// below.
var halfDone = errors.New("halfDone")

// reencryptFileItemsPartial re-encrypts the given key (only) under newPW and
// leaves everything else untouched, mimicking a crash between re-encryption
// and the hash rewrite. It always returns halfDone when the partial write
// itself succeeded, so the test can distinguish "partial step ran" from any
// unexpected error.
func (c *Client) reencryptFileItemsPartial(key string) error {
	item, err := c.ringFileItem(key)
	if err != nil {
		return err
	}
	if err := c.setFileItem(key, item.Data, "new-password"); err != nil {
		return err
	}
	return halfDone
}

// ringFileItem reads the item stored at key directly from the file backend,
// decrypted with the client's in-memory master password.
func (c *Client) ringFileItem(key string) (keyring.Item, error) {
	home, _ := os.UserHomeDir()
	ring, err := keyring.Open(keyring.Config{
		ServiceName:      ServiceName,
		AllowedBackends:  []keyring.BackendType{keyring.FileBackend},
		FileDir:          filepath.Join(home, ".pairadmin", "keyring"),
		FilePasswordFunc: keyring.FixedStringPrompt(c.masterPW),
	})
	if err != nil {
		return keyring.Item{}, err
	}
	return ring.Get(sanitizeKey(key))
}

// setFileItem writes data at key directly into the file backend, encrypted
// under pw (bypassing the client's in-memory master password).
func (c *Client) setFileItem(key string, data []byte, pw string) error {
	home, _ := os.UserHomeDir()
	ring, err := keyring.Open(keyring.Config{
		ServiceName:      ServiceName,
		AllowedBackends:  []keyring.BackendType{keyring.FileBackend},
		FileDir:          filepath.Join(home, ".pairadmin", "keyring"),
		FilePasswordFunc: keyring.FixedStringPrompt(pw),
	})
	if err != nil {
		return err
	}
	return ring.Set(keyring.Item{Key: sanitizeKey(key), Data: data})
}

// TestMasterPassword_ChangeMasterPassword_RetryAfterInterruptedRun covers the
// crash-recovery path: an interrupted change leaves some items under the new
// password and some under the old; retrying ChangeMasterPassword with the
// same arguments must complete the migration instead of failing on the
// already-re-encrypted items.
func TestMasterPassword_ChangeMasterPassword_RetryAfterInterruptedRun(t *testing.T) {
	isolateHome(t)
	c := newFileBackendTestClient()

	if err := c.SetMasterPassword("old-password"); err != nil {
		t.Fatalf("SetMasterPassword() unexpected error: %v", err)
	}
	if err := c.Set("openai", "sk-test-12345"); err != nil {
		t.Fatalf("Set(openai) unexpected error: %v", err)
	}
	if err := c.Set("anthropic", "sk-ant-67890"); err != nil {
		t.Fatalf("Set(anthropic) unexpected error: %v", err)
	}

	// Simulate an interrupted run: re-encrypt only the first item under the
	// new password without touching the hash file.
	if err := c.reencryptFileItemsPartial("openai"); !errors.Is(err, halfDone) {
		t.Fatalf("reencryptFileItemsPartial(openai) err = %v, want halfDone", err)
	}

	// Retry the full change with the same arguments.
	if err := c.ChangeMasterPassword("old-password", "new-password"); err != nil {
		t.Fatalf("ChangeMasterPassword() retry after interrupted run: unexpected error: %v", err)
	}

	// Both items must now decrypt under the new password.
	fresh := newFileBackendTestClient()
	if ok, err := fresh.VerifyMasterPassword("new-password"); err != nil || !ok {
		t.Fatalf("VerifyMasterPassword(new) on fresh client: ok=%v err=%v", ok, err)
	}
	for _, tt := range []struct {
		key  string
		want string
	}{{"openai", "sk-test-12345"}, {"anthropic", "sk-ant-67890"}} {
		got, err := fresh.Get(tt.key)
		if err != nil {
			t.Errorf("Get(%q) after retry: unexpected error: %v", tt.key, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Get(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}
