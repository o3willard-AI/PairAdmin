package scanner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"time"
)

// DefaultSSHPort is the port the network scanner probes for SSH.
const DefaultSSHPort = 22

// Ident-line read bounds: SSH servers send their identification line
// ("SSH-2.0-OpenSSH_...") before any key exchange or auth, so a single short
// read with a deadline is enough to confirm an SSH listener. The read
// timeout is a var (not a const) so tests can shrink it instead of burning
// the full 5s waiting on a deliberately-silent server.
const maxIdentLine = 256

var identReadTimeout = 5 * time.Second

// dialFunc is the injectable TCP dial seam for the SSH probe — production
// code uses net.DialTimeout, tests can simulate refused/filtered dials
// without touching the network.
var dialFunc = net.DialTimeout

// ProbeState classifies the outcome of an SSH port probe. The three states
// are deliberately distinct and must never be collapsed: "closed" means the
// host answered (RST) so it is alive without SSH on the probed port, while
// "filtered" means we simply don't know (packet dropped / firewall).
type ProbeState string

const (
	// StateOpen: the dial succeeded and the server identification line was
	// read — an SSH server is listening.
	StateOpen ProbeState = "open"
	// StateClosed: the dial was refused (RST) — the host exists but nothing
	// is listening for SSH on the probed port.
	StateClosed ProbeState = "closed"
	// StateFiltered: the dial timed out — unknown; the host may well run SSH
	// behind a firewall that drops the probe packet.
	StateFiltered ProbeState = "filtered"
)

// SSHProbeResult is the outcome of ProbeSSH.
type SSHProbeResult struct {
	// State is one of StateOpen / StateClosed / StateFiltered.
	State ProbeState
	// Banner is the server identification line (CR/LF trimmed) when State
	// is StateOpen.
	Banner string
	// KeyType and Fingerprint are filled only when withHostKey was true and
	// the shared host-key probe succeeded.
	KeyType     string
	Fingerprint string
	// Err preserves the underlying dial/read error for closed/filtered
	// diagnostics; nil for a clean open probe.
	Err error
}

// ProbeSSH dials host:port (callers pass DefaultSSHPort for the standard
// :22 probe), reads the server identification line (5s deadline, 256-byte
// cap), closes the connection immediately, and — when withHostKey is true —
// follows up with the shared host-key probe to capture the key type and
// fingerprint. No authentication is ever attempted.
func ProbeSSH(host string, port int, withHostKey bool) SSHProbeResult {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	conn, err := dialFunc("tcp", addr, identReadTimeout)
	if err != nil {
		if isRefused(err) {
			return SSHProbeResult{State: StateClosed, Err: err}
		}
		// Timeouts and anything else unknowable (unreachable network, DNS
		// failure) stay "filtered" — never "closed", which would falsely
		// assert the host answered.
		return SSHProbeResult{State: StateFiltered, Err: err}
	}

	banner, readErr := readIdentLine(conn, identReadTimeout, maxIdentLine)
	conn.Close()
	if readErr != nil {
		return SSHProbeResult{
			State: StateFiltered,
			Err:   fmt.Errorf("failed to read ident line from %s: %w", addr, readErr),
		}
	}

	res := SSHProbeResult{State: StateOpen, Banner: banner}
	if withHostKey {
		keyType, fingerprint, keyErr := ProbeHostKey(host, port)
		res.KeyType = keyType
		res.Fingerprint = fingerprint
		if keyErr != nil {
			res.Err = keyErr
		}
	}
	return res
}

// readIdentLine reads up to max bytes from conn, stopping at the first
// newline (the identification line terminator), and returns the line with
// CR/LF trimmed. A deadline bounds the wait so a server that accepts but
// never sends cannot wedge the probe.
func readIdentLine(conn net.Conn, timeout time.Duration, max int) (string, error) {
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return "", err
	}
	buf := make([]byte, max)
	n := 0
	for n < max {
		rn, err := conn.Read(buf[n:])
		n += rn
		if idx := bytes.IndexByte(buf[:n], '\n'); idx >= 0 {
			return trimIdentEOL(buf[:idx]), nil
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, os.ErrDeadlineExceeded) {
				if n > 0 {
					// Server sent bytes but no newline: accept what we got
					// (capped at max) as the identification line.
					return trimIdentEOL(buf[:n]), nil
				}
			}
			return "", err
		}
	}
	// No newline within max bytes: accept the truncated line.
	return trimIdentEOL(buf[:n]), nil
}

// trimIdentEOL strips a trailing \r and/or \n from the identification line.
func trimIdentEOL(b []byte) string {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return string(b)
}

// isRefused reports whether err represents an explicit connection refusal
// (TCP RST) — the host answered "nothing listening here", as opposed to a
// timeout where the host's state is unknown.
func isRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}
