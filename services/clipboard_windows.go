//go:build windows
// +build windows

package services

import (
	"os/exec"
	"strings"
)

// copyToClipboardWindows sets the Windows clipboard using PowerShell.
// On failure, falls back to clip.exe for plain text.
func copyToClipboardWindows(text string) error {
	// Primary: PowerShell Set-Clipboard (handles Unicode, works on Win 10+)
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Clipboard]::SetText($input)")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fallback: clip.exe (ASCII-only, Win Vista+)
	clip := exec.Command("clip")
	clip.Stdin = strings.NewReader(text)
	return clip.Run()
}
