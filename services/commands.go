package services

import (
	"context"
	"os/exec"

	"os"

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
	ctx context.Context
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
func (c *CommandService) CopyToClipboard(text string) error {
	if isWayland() {
		return copyViaWlCopy(text)
	}
	runtime.ClipboardSetText(c.ctx, text)
	return nil
}

// copyViaWlCopy copies text to the clipboard using the wl-copy command (Wayland).
func copyViaWlCopy(text string) error {
	cmd := exec.Command("wl-copy", text)
	return cmd.Run()
}
