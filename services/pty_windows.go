//go:build windows
// +build windows

package services

import (
	"fmt"
	"os"
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

	shell := os.Getenv("ComSpec")
	if shell == "" {
		shell = "cmd.exe"
	}

	cpty, err := WinConPtyStart(shell, 120, 40)
	if err != nil {
		logPty(fmt.Sprintf("WinConPtyStart failed: %v", err))
		return "", fmt.Errorf("WinConPtyStart: %w", err)
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

	// Watcher goroutine — ConPTY's pipes stay open after the shell process
	// exits (e.g. user types "exit"); nothing closes them until we explicitly
	// call Close(), so the read loop below would otherwise block forever and
	// leave the session in limbo. Wait() blocks until the process exits, then
	// forces a Close() to unblock the read loop and trigger normal cleanup.
	//
	// Whichever of this goroutine and the read loop below observes the
	// session first deletes it from the map under the same lock it checks —
	// that's what guarantees Close() is called exactly once. Without deleting
	// here, the read loop (unblocked by this goroutine's Close()) would still
	// find the session present and call Close() a second time, double-closing
	// the same Windows handles — this previously caused STATUS_HEAP_CORRUPTION
	// crashes that took the whole app down with it.
	go func() {
		cpty.Wait(s.ctx)
		s.mu.Lock()
		_, stillOpen := s.sessions[tabId]
		if stillOpen {
			delete(s.sessions, tabId)
		}
		s.mu.Unlock()
		if stillOpen {
			cpty.Close()
		}
	}()

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
	cpty, ok := winPty.(*WinConPty)
	if !ok || cpty == nil {
		return nil
	}
	_, err := cpty.Write([]byte(data))
	return err
}

// resizeConPTY resizes the pseudoconsole to match xterm.js dimensions.
func (s *PTYService) resizeConPTY(winPty interface{}, cols, rows int) error {
	cpty, ok := winPty.(*WinConPty)
	if !ok || cpty == nil {
		return nil
	}
	return cpty.Resize(cols, rows)
}

// closeConPTY closes the pseudoconsole and terminates its attached process.
func (s *PTYService) closeConPTY(winPty interface{}) {
	cpty, ok := winPty.(*WinConPty)
	if !ok || cpty == nil {
		return
	}
	cpty.Close()
}
