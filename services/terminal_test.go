package services

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockExecCommand returns a function that replaces execCommand, producing a cmd
// whose stdout is set to the given output string and whose stderr is empty.
// If exitErr is non-nil, the command's Run/Output will return that error.
func mockExecCommand(output string, exitErr error) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Use "cat" to echo our canned output through a real process so that
		// cmd.Output() works correctly.
		cmd := exec.CommandContext(ctx, "cat")
		cmd.Stdin = bytes.NewBufferString(output)
		if exitErr != nil {
			// For error cases we need a command that fails.
			// We create a cmd whose stderr/stdout come from our fake reader.
			cmd = exec.CommandContext(ctx, "false")
			// Attach a fake reader so stderr contains the error message.
			pr, pw := io.Pipe()
			cmd.Stderr = pw
			go func() {
				io.WriteString(pw, exitErr.Error())
				pw.Close()
			}()
			_ = pr
		}
		return cmd
	}
}

// mockExecCommandOutput returns an execCommand override that always produces
// the given stdout without error.
func mockExecCommandOutput(output string) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, "cat")
		cmd.Stdin = bytes.NewBufferString(output)
		return cmd
	}
}

// TestListPanes verifies that listPanes parses the tmux list-panes output correctly.
func TestListPanes(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()

	execCommand = mockExecCommandOutput("%0\tmain\t0\t0\n%1\twork\t1\t0\n")

	ctx := context.Background()
	panes, err := listPanes(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(panes))
	}
	if panes[0].ID != "%0" {
		t.Errorf("expected pane 0 ID %%0, got %q", panes[0].ID)
	}
	if panes[0].Name != "main:0.0" {
		t.Errorf("expected pane 0 Name main:0.0, got %q", panes[0].Name)
	}
	if panes[1].ID != "%1" {
		t.Errorf("expected pane 1 ID %%1, got %q", panes[1].ID)
	}
	if panes[1].Name != "work:1.0" {
		t.Errorf("expected pane 1 Name work:1.0, got %q", panes[1].Name)
	}
}

// TestListPanesNoTmux verifies that listPanes returns empty slice and nil error
// when tmux reports "no server running".
func TestListPanesNoTmux(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()

	// Use a command that writes the error to stderr and exits non-zero.
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, "sh", "-c", `echo "no server running" >&2; exit 1`)
		return cmd
	}

	ctx := context.Background()
	panes, err := listPanes(ctx)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(panes) != 0 {
		t.Errorf("expected empty panes, got %d", len(panes))
	}
}

// TestListPanesConnectionError verifies that listPanes handles "error connecting to" gracefully.
func TestListPanesConnectionError(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()

	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, "sh", "-c", `echo "error connecting to /tmp/tmux" >&2; exit 1`)
		return cmd
	}

	ctx := context.Background()
	panes, err := listPanes(ctx)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(panes) != 0 {
		t.Errorf("expected empty panes, got %d", len(panes))
	}
}

// TestCapturePane verifies that capturePane trims trailing newlines.
func TestCapturePane(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()

	execCommand = mockExecCommandOutput("$ ls\nfile1\nfile2\n")

	ctx := context.Background()
	content, err := capturePane(ctx, "%0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "$ ls\nfile1\nfile2"
	if content != expected {
		t.Errorf("expected %q, got %q", expected, content)
	}
}

// TestHashContent verifies consistency and distinctness of hashContent.
func TestHashContent(t *testing.T) {
	h1 := hashContent("hello")
	h2 := hashContent("hello")
	h3 := hashContent("world")

	if h1 == 0 {
		t.Error("expected non-zero hash")
	}
	if h1 != h2 {
		t.Errorf("expected consistent hash: %d != %d", h1, h2)
	}
	if h1 == h3 {
		t.Errorf("expected different hashes for different inputs, both got %d", h1)
	}
}

// newTestTerminalService creates a TerminalService with injectable emitFn for testing.
func newTestTerminalService(emitFn func(ctx context.Context, eventName string, optionalData ...interface{})) *TerminalService {
	svc := NewTerminalService()
	svc.emitFn = emitFn
	ctx, cancel := context.WithCancel(context.Background())
	svc.ctx = ctx
	svc.cancel = cancel
	return svc
}

// TestDedup verifies that calling tick() twice with identical content emits terminal:update only once.
func TestDedup(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()

	// First call: list-panes returns one pane; capture-pane returns static content.
	callCount := 0
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		// list-panes calls use "list-panes" in args
		for _, a := range args {
			if a == "list-panes" {
				return mockExecCommandOutput("%0\tmain\t0\t0\n")(ctx, name, args...)
			}
		}
		// capture-pane returns static content
		return mockExecCommandOutput("static content\n")(ctx, name, args...)
	}

	var emitCalls []string
	var mu sync.Mutex
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		mu.Lock()
		emitCalls = append(emitCalls, eventName)
		mu.Unlock()
	}

	svc := newTestTerminalService(emitFn)
	defer svc.cancel()

	// First tick: new pane discovered → terminal:tabs + terminal:update
	svc.tick()

	mu.Lock()
	firstCount := countEvents(emitCalls, "terminal:update")
	mu.Unlock()

	if firstCount != 1 {
		t.Errorf("expected 1 terminal:update on first tick, got %d", firstCount)
	}

	// Second tick: same content → no terminal:update
	svc.tick()

	mu.Lock()
	secondCount := countEvents(emitCalls, "terminal:update")
	mu.Unlock()

	if secondCount != 1 {
		t.Errorf("expected still 1 terminal:update after second tick with same content, got %d", secondCount)
	}
}

// TestDedupChanged verifies that tick() emits terminal:update when content changes.
func TestDedupChanged(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()

	tick := 0
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		for _, a := range args {
			if a == "list-panes" {
				return mockExecCommandOutput("%0\tmain\t0\t0\n")(ctx, name, args...)
			}
		}
		tick++
		if tick == 1 {
			return mockExecCommandOutput("content v1\n")(ctx, name, args...)
		}
		return mockExecCommandOutput("content v2\n")(ctx, name, args...)
	}

	var emitCalls []string
	var mu sync.Mutex
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		mu.Lock()
		emitCalls = append(emitCalls, eventName)
		mu.Unlock()
	}

	svc := newTestTerminalService(emitFn)
	defer svc.cancel()

	svc.tick()
	svc.tick()

	mu.Lock()
	count := countEvents(emitCalls, "terminal:update")
	mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 terminal:update events for changed content, got %d", count)
	}
}

// TestPollNewPane verifies that a new pane appearing in tmux triggers a terminal:tabs event.
func TestPollNewPane(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()

	listCall := 0
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		for _, a := range args {
			if a == "list-panes" {
				listCall++
				if listCall == 1 {
					// First tick: one pane
					return mockExecCommandOutput("%0\tmain\t0\t0\n")(ctx, name, args...)
				}
				// Second tick: two panes
				return mockExecCommandOutput("%0\tmain\t0\t0\n%1\twork\t1\t0\n")(ctx, name, args...)
			}
		}
		return mockExecCommandOutput("content\n")(ctx, name, args...)
	}

	var tabsEvents []TerminalTabsEvent
	var mu sync.Mutex
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		if eventName == "terminal:tabs" && len(optionalData) > 0 {
			if ev, ok := optionalData[0].(TerminalTabsEvent); ok {
				mu.Lock()
				tabsEvents = append(tabsEvents, ev)
				mu.Unlock()
			}
		}
	}

	svc := newTestTerminalService(emitFn)
	defer svc.cancel()

	svc.tick() // first tick: discovers %0 → emits tabs event
	svc.tick() // second tick: discovers %1 (new) → emits tabs event

	mu.Lock()
	count := len(tabsEvents)
	mu.Unlock()

	if count < 2 {
		t.Errorf("expected at least 2 terminal:tabs events (initial + new pane), got %d", count)
	}

	// Last tabs event should contain both panes
	mu.Lock()
	last := tabsEvents[len(tabsEvents)-1]
	mu.Unlock()
	if len(last.Tabs) != 2 {
		t.Errorf("expected 2 tabs in last event, got %d", len(last.Tabs))
	}
}

// TestPollRemovedPane verifies that a pane disappearing from tmux triggers a terminal:tabs event.
func TestPollRemovedPane(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()

	listCall := 0
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		for _, a := range args {
			if a == "list-panes" {
				listCall++
				if listCall == 1 {
					// First tick: two panes
					return mockExecCommandOutput("%0\tmain\t0\t0\n%1\twork\t1\t0\n")(ctx, name, args...)
				}
				// Second tick: one pane (removed %1)
				return mockExecCommandOutput("%0\tmain\t0\t0\n")(ctx, name, args...)
			}
		}
		return mockExecCommandOutput("content\n")(ctx, name, args...)
	}

	var tabsEvents []TerminalTabsEvent
	var mu sync.Mutex
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		if eventName == "terminal:tabs" && len(optionalData) > 0 {
			if ev, ok := optionalData[0].(TerminalTabsEvent); ok {
				mu.Lock()
				tabsEvents = append(tabsEvents, ev)
				mu.Unlock()
			}
		}
	}

	svc := newTestTerminalService(emitFn)
	defer svc.cancel()

	svc.tick() // first tick: discovers both panes
	svc.tick() // second tick: pane removed → emits tabs event

	mu.Lock()
	count := len(tabsEvents)
	mu.Unlock()

	if count < 2 {
		t.Errorf("expected at least 2 terminal:tabs events (initial + removal), got %d", count)
	}

	// Last tabs event should contain only one pane
	mu.Lock()
	last := tabsEvents[len(tabsEvents)-1]
	mu.Unlock()
	if len(last.Tabs) != 1 {
		t.Errorf("expected 1 tab in last event after removal, got %d", len(last.Tabs))
	}
}

// TestSemaphoreBounds verifies that no more than 4 capture goroutines run simultaneously
// when 8 panes are being captured.
func TestSemaphoreBounds(t *testing.T) {
	orig := execCommand
	defer func() { execCommand = orig }()

	var concurrent int64
	var maxConcurrent int64
	var cmu sync.Mutex

	// A barrier channel to control release of blocked goroutines
	release := make(chan struct{})
	started := make(chan struct{}, 8)

	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		for _, a := range args {
			if a == "list-panes" {
				// Return 8 panes
				output := "%0\tmain\t0\t0\n%1\tmain\t0\t1\n%2\tmain\t0\t2\n%3\tmain\t0\t3\n" +
					"%4\tmain\t0\t4\n%5\tmain\t0\t5\n%6\tmain\t0\t6\n%7\tmain\t0\t7\n"
				return mockExecCommandOutput(output)(ctx, name, args...)
			}
		}

		// capture-pane: count concurrent, block, then release
		n := atomic.AddInt64(&concurrent, 1)
		started <- struct{}{}
		cmu.Lock()
		if n > maxConcurrent {
			maxConcurrent = n
		}
		cmu.Unlock()
		<-release
		atomic.AddInt64(&concurrent, -1)

		// Return via a cat command
		cmd := exec.CommandContext(ctx, "cat")
		cmd.Stdin = bytes.NewBufferString("content\n")
		return cmd
	}

	var emitCalls []string
	var emu sync.Mutex
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		emu.Lock()
		emitCalls = append(emitCalls, eventName)
		emu.Unlock()
	}

	svc := newTestTerminalService(emitFn)
	defer svc.cancel()

	// Run tick in background
	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.tick()
	}()

	// Wait for at least 4 goroutines to start (or timeout)
	startedCount := 0
	timeout := time.After(2 * time.Second)
	for startedCount < 4 {
		select {
		case <-started:
			startedCount++
		case <-timeout:
			t.Logf("timeout waiting for goroutines to start, got %d", startedCount)
			close(release)
			<-done
			t.FailNow()
		}
	}

	// At this point, ≤4 goroutines should be running due to semaphore
	cmu.Lock()
	peak := maxConcurrent
	cmu.Unlock()

	if peak > 4 {
		t.Errorf("semaphore violation: %d goroutines ran concurrently, expected max 4", peak)
	}

	// Release all blocked goroutines
	close(release)
	<-done
}

// countEvents counts how many times eventName appears in calls.
func countEvents(calls []string, eventName string) int {
	count := 0
	for _, c := range calls {
		if c == eventName {
			count++
		}
	}
	return count
}
