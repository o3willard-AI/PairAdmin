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
// The context_* and redactions fields are statistics about the terminal
// context that was redacted — they never carry the context content itself.
// All three are omitempty so older events (which have no stats) serialize
// unchanged.
type AuditEntry struct {
	Event         string         `json:"event"`
	SessionID     string         `json:"session_id"`
	TerminalID    string         `json:"terminal_id,omitempty"`
	Content       string         `json:"content,omitempty"`
	ContextLines  int            `json:"context_lines,omitempty"`
	ContextBytes  int            `json:"context_bytes,omitempty"`
	Redactions    map[string]int `json:"redactions,omitempty"`
}

// AuditLogger writes JSON-lines audit records to a rotating log file.
type AuditLogger struct {
	logger  *slog.Logger
	rotator *lumberjack.Logger
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

	return &AuditLogger{logger: slog.New(handler), rotator: rotator}, nil
}

// Close closes the underlying log file handle. Close is nil-safe.
// Required on Windows, where an open file handle blocks deletion/rename of
// its containing directory (e.g. test TempDir cleanup); on Unix this is a
// no-op in practice but still good hygiene.
func (a *AuditLogger) Close() error {
	if a == nil || a.rotator == nil {
		return nil
	}
	return a.rotator.Close()
}

// Write records an audit entry to the log file.
// Write is nil-safe: a nil receiver or nil logger returns nil without panicking.
func (a *AuditLogger) Write(entry AuditEntry) error {
	if a == nil || a.logger == nil {
		return nil
	}

	// Context statistics honor the same "omitempty" semantics as the struct's
	// JSON tags: events that carry no stats (session_start, ai_response, ...)
	// are emitted exactly as before, with no zero-valued placeholder fields.
	attrs := make([]any, 0, 7)
	attrs = append(attrs, slog.String("event", entry.Event))
	attrs = append(attrs, slog.String("session_id", entry.SessionID))
	attrs = append(attrs, slog.String("terminal_id", entry.TerminalID))
	attrs = append(attrs, slog.String("content", entry.Content))
	if entry.ContextLines != 0 {
		attrs = append(attrs, slog.Int("context_lines", entry.ContextLines))
	}
	if entry.ContextBytes != 0 {
		attrs = append(attrs, slog.Int("context_bytes", entry.ContextBytes))
	}
	if len(entry.Redactions) != 0 {
		attrs = append(attrs, slog.Any("redactions", entry.Redactions))
	}

	a.logger.Info("audit", attrs...)

	return nil
}
