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
	procEnumWindows                = moduser32.NewProc("EnumWindows")
	procGetClassName               = moduser32.NewProc("GetClassNameW")
	procGetWindowThreadProcessId   = moduser32.NewProc("GetWindowThreadProcessId")
	procGetWindowText              = moduser32.NewProc("GetWindowTextW")
	procAttachConsole              = modkernel32.NewProc("AttachConsole")
	procFreeConsole                = modkernel32.NewProc("FreeConsole")
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
	mu sync.Mutex
}

func NewWindowsAdapter() *WindowsAdapter {
	return &WindowsAdapter{}
}

func (a *WindowsAdapter) Name() string { return "windows" }

func (a *WindowsAdapter) IsAvailable(ctx context.Context) bool {
	return true
}

func (a *WindowsAdapter) Discover(ctx context.Context) ([]PaneInfo, error) {
	var panes []PaneInfo
	cb := syscall.NewCallback(func(hwnd syscall.Handle, lparam uintptr) uintptr {
		buf := make([]uint16, 256)
		r0, _, _ := syscall.SyscallN(procGetClassName.Addr(), uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if r0 > 0 {
			className := syscall.UTF16ToString(buf[:r0])
			if className == "ConsoleWindowClass" {
				var pid uint32
				syscall.SyscallN(procGetWindowThreadProcessId.Addr(), uintptr(hwnd), uintptr(unsafe.Pointer(&pid)))

				textBuf := make([]uint16, 256)
				r1, _, _ := syscall.SyscallN(procGetWindowText.Addr(), uintptr(hwnd), uintptr(unsafe.Pointer(&textBuf[0])), uintptr(len(textBuf)))
				title := "Windows Console"
				if r1 > 0 {
					title = syscall.UTF16ToString(textBuf[:r1])
				}

				panes = append(panes, PaneInfo{
					ID:          fmt.Sprintf("windows:%d", pid),
					AdapterType: "windows",
					DisplayName: title,
				})
			}
		}
		return 1 // continue enumeration
	})

	syscall.SyscallN(procEnumWindows.Addr(), cb, 0)
	return panes, nil
}

func (a *WindowsAdapter) Capture(ctx context.Context, pane PaneInfo) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var pid uint32
	if _, err := fmt.Sscanf(pane.ID, "windows:%d", &pid); err != nil {
		return "", err
	}

	// Free our own console just in case we have one attached
	syscall.SyscallN(procFreeConsole.Addr())

	// Attach to target console
	r0, _, err := syscall.SyscallN(procAttachConsole.Addr(), uintptr(pid))
	if r0 == 0 {
		return "", fmt.Errorf("AttachConsole failed: %v", err)
	}
	// Always detach when done
	defer syscall.SyscallN(procFreeConsole.Addr())

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
	if width <= 0 || height <= 0 {
		return "", nil // empty window
	}
	length := uint32(width * height)

	buf := make([]uint16, length)
	var charsRead uint32
	startCoord := coord{X: info.Window.Left, Y: info.Window.Top}

	// coord is a struct packed into a single 32-bit integer for syscall
	coordPacked := uintptr(*(*uint32)(unsafe.Pointer(&startCoord)))

	r2, _, _ := syscall.SyscallN(procReadConsoleOutputCharacter.Addr(), hConsole, uintptr(unsafe.Pointer(&buf[0])), uintptr(length), coordPacked, uintptr(unsafe.Pointer(&charsRead)))
	if r2 == 0 {
		return "", fmt.Errorf("ReadConsoleOutputCharacter failed")
	}

	text := string(utf16.Decode(buf[:charsRead]))
	text = strings.ReplaceAll(text, "\x00", " ")

	var sb strings.Builder
	for i := 0; i < len(text); i += width {
		end := i + width
		if end > len(text) {
			end = len(text)
		}
		sb.WriteString(strings.TrimRight(text[i:end], " "))
		sb.WriteString("\n")
	}

	return applyFilterPipeline(sb.String()), nil
}

func (a *WindowsAdapter) Close() error {
	return nil
}
