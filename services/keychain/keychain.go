package keychain

import (
	"os"
	"path/filepath"

	"github.com/99designs/keyring"
)

// ServiceName is the keychain service identifier for PairAdmin.
const ServiceName = "pairadmin"

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

// ring opens the keyring using the configured open function.
func (c *Client) ring() (keyring.Keyring, error) {
	home, _ := os.UserHomeDir()
	return c.open(keyring.Config{
		ServiceName:     ServiceName,
		AllowedBackends: []keyring.BackendType{keyring.SecretServiceBackend, keyring.FileBackend},
		FileDir:         filepath.Join(home, ".pairadmin", "keyring"),
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
	item, err := kr.Get(provider)
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
		Key:  provider,
		Data: []byte(key),
	})
}

// Remove deletes the API key for the given provider from the OS keychain.
func (c *Client) Remove(provider string) error {
	kr, err := c.ring()
	if err != nil {
		return err
	}
	return kr.Remove(provider)
}
