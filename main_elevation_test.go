package main

import (
	"strings"
	"testing"
)

// The "Launch PairAdmin as Local Windows Administrator" relaunch has to happen
// at one specific point in main():
//
//   - AFTER the --uninstall-cleanup early exit, which must stay the first thing
//     in main() and must never turn into a UAC prompt (the uninstaller has no
//     one to answer one);
//   - BEFORE wails.Run(), so the elevated copy is the process that builds the
//     window and the unelevated one gets out of the way.
//
// A Go test cannot observe a real UAC relaunch, and a test that could would
// still not see WHERE in main() it was called from. So this asserts the source
// ordering that guarantees it, the same way main_installer_test.go asserts the
// uninstaller's ordering.

const mainGoPath = "main.go"

// lineOf returns the 1-based number of the first CODE line containing needle, or
// -1. Comment lines are skipped deliberately: the call site is documented in
// comments that name the very strings being searched for (and the uninstall
// block's comment explains itself at length), and a comment can never run.
func lineOf(src, needle string) int {
	for i, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		if strings.Contains(line, needle) {
			return i + 1
		}
	}
	return -1
}

// TestMainRelaunchesElevatedBetweenCleanupAndWailsRun pins both edges of that
// window.
//
// Mutation check: deleting the services.RelaunchAsAdminIfNeeded() call from
// main() fails immediately — the setting would be persisted by the checkbox and
// then never acted on. Moving it above the uninstall block fails the first
// ordering assertion; moving it below wails.Run() fails the second.
func TestMainRelaunchesElevatedBetweenCleanupAndWailsRun(t *testing.T) {
	src := readFileOrSkip(t, mainGoPath)

	cleanup := lineOf(src, `os.Args[1] == "--uninstall-cleanup"`)
	relaunch := lineOf(src, "services.RelaunchAsAdminIfNeeded()")
	wailsRun := lineOf(src, "wails.Run(&options.App{")

	if cleanup < 0 {
		t.Fatal("expected the existing --uninstall-cleanup block in main.go")
	}
	if wailsRun < 0 {
		t.Fatal("expected the existing wails.Run() call in main.go")
	}
	if relaunch < 0 {
		t.Fatal("main() never calls services.RelaunchAsAdminIfNeeded(): the " +
			"\"Launch PairAdmin as Local Windows Administrator\" setting would " +
			"be saved and never acted on")
	}
	if relaunch < cleanup {
		t.Errorf("RelaunchAsAdminIfNeeded() is called at line %d, before the "+
			"--uninstall-cleanup block at line %d: an uninstall could then raise "+
			"a UAC prompt it cannot answer", relaunch, cleanup)
	}
	if relaunch > wailsRun {
		t.Errorf("RelaunchAsAdminIfNeeded() is called at line %d, after "+
			"wails.Run() at line %d: the unelevated process would build the "+
			"window and the relaunch would happen too late to matter",
			relaunch, wailsRun)
	}
}
