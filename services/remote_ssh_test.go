package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"pairadmin/services/config"

	"golang.org/x/crypto/ssh"
)

// startTestSSHServer spins up a minimal in-process SSH server on 127.0.0.1
// accepting the given username/password and echoing back anything written to
// its "session" channel (simulating a shell that echoes typed input). This
// lets openSSHTerminal be exercised against a real network connection without
// depending on a live external host, per the ssh.Client concrete-type
// constraint noted in the implementation plan.
func startTestSSHServer(t *testing.T, user, password string) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if conn.User() == user && string(pass) == password {
				return nil, nil
			}
			return nil, fmt.Errorf("authentication rejected for %q", conn.User())
		},
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			nConn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveTestSSHConn(nConn, config)
		}
	}()

	return listener.Addr().String()
}

func serveTestSSHConn(nConn net.Conn, config *ssh.ServerConfig) {
	conn, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		return
	}
	defer conn.Close()
	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}

		go func(in <-chan *ssh.Request) {
			for req := range in {
				switch req.Type {
				case "pty-req", "shell", "window-change":
					req.Reply(true, nil)
				default:
					req.Reply(false, nil)
				}
			}
		}(requests)

		go func(ch ssh.Channel) {
			defer ch.Close()
			buf := make([]byte, 1024)
			for {
				n, err := ch.Read(buf)
				if n > 0 {
					ch.Write(buf[:n]) // echo back — simulates a shell echoing typed input
				}
				if err != nil {
					return
				}
			}
		}(channel)
	}
}

// newTestPTYService returns a PTYService with an emitFn that records emitted
// events for assertions, instead of the default Wails runtime.EventsEmit
// (which would panic/log-fatal outside a real Wails context).
func newTestPTYService() (*PTYService, func(event string) []interface{}) {
	svc := NewPTYService()
	svc.ctx = context.Background()

	var mu sync.Mutex
	events := make(map[string][]interface{})
	svc.emitFn = func(_ context.Context, event string, optionalData ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		if len(optionalData) > 0 {
			events[event] = append(events[event], optionalData[0])
		} else {
			events[event] = append(events[event], nil)
		}
	}

	getEvents := func(event string) []interface{} {
		mu.Lock()
		defer mu.Unlock()
		out := make([]interface{}, len(events[event]))
		copy(out, events[event])
		return out
	}

	return svc, getEvents
}

func splitTestAddr(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to split test server address %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse port from %q: %v", portStr, err)
	}
	return host, port
}

// isolateHomeDir points os.UserHomeDir() (and therefore
// config.LoadKnownHosts/SaveKnownHosts's ~/.pairadmin/known_hosts.yaml) at a
// fresh temp directory for the duration of the test, matching the pattern
// already used throughout services/config/config_test.go — without this,
// tests that connect via openSSHTerminal would pin real host keys into the
// actual developer's ~/.pairadmin/known_hosts.yaml.
func isolateHomeDir(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME
}

func TestOpenSSHTerminal_PasswordAuth_EchoesInputAndCloses(t *testing.T) {
	isolateHomeDir(t)
	addr := startTestSSHServer(t, "testuser", "testpass")
	host, port := splitTestAddr(t, addr)

	svc, getEvents := newTestPTYService()

	tabId := "ssh:test-tab"
	resolvedId, err := svc.openSSHTerminal(tabId, RemoteConnectParams{
		Kind:     RemoteKindSSH,
		Host:     host,
		Port:     port,
		Username: "testuser",
		AuthType: RemoteAuthPassword,
		Password: "testpass",
	})
	if err != nil {
		t.Fatalf("openSSHTerminal() unexpected error: %v", err)
	}
	if resolvedId != tabId {
		t.Errorf("expected resolved tabId %q, got %q", tabId, resolvedId)
	}

	if err := svc.WriteInput(tabId, "hello\n"); err != nil {
		t.Fatalf("WriteInput() unexpected error: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	found := false
	for time.Now().Before(deadline) && !found {
		for _, ev := range getEvents("pty:output") {
			if out, ok := ev.(PTYOutputEvent); ok && strings.Contains(out.Data, "hello") {
				found = true
				break
			}
		}
		if !found {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if !found {
		t.Fatal("expected echoed 'hello' in a pty:output event, got none")
	}

	if err := svc.ResizeTerminal(tabId, 100, 30); err != nil {
		t.Errorf("ResizeTerminal() unexpected error: %v", err)
	}

	if err := svc.CloseTerminal(tabId); err != nil {
		t.Errorf("CloseTerminal() unexpected error: %v", err)
	}

	svc.mu.Lock()
	_, stillPresent := svc.sessions[tabId]
	svc.mu.Unlock()
	if stillPresent {
		t.Error("expected session to be removed from sessions map after CloseTerminal")
	}
}

func TestOpenSSHTerminal_WrongPassword_ReturnsErrorAndNoSession(t *testing.T) {
	isolateHomeDir(t)
	addr := startTestSSHServer(t, "testuser", "testpass")
	host, port := splitTestAddr(t, addr)

	svc, _ := newTestPTYService()

	tabId := "ssh:test-tab-badauth"
	_, err := svc.openSSHTerminal(tabId, RemoteConnectParams{
		Kind:     RemoteKindSSH,
		Host:     host,
		Port:     port,
		Username: "testuser",
		AuthType: RemoteAuthPassword,
		Password: "wrong-password",
	})
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}

	svc.mu.Lock()
	_, present := svc.sessions[tabId]
	svc.mu.Unlock()
	if present {
		t.Error("expected no session stored after failed auth")
	}
}

func TestSanitizeTmuxSessionName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"valid alnum passes through unchanged", "work-session_1.2", "work-session_1.2"},
		{"empty falls back to default", "", defaultTmuxSessionName},
		{"whitespace-only falls back to default", "   ", defaultTmuxSessionName},
		{"shell metacharacters stripped", "a;rm -rf $HOME`whoami`", "arm-rfHOMEwhoami"},
		{"fully-invalid input falls back to default", ";&|$()", defaultTmuxSessionName},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeTmuxSessionName(tc.in)
			if got != tc.want {
				t.Errorf("sanitizeTmuxSessionName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestOpenSSHTerminal_UseTmux_SendsCreateOrAttachCommand(t *testing.T) {
	isolateHomeDir(t)
	addr := startTestSSHServer(t, "testuser", "testpass")
	host, port := splitTestAddr(t, addr)

	svc, getEvents := newTestPTYService()

	origDelay := tmuxAttachDelay
	tmuxAttachDelay = time.Millisecond
	defer func() { tmuxAttachDelay = origDelay }()

	tabId := "ssh:tmux-tab"
	_, err := svc.openSSHTerminal(tabId, RemoteConnectParams{
		Kind:            RemoteKindSSH,
		Host:            host,
		Port:            port,
		Username:        "testuser",
		AuthType:        RemoteAuthPassword,
		Password:        "testpass",
		UseTmux:         true,
		TmuxSessionName: "my session!", // contains a space and '!' — must be sanitized
	})
	if err != nil {
		t.Fatalf("openSSHTerminal() unexpected error: %v", err)
	}
	defer svc.CloseTerminal(tabId)

	deadline := time.Now().Add(3 * time.Second)
	found := false
	for time.Now().Before(deadline) && !found {
		for _, ev := range getEvents("pty:output") {
			if out, ok := ev.(PTYOutputEvent); ok && strings.Contains(out.Data, "tmux new-session -A -s mysession") {
				found = true
				break
			}
		}
		if !found {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if !found {
		t.Fatal("expected echoed tmux create-or-attach command with sanitized session name, got none")
	}
}

func TestOpenSSHTerminal_NoTmux_NoTmuxCommandSent(t *testing.T) {
	isolateHomeDir(t)
	addr := startTestSSHServer(t, "testuser", "testpass")
	host, port := splitTestAddr(t, addr)

	svc, getEvents := newTestPTYService()

	tabId := "ssh:no-tmux-tab"
	_, err := svc.openSSHTerminal(tabId, RemoteConnectParams{
		Kind:     RemoteKindSSH,
		Host:     host,
		Port:     port,
		Username: "testuser",
		AuthType: RemoteAuthPassword,
		Password: "testpass",
		UseTmux:  false,
	})
	if err != nil {
		t.Fatalf("openSSHTerminal() unexpected error: %v", err)
	}
	defer svc.CloseTerminal(tabId)

	// Give any (incorrectly-firing) tmux goroutine a chance to run before asserting absence.
	time.Sleep(100 * time.Millisecond)
	for _, ev := range getEvents("pty:output") {
		if out, ok := ev.(PTYOutputEvent); ok && strings.Contains(out.Data, "tmux") {
			t.Fatalf("expected no tmux command when UseTmux=false, got output: %q", out.Data)
		}
	}
}

func TestBuildSSHAuthMethods_Password(t *testing.T) {
	methods, err := buildSSHAuthMethods(RemoteConnectParams{AuthType: RemoteAuthPassword, Password: "secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestBuildSSHAuthMethods_PrivateKey_Unencrypted(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	keyPath := filepath.Join(t.TempDir(), "id_test")
	if err := writeUnencryptedPEMKey(keyPath, key); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	methods, err := buildSSHAuthMethods(RemoteConnectParams{
		AuthType:       RemoteAuthPrivateKey,
		PrivateKeyPath: keyPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestBuildSSHAuthMethods_PrivateKey_MissingFile(t *testing.T) {
	_, err := buildSSHAuthMethods(RemoteConnectParams{
		AuthType:       RemoteAuthPrivateKey,
		PrivateKeyPath: filepath.Join(t.TempDir(), "does-not-exist"),
	})
	if err == nil {
		t.Fatal("expected error for missing key file, got nil")
	}
}

// TestBuildSSHAuthMethods_PrivateKey_TildeExpansion is a regression test: the
// New Terminal dialog's own placeholder text suggests "~/.ssh/id_ed25519",
// but os.ReadFile does no shell-style ~ expansion on its own — without
// expandHomeDir, every connection using that exact suggested syntax would
// fail with a confusing "no such file" error.
func TestBuildSSHAuthMethods_PrivateKey_TildeExpansion(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir) // os.UserHomeDir() reads USERPROFILE on Windows, not HOME

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("failed to create .ssh dir: %v", err)
	}
	if err := writeUnencryptedPEMKey(filepath.Join(sshDir, "id_ed25519"), key); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	methods, err := buildSSHAuthMethods(RemoteConnectParams{
		AuthType:       RemoteAuthPrivateKey,
		PrivateKeyPath: "~/.ssh/id_ed25519",
	})
	if err != nil {
		t.Fatalf("unexpected error resolving ~-prefixed path: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestExpandHomeDir(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bare tilde", "~", homeDir},
		{"tilde slash", "~/.ssh/id_ed25519", filepath.Join(homeDir, ".ssh", "id_ed25519")},
		{"absolute path unchanged", "/etc/ssh/id_ed25519", "/etc/ssh/id_ed25519"},
		{"no tilde unchanged", "relative/path", "relative/path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := expandHomeDir(tc.in); got != tc.want {
				t.Errorf("expandHomeDir(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestOpenSSHTerminal_FirstConnect_PinsHostKeySilently(t *testing.T) {
	isolateHomeDir(t)
	addr := startTestSSHServer(t, "testuser", "testpass")
	host, port := splitTestAddr(t, addr)
	svc, _ := newTestPTYService()

	tabId := "ssh:pin-tab"
	if _, err := svc.openSSHTerminal(tabId, RemoteConnectParams{
		Kind: RemoteKindSSH, Host: host, Port: port,
		Username: "testuser", AuthType: RemoteAuthPassword, Password: "testpass",
	}); err != nil {
		t.Fatalf("openSSHTerminal() unexpected error: %v", err)
	}
	defer svc.CloseTerminal(tabId)

	hosts, err := config.LoadKnownHosts()
	if err != nil {
		t.Fatalf("LoadKnownHosts() unexpected error: %v", err)
	}
	pinned, ok := hosts[hostPortKey(host, port)]
	if !ok {
		t.Fatal("expected host key to be pinned after first connect (default PromptNewHostKeys=false)")
	}
	if pinned.Fingerprint == "" {
		t.Error("expected a non-empty pinned fingerprint")
	}
}

func TestOpenSSHTerminal_SecondConnect_MatchingPinnedKey_Succeeds(t *testing.T) {
	isolateHomeDir(t)
	addr := startTestSSHServer(t, "testuser", "testpass")
	host, port := splitTestAddr(t, addr)
	svc, _ := newTestPTYService()

	params := RemoteConnectParams{
		Kind: RemoteKindSSH, Host: host, Port: port,
		Username: "testuser", AuthType: RemoteAuthPassword, Password: "testpass",
	}
	firstTab := "ssh:first-tab"
	if _, err := svc.openSSHTerminal(firstTab, params); err != nil {
		t.Fatalf("first openSSHTerminal() unexpected error: %v", err)
	}
	svc.CloseTerminal(firstTab)

	secondTab := "ssh:second-tab"
	if _, err := svc.openSSHTerminal(secondTab, params); err != nil {
		t.Fatalf("second openSSHTerminal() with matching pinned key unexpected error: %v", err)
	}
	svc.CloseTerminal(secondTab)
}

func TestOpenSSHTerminal_KeyMismatch_AlwaysRejected(t *testing.T) {
	isolateHomeDir(t)
	addr := startTestSSHServer(t, "testuser", "testpass")
	host, port := splitTestAddr(t, addr)
	svc, _ := newTestPTYService()

	// Pre-seed known_hosts with a bogus fingerprint for this host:port,
	// simulating a previously-trusted key that the server no longer presents.
	if err := config.SaveKnownHosts(map[string]config.KnownHostKey{
		hostPortKey(host, port): {KeyType: "ssh-rsa", Fingerprint: "SHA256:not-the-real-key"},
	}); err != nil {
		t.Fatalf("SaveKnownHosts() unexpected error: %v", err)
	}

	_, err := svc.openSSHTerminal("ssh:mismatch-tab", RemoteConnectParams{
		Kind: RemoteKindSSH, Host: host, Port: port,
		Username: "testuser", AuthType: RemoteAuthPassword, Password: "testpass",
	})
	if err == nil {
		t.Fatal("expected error connecting to a host presenting a key that doesn't match the pinned one")
	}
	var mismatchErr *HostKeyMismatchError
	if !errors.As(err, &mismatchErr) {
		t.Errorf("expected error to wrap *HostKeyMismatchError, got: %v", err)
	}

	// Setting PromptNewHostKeys or TrustNewHostKey must NOT bypass this — it
	// only applies to genuinely unrecognized hosts, not key changes.
	_, err = svc.openSSHTerminal("ssh:mismatch-tab-2", RemoteConnectParams{
		Kind: RemoteKindSSH, Host: host, Port: port,
		Username: "testuser", AuthType: RemoteAuthPassword, Password: "testpass",
		TrustNewHostKey: true,
	})
	if err == nil {
		t.Fatal("expected TrustNewHostKey=true to NOT bypass a host key mismatch")
	}
}

func TestOpenSSHTerminal_PromptNewHostKeys_UnknownHost_RejectsWithoutTrust(t *testing.T) {
	isolateHomeDir(t)
	if err := config.SaveAppConfig(&config.AppConfig{PromptNewHostKeys: true}); err != nil {
		t.Fatalf("SaveAppConfig() unexpected error: %v", err)
	}
	addr := startTestSSHServer(t, "testuser", "testpass")
	host, port := splitTestAddr(t, addr)
	svc, _ := newTestPTYService()

	_, err := svc.openSSHTerminal("ssh:prompt-tab", RemoteConnectParams{
		Kind: RemoteKindSSH, Host: host, Port: port,
		Username: "testuser", AuthType: RemoteAuthPassword, Password: "testpass",
	})
	if err == nil {
		t.Fatal("expected error connecting to an unrecognized host with PromptNewHostKeys=true and no trust flag")
	}
	var unknownErr *UnknownHostKeyError
	if !errors.As(err, &unknownErr) {
		t.Errorf("expected error to wrap *UnknownHostKeyError, got: %v", err)
	}

	hosts, err := config.LoadKnownHosts()
	if err != nil {
		t.Fatalf("LoadKnownHosts() unexpected error: %v", err)
	}
	if _, pinned := hosts[hostPortKey(host, port)]; pinned {
		t.Error("expected host key to NOT be pinned when the prompt was required but not accepted")
	}
}

func TestOpenSSHTerminal_PromptNewHostKeys_TrustNewHostKey_PinsAndSucceeds(t *testing.T) {
	isolateHomeDir(t)
	if err := config.SaveAppConfig(&config.AppConfig{PromptNewHostKeys: true}); err != nil {
		t.Fatalf("SaveAppConfig() unexpected error: %v", err)
	}
	addr := startTestSSHServer(t, "testuser", "testpass")
	host, port := splitTestAddr(t, addr)
	svc, _ := newTestPTYService()

	tabId := "ssh:trust-tab"
	if _, err := svc.openSSHTerminal(tabId, RemoteConnectParams{
		Kind: RemoteKindSSH, Host: host, Port: port,
		Username: "testuser", AuthType: RemoteAuthPassword, Password: "testpass",
		TrustNewHostKey: true,
	}); err != nil {
		t.Fatalf("openSSHTerminal() with TrustNewHostKey=true unexpected error: %v", err)
	}
	defer svc.CloseTerminal(tabId)

	hosts, err := config.LoadKnownHosts()
	if err != nil {
		t.Fatalf("LoadKnownHosts() unexpected error: %v", err)
	}
	if _, pinned := hosts[hostPortKey(host, port)]; !pinned {
		t.Error("expected host key to be pinned after explicit TrustNewHostKey accept")
	}
}

func TestCheckHostKeyTrust_UnknownHost_ReturnsKnownFalse(t *testing.T) {
	isolateHomeDir(t)
	addr := startTestSSHServer(t, "testuser", "testpass")
	host, port := splitTestAddr(t, addr)
	svc, _ := newTestPTYService()

	status, err := svc.CheckHostKeyTrust(host, port)
	if err != nil {
		t.Fatalf("CheckHostKeyTrust() unexpected error: %v", err)
	}
	if status.Known {
		t.Error("expected Known=false for a host with no pinned key yet")
	}
	if status.Fingerprint == "" {
		t.Error("expected a non-empty fingerprint from the probe")
	}
}

func TestCheckHostKeyTrust_AfterPinning_ReturnsKnownTrue(t *testing.T) {
	isolateHomeDir(t)
	addr := startTestSSHServer(t, "testuser", "testpass")
	host, port := splitTestAddr(t, addr)
	svc, _ := newTestPTYService()

	tabId := "ssh:precheck-tab"
	if _, err := svc.openSSHTerminal(tabId, RemoteConnectParams{
		Kind: RemoteKindSSH, Host: host, Port: port,
		Username: "testuser", AuthType: RemoteAuthPassword, Password: "testpass",
	}); err != nil {
		t.Fatalf("openSSHTerminal() unexpected error: %v", err)
	}
	svc.CloseTerminal(tabId)

	status, err := svc.CheckHostKeyTrust(host, port)
	if err != nil {
		t.Fatalf("CheckHostKeyTrust() unexpected error: %v", err)
	}
	if !status.Known {
		t.Error("expected Known=true after the key was already pinned by a prior connect")
	}
}

func TestCheckHostKeyTrust_MismatchedPinnedKey_ReturnsChangedTrue(t *testing.T) {
	isolateHomeDir(t)
	addr := startTestSSHServer(t, "testuser", "testpass")
	host, port := splitTestAddr(t, addr)
	svc, _ := newTestPTYService()

	if err := config.SaveKnownHosts(map[string]config.KnownHostKey{
		hostPortKey(host, port): {KeyType: "ssh-rsa", Fingerprint: "SHA256:not-the-real-key"},
	}); err != nil {
		t.Fatalf("SaveKnownHosts() unexpected error: %v", err)
	}

	status, err := svc.CheckHostKeyTrust(host, port)
	if err != nil {
		t.Fatalf("CheckHostKeyTrust() unexpected error: %v", err)
	}
	if status.Known {
		t.Error("expected Known=false when the pinned key no longer matches")
	}
	if !status.Changed {
		t.Error("expected Changed=true when a previously pinned key no longer matches")
	}
}

// writeUnencryptedPEMKey writes an RSA private key to path in unencrypted PKCS#1 PEM form.
func writeUnencryptedPEMKey(path string, key *rsa.PrivateKey) error {
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, block)
}
