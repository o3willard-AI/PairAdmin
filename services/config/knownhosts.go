package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// KnownHostKey is a pinned SSH host key fingerprint, keyed by "host:port" in
// the map returned by LoadKnownHosts. Recorded the first time PairAdmin
// connects to a given host:port (trust-on-first-use); every later connection
// to the same host:port must present a key matching Fingerprint or the
// connection is refused — that check, not the initial pin, is what actually
// defends against a MITM swapping in a different key later.
type KnownHostKey struct {
	KeyType     string `yaml:"key_type"`
	Fingerprint string `yaml:"fingerprint"` // ssh.FingerprintSHA256 form, e.g. "SHA256:abc123..."
}

// knownHostsPath returns the full path to ~/.pairadmin/known_hosts.yaml.
// Deliberately a separate file from config.yaml: it's keyed data (not a
// simple settings struct) and has no reason to go through Viper.
func knownHostsPath() string {
	return filepath.Join(configDir(), "known_hosts.yaml")
}

// LoadKnownHosts reads ~/.pairadmin/known_hosts.yaml. A missing file is not
// an error — it just means no host keys have been pinned yet.
func LoadKnownHosts() (map[string]KnownHostKey, error) {
	data, err := os.ReadFile(knownHostsPath())
	if os.IsNotExist(err) {
		return map[string]KnownHostKey{}, nil
	}
	if err != nil {
		return nil, err
	}
	hosts := map[string]KnownHostKey{}
	if err := yaml.Unmarshal(data, &hosts); err != nil {
		return nil, err
	}
	if hosts == nil {
		hosts = map[string]KnownHostKey{}
	}
	return hosts, nil
}

// SaveKnownHosts writes the full known-hosts map to
// ~/.pairadmin/known_hosts.yaml, creating ~/.pairadmin/ if it doesn't exist.
func SaveKnownHosts(hosts map[string]KnownHostKey) error {
	if err := os.MkdirAll(configDir(), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(hosts)
	if err != nil {
		return err
	}
	return os.WriteFile(knownHostsPath(), data, 0o600)
}
