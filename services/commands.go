package services

import (
	"context"
	"os/exec"
	"sync"
	"time"

	"os"

	"pairadmin/services/config"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// lookPath is a variable so tests can override it to simulate wl-copy presence/absence.
var lookPath = exec.LookPath

// WaylandWarning is emitted when Wayland is detected but wl-clipboard is not installed.
type WaylandWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CommandService provides clipboard and system command operations for the frontend.
type CommandService struct {
	ctx        context.Context
	clearTimer *time.Timer
	clearMu    sync.Mutex
}

// NewCommandService creates a new CommandService.
func NewCommandService() *CommandService {
	return &CommandService{}
}

// Startup is called by Wails after the application context is available.
// It checks for Wayland without wl-clipboard and emits a warning event if needed.
func (c *CommandService) Startup(ctx context.Context) {
	c.ctx = ctx
	if warning := CheckWayland(); warning != nil {
		runtime.EventsEmit(ctx, "app:warning", warning)
	}
}

// isWayland returns true when running under a Wayland display server.
func isWayland() bool {
	return os.Getenv("WAYLAND_DISPLAY") != ""
}

// CheckWayland checks if the environment is Wayland and whether wl-copy is available.
// Returns a WaylandWarning with code "WAYLAND_CLIPBOARD" when wl-copy is missing on Wayland.
// Returns nil on X11 or when wl-copy is found.
func CheckWayland() *WaylandWarning {
	if !isWayland() {
		return nil
	}
	if _, err := lookPath("wl-copy"); err != nil {
		return &WaylandWarning{
			Code:    "WAYLAND_CLIPBOARD",
			Message: "Wayland detected but wl-clipboard not found. Clipboard copy will not work. Install with: sudo apt install wl-clipboard",
		}
	}
	return nil
}

// CopyToClipboard copies text to the system clipboard.
// On Wayland, it uses wl-copy. On X11, it uses the Wails runtime clipboard API.
// After a successful copy, a timer is started (using ClipboardClearSecs from config)
// to clear the clipboard. Any previous pending clear timer is cancelled first.
func (c *CommandService) CopyToClipboard(text string) error {
	var copyErr error
	if isWayland() {
		copyErr = copyViaWlCopy(text)
	} else {
		runtime.ClipboardSetText(c.ctx, text)
	}
	if copyErr != nil {
		return copyErr
	}

	// Schedule clipboard auto-clear after configurable interval.
	cfg, _ := config.LoadAppConfig()
	secs := 0
	if cfg != nil {
		secs = cfg.ClipboardClearSecs
	}
	if secs <= 0 {
		secs = 60
	}

	c.clearMu.Lock()
	if c.clearTimer != nil {
		c.clearTimer.Stop()
	}
	ctx := c.ctx
	c.clearTimer = time.AfterFunc(time.Duration(secs)*time.Second, func() {
		if isWayland() {
			_ = copyViaWlCopy("")
		} else if ctx != nil {
			runtime.ClipboardSetText(ctx, "")
		}
	})
	c.clearMu.Unlock()

	return nil
}

// copyViaWlCopy copies text to the clipboard using the wl-copy command (Wayland).
func copyViaWlCopy(text string) error {
	cmd := exec.Command("wl-copy", text)
	return cmd.Run()
}
