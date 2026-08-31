package keychain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/99designs/keyring"
)

// ServiceName is the keychain service identifier for PairAdmin.
const ServiceName = "pairadmin"

// windowsUnsafeFilenameChars matches characters invalid in Windows filenames
// (< > : " / \ | ? *), plus '%' itself so percent-encoding stays unambiguous.
// The 99designs/keyring FileBackend stores one file per key and only escapes
// forward slashes in the key (percent.Encode(key, "/") — verified against
// mtibben/percent's source: the second argument is the set of characters TO
// encode, not a safe-list). Any caller using a composite key with a colon
// delimiter (e.g. "remote:<uuid>:password") breaks on Windows with
// "The filename, directory name, or volume label syntax is incorrect."
var windowsUnsafeFilenameChars = regexp.MustCompile(`[<>:"/\\|?*%]`)

// sanitizeKey percent-encodes characters that are invalid in Windows
// filenames so any keychain key is safe under the FileBackend regardless of
// what delimiter the caller chose. A no-op for plain alphanumeric keys (e.g.
// "openai"), so existing simple provider keys are unaffected.
func sanitizeKey(key string) string {
	return windowsUnsafeFilenameChars.ReplaceAllStringFunc(key, func(c string) string {
		return fmt.Sprintf("%%%02X", c[0])
	})
}

// Client is a thin wrapper around 99designs/keyring with an injectable open function
// for test isolation.
//
// When no OS backend (macOS Keychain, Windows Credential Manager, Linux
// Secret Service) is available, keyring falls back to the encrypted file
// backend. That backend is unlocked with a user-chosen master password: the
// Client holds the verified password in memory and persists only a salted
// scrypt hash on disk (see master_password.go). Callers check
// NeedsMasterPassword and drive SetMasterPassword / VerifyMasterPassword /
// ChangeMasterPassword as appropriate.
type Client struct {
	// open is the function used to open the keyring. Defaults to keyring.Open.
	// Tests may replace this with a function returning a mock keyring.
	open func(keyring.Config) (keyring.Keyring, error)

	// masterPW holds the verified user master password used to unlock the
	// keyring's file backend. Empty when no master password has been set or
	// verified in this process.
	masterPW string
}

// New creates a new Client using the default keyring.Open function.
func New() *Client {
	return &Client{
		open: keyring.Open,
	}
}

// NewWithOpenFunc creates a Client with an injectable open function.
// Used by tests outside this package to inject a mock keyring.
func NewWithOpenFunc(openFn func(keyring.Config) (keyring.Keyring, error)) *Client {
	return &Client{open: openFn}
}

// ring opens the keyring using the configured open function. When no OS
// backend is available, keyring falls through to the file backend, whose
// password comes from c.masterPW via filePasswordFunc. filePasswordFunc is
// invoked lazily — only by the file backend itself, and only when it is
// about to encrypt or decrypt an item (file.go unlock()) — so the "file
// backend in use" condition is enforced at exactly the right moment: an OS
// backend never triggers it, and the file backend can never silently get
// the old hardcoded password as a fallback.
func (c *Client) ring() (keyring.Keyring, error) {
	home, _ := os.UserHomeDir()
	return c.open(keyring.Config{
		ServiceName:     ServiceName,
		AllowedBackends: []keyring.BackendType{keyring.KeychainBackend, keyring.WinCredBackend, keyring.SecretServiceBackend, keyring.FileBackend},
		FileDir:         filepath.Join(home, ".pairadmin", "keyring"),
		FilePasswordFunc: func(_ string) (string, error) {
			if c.masterPW == "" {
				return "", ErrNoMasterPassword
			}
			return c.masterPW, nil
		},
	})
}

// NeedsMasterPassword probes ONLY the OS backends (macOS Keychain, Windows
// Credential Manager, Linux Secret Service) and reports whether none of them
// opens — i.e. keyring would fall back to the file backend, which requires a
// master password. No prompting occurs.
func (c *Client) NeedsMasterPassword() (bool, error) {
	home, _ := os.UserHomeDir()
	_, err := c.open(keyring.Config{
		ServiceName:     ServiceName,
		AllowedBackends: []keyring.BackendType{keyring.KeychainBackend, keyring.WinCredBackend, keyring.SecretServiceBackend},
		FileDir:         filepath.Join(home, ".pairadmin", "keyring"),
	})
	if err == keyring.ErrNoAvailImpl {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("probe OS keychain backends: %w", err)
	}
	return false, nil
}

// Get retrieves the API key for the given provider from the OS keychain.
// Returns an empty string (not an error) when the key does not exist.
func (c *Client) Get(provider string) (string, error) {
	kr, err := c.ring()
	if err != nil {
		return "", err
	}
	item, err := kr.Get(sanitizeKey(provider))
	if err == keyring.ErrKeyNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(item.Data), nil
}

// Set stores the API key for the given provider in the OS keychain.
func (c *Client) Set(provider, key string) error {
	kr, err := c.ring()
	if err != nil {
		return err
	}
	return kr.Set(keyring.Item{
		Key:  sanitizeKey(provider),
		Data: []byte(key),
	})
}

// Remove deletes the API key for the given provider from the OS keychain.
func (c *Client) Remove(provider string) error {
	kr, err := c.ring()
	if err != nil {
		return err
	}
	return kr.Remove(sanitizeKey(provider))
}

// HasMasterPassword reports whether a master password hash file exists —
// i.e. the user has configured a master password at some point. It says
// nothing about whether the password is currently held in memory.
func (c *Client) HasMasterPassword() bool {
	path, err := masterPasswordHashPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// SetMasterPassword sets the master password for the first time: it writes
// the scrypt hash file and holds the verified password in memory for
// subsequent file-backend operations. It fails if a master password is
// already configured (use ChangeMasterPassword instead).
func (c *Client) SetMasterPassword(pw string) error {
	if pw == "" {
		return errors.New("master password must not be empty")
	}
	path, err := masterPasswordHashPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return ErrMasterPasswordExists
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check master password hash: %w", err)
	}
	if err := writeMasterPasswordHash(path, pw); err != nil {
		return err
	}
	c.masterPW = pw
	return nil
}

// VerifyMasterPassword checks pw against the stored scrypt hash. On a match
// the password is held in memory so the file backend can be unlocked. A
// missing hash file yields (false, nil): there is nothing to verify yet.
func (c *Client) VerifyMasterPassword(pw string) (bool, error) {
	path, err := masterPasswordHashPath()
	if err != nil {
		return false, err
	}
	ok, err := verifyMasterPasswordHash(path, pw)
	if err != nil {
		return false, err
	}
	if ok {
		c.masterPW = pw
	}
	return ok, nil
}

// ChangeMasterPassword verifies oldPW and, on success, re-encrypts every
// file-backend item under newPW before rewriting the hash file. If the
// process dies after some items were re-encrypted but before the hash was
// rewritten, simply retrying ChangeMasterPassword(oldPW, newPW) completes
// the job: items already under newPW are detected and skipped (see
// reencryptFileItems).
func (c *Client) ChangeMasterPassword(oldPW, newPW string) error {
	if newPW == "" {
		return errors.New("new master password must not be empty")
	}
	path, err := masterPasswordHashPath()
	if err != nil {
		return err
	}
	ok, err := verifyMasterPasswordHash(path, oldPW)
	if err != nil {
		return err
	}
	if !ok {
		return ErrWrongMasterPassword
	}

	if err := c.reencryptFileItems(oldPW, newPW); err != nil {
		return err
	}

	if err := writeMasterPasswordHash(path, newPW); err != nil {
		return err
	}
	c.masterPW = newPW
	return nil
}

// reencryptFileItems re-encrypts every item in the file backend under newPW.
// The backend is opened directly with AllowedBackends=[FileBackend] so OS
// backends are bypassed even when one is available (a mixed setup may still
// carry legacy items from before the OS backend existed). Each item is read
// with a keyring instance unlocked under oldPW and written back with one
// unlocked under newPW.
func (c *Client) reencryptFileItems(oldPW, newPW string) error {
	home, _ := os.UserHomeDir()
	oldRing, err := keyring.Open(keyring.Config{
		ServiceName:      ServiceName,
		AllowedBackends:  []keyring.BackendType{keyring.FileBackend},
		FileDir:          filepath.Join(home, ".pairadmin", "keyring"),
		FilePasswordFunc: keyring.FixedStringPrompt(oldPW),
	})
	if err != nil {
		return fmt.Errorf("open file keyring with old password: %w", err)
	}
	newRing, err := keyring.Open(keyring.Config{
		ServiceName:      ServiceName,
		AllowedBackends:  []keyring.BackendType{keyring.FileBackend},
		FileDir:          filepath.Join(home, ".pairadmin", "keyring"),
		FilePasswordFunc: keyring.FixedStringPrompt(newPW),
	})
	if err != nil {
		return fmt.Errorf("open file keyring with new password: %w", err)
	}

	keys, err := oldRing.Keys()
	if err != nil {
		return fmt.Errorf("list file keyring keys: %w", err)
	}
	for _, key := range keys {
		item, err := oldRing.Get(key)
		if err != nil {
			// The item may already have been re-encrypted under newPW by an
			// interrupted earlier attempt (crash between re-encryption and
			// the hash rewrite). If it decrypts under newPW, it is done —
			// skip it so a retry is idempotent.
			if _, newErr := newRing.Get(key); newErr == nil {
				continue
			}
			return fmt.Errorf("read item %q under old password: %w", key, err)
		}
		if err := newRing.Set(item); err != nil {
			return fmt.Errorf("re-encrypt item %q: %w", key, err)
		}
	}
	return nil
}
