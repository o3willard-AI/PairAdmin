//go:build windows
// +build windows

package capture

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	moduser32                      = syscall.NewLazyDLL("user32.dll")
	modkernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procGetClassName               = moduser32.NewProc("GetClassNameW")
	procGetWindowThreadProcessId   = moduser32.NewProc("GetWindowThreadProcessId")
	procGetWindowText              = moduser32.NewProc("GetWindowTextW")
	procSendMessage                = moduser32.NewProc("SendMessageW")
	procAttachConsole              = modkernel32.NewProc("AttachConsole")
	procFreeConsole                = modkernel32.NewProc("FreeConsole")
	procGetConsoleWindow           = modkernel32.NewProc("GetConsoleWindow")
	procGetStdHandle               = modkernel32.NewProc("GetStdHandle")
	procGetConsoleScreenBufferInfo = modkernel32.NewProc("GetConsoleScreenBufferInfo")
	procReadConsoleOutputCharacter = modkernel32.NewProc("ReadConsoleOutputCharacterW")
	procCreateFileW                = modkernel32.NewProc("CreateFileW")
	procCloseHandle                = modkernel32.NewProc("CloseHandle")
)

const STD_OUTPUT_HANDLE = uint32(0xFFFFFFF5) // -11
const GENERIC_READ = 0x80000000
const GENERIC_WRITE = 0x40000000
const FILE_SHARE_READ = 0x00000001
const FILE_SHARE_WRITE = 0x00000002
const OPEN_EXISTING = 3

type consoleScreenBufferInfo struct {
	Size              coord
	CursorPosition    coord
	Attributes        uint16
	Window            smallRect
	MaximumWindowSize coord
}

type coord struct {
	X int16
	Y int16
}

type smallRect struct {
	Left   int16
	Top    int16
	Right  int16
	Bottom int16
}

type WindowsAdapter struct {
	mu          sync.Mutex
	allowedPids map[uint32]bool
}

func NewWindowsAdapter() *WindowsAdapter {
	return &WindowsAdapter{
		allowedPids: make(map[uint32]bool),
	}
}

// AddAllowedPid adds a PID to the whitelist of consoles this adapter is allowed to discover.
func (a *WindowsAdapter) AddAllowedPid(pid uint32) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.allowedPids[pid] = true
}

// RemoveAllowedPid removes a PID from the whitelist.
func (a *WindowsAdapter) RemoveAllowedPid(pid uint32) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.allowedPids, pid)
}

func (a *WindowsAdapter) Name() string { return "windows" }

func (a *WindowsAdapter) IsAvailable(ctx context.Context) bool {
	return true
}

func (a *WindowsAdapter) Discover(ctx context.Context) ([]PaneInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var panes []PaneInfo
	// Directly yield the allowed PIDs. We don't need EnumWindows because we track
	// the cmd.exe processes we launch directly. This avoids issues matching cmd.exe PIDs
	// with conhost.exe window owners.
	for pid := range a.allowedPids {
		panes = append(panes, PaneInfo{
			ID:          fmt.Sprintf("windows:%d", pid),
			AdapterType: "windows",
			DisplayName: fmt.Sprintf("Terminal %d", pid),
		})
	}
	return panes, nil
}

func (a *WindowsAdapter) WriteInput(ctx context.Context, pane PaneInfo, data string) error {
	if len(data) == 0 {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	var pid uint32
	if _, err := fmt.Sscanf(pane.ID, "windows:%d", &pid); err != nil {
		return err
	}

	// Attach to the console to get its HWND
	currentConsole, _, _ := syscall.SyscallN(procGetConsoleWindow.Addr())
	alreadyAttached := false
	if currentConsole != 0 {
		var currentPid uint32
		syscall.SyscallN(procGetWindowThreadProcessId.Addr(), currentConsole, uintptr(unsafe.Pointer(&currentPid)))
		if currentPid == pid {
			alreadyAttached = true
		} else {
			syscall.SyscallN(procFreeConsole.Addr())
		}
	}

	if !alreadyAttached {
		r0, _, _ := syscall.SyscallN(procAttachConsole.Addr(), uintptr(pid))
		if r0 == 0 {
			return fmt.Errorf("AttachConsole failed for PID %d", pid)
		}
		defer syscall.SyscallN(procFreeConsole.Addr())
	}

	hwnd, _, _ := syscall.SyscallN(procGetConsoleWindow.Addr())
	if hwnd == 0 {
		return fmt.Errorf("GetConsoleWindow failed")
	}

	for _, c := range data {
		if c == '\n' {
			c = '\r' // Windows console expects \r for Enter
		}
		syscall.SyscallN(procSendMessage.Addr(), hwnd, 0x0102, uintptr(c), 0)
	}
	return nil
}

func (a *WindowsAdapter) Capture(ctx context.Context, pane PaneInfo) (string, error) {
	// Refactoring to Pull-Based Model:
	// To prevent crashes caused by high-frequency AttachConsole/FreeConsole cycles
	// in the background polling loop, we no longer capture Windows content automatically.
	// The frontend must call GetCapturedContent(paneID) explicitly for the active tab.
	return "", nil
}

// GetCapturedContent provides an on-demand pull mechanism for Windows console content.
// This is called by the frontend (via PTYService) only for the active tab,
// ensuring we don't spam the Windows console subsystem with attach/detach cycles.
func (a *WindowsAdapter) GetCapturedContent(paneID string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var pid uint32
	if _, err := fmt.Sscanf(paneID, "windows:%d", &pid); err != nil {
		return "", err
	}

	// Attach to target console
	currentConsole, _, _ := syscall.SyscallN(procGetConsoleWindow.Addr())
	alreadyAttached := false
	if currentConsole != 0 {
		var currentPid uint32
		syscall.SyscallN(procGetWindowThreadProcessId.Addr(), currentConsole, uintptr(unsafe.Pointer(&currentPid)))
		if currentPid == pid {
			alreadyAttached = true
		} else {
			syscall.SyscallN(procFreeConsole.Addr())
		}
	}

	if !alreadyAttached {
		r0, _, _ := syscall.SyscallN(procAttachConsole.Addr(), uintptr(pid))
		if r0 == 0 {
			return "", fmt.Errorf("AttachConsole failed for PID %d", pid)
		}
		defer syscall.SyscallN(procFreeConsole.Addr())
	}

	conoutName, _ := syscall.UTF16PtrFromString("CONOUT$")
	hConsole, _, _ := syscall.SyscallN(procCreateFileW.Addr(), uintptr(unsafe.Pointer(conoutName)), GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE, 0, OPEN_EXISTING, 0, 0)
	if hConsole == 0 || hConsole == uintptr(syscall.InvalidHandle) {
		return "", fmt.Errorf("CreateFile CONOUT$ failed")
	}
	defer syscall.SyscallN(procCloseHandle.Addr(), hConsole)

	var info consoleScreenBufferInfo
	r1, _, _ := syscall.SyscallN(procGetConsoleScreenBufferInfo.Addr(), hConsole, uintptr(unsafe.Pointer(&info)))
	if r1 == 0 {
		return "", fmt.Errorf("GetConsoleScreenBufferInfo failed")
	}

	width := int(info.Window.Right - info.Window.Left + 1)
	height := int(info.Window.Bottom - info.Window.Top + 1)
	if width <= 0 || height <= 0 || width > 500 || height > 500 {
		return "", nil
	}

	var sb strings.Builder
	for y := info.Window.Top; y <= info.Window.Bottom; y++ {
		lineBuf := make([]uint16, width)
		var charsRead uint32
		startCoord := coord{X: info.Window.Left, Y: y}
		coordPacked := uintptr(*(*uint32)(unsafe.Pointer(&startCoord)))

		r2, _, _ := syscall.SyscallN(procReadConsoleOutputCharacter.Addr(), hConsole, uintptr(unsafe.Pointer(&lineBuf[0])), uintptr(width), coordPacked, uintptr(unsafe.Pointer(&charsRead)))
		if r2 != 0 {
			line := string(utf16.Decode(lineBuf[:charsRead]))
			line = strings.ReplaceAll(line, "\x00", " ")
			sb.WriteString(strings.TrimRight(line, " "))
		}
		sb.WriteString("\n")
	}

	result := applyFilterPipeline(sb.String())
	return result, nil
}

func (a *WindowsAdapter) Close() error {
	return nil
}
