package capture

import (
	"context"
	"hash/fnv"
	"sync"
	"time"

	"pairadmin/services/llm/filter"

	"golang.org/x/sync/semaphore"
)

// captureState tracks the last known hash of a pane's content for deduplication.
type captureState struct {
	lastHash uint64
}

// captureResult holds the result of a single pane capture.
type captureResult struct {
	pane    PaneInfo
	content string
	err     error
}

// CaptureManager owns a set of TerminalAdapters, polls them at 500ms intervals,
// merges pane lists, deduplicates content via FNV-64a, and emits Wails events.
type CaptureManager struct {
	adapters []TerminalAdapter
	active   []TerminalAdapter // adapters that passed IsAvailable check
	emitFn   func(ctx context.Context, eventName string, optionalData ...interface{})

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	panes  map[string]*captureState // keyed by namespaced pane ID
	sem    *semaphore.Weighted
}

// NewCaptureManager creates a CaptureManager with bounded concurrency of 4.
func NewCaptureManager(adapters []TerminalAdapter, emitFn func(ctx context.Context, eventName string, optionalData ...interface{})) *CaptureManager {
	return &CaptureManager{
		adapters: adapters,
		panes:    make(map[string]*captureState),
		sem:      semaphore.NewWeighted(4),
		emitFn:   emitFn,
	}
}

// Startup filters adapters by IsAvailable, then starts the poll loop.
// Called by Wails after the application context is available.
func (m *CaptureManager) Startup(ctx context.Context) {
	m.ctx, m.cancel = context.WithCancel(ctx)

	m.active = m.active[:0]
	for _, a := range m.adapters {
		if a.IsAvailable(m.ctx) {
			m.active = append(m.active, a)
		}
	}

	go m.pollLoop()
}

// pollLoop runs the capture tick at 500ms intervals until the context is cancelled.
func (m *CaptureManager) pollLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.tick()
		}
	}
}

// tick discovers panes from all active adapters, detects membership changes,
// captures content concurrently (max 4), and emits terminal:tabs / terminal:update.
func (m *CaptureManager) tick() {
	// Collect panes from all active adapters
	var allPanes []PaneInfo
	for _, a := range m.active {
		panes, err := a.Discover(m.ctx)
		if err != nil {
			continue
		}
		allPanes = append(allPanes, panes...)
	}

	// Build set of current pane IDs
	currentSet := make(map[string]PaneInfo, len(allPanes))
	for _, p := range allPanes {
		currentSet[p.ID] = p
	}

	m.mu.Lock()
	var membershipChanged bool
	for id := range currentSet {
		if _, exists := m.panes[id]; !exists {
			m.panes[id] = &captureState{}
			membershipChanged = true
		}
	}
	for id := range m.panes {
		if _, exists := currentSet[id]; !exists {
			delete(m.panes, id)
			membershipChanged = true
		}
	}
	m.mu.Unlock()

	if membershipChanged {
		tabs := make([]TabInfo, 0, len(allPanes))
		for _, p := range allPanes {
			tabs = append(tabs, TabInfo{
				ID:          p.ID,
				Name:        p.DisplayName,
				Degraded:    p.Degraded,
				DegradedMsg: p.DegradedMsg,
			})
		}
		m.emitFn(m.ctx, "terminal:tabs", TerminalTabsEvent{Tabs: tabs})
	}

	// Capture non-degraded panes concurrently (semaphore 4)
	// Build adapter lookup for capture calls
	adapterByType := make(map[string]TerminalAdapter, len(m.active))
	for _, a := range m.active {
		adapterByType[a.Name()] = a
	}

	results := make([]captureResult, 0, len(allPanes))
	var (
		wg  sync.WaitGroup
		rmu sync.Mutex
	)
	for _, p := range allPanes {
		if p.Degraded {
			continue
		}
		a, ok := adapterByType[p.AdapterType]
		if !ok {
			continue
		}
		wg.Add(1)
		go func(pane PaneInfo, adapter TerminalAdapter) {
			defer wg.Done()
			if err := m.sem.Acquire(m.ctx, 1); err != nil {
				rmu.Lock()
				results = append(results, captureResult{pane: pane, err: err})
				rmu.Unlock()
				return
			}
			content, err := adapter.Capture(m.ctx, pane)
			m.sem.Release(1)
			rmu.Lock()
			results = append(results, captureResult{pane: pane, content: content, err: err})
			rmu.Unlock()
		}(p, a)
	}
	wg.Wait()

	// Emit content change events from main goroutine after all captures complete
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range results {
		if r.err != nil {
			continue
		}
		state, exists := m.panes[r.pane.ID]
		if !exists {
			continue
		}
		h := hashContent(r.content)
		if h != state.lastHash {
			state.lastHash = h
			m.emitFn(m.ctx, "terminal:update", TerminalUpdateEvent{
				PaneID:  r.pane.ID,
				Content: r.content,
			})
		}
	}
}

// hashContent computes a FNV-64a hash of the given string for deduplication.
func hashContent(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// applyFilterPipeline applies the ANSI + credential filter pipeline to content.
// On failure, returns unfiltered content (degraded behavior — terminal availability > filter failure).
func applyFilterPipeline(content string) string {
	credFilter, err := filter.NewCredentialFilter()
	if err != nil {
		return content
	}
	pipeline := filter.NewPipeline(filter.NewANSIFilter(), credFilter)
	filtered, err := pipeline.Apply(content)
	if err != nil {
		return content
	}
	return filtered
}
