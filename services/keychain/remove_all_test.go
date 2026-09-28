package keychain

import (
	"errors"
	"sort"
	"testing"

	"github.com/99designs/keyring"
)

// scopedRing extends the package's existing fakeKeyring with the two things
// RemoveAll's contract needs and that double lacks: service scoping (so we can
// assert the wipe never touches another application's credentials) and fault
// injection (so the best-effort and partial-failure paths are reachable without
// breaking a real OS keyring).
//
// Get/Set/GetMetadata come from the embedded fakeKeyring and operate on the raw
// map, so probeBackend's canary Set/Get/Remove round-trip behaves normally.
type scopedRing struct {
	*fakeKeyring
	service   string
	keysErr   error
	removeErr error
	removed   []string
}

func newScopedRing(service string) *scopedRing {
	return &scopedRing{fakeKeyring: newFakeKeyring(), service: service}
}

func (r *scopedRing) scoped(key string) string { return r.service + ":" + key }

// seed stores an item directly, so it is not confused with probeBackend's
// canary.
func (r *scopedRing) seed(key string, data string) {
	r.items[r.scoped(key)] = keyring.Item{Key: r.scoped(key), Data: []byte(data)}
}

func (r *scopedRing) count() int { return len(r.items) }

func (r *scopedRing) Remove(key string) error {
	// The probe canary is PairAdmin's own bookkeeping, not a stored credential;
	// excluding it keeps the call log about real credentials only.
	if key != probeKey {
		r.removed = append(r.removed, key)
	}
	if r.removeErr != nil {
		return r.removeErr
	}
	// Delete under both names. The embedded Set (used by probeBackend's canary
	// round-trip) stores the key verbatim, while seed/Keys/Remove use the
	// service-prefixed form; a real backend has a single namespace and the
	// prefix is only a modelling device here, so clearing both keeps the double
	// self-consistent and stops the canary masquerading as a leftover secret.
	delete(r.items, r.scoped(key))
	delete(r.items, key)
	return nil
}

// Keys returns only this service's keys, mirroring wincred's prefix filter.
func (r *scopedRing) Keys() ([]string, error) {
	if r.keysErr != nil {
		return nil, r.keysErr
	}
	var out []string
	prefix := r.scoped("")
	for k := range r.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			out = append(out, k[len(prefix):])
		}
	}
	sort.Strings(out)
	return out, nil
}

func openFuncReturning(r *scopedRing) func(keyring.Config) (keyring.Keyring, error) {
	return func(keyring.Config) (keyring.Keyring, error) { return r, nil }
}

// --- RemoveAll (the uninstaller's opt-in --uninstall-cleanup wipe) ---

// TestRemoveAll_DeletesEveryPairAdminScopedCredential is the load-bearing test
// for the uninstall wipe.
//
// Mutation check: deleting the `for _, k := range keys { kr.Remove(k) }` loop
// body — the one line that actually deletes anything — makes this test FAIL
// with "credential still present". An implementation that called Keys() and
// ignored the result would still satisfy a weaker "RemoveAll returns no error"
// assertion, which is why the assertion here is on the surviving items and not
// on the returned error.
func TestRemoveAll_DeletesEveryPairAdminScopedCredential(t *testing.T) {
	r := newScopedRing(ServiceName)
	// Realistic key shapes, including the composite remote-credential form that
	// sanitizeKey percent-encodes on Windows.
	r.seed("openai", "sk-live-should-not-survive")
	r.seed("remote:6bb44fd2:password", "hunter2-should-not-survive")
	r.seed("remote:9f0c1a2b:password", "another-secret")

	c := NewWithOpenFunc(openFuncReturning(r))
	if err := c.RemoveAll(); err != nil {
		t.Fatalf("RemoveAll returned an unexpected error: %v", err)
	}

	keys, err := r.Keys()
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("credential(s) still present after RemoveAll: %v", keys)
	}
	if r.count() != 0 {
		t.Errorf("expected the ring to be empty, %d item(s) left", r.count())
	}
}

// TestRemoveAll_LeavesOtherServiceCredentialsAlone pins the property that
// matters most for a "delete everything" operation: it may only ever touch
// PairAdmin's own entries.
//
// Mutation check: opening the ring with a blank or different ServiceName — i.e.
// dropping the scoping — makes this test FAIL, because the foreign entry is
// deleted along with ours.
func TestRemoveAll_LeavesOtherServiceCredentialsAlone(t *testing.T) {
	// A shared ring holding another application's credential, as a system
	// keychain genuinely can.
	r := newScopedRing(ServiceName)
	r.items["someotherapp:github"] = keyring.Item{Key: "someotherapp:github", Data: []byte("not-ours")}

	c := NewWithOpenFunc(openFuncReturning(r))
	if err := c.RemoveAll(); err != nil {
		t.Fatalf("RemoveAll returned an unexpected error: %v", err)
	}

	if _, ok := r.items["someotherapp:github"]; !ok {
		t.Error("RemoveAll deleted a credential outside the " + ServiceName + " scope")
	}
}

// TestRemoveAll_EmptyAndAlreadyClean covers the two no-op shapes the uninstaller
// actually hits: a machine that never stored anything, and a re-run of a wipe
// that already succeeded. Both must be quiet successes.
//
// Mutation check: returning err instead of nil on the Keys() error path fails
// the third case; returning a non-nil "no credentials" error for the empty case
// fails the first.
func TestRemoveAll_EmptyAndAlreadyClean(t *testing.T) {
	// (a) nothing stored at all.
	if err := NewWithOpenFunc(openFuncReturning(newScopedRing(ServiceName))).RemoveAll(); err != nil {
		t.Errorf("RemoveAll on an empty ring should be a quiet success, got %v", err)
	}

	// (b) wiped twice — the second call must not error.
	r := newScopedRing(ServiceName)
	r.seed("openai", "secret")
	c := NewWithOpenFunc(openFuncReturning(r))
	if err := c.RemoveAll(); err != nil {
		t.Fatalf("first RemoveAll: %v", err)
	}
	if err := c.RemoveAll(); err != nil {
		t.Errorf("second RemoveAll on an already-clean ring should be a quiet success, got %v", err)
	}
}

// TestRemoveAll_UnopenableBackendIsNoOp pins the nil-safety the uninstaller
// depends on: in its headless context there is no master password, so the file
// backend cannot open, and a wipe that failed the uninstall over it would leave
// the user with a half-removed app.
//
// Mutation check: returning the ring() error instead of nil makes this test FAIL.
func TestRemoveAll_UnopenableBackendIsNoOp(t *testing.T) {
	c := NewWithOpenFunc(func(keyring.Config) (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	})
	if err := c.RemoveAll(); err != nil {
		t.Errorf("RemoveAll must be a quiet no-op when no backend can be opened, got %v", err)
	}
}

// TestRemoveAll_KeysErrorIsNoOp covers the enumeration-failure path.
//
// Mutation check: returning the Keys() error instead of nil makes this test FAIL.
func TestRemoveAll_KeysErrorIsNoOp(t *testing.T) {
	r := newScopedRing(ServiceName)
	r.keysErr = errors.New("listing unavailable")
	if err := NewWithOpenFunc(openFuncReturning(r)).RemoveAll(); err != nil {
		t.Errorf("RemoveAll must be a quiet no-op when enumeration fails, got %v", err)
	}
}

// TestRemoveAll_PartialFailureIsReported proves the best-effort path is not
// silently swallowing a partial wipe: every credential is still ATTEMPTED and
// the failure reaches the caller. The uninstaller ignores the returned error by
// design; this test is what keeps the information available to a caller that
// wants it.
//
// Mutation check: replacing `errors.Join(errs...)` with a bare `return nil`
// makes this test FAIL — the partial failure stops being reported.
func TestRemoveAll_PartialFailureIsReported(t *testing.T) {
	r := newScopedRing(ServiceName)
	r.seed("openai", "a")
	r.seed("anthropic", "b")
	r.removeErr = errors.New("remove failed")
	c := NewWithOpenFunc(openFuncReturning(r))

	err := c.RemoveAll()
	if err == nil {
		t.Fatal("expected RemoveAll to report the Remove failure, got nil")
	}
	// Both keys must still have been attempted — a fail-fast loop would leave
	// the second untouched and silently under-report the wipe.
	sort.Strings(r.removed)
	if len(r.removed) != 2 {
		t.Errorf("expected every credential to be attempted, got %v", r.removed)
	}
}

// TestRemoveAll_ErrorDoesNotLeakCredentialValues pins the AGENTS.md §3
// redaction rule on this path: a Remove failure may be reported, but the stored
// secret must never appear in the error text.
//
// Mutation check: changing the wrapped error to include the key or the value
// (e.g. fmt.Errorf("remove %s (%s): %w", k, item.Data, err)) makes this test
// FAIL on the secret or the provider name.
func TestRemoveAll_ErrorDoesNotLeakCredentialValues(t *testing.T) {
	const secret = "sk-live-THIS-MUST-NEVER-BE-PRINTED"
	r := newScopedRing(ServiceName)
	r.seed("openai", secret)
	r.removeErr = errors.New("backend exploded")
	c := NewWithOpenFunc(openFuncReturning(r))

	err := c.RemoveAll()
	if err == nil {
		t.Fatal("expected an error from the forced Remove failure")
	}
	msg := err.Error()
	if contains(msg, secret) {
		t.Error("RemoveAll's error leaked the stored credential value")
	}
	if contains(msg, "openai") {
		t.Error("RemoveAll's error named the credential; this path must stay generic")
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
