package main

import (
	"os"
	"strings"
	"testing"
)

// The uninstaller's credential purge only works if PairAdmin.exe is still on
// disk when nsExec invokes it. The uninstall section deletes $INSTDIR; if the
// cleanup call ever moves after that RMDir the binary is gone, the flag never
// runs, and the failure is silent -- uninstall "succeeds" and leaves every
// credential behind. Go tests cannot observe installer execution, so these
// tests assert the source ordering that guarantees it instead.

const nsiPath = "build/windows/installer/project.nsi"

func readFileOrSkip(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("cannot read %s: %v", path, err)
	}
	return string(b)
}

// TestInstallerInvokesCleanupBeforeDeletingBinary pins the ordering described
// above: --uninstall-cleanup must be invoked before $INSTDIR is removed.
func TestInstallerInvokesCleanupBeforeDeletingBinary(t *testing.T) {
	src := readFileOrSkip(t, nsiPath)

	sec := src[strings.Index(src, `Section "uninstall"`):]
	if sec == "" {
		t.Fatal("no uninstall section found in project.nsi")
	}

	cleanup := nsiCodeLine(sec, "--uninstall-cleanup")
	rmdir := nsiCodeLine(sec, "RMDir /r $INSTDIR")

	if cleanup < 0 {
		t.Fatal("uninstall section never invokes PairAdmin --uninstall-cleanup; " +
			"credentials would survive uninstall")
	}
	if rmdir < 0 {
		t.Fatal("expected the existing $INSTDIR removal in the uninstall section")
	}
	if cleanup > rmdir {
		t.Fatalf("--uninstall-cleanup is invoked at line %d, after $INSTDIR is "+
			"removed at line %d: the binary is deleted first, so the cleanup flag "+
			"can never run", cleanup, rmdir)
	}
}

// TestCredentialPurgeIsGatedOnTheKeepOrWipeChoice guards the behaviour the
// dialog promises: choosing No must keep the credentials. The purge has to sit
// inside the wipe_user_data block, after the MessageBox's Goto jumps over it.
// Placed before the label it would run unconditionally, silently wiping every
// credential for a user who explicitly asked to keep their data.
func TestCredentialPurgeIsGatedOnTheKeepOrWipeChoice(t *testing.T) {
	src := readFileOrSkip(t, nsiPath)

	sec := src[strings.Index(src, `Section "uninstall"`):]
	if sec == "" {
		t.Fatal("no uninstall section found in project.nsi")
	}

	label := nsiCodeLine(sec, "wipe_user_data:")
	cleanup := nsiCodeLine(sec, "--uninstall-cleanup")

	if label < 0 {
		t.Fatal("uninstall section has no wipe_user_data label for the dialog to jump to")
	}
	if cleanup < 0 {
		t.Fatal("uninstall section never invokes --uninstall-cleanup")
	}
	if cleanup < label {
		t.Fatalf("--uninstall-cleanup is invoked at line %d, before the "+
			"wipe_user_data label at line %d, so it runs whether or not the user "+
			"chose to wipe their data", cleanup, label)
	}
}

// codeLine returns the 1-based line number of the first non-comment line
// containing needle. Comment lines are skipped so that prose describing a
// call site cannot be mistaken for the call site itself.
func codeLine(src, needle string) int {
	for i, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "//") || strings.HasPrefix(t, "*") {
			continue
		}
		if strings.Contains(t, needle) {
			return i + 1
		}
	}
	return -1
}

// nsiCodeLine returns the 1-based line number of the first non-comment line
// containing needle. NSIS comments start with ';' or '#'. This matters: the
// uninstall section's prose explains the --uninstall-cleanup call in a comment,
// and a naive substring search finds that prose instead of the invocation.
func nsiCodeLine(src, needle string) int {
	for i, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, ";") || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.Contains(t, needle) {
			return i + 1
		}
	}
	return -1
}

// TestCleanupFlagHandledBeforeWindowOpens documents why the flag must be
// handled early in main(): anything earlier in a Wails startup path can block
// or fail on a headless, windowless invocation during uninstall.
func TestCleanupFlagHandledBeforeWindowOpens(t *testing.T) {
	src := readFileOrSkip(t, "main.go")

	flag := codeLine(src, `os.Args[1] == "--uninstall-cleanup"`)
	run := codeLine(src, "wails.Run(")

	if flag < 0 {
		t.Fatal("main.go does not handle the --uninstall-cleanup flag")
	}
	if run < 0 {
		t.Skip("no wails.Run( call in main.go")
	}
	if flag > run {
		t.Fatalf("--uninstall-cleanup is handled at line %d, after wails.Run( at "+
			"line %d: the window may open or startup may fail before cleanup runs",
			flag, run)
	}
}
