// Package scanner hosts the network-scanner feature's reachability probes.
// It MAY be imported by services/ but must never import back into services
// (no import cycles).
package scanner

import (
	"errors"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// DialFunc is the injectable dial seam for the host-key probe — production
// code uses ssh.Dial, tests point it at a fake that exercises the
// HostKeyCallback without a live SSH server. Exported so services-level
// tests can also swap it.
var DialFunc = ssh.Dial

// errHostKeyCaptured is a sentinel used only to abort ProbeHostKey's
// handshake immediately after the host key becomes available, before any
// authentication is attempted — probing a host's key should never risk
// triggering an auth-failure lockout on the remote server.
var errHostKeyCaptured = errors.New("host key captured")

// ProbeHostKey connects just far enough to observe the remote host's SSH key
// and reports its type/fingerprint, without attempting authentication or
// leaving a session open. Shared by the remote-SSH terminal path (which
// wraps it as services.probeHostKey for its host-key trust prompt) and the
// network scanner.
func ProbeHostKey(host string, port int) (keyType, fingerprint string, err error) {
	cfg := &ssh.ClientConfig{
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			keyType = key.Type()
			fingerprint = ssh.FingerprintSHA256(key)
			return errHostKeyCaptured
		},
		Timeout: 10 * time.Second,
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	_, dialErr := DialFunc("tcp", addr, cfg)
	if fingerprint == "" {
		return "", "", fmt.Errorf("failed to reach %s: %w", addr, dialErr)
	}
	return keyType, fingerprint, nil
}
