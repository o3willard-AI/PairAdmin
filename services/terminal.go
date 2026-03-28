package services

import (
	"context"
	"hash/fnv"
	"os/exec"
	"strings"
	"sync"
	"time"

	"pairadmin/services/llm/filter"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sync/semaphore"
)

// execCommand is a variable so tests can override it to simulate tmux output.
var execCommand = exec.CommandContext

// PaneRef holds a stable pane ID and a human-readable display name.
type PaneRef struct {
	ID   string // stable pane ID e.g. "%3"
	Name string // display name e.g. "main:0.0"
}

// TerminalUpdateEvent is the payload emitted on "terminal:update" events.
type TerminalUpdateEvent struct {
	PaneID  string `json:"paneId"`
	Content string `json:"content"`
}

// TerminalTabsEvent is the payload emitted on "terminal:tabs" events.
type TerminalTabsEvent struct {
	Tabs []TabInfo `json:"tabs"`
}

// TabInfo holds the ID and display name for a single tmux pane tab.
type TabInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// captureState tracks the last known hash of a pane's content.
type captureState struct {
	lastHash uint64
}

// TerminalService discovers tmux panes, captures their content at 500ms intervals,
// deduplicates via FNV-64a hashing, and emits Wails events to the frontend.
type TerminalService struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	panes  map[string]*captureState // keyed by pane ID
	sem    *semaphore.Weighted
	// emitFn is injectable for testing — defaults to runtime.EventsEmit
	emitFn func(ctx context.Context, eventName string, optionalData ...interface{})
}

// NewTerminalService creates a new TerminalService with bounded concurrency of 4.
func NewTerminalService() *TerminalService {
	return &TerminalService{
		panes:  make(map[string]*captureState),
		sem:    semaphore.NewWeighted(4),
		emitFn: runtime.EventsEmit,
	}
}

// Startup is called by Wails after the application context is available.
// It starts a background polling loop that captures tmux pane content every 500ms.
func (t *TerminalService) Startup(ctx context.Context) {
	t.ctx, t.cancel = context.WithCancel(ctx)
	go t.pollLoop()
}

// pollLoop runs the capture tick at 500ms intervals until the context is cancelled.
func (t *TerminalService) pollLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.tick()
		}
	}
}

// listPanes discovers all active tmux panes via `tmux list-panes -a`.
// If tmux is not running (no server / connection error), it returns nil, nil — no crash.
func listPanes(ctx context.Context) ([]PaneRef, error) {
	cmd := execCommand(ctx, "tmux", "list-panes", "-a", "-F",
		"#{pane_id}\t#{session_name}\t#{window_index}\t#{pane_index}")
	out, err := cmd.Output()
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "no server running") || strings.Contains(msg, "error connecting to") {
			return nil, nil
		}
		// Also check stderr from ExitError for tmux-specific messages
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "no server running") || strings.Contains(stderr, "error connecting to") {
				return nil, nil
			}
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	panes := make([]PaneRef, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}
		paneID := fields[0]
		name := fields[1] + ":" + fields[2] + "." + fields[3]
		panes = append(panes, PaneRef{ID: paneID, Name: name})
	}
	return panes, nil
}

// capturePane captures the visible content of a tmux pane and applies the filter pipeline.
// It uses plain-text output (no -e flag, no -S flag) to get only the visible screen.
// The ANSI filter strips any escape sequences; the credential filter redacts sensitive data.
func capturePane(ctx context.Context, paneID string) (string, error) {
	cmd := execCommand(ctx, "tmux", "capture-pane", "-p", "-t", paneID)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	content := strings.TrimRight(string(out), "\n")

	credFilter, err := filter.NewCredentialFilter()
	if err != nil {
		return content, nil // degraded: return unfiltered rather than crash
	}
	pipeline := filter.NewPipeline(filter.NewANSIFilter(), credFilter)
	filtered, err := pipeline.Apply(content)
	if err != nil {
		return content, nil // degraded: return unfiltered rather than crash
	}
	return filtered, nil
}

// hashContent computes a FNV-64a hash of the given string for deduplication.
func hashContent(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// captureResult holds the result of a single pane capture.
type captureResult struct {
	paneID  string
	content string
	err     error
}

// tick is called every 500ms. It discovers current panes, detects membership changes,
// captures all pane content concurrently (max 4), and emits events for changed content.
func (t *TerminalService) tick() {
	current, err := listPanes(t.ctx)
	if err != nil {
		return
	}

	// Build a set of current pane IDs
	currentSet := make(map[string]string, len(current)) // ID -> Name
	for _, p := range current {
		currentSet[p.ID] = p.Name
	}

	t.mu.Lock()
	// Detect new and removed panes
	var membershipChanged bool
	for id := range currentSet {
		if _, exists := t.panes[id]; !exists {
			t.panes[id] = &captureState{}
			membershipChanged = true
		}
	}
	for id := range t.panes {
		if _, exists := currentSet[id]; !exists {
			delete(t.panes, id)
			membershipChanged = true
		}
	}
	t.mu.Unlock()

	if membershipChanged {
		tabs := make([]TabInfo, 0, len(current))
		for _, p := range current {
			if _, exists := currentSet[p.ID]; exists {
				tabs = append(tabs, TabInfo{ID: p.ID, Name: p.Name})
			}
		}
		t.emitFn(t.ctx, "terminal:tabs", TerminalTabsEvent{Tabs: tabs})
	}

	// Capture all current panes concurrently with semaphore (max 4)
	results := make([]captureResult, len(current))
	var wg sync.WaitGroup
	for i, p := range current {
		wg.Add(1)
		go func(idx int, pane PaneRef) {
			defer wg.Done()
			if err := t.sem.Acquire(t.ctx, 1); err != nil {
				results[idx] = captureResult{paneID: pane.ID, err: err}
				return
			}
			content, err := capturePane(t.ctx, pane.ID)
			t.sem.Release(1)
			results[idx] = captureResult{paneID: pane.ID, content: content, err: err}
		}(i, p)
	}
	wg.Wait()

	// Emit content change events from the main goroutine after all captures complete
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, r := range results {
		if r.err != nil {
			continue
		}
		state, exists := t.panes[r.paneID]
		if !exists {
			continue
		}
		h := hashContent(r.content)
		if h != state.lastHash {
			state.lastHash = h
			t.emitFn(t.ctx, "terminal:update", TerminalUpdateEvent{
				PaneID:  r.paneID,
				Content: r.content,
			})
		}
	}
}
