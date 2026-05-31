//go:build windows
// +build windows

package services

// copyToClipboardPlatform is the Windows clipboard path.
// Uses PowerShell Set-Clipboard with clip.exe fallback.
func (c *CommandService) copyToClipboardPlatform(text string) error {
	return copyToClipboardWindows(text)
}
