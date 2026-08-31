package keychain

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/99designs/keyring"
)

// isolateHome points os.UserHomeDir() at a fresh temp dir so master password
// hash files and the keyring file backend never touch the real user home.
func isolateHome(t *testing.T) string {
	t.Helper()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME
	return homeDir
}

// newFileBackendTestClient returns a Client whose OS-backend stage behaves as
// "no OS backend available" (keyring.ErrNoAvailImpl) while file-backend opens
// go to the real keyring.Open. Tests that exercise the encrypted file backend
// must pin this: (a) on hosts where an OS backend "opens" but is
// non-functional (e.g. a D-Bus session bus without a Secret Service),
// keyring.Open's normal fallback would route operations to the broken OS
// backend instead of the file backend this suite targets; (b) the OS stage of
// Client.ring must not see a file-backend keyring, whose FilePasswordFunc is
// unset in that config and would nil-panic on first unlock.
func newFileBackendTestClient() *Client {
	return NewWithOpenFunc(func(cfg keyring.Config) (keyring.Keyring, error) {
		if configIncludesFileBackend(cfg) {
			return keyring.Open(cfg)
		}
		return nil, keyring.ErrNoAvailImpl
	})
}

// configIncludesFileBackend reports whether cfg.AllowedBackends contains the
// file backend, i.e. whether an open request is the file-backend stage of
// Client.ring (or a probe that included it).
func configIncludesFileBackend(cfg keyring.Config) bool {
	for _, b := range cfg.AllowedBackends {
		if b == keyring.FileBackend {
			return true
		}
	}
	return false
}

// TestMasterPassword_SetVerifyHoldsInMemory covers SetMasterPassword and
// VerifyMasterPassword: hash file creation (format + 0600), in-memory hold
// after Set/Verify, reject-on-wrong-password without holding, and
// no-master-password behavior for Get.
func TestMasterPassword_SetVerifyHoldsInMemory(t *testing.T) {
	home := isolateHome(t)
	c := newFileBackendTestClient()

	if c.HasMasterPassword() {
		t.Fatal("HasMasterPassword() = true before any SetMasterPassword, want false")
	}
	if err := c.SetMasterPassword("first-master"); err != nil {
		t.Fatalf("SetMasterPassword() unexpected error: %v", err)
	}
	if !c.HasMasterPassword() {
		t.Fatal("HasMasterPassword() = false after SetMasterPassword, want true")
	}

	// Hash file must exist at ~/.pairadmin/master_password.hash with 0600
	// permissions and the self-describing scrypt format.
	hashPath := filepath.Join(home, ".pairadmin", "master_password.hash")
	info, err := os.Stat(hashPath)
	if err != nil {
		t.Fatalf("hash file missing after SetMasterPassword: %v", err)
	}
	if perms := info.Mode().Perm(); perms != 0600 {
		t.Errorf("hash file mode = %o, want 600", perms)
	}
	data, err := os.ReadFile(hashPath)
	if err != nil {
		t.Fatalf("read hash file: %v", err)
	}
	line := strings.TrimSpace(string(data))
	parts := strings.Split(line, "$")
	if len(parts) != 6 || parts[0] != "scrypt" {
		t.Fatalf("hash file format = %q, want scrypt$N$r$p$<salt-hex>$<hash-hex>", line)
	}

	// SetMasterPassword must refuse to overwrite an existing password.
	if err := c.SetMasterPassword("second-master"); !errors.Is(err, ErrMasterPasswordExists) {
		t.Errorf("SetMasterPassword() twice: err = %v, want ErrMasterPasswordExists", err)
	}

	// Wrong password must not verify and must not be held in memory.
	c2 := New()
	ok, err := c2.VerifyMasterPassword("wrong-password")
	if err != nil {
		t.Fatalf("VerifyMasterPassword(wrong) unexpected error: %v", err)
	}
	if ok {
		t.Error("VerifyMasterPassword(wrong) = true, want false")
	}
	if c2.masterPW != "" {
		t.Errorf("wrong password was held in memory (masterPW = %q)", c2.masterPW)
	}

	// Correct password must verify and be held in memory.
	ok, err = c2.VerifyMasterPassword("first-master")
	if err != nil {
		t.Fatalf("VerifyMasterPassword(correct) unexpected error: %v", err)
	}
	if !ok {
		t.Error("VerifyMasterPassword(correct) = false, want true")
	}
	if c2.masterPW != "first-master" {
		t.Errorf("VerifyMasterPassword did not hold password in memory, masterPW = %q", c2.masterPW)
	}

	// A client that never verified/set has no in-memory password, so the
	// file backend must refuse to unlock with a clear error. Store one item
	// first (file.go only calls the password func after a successful file
	// read, so an empty keyring never reaches it).
	if err := c.Set("openai", "sk-in-memory-check"); err != nil {
		t.Fatalf("Set(openai) unexpected error: %v", err)
	}
	c3 := newFileBackendTestClient()
	_, err = c3.Get("openai")
	if !errors.Is(err, ErrNoMasterPassword) {
		t.Errorf("Get() without master password: err = %v, want ErrNoMasterPassword", err)
	}
}

// TestMasterPassword_NeedsMasterPassword_NoOSBackend verifies that
// NeedsMasterPassword returns true when no OS backend opens (the client's
// open func returns keyring.ErrNoAvailImpl, exactly what keyring.Open yields
// when every allowed backend fails) and false when an OS backend opens.
func TestMasterPassword_NeedsMasterPassword_NoOSBackend(t *testing.T) {
	isolateHome(t)

	tests := []struct {
		name string
		open func(keyring.Config) (keyring.Keyring, error)
		want bool
	}{
		{
			name: "no OS backend opens -> needs master password",
			open: func(keyring.Config) (keyring.Keyring, error) {
				return nil, keyring.ErrNoAvailImpl
			},
			want: true,
		},
		{
			name: "OS backend available -> no master password needed",
			open: func(keyring.Config) (keyring.Keyring, error) {
				return newFakeKeyring(), nil
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewWithOpenFunc(tt.open)
			got, err := c.NeedsMasterPassword()
			if err != nil {
				t.Fatalf("NeedsMasterPassword() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("NeedsMasterPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}
