package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"pairadmin/services/capture"

	"github.com/creack/pty"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ptySession struct {
	ptmx   *os.File    // Unix: pseudoterminal master
	winPty interface{} // Windows only: *conpty.ConPty (nil on Unix / non-ConPTY tabs)
	pid    int         // Windows only: child process ID (0 on Unix)
	cmd    *exec.Cmd   // Store cmd to allow killing process
}

// PTYOutputEvent is emitted on "pty:output" events.
type PTYOutputEvent struct {
	TabID string `json:"tabId"`
	Data  string `json:"data"`
}

// PTYService manages interactive shell sessions backed by pseudoterminals.
type PTYService struct {
	ctx            context.Context
	mu             sync.Mutex
	sessions       map[string]*ptySession
	emitFn         func(ctx context.Context, event string, optionalData ...interface{})
	captureManager *capture.CaptureManager
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
