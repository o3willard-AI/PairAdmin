//go:build windows
// +build windows

package capture

func GetDefaultAdapters() []TerminalAdapter {
	return []TerminalAdapter{
		NewTmuxAdapter(),
		NewWindowsAdapter(),
	}
}
