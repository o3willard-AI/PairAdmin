//go:build darwin
// +build darwin

package capture

func GetDefaultAdapters() []TerminalAdapter {
	return []TerminalAdapter{
		NewTmuxAdapter(),
	}
}
