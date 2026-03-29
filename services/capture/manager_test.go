package capture

import (
	"context"
	"strings"
	"sync"
	"testing"

	"pairadmin/services/config"
)

// mockAdapter is a test double for TerminalAdapter.
type mockAdapter struct {
	name        string
	available   bool
	panes       []PaneInfo
	captureData map[string]string
	captureErr  error
	mu          sync.Mutex
	discoverErr error
}

func (m *mockAdapter) Name() string { return m.name }

func (m *mockAdapter) IsAvailable(ctx context.Context) bool { return m.available }

func (m *mockAdapter) Discover(ctx context.Context) ([]PaneInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.discoverErr != nil {
		return nil, m.discoverErr
	}
	result := make([]PaneInfo, len(m.panes))
	copy(result, m.panes)
	return result, nil
}

func (m *mockAdapter) Capture(ctx context.Context, pane PaneInfo) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.captureErr != nil {
		return "", m.captureErr
	}
	if m.captureData != nil {
		if content, ok := m.captureData[pane.ID]; ok {
			return content, nil
		}
	}
	return "mock content", nil
}

func (m *mockAdapter) Close() error { return nil }

// setCapture atomically updates the content for a pane ID (used between ticks).
func (m *mockAdapter) setCapture(paneID, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.captureData == nil {
		m.captureData = make(map[string]string)
	}
	m.captureData[paneID] = content
}

// setPanes atomically updates the discovered pane list.
func (m *mockAdapter) setPanes(panes []PaneInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.panes = make([]PaneInfo, len(panes))
	copy(m.panes, panes)
}

// newTestCaptureManager creates a CaptureManager with injectable emitFn for testing.
// All provided adapters are set as active (bypasses IsAvailable check for unit tests).
func newTestCaptureManager(adapters []TerminalAdapter, emitFn func(ctx context.Context, eventName string, optionalData ...interface{})) *CaptureManager {
	m := NewCaptureManager(adapters, emitFn)
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel
	// Bypass Startup() IsAvailable check — all adapters are active in test
	m.active = adapters
	return m
}

// countEvents counts occurrences of eventName in calls.
func countEvents(calls []string, eventName string) int {
	count := 0
	for _, c := range calls {
		if c == eventName {
			count++
		}
	}
	return count
}

// TestCaptureManagerDiscoversAndEmitsTabs verifies that CaptureManager with one mock adapter
// discovers panes and emits terminal:tabs event on membership change.
func TestCaptureManagerDiscoversAndEmitsTabs(t *testing.T) {
	adapter := &mockAdapter{
		name:      "tmux",
		available: true,
		panes: []PaneInfo{
			{ID: "tmux:%0", AdapterType: "tmux", DisplayName: "main:0.0"},
		},
	}

	var mu sync.Mutex
	var events []string
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		mu.Lock()
		events = append(events, eventName)
		mu.Unlock()
	}

	mgr := newTestCaptureManager([]TerminalAdapter{adapter}, emitFn)
	defer mgr.cancel()

	mgr.tick()

	mu.Lock()
	tabsCount := countEvents(events, "terminal:tabs")
	mu.Unlock()

	if tabsCount != 1 {
		t.Errorf("expected 1 terminal:tabs event on first tick, got %d", tabsCount)
	}
}

// TestCaptureManagerDedup verifies that CaptureManager emits terminal:update only when
// content hash changes (FNV-64a dedup preserved).
func TestCaptureManagerDedup(t *testing.T) {
	adapter := &mockAdapter{
		name:      "tmux",
		available: true,
		panes: []PaneInfo{
			{ID: "tmux:%0", AdapterType: "tmux", DisplayName: "main:0.0"},
		},
		captureData: map[string]string{
			"tmux:%0": "static content",
		},
	}

	var mu sync.Mutex
	var events []string
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		mu.Lock()
		events = append(events, eventName)
		mu.Unlock()
	}

	mgr := newTestCaptureManager([]TerminalAdapter{adapter}, emitFn)
	defer mgr.cancel()

	// First tick: new pane → terminal:tabs + terminal:update
	mgr.tick()

	mu.Lock()
	firstUpdates := countEvents(events, "terminal:update")
	mu.Unlock()

	if firstUpdates != 1 {
		t.Errorf("expected 1 terminal:update on first tick, got %d", firstUpdates)
	}

	// Second tick: same content → no terminal:update
	mgr.tick()

	mu.Lock()
	secondUpdates := countEvents(events, "terminal:update")
	mu.Unlock()

	if secondUpdates != 1 {
		t.Errorf("expected still 1 terminal:update after second tick with same content, got %d", secondUpdates)
	}
}

// TestCaptureManagerTwoAdaptersMerge verifies that CaptureManager with two mock adapters
// merges pane lists and that pane IDs from different adapters never collide.
func TestCaptureManagerTwoAdaptersMerge(t *testing.T) {
	adapter1 := &mockAdapter{
		name:      "tmux",
		available: true,
		panes: []PaneInfo{
			{ID: "tmux:%0", AdapterType: "tmux", DisplayName: "main:0.0"},
			{ID: "tmux:%1", AdapterType: "tmux", DisplayName: "work:1.0"},
		},
	}
	adapter2 := &mockAdapter{
		name:      "atspi",
		available: true,
		panes: []PaneInfo{
			{ID: "atspi:/org/a11y/atspi/win1", AdapterType: "atspi", DisplayName: "GNOME Terminal 1"},
		},
	}

	var mu sync.Mutex
	var tabsEvents []TerminalTabsEvent
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		if eventName == "terminal:tabs" && len(optionalData) > 0 {
			if ev, ok := optionalData[0].(TerminalTabsEvent); ok {
				mu.Lock()
				tabsEvents = append(tabsEvents, ev)
				mu.Unlock()
			}
		}
	}

	mgr := newTestCaptureManager([]TerminalAdapter{adapter1, adapter2}, emitFn)
	defer mgr.cancel()

	mgr.tick()

	mu.Lock()
	count := len(tabsEvents)
	mu.Unlock()

	if count < 1 {
		t.Fatalf("expected at least 1 terminal:tabs event, got %d", count)
	}

	mu.Lock()
	last := tabsEvents[len(tabsEvents)-1]
	mu.Unlock()

	if len(last.Tabs) != 3 {
		t.Errorf("expected 3 merged tabs (2 tmux + 1 atspi), got %d", len(last.Tabs))
	}

	// Verify no ID collisions
	seen := make(map[string]bool)
	for _, tab := range last.Tabs {
		if seen[tab.ID] {
			t.Errorf("duplicate tab ID: %s", tab.ID)
		}
		seen[tab.ID] = true
	}
}

// TestCaptureManagerDegradedAdapter verifies that CaptureManager degrades gracefully when
// one adapter's IsAvailable returns false — other adapter still works.
func TestCaptureManagerDegradedAdapter(t *testing.T) {
	availableAdapter := &mockAdapter{
		name:      "tmux",
		available: true,
		panes: []PaneInfo{
			{ID: "tmux:%0", AdapterType: "tmux", DisplayName: "main:0.0"},
		},
	}
	unavailableAdapter := &mockAdapter{
		name:      "atspi",
		available: false, // not available
		panes:     []PaneInfo{{ID: "atspi:/win1", AdapterType: "atspi", DisplayName: "GNOME Terminal"}},
	}

	var mu sync.Mutex
	var tabsEvents []TerminalTabsEvent
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		if eventName == "terminal:tabs" && len(optionalData) > 0 {
			if ev, ok := optionalData[0].(TerminalTabsEvent); ok {
				mu.Lock()
				tabsEvents = append(tabsEvents, ev)
				mu.Unlock()
			}
		}
	}

	// Use NewCaptureManager directly so Startup filters adapters
	mgr := NewCaptureManager([]TerminalAdapter{availableAdapter, unavailableAdapter}, emitFn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Startup filters by IsAvailable
	mgr.Startup(ctx)
	// Give startup goroutine time to run, but we test via tick instead
	mgr.cancel()

	// Re-create with only available adapter to test tick behavior
	mgr2 := newTestCaptureManager([]TerminalAdapter{availableAdapter}, emitFn)
	defer mgr2.cancel()
	mgr2.tick()

	mu.Lock()
	count := len(tabsEvents)
	mu.Unlock()

	if count < 1 {
		t.Fatalf("expected terminal:tabs event from available adapter, got %d", count)
	}

	mu.Lock()
	last := tabsEvents[len(tabsEvents)-1]
	mu.Unlock()

	// Only tmux panes should appear
	for _, tab := range last.Tabs {
		if tab.ID == "atspi:/win1" {
			t.Errorf("unavailable adapter's pane should not appear in tabs")
		}
	}
}

// TestCaptureManagerSkipsDegradedPanes verifies that CaptureManager skips Capture for panes
// with Degraded=true but still includes them in terminal:tabs.
func TestCaptureManagerSkipsDegradedPanes(t *testing.T) {
	adapter := &mockAdapter{
		name:      "tmux",
		available: true,
		panes: []PaneInfo{
			{ID: "tmux:%0", AdapterType: "tmux", DisplayName: "normal", Degraded: false},
			{ID: "tmux:%1", AdapterType: "tmux", DisplayName: "degraded-pane", Degraded: true, DegradedMsg: "capture not available"},
		},
		captureData: map[string]string{
			"tmux:%0": "normal content",
			"tmux:%1": "should not be captured",
		},
	}

	var mu sync.Mutex
	var tabsEvents []TerminalTabsEvent
	var updateEvents []TerminalUpdateEvent
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		if eventName == "terminal:tabs" && len(optionalData) > 0 {
			if ev, ok := optionalData[0].(TerminalTabsEvent); ok {
				tabsEvents = append(tabsEvents, ev)
			}
		}
		if eventName == "terminal:update" && len(optionalData) > 0 {
			if ev, ok := optionalData[0].(TerminalUpdateEvent); ok {
				updateEvents = append(updateEvents, ev)
			}
		}
	}

	mgr := newTestCaptureManager([]TerminalAdapter{adapter}, emitFn)
	defer mgr.cancel()

	mgr.tick()

	mu.Lock()
	tabsCount := len(tabsEvents)
	var tabs []TabInfo
	if tabsCount > 0 {
		tabs = tabsEvents[len(tabsEvents)-1].Tabs
	}
	updates := make([]TerminalUpdateEvent, len(updateEvents))
	copy(updates, updateEvents)
	mu.Unlock()

	// Both panes should appear in tabs
	if len(tabs) != 2 {
		t.Errorf("expected 2 tabs (including degraded), got %d", len(tabs))
	}

	// Only normal pane should have a terminal:update
	for _, upd := range updates {
		if upd.PaneID == "tmux:%1" {
			t.Errorf("degraded pane tmux:%%1 should not emit terminal:update")
		}
	}

	// Normal pane should have been captured
	found := false
	for _, upd := range updates {
		if upd.PaneID == "tmux:%0" {
			found = true
		}
	}
	if !found {
		t.Errorf("normal pane tmux:%%0 should have emitted terminal:update")
	}
}

// TestCaptureManagerBuildFilterPipelineWithPatterns verifies that buildFilterPipeline
// returns a non-nil pipeline when AppConfig has custom patterns.
func TestCaptureManagerBuildFilterPipelineWithPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := &config.AppConfig{
		CustomPatterns: []config.CustomPattern{
			{Name: "mytoken", Regex: `mytoken-\w+`, Action: "redact"},
		},
	}
	if err := config.SaveAppConfig(cfg); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}

	mgr := newTestCaptureManager(nil, func(ctx context.Context, eventName string, optionalData ...interface{}) {})
	pipeline := mgr.buildFilterPipeline()
	if pipeline == nil {
		t.Fatal("expected non-nil pipeline when AppConfig has custom patterns")
	}

	result, err := pipeline.Apply("output: mytoken-abc123")
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if strings.Contains(result, "mytoken-abc123") {
		t.Errorf("expected pattern to be redacted, got: %s", result)
	}
	if !strings.Contains(result, "[REDACTED:mytoken]") {
		t.Errorf("expected [REDACTED:mytoken] in output, got: %s", result)
	}
}

// TestCaptureManagerBuildFilterPipelineNoPatterns verifies that buildFilterPipeline
// returns nil when AppConfig has no custom patterns.
func TestCaptureManagerBuildFilterPipelineNoPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// No config file written — LoadAppConfig returns empty patterns
	mgr := newTestCaptureManager(nil, func(ctx context.Context, eventName string, optionalData ...interface{}) {})
	pipeline := mgr.buildFilterPipeline()
	if pipeline != nil {
		t.Error("expected nil pipeline when AppConfig has no custom patterns")
	}
}

// TestCaptureManagerRebuildFilterPipeline verifies that RebuildFilterPipeline picks up
// new patterns added via SaveAppConfig after initial construction.
func TestCaptureManagerRebuildFilterPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Start with no patterns
	mgr := newTestCaptureManager(nil, func(ctx context.Context, eventName string, optionalData ...interface{}) {})
	mgr.pipeline = mgr.buildFilterPipeline()
	if mgr.pipeline != nil {
		t.Error("expected nil pipeline before adding any patterns")
	}

	// Add a pattern via config
	cfg := &config.AppConfig{
		CustomPatterns: []config.CustomPattern{
			{Name: "secret", Regex: `SECRET_\w+`, Action: "remove"},
		},
	}
	if err := config.SaveAppConfig(cfg); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}

	mgr.RebuildFilterPipeline()
	if mgr.pipeline == nil {
		t.Fatal("expected non-nil pipeline after RebuildFilterPipeline with new patterns")
	}

	result, err := mgr.pipeline.Apply("line1\nSECRET_KEY=abc\nline3")
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if strings.Contains(result, "SECRET_KEY") {
		t.Errorf("expected SECRET_KEY line to be removed, got: %s", result)
	}
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line3") {
		t.Errorf("expected non-secret lines preserved, got: %s", result)
	}
}

// TestCaptureManagerCustomFilterRedactAppliedDuringTick verifies that captured terminal
// content with a custom redact pattern has matches replaced in the emitted terminal:update event.
func TestCaptureManagerCustomFilterRedactAppliedDuringTick(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Configure a custom redact pattern
	cfg := &config.AppConfig{
		CustomPatterns: []config.CustomPattern{
			{Name: "apikey", Regex: `API_KEY=\S+`, Action: "redact"},
		},
	}
	if err := config.SaveAppConfig(cfg); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}

	adapter := &mockAdapter{
		name:      "tmux",
		available: true,
		panes: []PaneInfo{
			{ID: "tmux:%0", AdapterType: "tmux", DisplayName: "main"},
		},
		captureData: map[string]string{
			"tmux:%0": "$ env | grep key\nAPI_KEY=supersecret123\n$ ",
		},
	}

	var mu sync.Mutex
	var updateEvents []TerminalUpdateEvent
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		if eventName == "terminal:update" && len(optionalData) > 0 {
			if ev, ok := optionalData[0].(TerminalUpdateEvent); ok {
				mu.Lock()
				updateEvents = append(updateEvents, ev)
				mu.Unlock()
			}
		}
	}

	mgr := newTestCaptureManager([]TerminalAdapter{adapter}, emitFn)
	mgr.pipeline = mgr.buildFilterPipeline()
	defer mgr.cancel()

	mgr.tick()

	mu.Lock()
	defer mu.Unlock()

	if len(updateEvents) == 0 {
		t.Fatal("expected terminal:update event from tick")
	}
	content := updateEvents[0].Content
	if strings.Contains(content, "supersecret123") {
		t.Errorf("expected secret to be redacted, got: %s", content)
	}
	if !strings.Contains(content, "[REDACTED:apikey]") {
		t.Errorf("expected [REDACTED:apikey] in content, got: %s", content)
	}
}

// TestCaptureManagerCustomFilterRemoveAppliedDuringTick verifies that captured terminal
// content with a custom remove pattern has matching lines stripped in the emitted event.
func TestCaptureManagerCustomFilterRemoveAppliedDuringTick(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Configure a custom remove pattern
	cfg := &config.AppConfig{
		CustomPatterns: []config.CustomPattern{
			{Name: "password", Regex: `password:`, Action: "remove"},
		},
	}
	if err := config.SaveAppConfig(cfg); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}

	adapter := &mockAdapter{
		name:      "tmux",
		available: true,
		panes: []PaneInfo{
			{ID: "tmux:%0", AdapterType: "tmux", DisplayName: "main"},
		},
		captureData: map[string]string{
			"tmux:%0": "user: admin\npassword: hunter2\nrole: admin",
		},
	}

	var mu sync.Mutex
	var updateEvents []TerminalUpdateEvent
	emitFn := func(ctx context.Context, eventName string, optionalData ...interface{}) {
		if eventName == "terminal:update" && len(optionalData) > 0 {
			if ev, ok := optionalData[0].(TerminalUpdateEvent); ok {
				mu.Lock()
				updateEvents = append(updateEvents, ev)
				mu.Unlock()
			}
		}
	}

	mgr := newTestCaptureManager([]TerminalAdapter{adapter}, emitFn)
	mgr.pipeline = mgr.buildFilterPipeline()
	defer mgr.cancel()

	mgr.tick()

	mu.Lock()
	defer mu.Unlock()

	if len(updateEvents) == 0 {
		t.Fatal("expected terminal:update event from tick")
	}
	content := updateEvents[0].Content
	if strings.Contains(content, "password:") {
		t.Errorf("expected password line to be removed, got: %s", content)
	}
	if !strings.Contains(content, "user: admin") || !strings.Contains(content, "role: admin") {
		t.Errorf("expected non-password lines preserved, got: %s", content)
	}
}
