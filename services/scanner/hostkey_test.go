package scanner

import (
	"crypto/ed25519"
	"net"
	"testing"

	"golang.org/x/crypto/ssh"
)

// errFakeTimeout satisfies net's timeout interface so *net.OpError.Timeout()
// reports true — used to simulate a dropped/filtered dial without hitting a
// real network.
type errFakeTimeout struct{}

func (errFakeTimeout) Error() string { return "connection timed out" }
func (errFakeTimeout) Timeout() bool { return true }

// Mutation check: deleting the HostKeyCallback capture in ProbeHostKey makes
// this test fail — keyType/fingerprint stay empty even though the dial seam
// delivered a real public key.
func TestProbeHostKey_CapturesTypeAndFingerprint(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}
	testKey := signer.PublicKey()

	orig := DialFunc
	DialFunc = func(network, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
		// Reach the host-key exchange, then abort the handshake before any
		// authentication — exactly what ProbeHostKey's own sentinel does in
		// production. Capture happens before the return, so no real SSH
		// client is ever constructed.
		if cbErr := cfg.HostKeyCallback(addr, nil, testKey); cbErr != nil {
			return nil, cbErr
		}
		return nil, errHostKeyCaptured
	}
	t.Cleanup(func() { DialFunc = orig })

	keyType, fingerprint, err := ProbeHostKey("127.0.0.1", 22)
	if err != nil {
		t.Fatalf("ProbeHostKey() unexpected error: %v", err)
	}
	if keyType != testKey.Type() {
		t.Errorf("expected key type %q, got %q", testKey.Type(), keyType)
	}
	want := ssh.FingerprintSHA256(testKey)
	if fingerprint != want {
		t.Errorf("expected fingerprint %q, got %q", want, fingerprint)
	}
}

// Mutation check: removing the `if fingerprint == ""` error branch makes
// this test fail — an unreachable host would be reported as an empty
// successful probe instead of an error.
func TestProbeHostKey_Unreachable_ReturnsError(t *testing.T) {
	orig := DialFunc
	DialFunc = func(network, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errFakeTimeout{}}
	}
	t.Cleanup(func() { DialFunc = orig })

	_, _, err := ProbeHostKey("192.0.2.1", 22)
	if err == nil {
		t.Fatal("expected error when the dial never delivers a host key, got nil")
	}
}
