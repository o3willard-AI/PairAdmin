package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ptySession struct {
	ptmx *os.File
}

// PTYOutputEvent is emitted on "pty:output" events.
type PTYOutputEvent struct {
	TabID string `json:"tabId"`
	Data  string `json:"data"`
}

// PTYService manages interactive shell sessions backed by pseudoterminals.
type PTYService struct {
	ctx      context.Context
	mu       sync.Mutex
	sessions map[string]*ptySession
	emitFn   func(ctx context.Context, event string, optionalData ...interface{})
}

func NewPTYService() *PTYService {
	return &PTYService{
		sessions: make(map[string]*ptySession),
		emitFn:   runtime.EventsEmit,
	}
}

func (s *PTYService) Startup(ctx context.Context) {
	s.ctx = ctx
}

func (s *PTYService) OpenNewTerminal(tabId string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start terminal: %w", err)
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

	return nil
}

func (s *PTYService) WriteInput(tabId string, data string) error {
	s.mu.Lock()
	session, ok := s.sessions[tabId]
	s.mu.Unlock()
	if !ok {
		return nil // not a PTY tab — silently ignore
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
	return pty.Setsize(session.ptmx, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
}
