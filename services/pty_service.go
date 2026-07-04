package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"pairadmin/services/capture"
	"pairadmin/services/keychain"

	"github.com/creack/pty"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/ssh"
)

type ptySession struct {
	ptmx   *os.File    // Unix: pseudoterminal master
	winPty interface{} // Windows only: *conpty.ConPty (nil on Unix / non-ConPTY tabs)
	pid    int         // Windows only: child process ID (0 on Unix)
	cmd    *exec.Cmd   // Store cmd to allow killing process

	sshClient *ssh.Client    // non-nil for "ssh:" remote sessions
	sshSess   *ssh.Session   // non-nil for "ssh:" remote sessions
	sshStdin  io.WriteCloser // non-nil for "ssh:" remote sessions

	winrm *winrmSession // non-nil for "winrm:" remote sessions
}

// PTYOutputEvent is emitted on "pty:output" events.
type PTYOutputEvent struct {
	TabID string `json:"tabId"`
	Data  string `json:"data"`
}

// PTYService manages interactive shell sessions backed by pseudoterminals,
// as well as remote SSH and WinRM sessions.
type PTYService struct {
	ctx            context.Context
	mu             sync.Mutex
	sessions       map[string]*ptySession
	emitFn         func(ctx context.Context, event string, optionalData ...interface{})
	captureManager *capture.CaptureManager
	keychainClient *keychain.Client
}

func NewPTYService() *PTYService {
	return &PTYService{
		sessions: make(map[string]*ptySession),
		emitFn:   wailsruntime.EventsEmit,
	}
}

func (s *PTYService) SetCaptureManager(manager *capture.CaptureManager) {
	s.captureManager = manager
}

// SetKeychainClient wires the OS keychain client used to resolve saved remote
// host credentials (by RemoteHost.ID) when reconnecting via SavedHostId.
func (s *PTYService) SetKeychainClient(client *keychain.Client) {
	s.keychainClient = client
}

func (s *PTYService) Startup(ctx context.Context) {
	s.ctx = ctx
}

func (s *PTYService) OpenNewTerminal(tabId string) (string, error) {
	if runtime.GOOS == "windows" {
		return s.openWindowsTerminal(tabId)
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to start terminal: %w", err)
	}

	s.mu.Lock()
	s.sessions[tabId] = &ptySession{ptmx: ptmx, cmd: cmd}
	s.mu.Unlock()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				s.emitFn(s.ctx, "pty:output", PTYOutputEvent{
					TabID: tabId,
					Data:  string(buf[:n]),
				})
			}
			if err != nil {
				s.mu.Lock()
				_, stillOpen := s.sessions[tabId]
				delete(s.sessions, tabId)
				s.mu.Unlock()
				if stillOpen {
					ptmx.Close()
				}
				s.emitFn(s.ctx, "pty:closed", map[string]string{"tabId": tabId})
				return
			}
		}
	}()

	return tabId, nil
}

// OpenRemoteTerminal opens a remote SSH or WinRM session for tabId (expected to be
// prefixed "ssh:"/"winrm:" by the frontend, matching the tmux:/atspi:/windows: pane-ID
// namespacing convention used elsewhere). It never modifies OpenNewTerminal's behavior
// or signature — this is a sibling entry point for the "+ New" remote connection dialog.
func (s *PTYService) OpenRemoteTerminal(tabId string, params RemoteConnectParams) (string, error) {
	resolved, err := s.resolveRemoteCredentials(params)
	if err != nil {
		return "", err
	}
	switch resolved.Kind {
	case RemoteKindSSH:
		return s.openSSHTerminal(tabId, resolved)
	case RemoteKindWinRM:
		return s.openWinRMTerminal(tabId, resolved)
	default:
		return "", fmt.Errorf("unknown remote kind: %q", resolved.Kind)
	}
}

// resolveRemoteCredentials fills in a missing password/passphrase from the keychain
// when SavedHostId is set and the frontend didn't supply one inline (a one-click
// reconnect to a remembered host rather than a fresh connection form submission).
func (s *PTYService) resolveRemoteCredentials(params RemoteConnectParams) (RemoteConnectParams, error) {
	if params.SavedHostId == "" || s.keychainClient == nil {
		return params, nil
	}
	if params.AuthType == RemoteAuthPassword && params.Password == "" {
		pw, err := s.keychainClient.Get(remoteKeychainKey(params.SavedHostId, "password"))
		if err != nil {
			return params, fmt.Errorf("failed to load saved password: %w", err)
		}
		params.Password = pw
	}
	if params.AuthType == RemoteAuthPrivateKey && params.Passphrase == "" {
		// Passphrase may legitimately be empty (unencrypted key) — ignore lookup errors.
		if pp, err := s.keychainClient.Get(remoteKeychainKey(params.SavedHostId, "passphrase")); err == nil {
			params.Passphrase = pp
		}
	}
	return params, nil
}

func (s *PTYService) CloseTerminal(tabId string) error {
	s.mu.Lock()
	session, ok := s.sessions[tabId]
	if ok {
		delete(s.sessions, tabId)
	}
	s.mu.Unlock()

	if !ok {
		// If it's a discovered window (not opened via PTYService), we might not be able to "close" it
		// without a PID. For now, just return.
		return nil
	}

	if session.ptmx != nil {
		session.ptmx.Close()
	}
	if session.winPty != nil {
		s.closeConPTY(session.winPty)
	}
	if session.sshSess != nil {
		session.sshSess.Close()
	}
	if session.sshClient != nil {
		session.sshClient.Close()
	}
	if session.winrm != nil {
		session.winrm.Close()
	}
	if session.cmd != nil && session.cmd.Process != nil {
		pid := uint32(session.cmd.Process.Pid)
		// Remove from whitelist
		if s.captureManager != nil {
			s.captureManager.RemoveAllowedPid(pid)
		}

		// Force kill the process group on Windows if possible, or just the process.
		if runtime.GOOS == "windows" {
			// On Windows, taskkill is often more effective at cleaning up conhost.
			exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid)).Run()
		} else {
			session.cmd.Process.Kill()
		}
	}
	return nil
}

func (s *PTYService) WriteInput(tabId string, data string) error {
	s.mu.Lock()
	session, ok := s.sessions[tabId]
	s.mu.Unlock()

	if ok && session.sshStdin != nil {
		_, err := session.sshStdin.Write([]byte(data))
		return err
	}
	if ok && session.winrm != nil {
		// WinRM has no live PTY stream — buffer keystrokes per-tab until a full
		// line is submitted. Buffering itself is synchronous (and must stay so,
		// to keep keystroke order deterministic); only the resulting network
		// write is dispatched async, inside feedWinRMInput.
		s.feedWinRMInput(tabId, session.winrm, data)
		return nil
	}

	// If it's a native Windows console (no PTY), route to CaptureManager
	if !ok || (session.ptmx == nil && session.winPty == nil) {
		if s.captureManager != nil {
			return s.captureManager.WriteInput(tabId, data)
		}
		return nil
	}
	if runtime.GOOS == "windows" && session.winPty != nil {
		return s.writeConPTYInput(session.winPty, data)
	}
	_, err := session.ptmx.Write([]byte(data))
	return err
}

func (s *PTYService) ResizeTerminal(tabId string, cols, rows int) error {
	s.mu.Lock()
	session, ok := s.sessions[tabId]
	s.mu.Unlock()
	if !ok {
		return nil // not a PTY tab — silently ignore
	}
	if session.sshSess != nil {
		return session.sshSess.WindowChange(rows, cols)
	}
	if session.winrm != nil {
		return nil // WinRM has no PTY concept — resize is a no-op
	}
	if runtime.GOOS == "windows" && session.winPty != nil {
		return s.resizeConPTY(session.winPty, cols, rows)
	}
	return pty.Setsize(session.ptmx, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
}

// GetWindowsContent provides a bridge for the frontend to pull content from Windows console windows.
// This is only used on Windows and only for non-PTY tabs (native cmd.exe/powershell windows).
func (s *PTYService) GetWindowsContent(tabId string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", nil
	}
	if s.captureManager == nil {
		return "", nil
	}

	return s.captureManager.GetWindowsContent(tabId)
}
