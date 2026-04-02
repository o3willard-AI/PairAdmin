package audit

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// AuditEntry represents a single auditable event in the system.
type AuditEntry struct {
	Event      string `json:"event"`
	SessionID  string `json:"session_id"`
	TerminalID string `json:"terminal_id,omitempty"`
	Content    string `json:"content,omitempty"`
}

// AuditLogger writes JSON-lines audit records to a rotating log file.
type AuditLogger struct {
	logger *slog.Logger
}

// NewAuditLogger creates an AuditLogger that writes to logDir.
// Log files are named audit-YYYY-MM-DD.jsonl and rotated by lumberjack
// (MaxSize 100 MB, MaxAge 30 days, no compression).
func NewAuditLogger(logDir string) (*AuditLogger, error) {
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, fmt.Errorf("audit: create log directory: %w", err)
	}

	filename := filepath.Join(logDir, fmt.Sprintf("audit-%s.jsonl", time.Now().Format("2006-01-02")))

	rotator := &lumberjack.Logger{
		Filename: filename,
		MaxSize:  100,
		MaxAge:   30,
		Compress: false,
	}

	handler := slog.NewJSONHandler(rotator, &slog.HandlerOptions{Level: slog.LevelInfo})

	return &AuditLogger{logger: slog.New(handler)}, nil
}

// Write records an audit entry to the log file.
// Write is nil-safe: a nil receiver or nil logger returns nil without panicking.
func (a *AuditLogger) Write(entry AuditEntry) error {
	if a == nil || a.logger == nil {
		return nil
	}

	a.logger.Info("audit",
		slog.String("event", entry.Event),
		slog.String("session_id", entry.SessionID),
		slog.String("terminal_id", entry.TerminalID),
		slog.String("content", entry.Content),
	)

	return nil
}
