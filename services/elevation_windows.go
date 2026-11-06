//go:build windows
// +build windows

package services

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"

	"pairadmin/services/config"
)

// Windows half of the "Launch PairAdmin as Local Windows Administrator"
// feature. The policy (whether to relaunch at all) lives in the untagged
// elevation.go, which is what makes it testable on a Linux CI runner; this file
// only reads the process token and starts the elevated copy.

// TOKEN_ELEVATION_TYPE values and the TokenElevationType information class, from
// winnt.h. x/sys/windows v0.48.0 exposes neither, so they are spelled out here.
const (
	tokenElevationType        = 18 // TokenElevationType information class
	tokenElevationTypeDefault = 1  // no UAC split
	tokenElevationTypeFull    = 2  // the administrator token
	tokenElevationTypeLimited = 3  // administrator, filtered token
)

// SEE_MASK_FLAG_NO_UI: do not show a system error dialog if the call fails. A
// declined UAC prompt (ERROR_CANCELLED) is an ordinary outcome here, not
// something to interrupt the user with a second dialog about.
const seeMaskFlagNoUI = 0x00000400

var (
	elevationShell32        = windows.NewLazySystemDLL("shell32.dll")
	elevationShellExecuteEx = elevationShell32.NewProc("ShellExecuteExW")
)

// shellExecuteInfo mirrors SHELLEXECUTEINFOW. Go lays these fields out the same
// way the C struct is laid out (the two uint32s first, then pointer-sized
// fields, with the padding the compiler inserts), which is why Size is
// unsafe.Sizeof of this struct.
type shellExecuteInfo struct {
	Size       uint32
	Mask       uint32
	Hwnd       windows.Handle
	Verb       *uint16
	File       *uint16
	Parameters *uint16
	Directory  *uint16
	Show       int32
	InstApp    windows.Handle
	IdList     uintptr
	Class      *uint16
	HkeyClass  windows.Handle
	HotKey     uint32
	Icon       windows.Handle
	Process    windows.Handle
}

// isElevated reports whether the current process holds a full administrator
// token.
//
// Deliberately NOT a username check or an Administrators-group membership
// check: those answer "is this user an administrator", which is a different
// question. The one that matters here is "does this process hold the
// administrator token right now", and that is the token's elevation type.
func isElevated() (bool, error) {
	elevation, err := currentTokenElevation()
	if err != nil {
		return false, err
	}
	return elevation.IsFull(), nil
}

// currentTokenElevation reads TokenElevationType from the current process token.
func currentTokenElevation() (TokenElevation, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return TokenElevationUnknown, fmt.Errorf("open current process token: %w", err)
	}
	defer token.Close()

	var elevationType uint32
	var returned uint32
	if err := windows.GetTokenInformation(
		token,
		tokenElevationType,
		(*byte)(unsafe.Pointer(&elevationType)),
		uint32(unsafe.Sizeof(elevationType)),
		&returned,
	); err != nil {
		return TokenElevationUnknown, fmt.Errorf("read token elevation type: %w", err)
	}

	switch elevationType {
	case tokenElevationTypeFull:
		return TokenElevationFull, nil
	case tokenElevationTypeLimited:
		return TokenElevationLimited, nil
	case tokenElevationTypeDefault:
		return TokenElevationDefault, nil
	default:
		return TokenElevationUnknown, nil
	}
}

// relaunchAsAdmin starts a second copy of this executable elevated, via the
// "runas" verb — which is what raises the UAC consent prompt — preserving the
// command-line arguments and the working directory.
//
// It does NOT wait for the elevated copy: this process exits immediately after,
// and the new instance owns the window.
func relaunchAsAdmin() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return fmt.Errorf("encode verb: %w", err)
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return fmt.Errorf("encode executable path: %w", err)
	}
	params, err := windows.UTF16PtrFromString(buildArgString(os.Args[1:]))
	if err != nil {
		return fmt.Errorf("encode arguments: %w", err)
	}

	// The working directory is worth preserving (a relative --config, or a
	// relative path in an argument, means the same thing in the elevated copy),
	// but a failure to read it is not a reason to refuse to relaunch.
	var directory *uint16
	if wd, err := os.Getwd(); err == nil {
		if directory, err = windows.UTF16PtrFromString(wd); err != nil {
			directory = nil
		}
	}

	info := shellExecuteInfo{
		Size:       uint32(unsafe.Sizeof(shellExecuteInfo{})),
		Mask:       seeMaskFlagNoUI,
		Verb:       verb,
		File:       file,
		Parameters: params,
		Directory:  directory,
		Show:       windows.SW_SHOWNORMAL,
	}

	ret, _, callErr := elevationShellExecuteEx.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		// ERROR_CANCELLED (1223) means the user declined the UAC prompt.
		return fmt.Errorf("ShellExecuteExW runas: %w", callErr)
	}
	return nil
}

// RelaunchAsAdminIfNeeded is called from main() on every startup, on every
// platform (elevation_other.go provides the no-op for non-Windows builds).
//
// It is a no-op unless the persisted LaunchAsAdministrator setting is on. When
// it is on and this process is not elevated, it starts an elevated copy and
// exits this one; the elevated copy then reads the same setting, finds itself
// already elevated, and keeps running — which is what stops startup from
// becoming an endless UAC loop.
func RelaunchAsAdminIfNeeded() {
	cfg, err := config.LoadAppConfig()
	if err != nil || cfg == nil {
		// A config that cannot be read is not a reason to ask for elevation.
		return
	}

	switch elevationPlan(cfg.LaunchAsAdministrator, isElevated, relaunchAsAdmin, os.Exit) {
	case ElevationRelaunchFailed:
		// Most often a declined UAC prompt. Say so on stderr and carry on: a
		// dead app that was never elevated is a worse outcome than a running
		// one without the administrator token.
		fmt.Fprintln(os.Stderr, "pairadmin: could not relaunch with administrator rights; continuing without elevation")
	}
}
