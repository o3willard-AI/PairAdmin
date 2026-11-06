//go:build !windows
// +build !windows

package services

import "testing"

// TestRelaunchAsAdminIfNeeded_NonWindowsIsNoOp pins the platform gate for the
// exported entry point that main() calls unconditionally.
//
// There is no elevation concept on macOS/Linux, and the setting must never act
// there: if this file's no-op body were replaced by the Windows implementation
// (or its build tag dropped), this either fails to compile — isElevated and
// relaunchAsAdmin live only in elevation_windows.go — or it takes the relaunch
// path and os.Exit(0) kills the test binary. Both outcomes are loud.
//
// Mutation check: giving the !windows body anything but an empty no-op (e.g.
// honouring the config flag by exiting to "re-launch") makes this test either
// not build or terminate the test process.
func TestRelaunchAsAdminIfNeeded_NonWindowsIsNoOp(t *testing.T) {
	// Deliberately no config setup: on a non-Windows platform the function must
	// not even look at the setting, so a missing or malformed config file
	// cannot make startup behave differently here.
	RelaunchAsAdminIfNeeded()
}
