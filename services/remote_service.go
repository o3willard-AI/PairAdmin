package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"pairadmin/services/config"
	"pairadmin/services/keychain"

	"github.com/google/uuid"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// remoteKeychainKey builds the composite keychain key for a saved remote
// host's secret. UUID-keyed (not host:port:user) to avoid collisions between
// entries that share a host/user with different ports, and to avoid leaking
// hostnames into OS keychain UIs that surface item names (e.g. macOS Keychain
// Access). Underscore-delimited, not colon-delimited: keychain.Client also
// sanitizes keys defensively, but this avoids relying on that entirely for a
// key format under our own control.
func remoteKeychainKey(hostID, secretKind string) string {
	return fmt.Sprintf("remote_%s_%s", hostID, secretKind)
}

// RemoteService manages saved remote host metadata (config.RemoteHost) and
// their associated keychain-stored secrets. It is distinct from
// SettingsService (which owns LLM/app config) because remote-host
// bookkeeping has its own credential lifecycle, and distinct from PTYService
// (which owns live session I/O) because this is CRUD over persisted state,
// not connection handling.
type RemoteService struct {
	ctx            context.Context
	keychainClient *keychain.Client
	emitFn         func(ctx context.Context, event string, optionalData ...interface{})
}

func NewRemoteService(keychainClient *keychain.Client) *RemoteService {
	return &RemoteService{
		keychainClient: keychainClient,
		emitFn:         wailsruntime.EventsEmit,
	}
}

func (s *RemoteService) Startup(ctx context.Context) {
	s.ctx = ctx
}

// ListRemoteHosts returns saved (non-secret) host entries, most-recently-used first.
func (s *RemoteService) ListRemoteHosts() ([]config.RemoteHost, error) {
	cfg, err := config.LoadAppConfig()
	if err != nil {
		return nil, err
	}
	hosts := append([]config.RemoteHost{}, cfg.RemoteHosts...)
	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].LastUsed > hosts[j].LastUsed
	})
	return hosts, nil
}

// SaveRemoteHost upserts a RemoteHost entry (by ID, generating one if empty).
// Callers should only invoke this after a successful connection — secrets are
// never persisted for hosts that haven't been verified reachable. If
// password/passphrase is non-empty, it's stored in the keychain under this
// host's ID.
func (s *RemoteService) SaveRemoteHost(host config.RemoteHost, password, passphrase string) (config.RemoteHost, error) {
	if host.ID == "" {
		host.ID = uuid.New().String()
	}
	host.LastUsed = time.Now().UTC().Format(time.RFC3339)

	cfg, err := config.LoadAppConfig()
	if err != nil {
		return config.RemoteHost{}, err
	}

	found := false
	for i, h := range cfg.RemoteHosts {
		if h.ID == host.ID {
			cfg.RemoteHosts[i] = host
			found = true
			break
		}
	}
	if !found {
		cfg.RemoteHosts = append(cfg.RemoteHosts, host)
	}

	if err := config.SaveAppConfig(cfg); err != nil {
		return config.RemoteHost{}, err
	}

	if password != "" {
		if err := s.keychainClient.Set(remoteKeychainKey(host.ID, "password"), password); err != nil {
			return host, fmt.Errorf("host saved but failed to store password: %w", err)
		}
	}
	if passphrase != "" {
		if err := s.keychainClient.Set(remoteKeychainKey(host.ID, "passphrase"), passphrase); err != nil {
			return host, fmt.Errorf("host saved but failed to store passphrase: %w", err)
		}
	}

	if s.ctx != nil && s.emitFn != nil {
		s.emitFn(s.ctx, "remote:hosts-changed", nil)
	}

	return host, nil
}

// ForgetRemoteHost removes both the AppConfig entry and any keychain secrets for it.
func (s *RemoteService) ForgetRemoteHost(id string) error {
	cfg, err := config.LoadAppConfig()
	if err != nil {
		return err
	}
	filtered := cfg.RemoteHosts[:0]
	for _, h := range cfg.RemoteHosts {
		if h.ID != id {
			filtered = append(filtered, h)
		}
	}
	cfg.RemoteHosts = filtered
	if err := config.SaveAppConfig(cfg); err != nil {
		return err
	}

	_ = s.keychainClient.Remove(remoteKeychainKey(id, "password"))
	_ = s.keychainClient.Remove(remoteKeychainKey(id, "passphrase"))

	if s.ctx != nil && s.emitFn != nil {
		s.emitFn(s.ctx, "remote:hosts-changed", nil)
	}
	return nil
}

// TouchRemoteHost updates a saved host's LastUsed timestamp to now — called
// after a successful reconnect via a saved host ID.
func (s *RemoteService) TouchRemoteHost(id string) error {
	cfg, err := config.LoadAppConfig()
	if err != nil {
		return err
	}
	for i, h := range cfg.RemoteHosts {
		if h.ID == id {
			cfg.RemoteHosts[i].LastUsed = time.Now().UTC().Format(time.RFC3339)
			return config.SaveAppConfig(cfg)
		}
	}
	return fmt.Errorf("remote host %q not found", id)
}

// RenameRemoteHost sets a saved host's friendly display name, e.g. when the
// user renames the terminal tab it was opened from. Loads the full existing
// record and mutates only Name before saving — mirrors TouchRemoteHost's
// pattern rather than accepting a caller-supplied RemoteHost, which would
// risk the same partial-struct-overwrite bug fixed in SaveSettings (a caller
// here would plausibly only have the ID and new name in scope, not the rest
// of the record).
func (s *RemoteService) RenameRemoteHost(id, name string) error {
	cfg, err := config.LoadAppConfig()
	if err != nil {
		return err
	}
	for i, h := range cfg.RemoteHosts {
		if h.ID == id {
			cfg.RemoteHosts[i].Name = name
			return config.SaveAppConfig(cfg)
		}
	}
	return fmt.Errorf("remote host %q not found", id)
}
