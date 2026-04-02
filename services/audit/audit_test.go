package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewAuditLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "audit-logs")

	logger, err := NewAuditLogger(logDir)
	if err != nil {
		t.Fatalf("NewAuditLogger returned error: %v", err)
	}
	if logger == nil {
		t.Fatal("NewAuditLogger returned nil logger")
	}

	// Verify log directory was created
	info, err := os.Stat(logDir)
	if err != nil {
		t.Fatalf("log directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("log directory path is not a directory")
	}
}

func TestAuditLoggerWrite(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewAuditLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewAuditLogger returned error: %v", err)
	}

	entry := AuditEntry{
		Event:      "user_message",
		SessionID:  "test-uuid",
		TerminalID: "tmux:%3",
		Content:    "hello",
	}

	if err := logger.Write(entry); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	// Find the log file
	filename := filepath.Join(tmpDir, "audit-"+time.Now().Format("2006-01-02")+".jsonl")
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("could not read log file %s: %v", filename, err)
	}

	content := string(data)
	if !strings.Contains(content, `"user_message"`) {
		t.Errorf("log does not contain event user_message; got: %s", content)
	}
	if !strings.Contains(content, `"test-uuid"`) {
		t.Errorf("log does not contain session_id test-uuid; got: %s", content)
	}
	if !strings.Contains(content, `"tmux:%3"`) {
		t.Errorf("log does not contain terminal_id tmux:%%3; got: %s", content)
	}
	if !strings.Contains(content, `"hello"`) {
		t.Errorf("log does not contain content hello; got: %s", content)
	}
}

func TestAuditLoggerNilSafe(t *testing.T) {
	var logger *AuditLogger

	// Must not panic
	err := logger.Write(AuditEntry{Event: "test"})
	if err != nil {
		t.Errorf("nil receiver Write returned non-nil error: %v", err)
	}
}

func TestAuditLoggerNilLogger(t *testing.T) {
	logger := &AuditLogger{logger: nil}

	// Must not panic
	err := logger.Write(AuditEntry{Event: "test"})
	if err != nil {
		t.Errorf("nil logger field Write returned non-nil error: %v", err)
	}
}

func TestAuditLogFilename(t *testing.T) {
	tmpDir := t.TempDir()

	logger, err := NewAuditLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewAuditLogger returned error: %v", err)
	}

	// Write one entry to trigger file creation
	_ = logger.Write(AuditEntry{Event: "session_start", SessionID: "sid"})

	today := time.Now().Format("2006-01-02")
	expected := "audit-" + today + ".jsonl"

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("could not read tmpDir: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Name() == expected {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected log file %s not found in %s", expected, tmpDir)
	}
}

func TestAuditEntryAllEvents(t *testing.T) {
	events := []string{
		"session_start",
		"session_end",
		"user_message",
		"ai_response",
		"command_copied",
	}

	tmpDir := t.TempDir()
	logger, err := NewAuditLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewAuditLogger returned error: %v", err)
	}

	for _, event := range events {
		if err := logger.Write(AuditEntry{Event: event, SessionID: "test-sid"}); err != nil {
			t.Errorf("Write(%s) returned error: %v", event, err)
		}
	}

	filename := filepath.Join(tmpDir, "audit-"+time.Now().Format("2006-01-02")+".jsonl")
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("could not read log file: %v", err)
	}
	content := string(data)

	for _, event := range events {
		if !strings.Contains(content, `"`+event+`"`) {
			t.Errorf("log does not contain event %s; content: %s", event, content)
		}
	}
}
