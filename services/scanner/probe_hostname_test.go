package scanner

import (
	"errors"
	"testing"
	"time"
)

// errFakeLookupError is a sentinel error used by test mocks to simulate
// a reverse-DNS resolution failure.
var errFakeLookupError = errors.New("fake reverse-DNS lookup failure")

// swapLookupAddr replaces the lookupAddrFn seam for the duration of the test
// and restores it afterward.
func swapLookupAddr(t *testing.T, fake func(string) ([]string, error)) {
	t.Helper()
	orig := lookupAddrFn
	lookupAddrFn = fake
	t.Cleanup(func() { lookupAddrFn = orig })
}

// Mutation check: removing the `return HostnameResult{Hostname: ...}` line
// that copies the first PTR name makes this test fail — Hostname would stay
// empty even though the mock returned a name.
func TestProbeHostname_ResolvedName(t *testing.T) {
	swapLookupAddr(t, func(addr string) ([]string, error) {
		return []string{"myhost.example.com."}, nil
	})

	res := ProbeHostname("192.168.1.10", 5*time.Second)
	if res.Hostname != "myhost.example.com" {
		t.Errorf("expected hostname %q, got %q", "myhost.example.com", res.Hostname)
	}
}

// Mutation check: removing the `if len(res.names) == 0` guard (treating an
// empty slice as a name) or removing the error-to-empty mapping makes this
// test fail — an empty result from the mock would otherwise produce a
// non-empty (or panicking) Hostname.
func TestProbeHostname_EmptyResultReturnsEmpty(t *testing.T) {
	swapLookupAddr(t, func(addr string) ([]string, error) {
		return []string{}, nil
	})

	res := ProbeHostname("10.0.0.1", 5*time.Second)
	if res.Hostname != "" {
		t.Errorf("expected empty hostname for empty lookup result, got %q", res.Hostname)
	}
}

// Mutation check: removing the `if res.err != nil` guard (propagating the
// error as a panic or returning it) makes this test fail — the mock error
// would surface instead of yielding a clean empty result.
func TestProbeHostname_LookupErrorReturnsEmpty(t *testing.T) {
	swapLookupAddr(t, func(addr string) ([]string, error) {
		return nil, errFakeLookupError
	})

	res := ProbeHostname("10.0.0.99", 5*time.Second)
	if res.Hostname != "" {
		t.Errorf("expected empty hostname on lookup error, got %q", res.Hostname)
	}
}

// Mutation check: removing the context.WithTimeout / select on ctx.Done()
// makes this test fail — the function would block until the slow mock
// finishes, exceeding the timeout window.
func TestProbeHostname_TimeoutReturnsEmpty(t *testing.T) {
	mockStarted := make(chan struct{})
	swapLookupAddr(t, func(addr string) ([]string, error) {
		close(mockStarted)
		time.Sleep(200 * time.Millisecond)
		return []string{"should-not-reach"}, nil
	})

	start := time.Now()
	res := ProbeHostname("10.0.0.50", 10*time.Millisecond)
	elapsed := time.Since(start)

	// Must return before the mock finishes (10ms timeout, 200ms mock).
	if elapsed > 50*time.Millisecond {
		t.Errorf("expected timeout within ~10ms, took %v — context wrapping missing?", elapsed)
	}
	if res.Hostname != "" {
		t.Errorf("expected empty hostname on timeout, got %q", res.Hostname)
	}
}

// Mutation check: removing the strings.TrimSuffix(".", ...) call makes
// this test fail — the trailing dot would appear in the returned Hostname.
func TestProbeHostname_StripsTrailingDot(t *testing.T) {
	swapLookupAddr(t, func(addr string) ([]string, error) {
		return []string{"host.example.com."}, nil
	})

	res := ProbeHostname("172.16.0.1", 5*time.Second)
	if res.Hostname != "host.example.com" {
		t.Errorf("expected trailing dot stripped, got %q", res.Hostname)
	}
}
