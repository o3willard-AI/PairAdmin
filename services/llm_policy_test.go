package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pairadmin/services/audit"
)

// writePolicyConfig writes a config.yaml with the given disabled_providers
// list, simulating a user hand-editing the file to restrict themselves.
func writePolicyConfig(t *testing.T, disabled []string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	if err := os.MkdirAll(filepath.Join(dir, ".pairadmin"), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	body := "scanner_enabled: false\n"
	if len(disabled) > 0 {
		body += "disabled_providers:\n"
		for _, d := range disabled {
			body += "  - " + d + "\n"
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".pairadmin", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// auditEventNames returns the Event field of every JSON-lines audit entry in
// the given log directory.
func auditEventNames(t *testing.T, logDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("read audit dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(logDir, e.Name()))
		if err != nil {
			t.Fatalf("read audit file: %v", err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if line == "" {
				continue
			}
			var entry audit.AuditEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				t.Fatalf("unmarshal audit line %q: %v", line, err)
			}
			names = append(names, entry.Event)
		}
	}
	return names
}

func containsEvent(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

// TestSendMessage_PolicyDeniedIsBlockedAndAudited pins enforcement point 2 at
// the service layer: a provider the local policy denies is refused BEFORE any
// outbound call, the user gets a clear error, and an audit event records the
// denial.
//
// Mutation check: deleting the `if err := llm.CheckRequest(...)` block from
// SendMessage makes the "request was allowed through" assertion FAIL, and
// deleting only the `s.auditLogger.Write(...)` inside it makes the
// "denial was audited" assertion FAIL.
func TestSendMessage_PolicyDeniedIsBlockedAndAudited(t *testing.T) {
	writePolicyConfig(t, []string{"openai"})

	logDir := t.TempDir()
	logger, err := audit.NewAuditLogger(logDir)
	if err != nil {
		t.Fatalf("new audit logger: %v", err)
	}
	defer logger.Close()

	provider := &capturingMockProvider{}
	svc := &LLMService{
		cfg:            Config{Provider: "openai", Model: "gpt-4.1"},
		activeProvider: provider,
		auditLogger:    logger,
		sessionID:      "sess-1",
		emitFn:         func(_ context.Context, _ string, _ ...interface{}) {},
	}
	svc.ctx = context.Background()

	err = svc.SendMessage("tab-1", "hello", "")
	if err == nil {
		t.Fatal("SendMessage allowed a policy-denied provider through")
	}
	if !strings.Contains(err.Error(), "openai") {
		t.Errorf("error should name the denied provider, got: %v", err)
	}
	// The request must not have reached the provider. SendMessage streams
	// asynchronously on the allowed path, so give any stray goroutine the same
	// settle time the positive control uses before concluding nothing ran.
	time.Sleep(200 * time.Millisecond)
	if len(provider.received) != 0 {
		t.Errorf("denied request still reached the provider (%d messages)", len(provider.received))
	}
	// And the denial is on the record.
	if names := auditEventNames(t, logDir); !containsEvent(names, "llm_policy_denied") {
		t.Errorf("no llm_policy_denied audit event; got %v", names)
	}
}

// TestSendMessage_PolicyAllowedIsUnaffected is the positive control: with no
// restriction configured, the same call still goes through and audits nothing
// policy-related. Without this, a CheckRequest that denied unconditionally
// would pass the test above.
//
// Mutation check: making CheckRequest always return an error makes this test
// FAIL on the "provider was called" assertion.
func TestSendMessage_PolicyAllowedIsUnaffected(t *testing.T) {
	writePolicyConfig(t, nil)

	logDir := t.TempDir()
	logger, err := audit.NewAuditLogger(logDir)
	if err != nil {
		t.Fatalf("new audit logger: %v", err)
	}
	defer logger.Close()

	provider := &capturingMockProvider{}
	svc := &LLMService{
		cfg:            Config{Provider: "openai", Model: "gpt-4.1"},
		activeProvider: provider,
		auditLogger:    logger,
		sessionID:      "sess-2",
		emitFn:         func(_ context.Context, _ string, _ ...interface{}) {},
	}
	svc.ctx = context.Background()

	if err := svc.SendMessage("tab-1", "hello", ""); err != nil {
		t.Fatalf("unrestricted policy blocked a normal request: %v", err)
	}
	// Streaming is asynchronous, so settle before asserting it happened.
	time.Sleep(200 * time.Millisecond)
	if len(provider.received) == 0 {
		t.Error("provider was never called; the positive control is not exercising the path")
	}
	if names := auditEventNames(t, logDir); containsEvent(names, "llm_policy_denied") {
		t.Errorf("an allowed request wrote a denial audit event: %v", names)
	}
}

// TestSendMessage_PolicyAuditCarriesNoCredentials pins that the denial audit
// entry names the resource but carries no API key, even though the service
// holds one for the denied provider — the audit log is written to disk and
// must not become a credential store.
//
// Mutation check: writing the resolved API key into the AuditEntry's Content
// alongside the error makes this test FAIL.
func TestSendMessage_PolicyAuditCarriesNoCredentials(t *testing.T) {
	writePolicyConfig(t, []string{"openai"})

	logDir := t.TempDir()
	logger, err := audit.NewAuditLogger(logDir)
	if err != nil {
		t.Fatalf("new audit logger: %v", err)
	}
	defer logger.Close()

	const secret = "sk-live-SHOULD-NEVER-BE-AUDITED"
	svc := &LLMService{
		cfg:            Config{Provider: "openai", Model: "gpt-4.1", OpenAIKey: secret},
		activeProvider: &capturingMockProvider{},
		auditLogger:    logger,
		sessionID:      "sess-3",
		emitFn:         func(_ context.Context, _ string, _ ...interface{}) {},
	}
	_ = svc.SendMessage("tab-1", "hello", "")

	entries, err := os.ReadDir(logDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("no audit file written: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(logDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if strings.Contains(string(b), secret) {
		t.Errorf("audit log contains the API key %q", secret)
	}
	if !strings.Contains(string(b), "llm_policy_denied") {
		t.Error("expected a policy denial entry naming the event")
	}
}
