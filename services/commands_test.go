package services

import (
	"os"
	"testing"
)

func TestIsWayland_NotSet(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")
	if isWayland() {
		t.Error("expected isWayland() to return false when WAYLAND_DISPLAY is not set")
	}
}

func TestIsWayland_Set(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	if !isWayland() {
		t.Error("expected isWayland() to return true when WAYLAND_DISPLAY is set")
	}
}

func TestWaylandDetection_NotWayland(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")
	warning := CheckWayland()
	if warning != nil {
		t.Errorf("expected nil warning on non-Wayland, got %+v", warning)
	}
}

func TestWaylandDetection_WaylandMissing(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	// Override lookPath to simulate wl-copy not found
	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()
	lookPath = func(file string) (string, error) {
		return "", os.ErrNotExist
	}

	warning := CheckWayland()
	if warning == nil {
		t.Fatal("expected a WaylandWarning when wl-copy is missing, got nil")
	}
	if warning.Code != "WAYLAND_CLIPBOARD" {
		t.Errorf("expected Code 'WAYLAND_CLIPBOARD', got '%s'", warning.Code)
	}
	if !contains(warning.Message, "wl-clipboard") {
		t.Errorf("expected Message to contain 'wl-clipboard', got '%s'", warning.Message)
	}
}

func TestWaylandDetection_WaylandWithWlCopy(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	// Override lookPath to simulate wl-copy found
	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()
	lookPath = func(file string) (string, error) {
		return "/usr/bin/wl-copy", nil
	}

	warning := CheckWayland()
	if warning != nil {
		t.Errorf("expected nil warning when wl-copy is available, got %+v", warning)
	}
}

func TestCopyToClipboard_WaylandExec(t *testing.T) {
	// Only test if wl-copy is actually on PATH
	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()

	path, err := origLookPath("wl-copy")
	if err != nil || path == "" {
		t.Skip("wl-copy not available, skipping Wayland copy test")
	}
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	err = copyViaWlCopy("test text")
	if err != nil {
		t.Errorf("copyViaWlCopy returned error: %v", err)
	}
}

// contains is a helper for substring check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
