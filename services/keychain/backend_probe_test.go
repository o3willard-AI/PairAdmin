package keychain

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/99designs/keyring"
)

// errorKeyring is a configurable mock: each operation fails with err when
// its fail* flag is set, and succeeds (Get -> ErrKeyNotFound, others -> nil)
// when not.
type errorKeyring struct {
	err        error
	failGet    bool
	failSet    bool
	failRemove bool
}

func (e *errorKeyring) Get(string) (keyring.Item, error) {
	if e.failGet {
		return keyring.Item{}, e.err
	}
	return keyring.Item{}, keyring.ErrKeyNotFound
}
func (e *errorKeyring) GetMetadata(string) (keyring.Metadata, error) {
	return keyring.Metadata{}, e.err
}
func (e *errorKeyring) Set(keyring.Item) error {
	if e.failSet {
		return e.err
	}
	return nil
}
func (e *errorKeyring) Remove(string) error {
	if e.failRemove {
		return e.err
	}
	return nil
}
func (e *errorKeyring) Keys() ([]string, error) {
	return nil, e.err
}

// readOnlyKeyring passes read probes but fails every write — the exact
// half-working Secret Service shape observed live (gnome-keyring-daemon
// running, collection creation broken): Get on a missing key returns a
// clean ErrKeyNotFound while Set fails with D-Bus "Object does not exist
// at path /". A Get-only probe cannot detect this; a write probe must.
type readOnlyKeyring struct{}

func (readOnlyKeyring) Get(string) (keyring.Item, error) {
	return keyring.Item{}, keyring.ErrKeyNotFound
}
func (readOnlyKeyring) GetMetadata(string) (keyring.Metadata, error) {
	return keyring.Metadata{}, nil
}
func (readOnlyKeyring) Set(keyring.Item) error {
	return errors.New("Object does not exist at path /")
}
func (readOnlyKeyring) Remove(string) error { return nil }
func (readOnlyKeyring) Keys() ([]string, error) {
	return []string{}, nil
}

// probeTestOpen is an injected open func for probe tests. It delegates to
// osResult for the OS-backend stage and to the real keyring.Open against the
// isolated HOME for the file-backend stage, and counts every consultation in
// the returned counter.
func probeTestOpen(t *testing.T, osResult func() (keyring.Keyring, error)) (*Client, *int) {
	t.Helper()
	openCalls := 0
	open := func(cfg keyring.Config) (keyring.Keyring, error) {
		openCalls++
		if configIncludesFileBackend(cfg) {
			return keyring.Open(cfg)
		}
		return osResult()
	}
	return NewWithOpenFunc(open), &openCalls
}

// TestNeedsMasterPassword_FunctionalVsBrokenOSBackend is the Task 1.5 table:
// NeedsMasterPassword and ring() must distinguish an OS backend that merely
// opens from one that actually works.
func TestNeedsMasterPassword_FunctionalVsBrokenOSBackend(t *testing.T) {
	isolateHome(t)

	tests := []struct {
		name      string
		osResult  func() (keyring.Keyring, error)
		wantNeeds bool
		wantErr   bool
		// ring checks: osOpen counts OS-stage consultations; fileOpen counts
		// file-backend consultations (0 == ring never fell through to it).
		wantRingOSOpen bool
		wantFileUsed   bool
	}{
		{
			name: "functional empty OS backend (Get -> ErrKeyNotFound)",
			osResult: func() (keyring.Keyring, error) {
				return newFakeKeyring(), nil
			},
			wantNeeds:      false,
			wantRingOSOpen: true,
		},
		{
			name: "functional OS backend holding data",
			osResult: func() (keyring.Keyring, error) {
				fake := newFakeKeyring()
				fake.items["openai"] = keyring.Item{Key: "openai", Data: []byte("sk-test")}
				return fake, nil
			},
			wantNeeds:      false,
			wantRingOSOpen: true,
		},
		{
			name: "broken OS backend (opens, ops fail with D-Bus-style error)",
			osResult: func() (keyring.Keyring, error) {
				return &errorKeyring{err: errors.New("Object does not exist at path /")}, nil
			},
			wantNeeds:    true,
			wantFileUsed: true,
		},
		{
			name: "no backend at all (ErrNoAvailImpl)",
			osResult: func() (keyring.Keyring, error) {
				return nil, keyring.ErrNoAvailImpl
			},
			wantNeeds:    true,
			wantFileUsed: true,
		},
		{
			name: "half-working OS backend (reads healthy, writes broken) -> broken",
			osResult: func() (keyring.Keyring, error) {
				return readOnlyKeyring{}, nil
			},
			wantNeeds:    true,
			wantFileUsed: true,
		},
		{
			name: "OS stage open fails with an unexpected error -> surfaced",
			osResult: func() (keyring.Keyring, error) {
				return nil, errors.New("dbus connection refused")
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, openCalls := probeTestOpen(t, tt.osResult)

			needs, err := c.NeedsMasterPassword()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NeedsMasterPassword() expected error, got nil (needs=%v)", needs)
				}
				return
			}
			if err != nil {
				t.Fatalf("NeedsMasterPassword() unexpected error: %v", err)
			}
			if needs != tt.wantNeeds {
				t.Errorf("NeedsMasterPassword() = %v, want %v", needs, tt.wantNeeds)
			}

			// ring(): OS stage consulted every time; file stage only when
			// falling through.
			_, _ = c.ring()
			if tt.wantRingOSOpen && *openCalls == 0 {
				t.Error("ring() never consulted the OS-backend stage")
			}
			if tt.wantFileUsed {
				// Prove the file stage was reached (not the broken OS
				// backend): with no master password held, Set must surface
				// the clear file-backend unlock error. Set is used instead
				// of Get because Get on a missing key returns
				// ErrKeyNotFound before ever reaching the password func.
				err := c.Set("openai", "probe-value")
				if !errors.Is(err, ErrNoMasterPassword) {
					t.Errorf("Set() via file fallback: err = %v, want ErrNoMasterPassword", err)
				}
				if *openCalls < 2 {
					t.Errorf("ring() consulted open %d time(s), want >= 2 (OS stage + file stage)", *openCalls)
				}
			}
		})
	}
}

// TestNeedsMasterPassword_BrokenOSBackend_FallsBackToFileBackend verifies the
// end-to-end fallback path: a broken OS backend makes ring() use the real
// encrypted file backend, which works once the master password is set.
func TestNeedsMasterPassword_BrokenOSBackend_FallsBackToFileBackend(t *testing.T) {
	home := isolateHome(t)
	c, _ := probeTestOpen(t, func() (keyring.Keyring, error) {
		return &errorKeyring{err: errors.New("Object does not exist at path /")}, nil
	})

	needs, err := c.NeedsMasterPassword()
	if err != nil {
		t.Fatalf("NeedsMasterPassword() unexpected error: %v", err)
	}
	if !needs {
		t.Fatal("NeedsMasterPassword() = false with broken OS backend, want true")
	}

	if err := c.SetMasterPassword("fallback-password"); err != nil {
		t.Fatalf("SetMasterPassword() unexpected error: %v", err)
	}
	if err := c.Set("openai", "sk-fallback-12345"); err != nil {
		t.Fatalf("Set(openai) unexpected error: %v", err)
	}
	got, err := c.Get("openai")
	if err != nil {
		t.Fatalf("Get(openai) unexpected error: %v", err)
	}
	if got != "sk-fallback-12345" {
		t.Errorf("Get(openai) = %q, want %q", got, "sk-fallback-12345")
	}

	// The stored item must live in the real file backend on disk.
	keyringDir := filepath.Join(home, ".pairadmin", "keyring")
	entries, err := os.ReadDir(keyringDir)
	if err != nil {
		t.Fatalf("read file keyring dir: %v", err)
	}
	if len(entries) == 0 {
		t.Errorf("expected encrypted item files in %s, found none", keyringDir)
	}
}

// TestProbeBackend_Classification is a table over probeBackend's result
// classes: nil keyring, working backend (canary round-trips), the
// half-working "reads healthy, writes broken" backend observed live (a
// Get-only probe passes it; the write probe rejects it), an all-ops-fail
// backend, a backend whose canary read-back fails, and a backend whose
// read-back returns wrong data.
func TestProbeBackend_Classification(t *testing.T) {
	tests := []struct {
		name string
		kr   keyring.Keyring
		want bool
	}{
		{
			name: "nil keyring",
			kr:   nil,
			want: false,
		},
		{
			name: "working backend (canary Set/Get/Remove round-trips)",
			kr:   newFakeKeyring(),
			want: true,
		},
		{
			name: "reads healthy, writes broken (live-observed Secret Service)",
			kr:   readOnlyKeyring{},
			want: false,
		},
		{
			name: "all operations fail",
			kr:   &errorKeyring{err: errors.New("Object does not exist at path /")},
			want: false,
		},
		{
			name: "write succeeds but read-back fails",
			kr:   &errorKeyring{err: errors.New("read-back failed"), failGet: true},
			want: false,
		},
		{
			name: "write succeeds but read-back returns wrong data",
			kr:   &corruptReadKeyring{data: "something-else"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := probeBackend(tt.kr); got != tt.want {
				t.Errorf("probeBackend() = %v, want %v", got, tt.want)
			}
		})
	}
}

// corruptReadKeyring accepts writes but returns wrong data on read —
// a backend that swallows or mangles writes must be classified broken.
type corruptReadKeyring struct {
	data string
}

func (c corruptReadKeyring) Get(key string) (keyring.Item, error) {
	return keyring.Item{Key: key, Data: []byte(c.data)}, nil
}
func (c corruptReadKeyring) GetMetadata(string) (keyring.Metadata, error) {
	return keyring.Metadata{}, nil
}
func (c corruptReadKeyring) Set(keyring.Item) error { return nil }
func (c corruptReadKeyring) Remove(string) error    { return nil }
func (c corruptReadKeyring) Keys() ([]string, error) {
	return []string{}, nil
}
