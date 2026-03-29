package capture

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
)

// Helper to create an ATSPIAdapter with injectable functions for testing
func newTestATSPIAdapter(
	getA11yAddress func() (string, error),
	listBusNames func() ([]string, error),
	getCacheItems func(busName string) ([]CacheItem, error),
	getText func(busName string, path dbus.ObjectPath) (string, error),
	execCommandArgs ...string,
) *ATSPIAdapter {
	a := NewATSPIAdapter()
	if getA11yAddress != nil {
		a.getA11yAddress = getA11yAddress
	}
	if listBusNames != nil {
		a.listBusNames = listBusNames
	}
	if getCacheItems != nil {
		a.getCacheItems = getCacheItems
	}
	if getText != nil {
		a.getText = getText
	}
	return a
}

// Test 1: ATSPIAdapter.Name returns "atspi"
func TestATSPIName(t *testing.T) {
	a := NewATSPIAdapter()
	if a.Name() != "atspi" {
		t.Errorf("expected Name() == \"atspi\", got %q", a.Name())
	}
}

// Test 2: ATSPIAdapter.IsAvailable returns false when session bus connection fails
func TestATSPIIsAvailableFalseOnError(t *testing.T) {
	a := newTestATSPIAdapter(
		func() (string, error) { return "", errors.New("session bus unavailable") },
		nil, nil, nil,
	)
	if a.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable() == false when getA11yAddress returns error")
	}
}

// Test 3: ATSPIAdapter.IsAvailable returns true when GetAddress returns non-empty string
func TestATSPIIsAvailableTrueOnAddress(t *testing.T) {
	a := newTestATSPIAdapter(
		func() (string, error) { return "unix:path=/tmp/test", nil },
		nil, nil, nil,
	)
	if !a.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable() == true when getA11yAddress returns non-empty address")
	}
}

// Test 4: ATSPIAdapter.OnboardingRequired returns true when gsettings returns "false"
func TestATSPIOnboardingRequiredTrue(t *testing.T) {
	a := NewATSPIAdapter()
	a.gsettingsOutput = func(ctx context.Context, key string) (string, error) {
		return "false", nil
	}
	if !a.OnboardingRequired(context.Background()) {
		t.Error("expected OnboardingRequired() == true when gsettings returns 'false'")
	}
}

// Test 5: ATSPIAdapter.OnboardingRequired returns false when gsettings returns "true"
func TestATSPIOnboardingRequiredFalse(t *testing.T) {
	a := NewATSPIAdapter()
	a.gsettingsOutput = func(ctx context.Context, key string) (string, error) {
		return "true", nil
	}
	if a.OnboardingRequired(context.Background()) {
		t.Error("expected OnboardingRequired() == false when gsettings returns 'true'")
	}
}

// Test 6: ATSPIAdapter.Discover returns PaneInfo with "atspi:" prefixed IDs and AdapterType="atspi"
func TestATSPIDiscoverTerminalRole(t *testing.T) {
	a := newTestATSPIAdapter(
		func() (string, error) { return "unix:path=/tmp/test", nil },
		func() ([]string, error) { return []string{":1.100"}, nil },
		func(busName string) ([]CacheItem, error) {
			return []CacheItem{
				{
					Ref:  ObjectRef{Name: ":1.100", Path: "/org/a11y/atspi/accessible/0"},
					Name: "GNOME Terminal",
					Role: RoleTerminal,
				},
			}, nil
		},
		nil,
	)

	panes, err := a.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() returned error: %v", err)
	}
	if len(panes) != 1 {
		t.Fatalf("expected 1 pane, got %d", len(panes))
	}
	if panes[0].AdapterType != "atspi" {
		t.Errorf("expected AdapterType=atspi, got %q", panes[0].AdapterType)
	}
	if panes[0].ID[:6] != "atspi:" {
		t.Errorf("expected ID with 'atspi:' prefix, got %q", panes[0].ID)
	}
	if panes[0].DisplayName != "GNOME Terminal" {
		t.Errorf("expected DisplayName='GNOME Terminal', got %q", panes[0].DisplayName)
	}
}

// Test 7: ATSPIAdapter.Discover filters out non-terminal roles (role != 59)
func TestATSPIDiscoverFiltersNonTerminalRoles(t *testing.T) {
	a := newTestATSPIAdapter(
		func() (string, error) { return "unix:path=/tmp/test", nil },
		func() ([]string, error) { return []string{":1.100"}, nil },
		func(busName string) ([]CacheItem, error) {
			return []CacheItem{
				{Ref: ObjectRef{Name: ":1.100", Path: "/org/a11y/atspi/accessible/0"}, Name: "File Manager", Role: 70},
				{Ref: ObjectRef{Name: ":1.100", Path: "/org/a11y/atspi/accessible/1"}, Name: "Terminal", Role: RoleTerminal},
				{Ref: ObjectRef{Name: ":1.100", Path: "/org/a11y/atspi/accessible/2"}, Name: "Button", Role: 28},
			}, nil
		},
		nil,
	)

	panes, err := a.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() returned error: %v", err)
	}
	if len(panes) != 1 {
		t.Fatalf("expected 1 pane (only terminal role), got %d", len(panes))
	}
	if panes[0].DisplayName != "Terminal" {
		t.Errorf("expected the terminal-role item, got DisplayName=%q", panes[0].DisplayName)
	}
}

// Test 8: ATSPIAdapter.Capture calls GetText(0, -1) and returns content
func TestATSPICaptureGetsText(t *testing.T) {
	expectedText := "$ ls -la\ntotal 42\n"
	a := newTestATSPIAdapter(
		nil, nil, nil,
		func(busName string, path dbus.ObjectPath) (string, error) {
			return expectedText, nil
		},
	)

	pane := PaneInfo{
		ID:          "atspi::1.100/org/a11y/atspi/accessible/0",
		AdapterType: "atspi",
		DisplayName: "Terminal",
	}

	content, err := a.Capture(context.Background(), pane)
	if err != nil {
		t.Fatalf("Capture() returned error: %v", err)
	}
	// Content goes through filter pipeline; raw text without ANSI or credentials should be unchanged
	if content == "" {
		t.Error("expected non-empty content from Capture()")
	}
}

// Test 9: ATSPIAdapter.Capture applies filter pipeline (ANSI + credential)
func TestATSPICaptureAppliesFilterPipeline(t *testing.T) {
	// Text with ANSI escape code that should be stripped
	textWithANSI := "\x1b[31mred text\x1b[0m normal"
	a := newTestATSPIAdapter(
		nil, nil, nil,
		func(busName string, path dbus.ObjectPath) (string, error) {
			return textWithANSI, nil
		},
	)

	pane := PaneInfo{
		ID:          "atspi::1.100/org/a11y/atspi/accessible/0",
		AdapterType: "atspi",
		DisplayName: "Terminal",
	}

	content, err := a.Capture(context.Background(), pane)
	if err != nil {
		t.Fatalf("Capture() returned error: %v", err)
	}
	// ANSI codes should be stripped
	if content == textWithANSI {
		t.Error("expected ANSI codes to be stripped from Capture() output")
	}
}

// Test 11: ATSPIAdapter.Discover marks Konsole windows as Degraded=true when GetText fails
func TestATSPIAdapter_KonsoleDegradation(t *testing.T) {
	a := newTestATSPIAdapter(
		func() (string, error) { return "unix:path=/tmp/test", nil },
		func() ([]string, error) { return []string{":1.200"}, nil },
		func(busName string) ([]CacheItem, error) {
			return []CacheItem{
				{
					Ref:  ObjectRef{Name: ":1.200", Path: "/org/a11y/atspi/accessible/0"},
					Name: "Konsole",
					Role: RoleTerminal,
				},
			}, nil
		},
		func(busName string, path dbus.ObjectPath) (string, error) {
			// Simulate Konsole type mismatch / text extraction failure
			return "", errors.New("dbus: cannot unmarshal")
		},
	)

	panes, err := a.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() returned error: %v", err)
	}
	if len(panes) != 1 {
		t.Fatalf("expected 1 pane, got %d", len(panes))
	}
	if !panes[0].Degraded {
		t.Error("expected Degraded=true for Konsole with failing GetText")
	}
	if panes[0].DegradedMsg != "Konsole text extraction not available on this system." {
		t.Errorf("unexpected DegradedMsg: %q", panes[0].DegradedMsg)
	}
}

// Test 12: ATSPIAdapter.Discover does NOT mark as degraded when GetText succeeds
func TestATSPIAdapter_KonsoleSuccess(t *testing.T) {
	a := newTestATSPIAdapter(
		func() (string, error) { return "unix:path=/tmp/test", nil },
		func() ([]string, error) { return []string{":1.200"}, nil },
		func(busName string) ([]CacheItem, error) {
			return []CacheItem{
				{
					Ref:  ObjectRef{Name: ":1.200", Path: "/org/a11y/atspi/accessible/0"},
					Name: "Konsole",
					Role: RoleTerminal,
				},
			}, nil
		},
		func(busName string, path dbus.ObjectPath) (string, error) {
			return "$ ls -la\ntotal 42\n", nil
		},
	)

	panes, err := a.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() returned error: %v", err)
	}
	if len(panes) != 1 {
		t.Fatalf("expected 1 pane, got %d", len(panes))
	}
	if panes[0].Degraded {
		t.Error("expected Degraded=false when GetText succeeds")
	}
	if panes[0].DegradedMsg != "" {
		t.Errorf("expected empty DegradedMsg when not degraded, got %q", panes[0].DegradedMsg)
	}
}

// Test 10: ATSPIAdapter.Close closes the accessibility bus connection
func TestATSPIClose(t *testing.T) {
	a := NewATSPIAdapter()
	// Close with nil conn should not panic
	if err := a.Close(); err != nil {
		t.Errorf("expected Close() with nil conn to return nil, got %v", err)
	}

	// Close with a tracked closeCalled flag
	closeCalled := false
	a.closeConn = func() error {
		closeCalled = true
		return nil
	}
	if err := a.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
	if !closeCalled {
		t.Error("expected Close() to call closeConn")
	}
}
