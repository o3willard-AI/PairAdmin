//go:build !windows
// +build !windows

package services

// RelaunchAsAdminIfNeeded is a no-op on macOS and Linux.
//
// The "Launch PairAdmin as Local Windows Administrator" setting is Windows-only
// by construction: there is no UAC, no runas verb, and no elevation type to read
// here, so there is nothing to relaunch for. main() calls this unconditionally,
// which is why the no-op has to exist rather than the call being tagged.
//
// isElevated and relaunchAsAdmin live only in elevation_windows.go; they are not
// stubbed here because nothing on this platform is allowed to reach them.
func RelaunchAsAdminIfNeeded() {}
