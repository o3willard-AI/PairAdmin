package keychain

import (
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
type Client struct {
	// open is the function used to open the keyring. Defaults to keyring.Open.
	// Tests may replace this with a function returning a mock keyring.
	open func(keyring.Config) (keyring.Keyring, error)
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

// ring opens the keyring using the configured open function.
func (c *Client) ring() (keyring.Keyring, error) {
	home, _ := os.UserHomeDir()
	return c.open(keyring.Config{
		ServiceName:      ServiceName,
		AllowedBackends:  []keyring.BackendType{keyring.SecretServiceBackend, keyring.FileBackend},
		FileDir:          filepath.Join(home, ".pairadmin", "keyring"),
		FilePasswordFunc: keyring.FixedStringPrompt("pairadmin"),
	})
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
