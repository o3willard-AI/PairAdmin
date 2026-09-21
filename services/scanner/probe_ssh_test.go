package scanner

import (
	"crypto/ed25519"
	"net"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// startBannerServer spins up an in-process TCP server that writes a fixed
// SSH server identification line and closes — exactly what a real SSH server
// does before any key exchange or auth. No real SSH handshake is needed to
// test the banner-reading half of ProbeSSH.
func startBannerServer(t *testing.T, banner string) (host string, port int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				c.Write([]byte(banner))
				c.Close()
			}(conn)
		}
	}()

	return splitAddr(t, listener.Addr().String())
}

// startSilentServer listens but never writes anything — a TCP endpoint that
// accepts the connection and then stalls forever, to exercise the ident-line
// read deadline without burning the full 5s (we shrink the timeout via the
// test seam below is not available for read; instead we assert the read
// eventually errors rather than hanging the suite).
func startSilentServer(t *testing.T) (host string, port int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn // deliberately neither read nor written
		}
	}()
	return splitAddr(t, listener.Addr().String())
}

// findRefusedPort returns a loopback port that is guaranteed to have no
// listener: bind an ephemeral port, note it, close the listener, and hand it
// back. Dialing it produces a real RST (ECONNREFUSED) on loopback — the
// exact "host exists but no SSH on :22" signal StateClosed represents.
func findRefusedPort(t *testing.T) (host string, port int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()
	return splitAddr(t, addr)
}

func splitAddr(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to split %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse port from %q: %v", addr, err)
	}
	return host, port
}

// swapDialFunc replaces the dial seam for the duration of the test.
func swapDialFunc(t *testing.T, fake func(network, addr string, timeout time.Duration) (net.Conn, error)) {
	t.Helper()
	orig := dialFunc
	dialFunc = fake
	t.Cleanup(func() { dialFunc = orig })
}

// fakeHostKeySigner returns a throwaway ed25519 SSH public key for exercising
// ProbeHostKey's HostKeyCallback without a live SSH server.
func fakeHostKeySigner(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}
	return signer.PublicKey()
}

// Mutation check: removing the "read the ident line" step in ProbeSSH (or
// collapsing open/closed/filtered into one state) makes this test fail — the
// banner would be empty and/or the state would not be StateOpen.
func TestProbeSSH_Open_ReportsBanner(t *testing.T) {
	host, port := startBannerServer(t, "SSH-2.0-test\r\n")

	res := ProbeSSH(host, port, false)
	if res.State != StateOpen {
		t.Fatalf("expected StateOpen, got %q", res.State)
	}
	if res.Banner != "SSH-2.0-test" {
		t.Errorf("expected banner %q, got %q", "SSH-2.0-test", res.Banner)
	}
	if res.Err != nil {
		t.Errorf("expected no error for an open port, got %v", res.Err)
	}
}

// Mutation check: removing the ECONNREFUSED→StateClosed mapping (falling
// through to the filtered branch instead) makes this test fail — a refused
// dial must never be reported as filtered, and vice versa.
func TestProbeSSH_Refused_ReportsClosedNotFiltered(t *testing.T) {
	host, port := findRefusedPort(t)

	res := ProbeSSH(host, port, false)
	if res.State != StateClosed {
		t.Fatalf("expected StateClosed for a refused (RST) dial, got %q", res.State)
	}
	if res.State == StateFiltered {
		t.Fatal("refused dial must never collapse into StateFiltered")
	}
}

// Mutation check: removing the timeout→StateFiltered branch (e.g. lumping
// every dial error into closed) makes this test fail — a dropped/filtered
// dial outcome is unknown and must be distinct from closed.
func TestProbeSSH_DialTimeout_ReportsFilteredNotClosed(t *testing.T) {
	swapDialFunc(t, func(network, addr string, timeout time.Duration) (net.Conn, error) {
		return nil, &net.OpError{
			Op:   "dial",
			Net:  "tcp",
			Addr: nil,
			Err:  errFakeTimeout{},
		}
	})

	res := ProbeSSH("192.0.2.1", DefaultSSHPort, false)
	if res.State != StateFiltered {
		t.Fatalf("expected StateFiltered for a dial timeout, got %q", res.State)
	}
	if res.State == StateClosed {
		t.Fatal("a dial timeout must never collapse into StateClosed")
	}
	if res.Err == nil {
		t.Error("expected the underlying dial error to be preserved for diagnostics")
	}
}

// The three states must be three DISTINCT values — collapsing any two (e.g.
// closed and filtered sharing a string) makes this test fail.
func TestProbeSSH_StatesAreDistinct(t *testing.T) {
	states := []ProbeState{StateOpen, StateClosed, StateFiltered}
	for i := 0; i < len(states); i++ {
		for j := i + 1; j < len(states); j++ {
			if states[i] == states[j] {
				t.Errorf("states %q and %q must be distinct values", states[i], states[j])
			}
		}
	}
}

// Mutation check: removing the 256-byte cap on the ident read makes this
// test fail — a hostile/misbehaving server sending an unbounded
// "identification" line would be read into memory without bound.
func TestProbeSSH_BannerReadCappedAt256Bytes(t *testing.T) {
	long := "SSH-2.0-" + string(make([]byte, 400)) // no newline, > 256 bytes
	host, port := startBannerServer(t, long)

	res := ProbeSSH(host, port, false)
	if res.State != StateOpen {
		t.Fatalf("expected StateOpen even for a truncated (newline-less) banner, got %q", res.State)
	}
	if len(res.Banner) > maxIdentLine {
		t.Errorf("banner read must be capped at %d bytes, got %d", maxIdentLine, len(res.Banner))
	}
}

// Mutation check: removing the read deadline in readIdentLine makes this
// test fail (the suite hangs until go test's own timeout kills it) — a
// server that accepts but never sends anything must not wedge the probe.
// The deadline is shrunk via the test seam so this exercises the timeout
// path in milliseconds, not the production 5s.
func TestProbeSSH_ServerNeverSends_TimesOutAsFiltered(t *testing.T) {
	origTimeout := identReadTimeout
	identReadTimeout = 50 * time.Millisecond
	t.Cleanup(func() { identReadTimeout = origTimeout })

	host, port := startSilentServer(t)

	done := make(chan SSHProbeResult, 1)
	go func() { done <- ProbeSSH(host, port, false) }()

	select {
	case res := <-done:
		// Dial succeeded, but the ident line never arrived: the port state
		// is unknown to the banner reader — filtered, never closed.
		if res.State != StateFiltered {
			t.Fatalf("expected StateFiltered when the ident line never arrives, got %q", res.State)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ProbeSSH hung on a server that never sends an ident line — read deadline missing?")
	}
}

// Mutation check: removing the `if withHostKey` guard (always probing the
// host key) makes this test fail — the swapped DialFunc records the call and
// the test asserts it was NOT made.
func TestProbeSSH_WithoutHostKey_SkipsHostKeyProbe(t *testing.T) {
	host, port := startBannerServer(t, "SSH-2.0-test\r\n")

	origDial := DialFunc
	called := false
	DialFunc = func(network, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
		called = true
		return nil, nil
	}
	t.Cleanup(func() { DialFunc = origDial })

	res := ProbeSSH(host, port, false)
	if res.State != StateOpen {
		t.Fatalf("expected StateOpen, got %q", res.State)
	}
	if called {
		t.Error("host-key probe must not run when withHostKey is false")
	}
}

// Mutation check: removing the withHostKey→ProbeHostKey call makes this test
// fail — KeyType/Fingerprint would stay empty even though the callback
// captured a real key.
func TestProbeSSH_WithHostKey_FillsKeyTypeAndFingerprint(t *testing.T) {
	host, port := startBannerServer(t, "SSH-2.0-test\r\n")

	testKey := fakeHostKeySigner(t)
	origDial := DialFunc
	DialFunc = func(network, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
		// Simulate the handshake reaching the host-key exchange, then
		// aborting — exactly what ProbeHostKey's sentinel does in
		// production. Capture happens before the return, so no real SSH
		// client is ever constructed.
		if err := cfg.HostKeyCallback(addr, nil, testKey); err != nil {
			return nil, err
		}
		return nil, errHostKeyCaptured
	}
	t.Cleanup(func() { DialFunc = origDial })

	res := ProbeSSH(host, port, true)
	if res.State != StateOpen {
		t.Fatalf("expected StateOpen, got %q", res.State)
	}
	if res.KeyType != testKey.Type() {
		t.Errorf("expected key type %q, got %q", testKey.Type(), res.KeyType)
	}
	want := ssh.FingerprintSHA256(testKey)
	if res.Fingerprint != want {
		t.Errorf("expected fingerprint %q, got %q", want, res.Fingerprint)
	}
}
