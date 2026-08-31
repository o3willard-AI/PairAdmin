package services

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"pairadmin/services/config"

	"golang.org/x/crypto/ssh"
)

// sshDialFunc is an injectable seam for tests — production code uses ssh.Dial,
// tests can point it at an in-process SSH server instead of a real network dial.
var sshDialFunc = ssh.Dial

// tmuxAttachDelay is how long openSSHTerminal waits after Shell() before
// sending the tmux create-or-attach command, giving the remote shell time to
// print any MOTD/banner and reach a usable prompt. This is a fixed heuristic,
// not real prompt detection — injectable so tests don't have to actually wait.
var tmuxAttachDelay = 500 * time.Millisecond

const defaultTmuxSessionName = "pairadmin"

// tmuxSessionNamePattern whitelists characters safe to interpolate into a
// shell command line unquoted. The session name is user-supplied and gets
// written as literal keystrokes into a live remote shell, so anything outside
// this set (spaces, quotes, semicolons, backticks, $, etc.) must be stripped
// to prevent it from being used as a command-injection vector.
var tmuxSessionNamePattern = regexp.MustCompile(`[^A-Za-z0-9_.-]`)

// sanitizeTmuxSessionName strips any character not in the safe whitelist and
// falls back to defaultTmuxSessionName if nothing safe remains.
func sanitizeTmuxSessionName(name string) string {
	cleaned := tmuxSessionNamePattern.ReplaceAllString(name, "")
	if cleaned == "" {
		return defaultTmuxSessionName
	}
	return cleaned
}

// expandHomeDir expands a leading "~" or "~/" to the current user's home
// directory. Go's os.ReadFile does no shell-style expansion on its own — a
// literal "~" is just a character to the filesystem — but the New Terminal
// dialog's own placeholder text ("~/.ssh/id_ed25519") actively invites users
// to type exactly that, so without this every private-key-file connection
// using the suggested syntax would fail with a confusing "no such file"
// error. Left unchanged if it doesn't start with "~" (an absolute path
// already works fine as-is).
func expandHomeDir(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// buildSSHAuthMethods constructs the auth methods for an SSH connection from the
// user-supplied connection params. Only password and private-key-file auth are
// supported in v1 (no ssh-agent, no Kerberos).
func buildSSHAuthMethods(params RemoteConnectParams) ([]ssh.AuthMethod, error) {
	switch params.AuthType {
	case RemoteAuthPassword:
		return []ssh.AuthMethod{ssh.Password(params.Password)}, nil
	case RemoteAuthPrivateKey:
		keyBytes, err := os.ReadFile(expandHomeDir(params.PrivateKeyPath))
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", err)
		}
		var signer ssh.Signer
		if params.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(params.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyBytes)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	default:
		return nil, fmt.Errorf("unsupported SSH auth type: %q", params.AuthType)
	}
}

// HostKeyMismatchError is returned when a remote host presents a DIFFERENT
// key than the one PairAdmin previously pinned for that host:port. This is
// the actual man-in-the-middle defense — unlike first-connection trust, it is
// never bypassable by config.AppConfig.PromptNewHostKeys or
// RemoteConnectParams.TrustNewHostKey.
type HostKeyMismatchError struct {
	HostPort       string
	OldFingerprint string
	NewFingerprint string
}

func (e *HostKeyMismatchError) Error() string {
	return fmt.Sprintf(
		"REMOTE HOST IDENTIFICATION HAS CHANGED for %s: previously trusted key %s, "+
			"but the server just presented %s. Refusing to connect — this can mean a "+
			"man-in-the-middle attack, or that the host was legitimately reinstalled/rekeyed. "+
			"If you're certain the host is legitimate, ask an administrator to remove its "+
			"entry from known_hosts.yaml before reconnecting.",
		e.HostPort, e.OldFingerprint, e.NewFingerprint,
	)
}

// hostPortKey normalizes a host:port pair into the string key used in both
// the known-hosts store and RemoteConnectParams-derived dial addresses.
func hostPortKey(host string, port int) string {
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}

// verifyHostKeyCallback implements trust-on-first-use SSH host key
// verification pinned to ~/.pairadmin/known_hosts.yaml (see
// config.LoadKnownHosts/SaveKnownHosts), keyed by host:port:
//
//   - A key matching what's already pinned for this host:port is accepted
//     silently — the common case on every connection after the first.
//   - A key that DIFFERS from what's pinned is always rejected with
//     *HostKeyMismatchError, regardless of promptForNewKeys/trustNewKey.
//   - An unrecognized host:port (first-ever connection) is pinned
//     immediately unless promptForNewKeys is true and trustNewKey is false,
//     in which case it's rejected with *UnknownHostKeyKeyError so the caller
//     can show the user an accept/reject prompt (see
//     PTYService.CheckHostKeyTrust) and retry with trustNewKey set.
func verifyHostKeyCallback(promptForNewKeys, trustNewKey bool) ssh.HostKeyCallback {
	return func(hostname string, _ net.Addr, key ssh.PublicKey) error {
		hosts, err := config.LoadKnownHosts()
		if err != nil {
			return fmt.Errorf("failed to load known hosts: %w", err)
		}
		fingerprint := ssh.FingerprintSHA256(key)

		if existing, ok := hosts[hostname]; ok {
			if existing.Fingerprint != fingerprint {
				return &HostKeyMismatchError{
					HostPort:       hostname,
					OldFingerprint: existing.Fingerprint,
					NewFingerprint: fingerprint,
				}
			}
			return nil
		}

		if promptForNewKeys && !trustNewKey {
			return &UnknownHostKeyError{HostPort: hostname, KeyType: key.Type(), Fingerprint: fingerprint}
		}

		hosts[hostname] = config.KnownHostKey{KeyType: key.Type(), Fingerprint: fingerprint}
		if err := config.SaveKnownHosts(hosts); err != nil {
			return fmt.Errorf("failed to save trusted host key: %w", err)
		}
		return nil
	}
}

// UnknownHostKeyError is returned by verifyHostKeyCallback when
// PromptNewHostKeys is enabled and the host:port has no pinned key yet.
type UnknownHostKeyError struct {
	HostPort    string
	KeyType     string
	Fingerprint string
}

func (e *UnknownHostKeyError) Error() string {
	return fmt.Sprintf("unrecognized host key for %s (%s %s) — accept it before connecting", e.HostPort, e.KeyType, e.Fingerprint)
}

// errHostKeyCaptured is a sentinel used only to abort probeHostKey's
// handshake immediately after the host key becomes available, before any
// authentication is attempted — probing a host's key should never risk
// triggering an auth-failure lockout on the remote server.
var errHostKeyCaptured = errors.New("host key captured")

// probeHostKey connects just far enough to observe the remote host's SSH key
// and reports its type/fingerprint, without attempting authentication or
// leaving a session open. Used by PTYService.CheckHostKeyTrust so the
// frontend can show a real fingerprint in its accept/reject prompt before
// the actual (authenticating) connection is attempted.
func probeHostKey(host string, port int) (keyType, fingerprint string, err error) {
	cfg := &ssh.ClientConfig{
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			keyType = key.Type()
			fingerprint = ssh.FingerprintSHA256(key)
			return errHostKeyCaptured
		},
		Timeout: 10 * time.Second,
	}
	addr := hostPortKey(host, port)
	_, dialErr := sshDialFunc("tcp", addr, cfg)
	if fingerprint == "" {
		return "", "", fmt.Errorf("failed to reach %s: %w", addr, dialErr)
	}
	return keyType, fingerprint, nil
}

// openSSHTerminal dials the remote host and opens a real interactive PTY shell
// over SSH, wiring its output into the same "pty:output"/"pty:closed" Wails
// events used by local and ConPTY sessions — the frontend needs no new
// event-handling code for remote SSH tabs.
func (s *PTYService) openSSHTerminal(tabId string, params RemoteConnectParams) (string, error) {
	authMethods, err := buildSSHAuthMethods(params)
	if err != nil {
		return "", err
	}

	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load app config: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            params.Username,
		Auth:            authMethods,
		HostKeyCallback: verifyHostKeyCallback(appCfg.PromptNewHostKeys, params.TrustNewHostKey),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(params.Host, fmt.Sprintf("%d", params.Port))
	client, err := sshDialFunc("tcp", addr, sshConfig)
	if err != nil {
		return "", fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	sess, err := client.NewSession()
	if err != nil {
		client.Close()
		return "", fmt.Errorf("failed to open SSH session: %w", err)
	}

	stdin, err := sess.StdinPipe()
	if err != nil {
		sess.Close()
		client.Close()
		return "", fmt.Errorf("failed to open stdin pipe: %w", err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		sess.Close()
		client.Close()
		return "", fmt.Errorf("failed to open stdout pipe: %w", err)
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		sess.Close()
		client.Close()
		return "", fmt.Errorf("failed to open stderr pipe: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		sess.Close()
		client.Close()
		return "", fmt.Errorf("failed to request pty: %w", err)
	}
	if err := sess.Shell(); err != nil {
		sess.Close()
		client.Close()
		return "", fmt.Errorf("failed to start remote shell: %w", err)
	}

	session := &ptySession{sshClient: client, sshSess: sess, sshStdin: stdin}

	s.mu.Lock()
	s.sessions[tabId] = session
	s.mu.Unlock()

	// Two pump goroutines (stdout + stderr) share the same cleanup path; the
	// stillOpen guard (mirroring the local-terminal read loop) ensures only
	// one of them actually closes the session and emits "pty:closed".
	go s.pumpSSHOutput(tabId, stdout)
	go s.pumpSSHOutput(tabId, stderr)

	if params.UseTmux {
		sessionName := sanitizeTmuxSessionName(params.TmuxSessionName)
		go func() {
			// `-A` makes this create-or-attach: if sessionName doesn't exist
			// (first connect, or it was destroyed by someone else logging in
			// directly and killing it) tmux creates it fresh; if it does
			// exist, tmux attaches to the existing one, resuming state. No
			// special-case handling is needed for the "session vanished
			// underneath us" edge case — tmux's own -A flag covers it.
			time.Sleep(tmuxAttachDelay)
			stdin.Write([]byte(fmt.Sprintf("tmux new-session -A -s %s\r", sessionName)))
		}()
	}

	return tabId, nil
}

func (s *PTYService) pumpSSHOutput(tabId string, r io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			s.emitFn(s.ctx, "pty:output", PTYOutputEvent{
				TabID: tabId,
				Data:  string(buf[:n]),
			})
		}
		if err != nil {
			s.mu.Lock()
			session, stillOpen := s.sessions[tabId]
			delete(s.sessions, tabId)
			s.mu.Unlock()
			if stillOpen {
				if session.sshSess != nil {
					session.sshSess.Close()
				}
				if session.sshClient != nil {
					session.sshClient.Close()
				}
				s.emitFn(s.ctx, "pty:closed", map[string]string{"tabId": tabId})
			}
			return
		}
	}
}
