package services

import (
	"reflect"
	"testing"

	"github.com/masterzen/winrm"
)

func TestProcessWinRMKeystrokes_SimpleLine(t *testing.T) {
	lines, buf := processWinRMKeystrokes(nil, "dir\n")
	if !reflect.DeepEqual(lines, []string{"dir"}) {
		t.Errorf("expected completed line %q, got %v", "dir", lines)
	}
	if len(buf) != 0 {
		t.Errorf("expected empty remaining buffer, got %q", buf)
	}
}

func TestProcessWinRMKeystrokes_PartialLineBuffered(t *testing.T) {
	lines, buf := processWinRMKeystrokes(nil, "di")
	if len(lines) != 0 {
		t.Errorf("expected no completed lines yet, got %v", lines)
	}
	if string(buf) != "di" {
		t.Errorf("expected buffered 'di', got %q", buf)
	}

	// Continue typing across a second WriteInput call.
	lines, buf = processWinRMKeystrokes(buf, "r\r")
	if !reflect.DeepEqual(lines, []string{"dir"}) {
		t.Errorf("expected completed line 'dir' after continuation, got %v", lines)
	}
	if len(buf) != 0 {
		t.Errorf("expected empty remaining buffer, got %q", buf)
	}
}

func TestProcessWinRMKeystrokes_Backspace(t *testing.T) {
	// User types "dirr" then backspaces once, then Enter -> should submit "dir".
	lines, buf := processWinRMKeystrokes(nil, "dirr\x7f\r")
	if !reflect.DeepEqual(lines, []string{"dir"}) {
		t.Errorf("expected 'dir' after backspace correction, got %v", lines)
	}
	if len(buf) != 0 {
		t.Errorf("expected empty remaining buffer, got %q", buf)
	}
}

func TestProcessWinRMKeystrokes_BackspaceOnEmptyBufferIsNoop(t *testing.T) {
	lines, buf := processWinRMKeystrokes(nil, "\x7f\x7fdir\r")
	if !reflect.DeepEqual(lines, []string{"dir"}) {
		t.Errorf("expected 'dir', leading backspaces on empty buffer should no-op, got %v", lines)
	}
	if len(buf) != 0 {
		t.Errorf("expected empty remaining buffer, got %q", buf)
	}
}

func TestProcessWinRMKeystrokes_CtrlCIsSwallowed(t *testing.T) {
	// Ctrl+C (0x03) mid-line should be dropped, not submitted or buffered —
	// WinRM has no SIGINT equivalent for an in-flight remote command.
	lines, buf := processWinRMKeystrokes(nil, "di\x03r\r")
	if !reflect.DeepEqual(lines, []string{"dir"}) {
		t.Errorf("expected Ctrl+C to be swallowed leaving 'dir', got %v", lines)
	}
	if len(buf) != 0 {
		t.Errorf("expected empty remaining buffer, got %q", buf)
	}
}

func TestProcessWinRMKeystrokes_MultipleLinesInOneChunk(t *testing.T) {
	lines, buf := processWinRMKeystrokes(nil, "cd C:\\\r\ndir\r\n")
	if !reflect.DeepEqual(lines, []string{"cd C:\\", "dir"}) {
		t.Errorf("expected two completed lines in order, got %v", lines)
	}
	if len(buf) != 0 {
		t.Errorf("expected empty remaining buffer, got %q", buf)
	}
}

func TestOpenWinRMTerminal_RejectsNonPasswordAuth(t *testing.T) {
	svc, _ := newTestPTYService()

	// Swap the factory with one that fails the test if called — auth
	// validation must happen before attempting to connect.
	orig := winrmClientFactory
	defer func() { winrmClientFactory = orig }()
	called := false
	winrmClientFactory = func(endpoint *winrm.Endpoint, user, password string) (*winrm.Client, error) {
		called = true
		return orig(endpoint, user, password)
	}

	_, err := svc.openWinRMTerminal("winrm:test-tab", RemoteConnectParams{
		Kind:     RemoteKindWinRM,
		Host:     "192.0.2.1", // TEST-NET-1, guaranteed unreachable
		Port:     5985,
		Username: "Administrator",
		AuthType: RemoteAuthPrivateKey, // unsupported for WinRM
	})
	if err == nil {
		t.Fatal("expected error for non-password WinRM auth, got nil")
	}
	if called {
		t.Error("expected winrmClientFactory not to be called when auth validation fails first")
	}

	svc.mu.Lock()
	_, present := svc.sessions["winrm:test-tab"]
	svc.mu.Unlock()
	if present {
		t.Error("expected no session stored after auth validation failure")
	}
}

func TestOpenWinRMTerminal_PropagatesClientFactoryError(t *testing.T) {
	svc, _ := newTestPTYService()

	orig := winrmClientFactory
	defer func() { winrmClientFactory = orig }()
	winrmClientFactory = func(endpoint *winrm.Endpoint, user, password string) (*winrm.Client, error) {
		return nil, errClientFactoryBoom
	}

	_, err := svc.openWinRMTerminal("winrm:test-tab-2", RemoteConnectParams{
		Kind:     RemoteKindWinRM,
		Host:     "192.0.2.1",
		Port:     5985,
		Username: "Administrator",
		AuthType: RemoteAuthPassword,
		Password: "irrelevant",
	})
	if err == nil {
		t.Fatal("expected error propagated from winrmClientFactory, got nil")
	}

	svc.mu.Lock()
	_, present := svc.sessions["winrm:test-tab-2"]
	svc.mu.Unlock()
	if present {
		t.Error("expected no session stored after client factory failure")
	}
}

var errClientFactoryBoom = &testError{"simulated client factory failure"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
