package capture

import (
	"context"
	"hash/fnv"
	"sync"
	"time"

	"pairadmin/services/config"
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

	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	panes    map[string]*captureState // keyed by namespaced pane ID
	sem      *semaphore.Weighted
	pipeline *filter.Pipeline // custom filter pipeline including CustomFilter patterns
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

// Startup filters adapters by IsAvailable, builds the initial filter pipeline, then starts the poll loop.
// Called by Wails after the application context is available.
func (m *CaptureManager) Startup(ctx context.Context) {
	m.ctx, m.cancel = context.WithCancel(ctx)

	m.active = m.active[:0]
	for _, a := range m.adapters {
		if a.IsAvailable(m.ctx) {
			m.active = append(m.active, a)
		}
	}

	m.pipeline = m.buildFilterPipeline()

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
		// Apply custom filter pipeline (includes CustomFilter patterns from AppConfig)
		content := r.content
		if m.pipeline != nil {
			if filtered, err := m.pipeline.Apply(content); err == nil {
				content = filtered
			}
		}
		h := hashContent(content)
		if h != state.lastHash {
			state.lastHash = h
			m.emitFn(m.ctx, "terminal:update", TerminalUpdateEvent{
				PaneID:  r.pane.ID,
				Content: content,
			})
		}
	}
}

// AdapterStatusInfo holds the status of a single adapter for the frontend onboarding UI.
type AdapterStatusInfo struct {
	Name    string `json:"name"`
	Status  string `json:"status"`  // "active" | "unavailable" | "onboarding"
	Message string `json:"message"` // onboarding instructions if applicable
}

// GetAdapterStatus returns the current status of all registered adapters.
// Possible status values:
//   - "active"      — adapter is available and running
//   - "unavailable" — adapter's backend is not reachable
//   - "onboarding"  — adapter bus is running but GSettings accessibility is not enabled
func (m *CaptureManager) GetAdapterStatus() []AdapterStatusInfo {
	activeSet := make(map[string]bool, len(m.active))
	for _, a := range m.active {
		activeSet[a.Name()] = true
	}

	result := make([]AdapterStatusInfo, 0, len(m.adapters))
	for _, a := range m.adapters {
		info := AdapterStatusInfo{Name: a.Name()}
		if !activeSet[a.Name()] {
			info.Status = "unavailable"
			result = append(result, info)
			continue
		}
		// Check onboarding status for AT-SPI2 adapter if it supports it.
		type onboardingChecker interface {
			OnboardingRequired(ctx context.Context) bool
		}
		if oc, ok := a.(onboardingChecker); ok && oc.OnboardingRequired(m.ctx) {
			info.Status = "onboarding"
			info.Message = "Enable accessibility: gsettings set org.gnome.desktop.interface toolkit-accessibility true"
		} else {
			info.Status = "active"
		}
		result = append(result, info)
	}
	return result
}

// hashContent computes a FNV-64a hash of the given string for deduplication.
func hashContent(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// applyFilterPipeline applies the ANSI + credential filter pipeline to content.
// On failure, returns unfiltered content (degraded behavior — terminal availability > filter failure).
// Used by individual adapters (ATSPIAdapter) for per-capture ANSI stripping and credential redaction.
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

// buildFilterPipeline creates the custom filter pipeline including CustomFilter patterns from AppConfig.
// The pipeline is applied by CaptureManager to captured content before emitting terminal:update events.
// Note: individual adapters (TmuxAdapter, ATSPIAdapter) apply their own ANSI + credential filtering;
// this pipeline adds user-configured CustomFilter patterns on top.
func (m *CaptureManager) buildFilterPipeline() *filter.Pipeline {
	var filters []filter.Filter

	cfg, err := config.LoadAppConfig()
	if err == nil && len(cfg.CustomPatterns) > 0 {
		inputs := make([]filter.CustomPatternInput, len(cfg.CustomPatterns))
		for i, p := range cfg.CustomPatterns {
			inputs[i] = filter.CustomPatternInput{
				Name:   p.Name,
				Regex:  p.Regex,
				Action: p.Action,
			}
		}
		customFilter, err := filter.NewCustomFilter(inputs)
		if err == nil {
			filters = append(filters, customFilter)
		}
	}

	if len(filters) == 0 {
		return nil // no custom patterns — skip pipeline to avoid unnecessary allocation
	}
	return filter.NewPipeline(filters...)
}

// RebuildFilterPipeline reloads custom patterns from AppConfig and rebuilds the filter pipeline.
// Called by LLMService after /filter add or /filter remove commands.
func (m *CaptureManager) RebuildFilterPipeline() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pipeline = m.buildFilterPipeline()
}

// ForceCapture triggers an immediate tick outside the 500ms poll interval.
// Called by SettingsService.ForceRefresh via the /refresh slash command.
func (m *CaptureManager) ForceCapture() {
	m.tick()
}
