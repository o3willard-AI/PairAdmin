//go:build !windows
// +build !windows

package services

// openWindowsTerminal is a stub for non-Windows platforms.
// The Windows implementation in pty_windows.go takes over on Windows.
func (s *PTYService) openWindowsTerminal(tabId string) (string, error) {
	return "", nil
}

// closeConPTY is a stub for non-Windows platforms.
func (s *PTYService) closeConPTY(winPty interface{}) {}

// writeConPTYInput is a stub for non-Windows platforms.
func (s *PTYService) writeConPTYInput(winPty interface{}, data string) error {
	return nil
}

// resizeConPTY is a stub for non-Windows platforms.
func (s *PTYService) resizeConPTY(winPty interface{}, cols, rows int) error {
	return nil
}
