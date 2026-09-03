package services

import (
	"os"
	"strings"
	"testing"
	"time"
)

// --- tmux-name sanitize (shared helper) ---

// TestSanitizeTmuxSessionName_Whitelist pins the shared whitelist helper that
// both the SSH path (remote_ssh.go) and the local path (pty_service.go)
// interpolate through: only [A-Za-z0-9_.-] survives, and input that reduces
// to nothing falls back to the default session name.
func TestSanitizeTmuxSessionName_Whitelist(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"valid alnum passes through unchanged", "work-session_1.2", "work-session_1.2"},
		{"empty falls back to default", "", defaultTmuxSessionName},
		{"whitespace-only falls back to default", "   ", defaultTmuxSessionName},
		{"shell metacharacters stripped", "a;rm -rf $HOME`whoami`", "arm-rfHOMEwhoami"},
		{"fully-invalid input falls back to default", ";&|$()", defaultTmuxSessionName},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeTmuxSessionName(tc.in); got != tc.want {
				t.Errorf("sanitizeTmuxSessionName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// --- openLocalTMTerminal ---

// TestOpenLocalTMTerminal_CommandConstructionWithSanitizedName verifies the
// local tmux path end to end: a real PTY is spawned running $SHELL, the
// session is registered, and after tmuxAttachDelay the create-or-attach
// command is written to the PTY master with the SANITIZED session name
// interpolated (a user-supplied "my session!" must not survive verbatim —
// the echoed command reads `tmux new-session -A -s mysession`).
//
// The command echo works because the pty master loop-backs written input to
// the child (and the child shell echoes it) — the same assertion style
// TestOpenSSHTerminal_UseTmux_SendsCreateOrAttachCommand uses for the SSH path.
func TestOpenLocalTMTerminal_CommandConstructionWithSanitizedName(t *testing.T) {
	if os.Getenv("SHELL") == "" && os.Getenv("CI") != "" {
		t.Skip("no $SHELL in environment")
	}
	svc, getEvents := newTestPTYService()

	origDelay := tmuxAttachDelay
	tmuxAttachDelay = time.Millisecond
	defer func() { tmuxAttachDelay = origDelay }()

	tabId := "local:tmux-tab"
	_, err := svc.openLocalTMTerminal(tabId, "my session!")
	if err != nil {
		t.Fatalf("openLocalTMTerminal() unexpected error: %v", err)
	}
	defer svc.CloseTerminal(tabId)

	// Session must be registered synchronously (so TerminalPreview can bind
	// to it before the tmux command is even written).
	svc.mu.Lock()
	_, registered := svc.sessions[tabId]
	svc.mu.Unlock()
	if !registered {
		t.Fatal("expected tab session to be registered in svc.sessions")
	}

	deadline := time.Now().Add(3 * time.Second)
	found := false
	for time.Now().Before(deadline) && !found {
		for _, ev := range getEvents("pty:output") {
			if out, ok := ev.(PTYOutputEvent); ok && strings.Contains(out.Data, "tmux new-session -A -s mysession") {
				found = true
				break
			}
		}
		if !found {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if !found {
		t.Fatal("expected echoed tmux create-or-attach command with sanitized session name, got none")
	}
}

// TestOpenLocalTMTerminal_PlainShellWhenTmuxNameEmpty verifies the no-name
// fallback still spawns a usable shell and writes the create-or-attach for
// the DEFAULT session name (mirroring sanitizeTmuxSessionName's fallback).
func TestOpenLocalTMTerminal_PlainShellWhenTmuxNameEmpty(t *testing.T) {
	svc, getEvents := newTestPTYService()

	origDelay := tmuxAttachDelay
	tmuxAttachDelay = time.Millisecond
	defer func() { tmuxAttachDelay = origDelay }()

	tabId := "local:tmux-default"
	_, err := svc.openLocalTMTerminal(tabId, "")
	if err != nil {
		t.Fatalf("openLocalTMTerminal() unexpected error: %v", err)
	}
	defer svc.CloseTerminal(tabId)

	deadline := time.Now().Add(3 * time.Second)
	found := false
	for time.Now().Before(deadline) && !found {
		for _, ev := range getEvents("pty:output") {
			if out, ok := ev.(PTYOutputEvent); ok && strings.Contains(out.Data, "tmux new-session -A -s "+defaultTmuxSessionName) {
				found = true
				break
			}
		}
		if !found {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if !found {
		t.Fatalf("expected tmux create-or-attach with default name %q, got none", defaultTmuxSessionName)
	}
}

// TestOpenRemoteTerminal_LocalKindDispatch verifies OpenRemoteTerminal's
// switch routes Kind "local" to the local tmux path (a full integration-style
// check of the new case, using the same injectable emitFn as the SSH tests —
// resolveRemoteCredentials is a no-op without SavedHostId).
func TestOpenRemoteTerminal_LocalKindDispatch(t *testing.T) {
	svc, getEvents := newTestPTYService()

	origDelay := tmuxAttachDelay
	tmuxAttachDelay = time.Millisecond
	defer func() { tmuxAttachDelay = origDelay }()

	tabId := "local:dispatch-tab"
	_, err := svc.OpenRemoteTerminal(tabId, RemoteConnectParams{
		Kind:            RemoteKindLocal,
		UseTmux:         true,
		TmuxSessionName: "dispatch-test",
	})
	if err != nil {
		t.Fatalf("OpenRemoteTerminal(local) unexpected error: %v", err)
	}
	defer svc.CloseTerminal(tabId)

	deadline := time.Now().Add(3 * time.Second)
	found := false
	for time.Now().Before(deadline) && !found {
		for _, ev := range getEvents("pty:output") {
			if out, ok := ev.(PTYOutputEvent); ok && strings.Contains(out.Data, "tmux new-session -A -s dispatch-test") {
				found = true
				break
			}
		}
		if !found {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if !found {
		t.Fatal("expected OpenRemoteTerminal(Kind=local) to write the tmux create-or-attach command")
	}
}
