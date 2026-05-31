//go:build !windows
// +build !windows

package services

// copyToClipboardPlatform is the Unix (X11/Wayland) clipboard path.
// On Wayland, uses wl-copy. On X11, uses Wails runtime clipboard API.
func (c *CommandService) copyToClipboardPlatform(text string) error {
	if isWayland() {
		return copyViaWlCopy(text)
	}
	if c.clipboardSetFn != nil {
		return c.clipboardSetFn(c.ctx, text)
	}
	return nil
}
