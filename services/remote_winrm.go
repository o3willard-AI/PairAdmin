package services

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/masterzen/winrm"
)

// winrmClientFactory is an injectable seam for tests — production code builds
// an NTLM-authenticated winrm.Client, tests can substitute a fake Transporter
// via winrm.Parameters.TransportDecorator without a live WinRM endpoint.
var winrmClientFactory = func(endpoint *winrm.Endpoint, user, password string) (*winrm.Client, error) {
	// Copy DefaultParameters rather than mutating the shared package-level
	// pointer, which would race across concurrent connections.
	params := *winrm.DefaultParameters
	params.TransportDecorator = func() winrm.Transporter { return &winrm.ClientNTLM{} }
	return winrm.NewClientWithParameters(endpoint, user, password, &params)
}

// winrmSession wraps a persistent remote cmd.exe process reached over WinRM.
// Unlike SSH, WinRM has no character-mode PTY: keystrokes are line-buffered on
// this side (see feedWinRMInput) and forwarded a full line at a time to the
// remote process's stdin; output streams back as it's produced. There is no
// resize concept and no SIGINT equivalent for an in-flight command.
type winrmSession struct {
	client *winrm.Client
	shell  *winrm.Shell
	cmd    *winrm.Command
	cancel context.CancelFunc

	mu      sync.Mutex
	lineBuf []byte
}

// Close terminates the remote command and shell. Safe to call more than once.
func (w *winrmSession) Close() error {
	if w.cancel != nil {
		w.cancel()
	}
	if w.cmd != nil {
		w.cmd.Close()
	}
	if w.shell != nil {
		w.shell.Close()
	}
	return nil
}

// openWinRMTerminal connects to a remote Windows host over WinRM (NTLM auth
// only) and starts a persistent cmd.exe process. Transport: TLS when
// params.UseTLS is true (HTTPS, the default on new UI connections), plaintext
// WinRM when false — an explicit opt-in for legacy hosts, and the Go
// zero-value false keeps every existing saved host connecting exactly as
// before. With TLS on, params.InsecureSkipVerify selects whether the server
// certificate is verified (off) or not (opt-in for self-signed certs).
// Stdin/stdout/stderr are wired into the same
// "pty:output"/"pty:closed" events used by every other terminal kind, so the
// frontend needs no new event-handling code. Because WinRM has no true PTY,
// output only appears after a full line is submitted (see feedWinRMInput),
// there is no keystroke-level interactivity (no arrow keys, no tab-complete),
// and Ctrl+C is a documented no-op rather than a real interrupt.
func (s *PTYService) openWinRMTerminal(tabId string, params RemoteConnectParams) (string, error) {
	if params.AuthType != RemoteAuthPassword {
		return "", fmt.Errorf("WinRM only supports password authentication in this version")
	}

	// Port default: 5986 with TLS, 5985 plaintext. An explicitly set port
	// always wins — never force 5985 onto a TLS connection.
	port := params.Port
	if port == 0 {
		if params.UseTLS {
			port = 5986
		} else {
			port = 5985
		}
	}

	// winrm.NewEndpoint signature: (host, port, https, insecure, CA, cert,
	// key, timeout) — the 3rd arg selects TLS, the 4th skips cert
	// verification. Note the ordering is the REVERSE of the params above.
	endpoint := winrm.NewEndpoint(params.Host, port, params.UseTLS, params.InsecureSkipVerify, nil, nil, nil, 0)
	client, err := winrmClientFactory(endpoint, params.Username, params.Password)
	if err != nil {
		return "", fmt.Errorf("failed to create WinRM client: %w", err)
	}

	shell, err := client.CreateShell()
	if err != nil {
		return "", fmt.Errorf("failed to open WinRM shell on %s: %w", params.Host, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd, err := shell.ExecuteWithContext(ctx, "cmd.exe")
	if err != nil {
		cancel()
		shell.Close()
		return "", fmt.Errorf("failed to start remote cmd.exe: %w", err)
	}

	winrmSess := &winrmSession{client: client, shell: shell, cmd: cmd, cancel: cancel}

	s.mu.Lock()
	s.sessions[tabId] = &ptySession{winrm: winrmSess}
	s.mu.Unlock()

	go s.pumpWinRMOutput(tabId, cmd.Stdout)
	go s.pumpWinRMOutput(tabId, cmd.Stderr)

	return tabId, nil
}

func (s *PTYService) pumpWinRMOutput(tabId string, r io.Reader) {
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
			if stillOpen && session.winrm != nil {
				session.winrm.Close()
				s.emitFn(s.ctx, "pty:closed", map[string]string{"tabId": tabId})
			}
			return
		}
	}
}

// processWinRMKeystrokes applies line-buffering semantics to a chunk of raw
// terminal input: Ctrl+C is swallowed (no SIGINT equivalent over WinRM),
// backspace/delete edits the pending buffer, and each \r or \n completes a
// line. Returns the completed lines (in order) and the new pending buffer.
// Pulled out as a pure function so this logic is unit-testable without a live
// WinRM session (winrm.Command is a concrete type with no exported
// constructor, so it cannot be faked directly in tests).
func processWinRMKeystrokes(buf []byte, data string) (completedLines []string, newBuf []byte) {
	for i := 0; i < len(data); i++ {
		b := data[i]
		switch {
		case b == 0x03: // Ctrl+C — no-op, no SIGINT equivalent over WinRM
			continue
		case b == 0x7f || b == 0x08: // Backspace/Delete
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
			}
		case b == '\r' || b == '\n':
			completedLines = append(completedLines, string(buf))
			buf = buf[:0]
			// Treat a CRLF pair as a single line terminator, not two.
			if b == '\r' && i+1 < len(data) && data[i+1] == '\n' {
				i++
			}
		default:
			buf = append(buf, b)
		}
	}
	return completedLines, buf
}

// feedWinRMInput buffers keystrokes for a WinRM tab until a full line is
// submitted, then dispatches that line to the remote process's stdin.
// Buffering (processWinRMKeystrokes) is synchronous so keystroke order stays
// deterministic even though WriteInput may be called rapidly from the
// frontend's onData handler; only the resulting network write runs in its
// own goroutine, so a slow or stalled remote connection can't block the caller.
func (s *PTYService) feedWinRMInput(tabId string, session *winrmSession, data string) {
	session.mu.Lock()
	lines, newBuf := processWinRMKeystrokes(session.lineBuf, data)
	session.lineBuf = newBuf
	session.mu.Unlock()

	for _, line := range lines {
		cmd := session.cmd
		go func(line string) {
			if _, err := cmd.Stdin.Write([]byte(line + "\r\n")); err != nil {
				s.emitFn(s.ctx, "pty:output", PTYOutputEvent{
					TabID: tabId,
					Data:  fmt.Sprintf("\r\n[winrm write error: %v]\r\n", err),
				})
			}
		}(line)
	}
}
