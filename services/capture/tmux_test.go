package capture

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

// mockTmuxCommand returns an execCommand override that produces given stdout and optional error.
func mockTmuxCommand(output string, exitErr error) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, "cat")
		cmd.Stdin = bytes.NewBufferString(output)
		if exitErr != nil {
			cmd = exec.CommandContext(ctx, "false")
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

// mockTmuxCommandOutput returns an execCommand override that always succeeds with given output.
func mockTmuxCommandOutput(output string) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, "cat")
		cmd.Stdin = bytes.NewBufferString(output)
		return cmd
	}
}

// TestTmuxAdapterIsAvailableTrue verifies IsAvailable returns true when tmux list-panes succeeds.
func TestTmuxAdapterIsAvailableTrue(t *testing.T) {
	adapter := NewTmuxAdapter()
	adapter.execCommand = mockTmuxCommandOutput("%0\tmain\t0\t0\n")

	ctx := context.Background()
	if !adapter.IsAvailable(ctx) {
		t.Error("expected IsAvailable=true when tmux list-panes succeeds")
	}
}

// TestTmuxAdapterIsAvailableFalse verifies IsAvailable returns false when tmux is not running.
func TestTmuxAdapterIsAvailableFalse(t *testing.T) {
	adapter := NewTmuxAdapter()
	adapter.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", `echo "no server running" >&2; exit 1`)
	}

	ctx := context.Background()
	if adapter.IsAvailable(ctx) {
		t.Error("expected IsAvailable=false when tmux is not running")
	}
}

// TestTmuxAdapterDiscover verifies Discover returns PaneInfo with "tmux:" prefixed IDs.
func TestTmuxAdapterDiscover(t *testing.T) {
	adapter := NewTmuxAdapter()
	adapter.execCommand = mockTmuxCommandOutput("%0\tmain\t0\t0\n%1\twork\t1\t0\n")

	ctx := context.Background()
	panes, err := adapter.Discover(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(panes))
	}

	// IDs must be "tmux:" prefixed
	if panes[0].ID != "tmux:%0" {
		t.Errorf("expected ID tmux:%%0, got %q", panes[0].ID)
	}
	if panes[0].DisplayName != "main:0.0" {
		t.Errorf("expected DisplayName main:0.0, got %q", panes[0].DisplayName)
	}
	if panes[0].AdapterType != "tmux" {
		t.Errorf("expected AdapterType tmux, got %q", panes[0].AdapterType)
	}
	if panes[1].ID != "tmux:%1" {
		t.Errorf("expected ID tmux:%%1, got %q", panes[1].ID)
	}
	if panes[1].DisplayName != "work:1.0" {
		t.Errorf("expected DisplayName work:1.0, got %q", panes[1].DisplayName)
	}
}

// TestTmuxAdapterCapture verifies Capture returns filtered content (ANSI stripped, trailing newline trimmed).
func TestTmuxAdapterCapture(t *testing.T) {
	adapter := NewTmuxAdapter()
	adapter.execCommand = mockTmuxCommandOutput("$ ls\nfile1\nfile2\n")

	ctx := context.Background()
	pane := PaneInfo{ID: "tmux:%0", AdapterType: "tmux", DisplayName: "main:0.0"}
	content, err := adapter.Capture(ctx, pane)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "$ ls\nfile1\nfile2"
	if content != expected {
		t.Errorf("expected %q, got %q", expected, content)
	}
}

// TestTmuxAdapterName verifies the adapter name is "tmux".
func TestTmuxAdapterName(t *testing.T) {
	adapter := NewTmuxAdapter()
	if adapter.Name() != "tmux" {
		t.Errorf("expected Name()=tmux, got %q", adapter.Name())
	}
}

// TestTmuxAdapterDiscoverNoServer verifies Discover returns empty slice and nil error when tmux is not running.
func TestTmuxAdapterDiscoverNoServer(t *testing.T) {
	adapter := NewTmuxAdapter()
	adapter.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", `echo "no server running" >&2; exit 1`)
	}

	ctx := context.Background()
	panes, err := adapter.Discover(ctx)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(panes) != 0 {
		t.Errorf("expected empty panes, got %d", len(panes))
	}
}

// TestTmuxAdapterDiscoverConnectionError verifies Discover handles "error connecting to" gracefully.
func TestTmuxAdapterDiscoverConnectionError(t *testing.T) {
	adapter := NewTmuxAdapter()
	adapter.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", `echo "error connecting to /tmp/tmux" >&2; exit 1`)
	}

	ctx := context.Background()
	panes, err := adapter.Discover(ctx)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(panes) != 0 {
		t.Errorf("expected empty panes, got %d", len(panes))
	}
}

// TestTmuxAdapterHashContent verifies consistency and distinctness of hashContent.
func TestTmuxAdapterHashContent(t *testing.T) {
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

// TestTmuxAdapterDedup verifies dedup with CaptureManager integration.
func TestTmuxAdapterDedup(t *testing.T) {
	adapter := NewTmuxAdapter()

	tick := 0
	adapter.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		for _, a := range args {
			if a == "list-panes" {
				return mockTmuxCommandOutput("%0\tmain\t0\t0\n")(ctx, name, args...)
			}
		}
		tick++
		if tick == 1 {
			return mockTmuxCommandOutput("static content\n")(ctx, name, args...)
		}
		return mockTmuxCommandOutput("static content\n")(ctx, name, args...)
	}

	var emitCalls []string
	var mu sync.Mutex
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		mu.Lock()
		emitCalls = append(emitCalls, eventName)
		mu.Unlock()
	}

	mgr := newTestCaptureManager([]TerminalAdapter{adapter}, emitFn)
	defer mgr.cancel()

	mgr.tick()
	mu.Lock()
	firstCount := countEvents(emitCalls, "terminal:update")
	mu.Unlock()

	if firstCount != 1 {
		t.Errorf("expected 1 terminal:update on first tick, got %d", firstCount)
	}

	mgr.tick()
	mu.Lock()
	secondCount := countEvents(emitCalls, "terminal:update")
	mu.Unlock()

	if secondCount != 1 {
		t.Errorf("expected still 1 terminal:update after second tick with same content, got %d", secondCount)
	}
}

// TestTmuxAdapterDedupChanged verifies that content changes emit new terminal:update events.
func TestTmuxAdapterDedupChanged(t *testing.T) {
	adapter := NewTmuxAdapter()

	captureCall := 0
	adapter.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		for _, a := range args {
			if a == "list-panes" {
				return mockTmuxCommandOutput("%0\tmain\t0\t0\n")(ctx, name, args...)
			}
		}
		captureCall++
		if captureCall == 1 {
			return mockTmuxCommandOutput("content v1\n")(ctx, name, args...)
		}
		return mockTmuxCommandOutput("content v2\n")(ctx, name, args...)
	}

	var emitCalls []string
	var mu sync.Mutex
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		mu.Lock()
		emitCalls = append(emitCalls, eventName)
		mu.Unlock()
	}

	mgr := newTestCaptureManager([]TerminalAdapter{adapter}, emitFn)
	defer mgr.cancel()

	mgr.tick()
	mgr.tick()

	mu.Lock()
	count := countEvents(emitCalls, "terminal:update")
	mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 terminal:update events for changed content, got %d", count)
	}
}

// TestTmuxAdapterNewPane verifies that a new pane appearing triggers a terminal:tabs event.
func TestTmuxAdapterNewPane(t *testing.T) {
	adapter := NewTmuxAdapter()

	listCall := 0
	adapter.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		for _, a := range args {
			if a == "list-panes" {
				listCall++
				if listCall == 1 {
					return mockTmuxCommandOutput("%0\tmain\t0\t0\n")(ctx, name, args...)
				}
				return mockTmuxCommandOutput("%0\tmain\t0\t0\n%1\twork\t1\t0\n")(ctx, name, args...)
			}
		}
		return mockTmuxCommandOutput("content\n")(ctx, name, args...)
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

	mgr := newTestCaptureManager([]TerminalAdapter{adapter}, emitFn)
	defer mgr.cancel()

	mgr.tick()
	mgr.tick()

	mu.Lock()
	count := len(tabsEvents)
	mu.Unlock()

	if count < 2 {
		t.Errorf("expected at least 2 terminal:tabs events, got %d", count)
	}

	mu.Lock()
	last := tabsEvents[len(tabsEvents)-1]
	mu.Unlock()
	if len(last.Tabs) != 2 {
		t.Errorf("expected 2 tabs in last event, got %d", len(last.Tabs))
	}
}

// TestTmuxAdapterRemovedPane verifies that a removed pane triggers a terminal:tabs event.
func TestTmuxAdapterRemovedPane(t *testing.T) {
	adapter := NewTmuxAdapter()

	listCall := 0
	adapter.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		for _, a := range args {
			if a == "list-panes" {
				listCall++
				if listCall == 1 {
					return mockTmuxCommandOutput("%0\tmain\t0\t0\n%1\twork\t1\t0\n")(ctx, name, args...)
				}
				return mockTmuxCommandOutput("%0\tmain\t0\t0\n")(ctx, name, args...)
			}
		}
		return mockTmuxCommandOutput("content\n")(ctx, name, args...)
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

	mgr := newTestCaptureManager([]TerminalAdapter{adapter}, emitFn)
	defer mgr.cancel()

	mgr.tick()
	mgr.tick()

	mu.Lock()
	count := len(tabsEvents)
	mu.Unlock()

	if count < 2 {
		t.Errorf("expected at least 2 terminal:tabs events, got %d", count)
	}

	mu.Lock()
	last := tabsEvents[len(tabsEvents)-1]
	mu.Unlock()
	if len(last.Tabs) != 1 {
		t.Errorf("expected 1 tab after removal, got %d", len(last.Tabs))
	}
}

// TestTmuxAdapterSemaphoreBounds verifies no more than 4 capture goroutines run simultaneously.
func TestTmuxAdapterSemaphoreBounds(t *testing.T) {
	adapter := NewTmuxAdapter()

	var concurrent int64
	var maxConcurrent int64
	var cmu sync.Mutex

	release := make(chan struct{})
	started := make(chan struct{}, 8)

	adapter.execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		for _, a := range args {
			if a == "list-panes" {
				output := "%0\tmain\t0\t0\n%1\tmain\t0\t1\n%2\tmain\t0\t2\n%3\tmain\t0\t3\n" +
					"%4\tmain\t0\t4\n%5\tmain\t0\t5\n%6\tmain\t0\t6\n%7\tmain\t0\t7\n"
				return mockTmuxCommandOutput(output)(ctx, name, args...)
			}
		}

		n := atomic.AddInt64(&concurrent, 1)
		started <- struct{}{}
		cmu.Lock()
		if n > maxConcurrent {
			maxConcurrent = n
		}
		cmu.Unlock()
		<-release
		atomic.AddInt64(&concurrent, -1)

		cmd := exec.CommandContext(ctx, "cat")
		cmd.Stdin = bytes.NewBufferString("content\n")
		return cmd
	}

	var emu sync.Mutex
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		emu.Lock()
		emu.Unlock()
	}

	mgr := newTestCaptureManager([]TerminalAdapter{adapter}, emitFn)
	defer mgr.cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		mgr.tick()
	}()

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

	cmu.Lock()
	peak := maxConcurrent
	cmu.Unlock()

	if peak > 4 {
		t.Errorf("semaphore violation: %d goroutines ran concurrently, expected max 4", peak)
	}

	close(release)
	<-done
}
