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
	ptmx   *os.File       // Unix: pseudoterminal master; Windows: console output reader
	ptyOut *os.File       // Windows only: console input writer (nil on Unix)
	hPC    uintptr        // Windows only: pseudoconsole handle (0 on Unix)
	pid    int            // Windows only: child process ID (0 on Unix)
	cmd    *exec.Cmd      // Unix: the shell command (nil on Windows)
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

func (s *PTYService) OpenNewTerminal(tabId string) (bool, error) {
	if runtime.GOOS == "windows" {
		// Use Windows ConPTY API (Win 10 1809+) for proper pseudoconsole support.
		// Falls back to launching an external console window if ConPTY fails.
		ok, err := s.openWindowsConPTY(tabId)
		if err == nil {
			return ok, nil
		}
		// ConPTY unavailable (pre-Win10 or API failure) — launch external window.
		cmd := exec.Command("cmd.exe", "/c", "start", "cmd.exe")
		return false, cmd.Run()
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return false, fmt.Errorf("failed to start terminal: %w", err)
	}

	s.mu.Lock()
	s.sessions[tabId] = &ptySession{ptmx: ptmx}
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
				delete(s.sessions, tabId)
				s.mu.Unlock()
				ptmx.Close()
				s.emitFn(s.ctx, "pty:closed", map[string]string{"tabId": tabId})
				return
			}
		}
	}()

	return true, nil
}

func (s *PTYService) WriteInput(tabId string, data string) error {
	s.mu.Lock()
	session, ok := s.sessions[tabId]
	s.mu.Unlock()
	if !ok {
		// Fallback to CaptureManager for non-PTY tabs
		if s.captureManager != nil {
			return s.captureManager.WriteInput(tabId, data)
		}
		return nil
	}
	if runtime.GOOS == "windows" {
		return s.writeConPTYInput(tabId, data)
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
	if runtime.GOOS == "windows" {
		return s.resizeConPTY(tabId, cols, rows)
	}
	return pty.Setsize(session.ptmx, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
}
