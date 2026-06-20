//go:build windows
// +build windows

package services

import (
	"fmt"
	"os"

	"github.com/UserExistsError/conpty"
)

func logPty(msg string) {
	f, err := os.OpenFile("pty_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		f.WriteString(fmt.Sprintf("[%s] %s\n", os.Getenv("USERNAME"), msg))
	}
}

// openWindowsConPTY creates a pseudoconsole on Windows 10 1809+,
// spawns the user's preferred shell inside it, and wires its I/O
// into the PTY session so xterm.js gets a real bidirectional stream.
func (s *PTYService) openWindowsConPTY(tabId string) (string, error) {
	logPty("openWindowsConPTY called")

	if !conpty.IsConPtyAvailable() {
		logPty("ConPTY not supported on this Windows version")
		return "", fmt.Errorf("ConPTY not supported")
	}

	shell := os.Getenv("ComSpec")
	if shell == "" {
		shell = "cmd.exe"
	}

	cpty, err := conpty.Start(shell, conpty.ConPtyDimensions(120, 40))
	if err != nil {
		logPty(fmt.Sprintf("conpty.Start failed: %v", err))
		return "", fmt.Errorf("conpty.Start: %w", err)
	}

	logPty(fmt.Sprintf("Successfully started %s (PID: %d)", shell, cpty.Pid()))

	session := &ptySession{
		winPty: cpty,
		pid:    cpty.Pid(),
		cmd:    nil,
	}

	s.mu.Lock()
	s.sessions[tabId] = session
	s.mu.Unlock()

	// Read goroutine — pumps console output to xterm.js.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := cpty.Read(buf)
			if n > 0 {
				logPty(fmt.Sprintf("Read %d bytes for %s: %q", n, tabId, string(buf[:n])))
				s.emitFn(s.ctx, "pty:output", PTYOutputEvent{
					TabID: tabId,
					Data:  string(buf[:n]),
				})
			}
			if err != nil {
				logPty(fmt.Sprintf("Read goroutine exiting for %s: %v", tabId, err))
				s.mu.Lock()
				_, stillOpen := s.sessions[tabId]
				delete(s.sessions, tabId)
				if stillOpen {
					cpty.Close()
				}
				s.mu.Unlock()
				s.emitFn(s.ctx, "pty:closed", map[string]string{"tabId": tabId})
				return
			}
		}
	}()

	return tabId, nil
}

// writeConPTYInput writes data to the pseudoconsole's input stream.
func (s *PTYService) writeConPTYInput(winPty interface{}, data string) error {
	cpty, ok := winPty.(*conpty.ConPty)
	if !ok || cpty == nil {
		return nil
	}
	_, err := cpty.Write([]byte(data))
	return err
}

// resizeConPTY resizes the pseudoconsole to match xterm.js dimensions.
func (s *PTYService) resizeConPTY(winPty interface{}, cols, rows int) error {
	cpty, ok := winPty.(*conpty.ConPty)
	if !ok || cpty == nil {
		return nil
	}
	return cpty.Resize(cols, rows)
}

// closeConPTY closes the pseudoconsole and terminates its attached process.
func (s *PTYService) closeConPTY(winPty interface{}) {
	cpty, ok := winPty.(*conpty.ConPty)
	if !ok || cpty == nil {
		return
	}
	cpty.Close()
}
