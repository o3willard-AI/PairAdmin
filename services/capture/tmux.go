package capture

import (
	"context"
	"os/exec"
	"strings"

	"pairadmin/services/llm/filter"
)

// TmuxAdapter implements TerminalAdapter using tmux subprocess calls.
// It discovers panes via `tmux list-panes -a` and captures content via `tmux capture-pane`.
type TmuxAdapter struct {
	// execCommand is injectable for testing — defaults to exec.CommandContext.
	execCommand func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewTmuxAdapter creates a TmuxAdapter with the default execCommand.
func NewTmuxAdapter() *TmuxAdapter {
	return &TmuxAdapter{execCommand: exec.CommandContext}
}

// Name returns the adapter type name.
func (a *TmuxAdapter) Name() string { return "tmux" }

// IsAvailable returns true if tmux is running and accessible.
// It runs `tmux list-panes -a` and treats "no server running" / "error connecting" as unavailable.
func (a *TmuxAdapter) IsAvailable(ctx context.Context) bool {
	cmd := a.execCommand(ctx, "tmux", "list-panes", "-a", "-F", "#{pane_id}")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	_ = out
	return true
}

// Discover returns all active tmux panes with "tmux:" namespaced IDs.
// Returns nil, nil when tmux is not running — safe to call repeatedly.
func (a *TmuxAdapter) Discover(ctx context.Context) ([]PaneInfo, error) {
	cmd := a.execCommand(ctx, "tmux", "list-panes", "-a", "-F",
		"#{pane_id}\t#{session_name}\t#{window_index}\t#{pane_index}")
	out, err := cmd.Output()
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "no server running") || strings.Contains(msg, "error connecting to") {
			return nil, nil
		}
		// Check stderr from ExitError for tmux-specific messages
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "no server running") || strings.Contains(stderr, "error connecting to") {
				return nil, nil
			}
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	panes := make([]PaneInfo, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}
		paneID := fields[0]
		displayName := fields[1] + ":" + fields[2] + "." + fields[3]
		panes = append(panes, PaneInfo{
			ID:          "tmux:" + paneID,
			AdapterType: "tmux",
			DisplayName: displayName,
		})
	}
	return panes, nil
}

// Capture retrieves the visible content of the given tmux pane and applies the filter pipeline.
// The "tmux:" prefix is stripped before passing the pane ID to tmux capture-pane.
func (a *TmuxAdapter) Capture(ctx context.Context, pane PaneInfo) (string, error) {
	// Strip "tmux:" namespace prefix to get the raw tmux pane ID (e.g., "%3")
	rawID := strings.TrimPrefix(pane.ID, "tmux:")

	cmd := a.execCommand(ctx, "tmux", "capture-pane", "-p", "-t", rawID)
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

// Close releases any resources held by the adapter. TmuxAdapter has none.
func (a *TmuxAdapter) Close() error { return nil }
