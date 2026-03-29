package capture

import "context"

// PaneInfo holds information about a single terminal pane discovered by an adapter.
type PaneInfo struct {
	ID          string // namespaced: "tmux:%3" or "atspi:/org/a11y/atspi/..."
	AdapterType string // "tmux" | "atspi"
	DisplayName string // human-readable label
	Degraded    bool   // true if discovery ok but capture will fail (Konsole)
	DegradedMsg string // tooltip when Degraded=true
}

// TerminalAdapter is implemented by each backend (tmux, AT-SPI2, etc).
// CaptureManager owns and polls a list of TerminalAdapters.
type TerminalAdapter interface {
	// Name returns the adapter type name (e.g., "tmux", "atspi").
	Name() string
	// IsAvailable reports whether the adapter's backend is reachable.
	// Called once at Startup; unavailable adapters are skipped.
	IsAvailable(ctx context.Context) bool
	// Discover returns the current list of panes visible to this adapter.
	Discover(ctx context.Context) ([]PaneInfo, error)
	// Capture retrieves the visible content of the given pane.
	// Only called for panes where Degraded==false.
	Capture(ctx context.Context, pane PaneInfo) (string, error)
	// Close releases any resources held by the adapter.
	Close() error
}

// TerminalUpdateEvent is the payload emitted on "terminal:update" events.
// Kept JSON-compatible with the old services.TerminalUpdateEvent for frontend compatibility.
type TerminalUpdateEvent struct {
	PaneID  string `json:"paneId"`
	Content string `json:"content"`
}

// TerminalTabsEvent is the payload emitted on "terminal:tabs" events.
// Kept JSON-compatible with the old services.TerminalTabsEvent for frontend compatibility.
type TerminalTabsEvent struct {
	Tabs []TabInfo `json:"tabs"`
}

// TabInfo holds the ID and display name for a single pane tab.
type TabInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Degraded    bool   `json:"degraded,omitempty"`
	DegradedMsg string `json:"degradedMsg,omitempty"`
}
