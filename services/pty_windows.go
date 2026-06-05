//go:build windows
// +build windows

package services

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procCreatePseudoConsole           = modkernel32.NewProc("CreatePseudoConsole")
	procClosePseudoConsole            = modkernel32.NewProc("ClosePseudoConsole")
	procResizePseudoConsole           = modkernel32.NewProc("ResizePseudoConsole")
	procInitializeProcThreadAttributeList = modkernel32.NewProc("InitializeProcThreadAttributeList")
	procUpdateProcThreadAttribute     = modkernel32.NewProc("UpdateProcThreadAttribute")
	procDeleteProcThreadAttributeList = modkernel32.NewProc("DeleteProcThreadAttributeList")
)

func logPty(msg string) {
	f, err := os.OpenFile("pty_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		f.WriteString(fmt.Sprintf("[%s] %s\n", os.Getenv("USERNAME"), msg))
	}
}

// Windows ConPTY constants.
const (
	_STILL_ACTIVE                    = 259
	_PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE = 0x00020016
	_EXTENDED_STARTUPINFO_PRESENT    = 0x00080000
)

type _coord struct {
	X int16
	Y int16
}

// openWindowsConPTY creates a pseudoconsole on Windows 10 1809+,
// spawns the user's preferred shell inside it, and wires the pipes
// into the PTY session so xterm.js gets a real bidirectional stream.
func (s *PTYService) openWindowsConPTY(tabId string) (string, error) {
	logPty("openWindowsConPTY called")

	// Check if ConPTY is supported on this Windows version.
	if err := procCreatePseudoConsole.Find(); err != nil {
		logPty("ConPTY not supported (CreatePseudoConsole not found)")
		return "", fmt.Errorf("ConPTY not supported: %w", err)
	}

	// Determine shell: prefer ComSpec, fall back to cmd.exe
	shell := os.Getenv("ComSpec")
	if shell == "" {
		shell = "cmd.exe"
	}

	// Create pipes for ConPTY communication.
	// We use syscall.CreatePipe to ensure handles are inheritable (required by ConPTY conhost).
	var sa syscall.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1 // TRUE

	var inR, inW, outR, outW syscall.Handle
	if err := syscall.CreatePipe(&inR, &inW, &sa, 0); err != nil {
		logPty(fmt.Sprintf("Failed to create input pipe: %v", err))
		return "", fmt.Errorf("input pipe: %w", err)
	}
	if err := syscall.CreatePipe(&outR, &outW, &sa, 0); err != nil {
		syscall.CloseHandle(inR)
		syscall.CloseHandle(inW)
		logPty(fmt.Sprintf("Failed to create output pipe: %v", err))
		return "", fmt.Errorf("output pipe: %w", err)
	}

	// Create pseudoconsole (default 120x40).
	var hPC syscall.Handle
	conSize := _coord{X: 120, Y: 40}
	r0, _, e1 := procCreatePseudoConsole.Call(
		uintptr(*(*uint32)(unsafe.Pointer(&conSize))),
		uintptr(inR),  // ConPTY reads from here
		uintptr(outW), // ConPTY writes to here
		0,
		uintptr(unsafe.Pointer(&hPC)),
	)
	if r0 != 0 {
		syscall.CloseHandle(inR); syscall.CloseHandle(inW)
		syscall.CloseHandle(outR); syscall.CloseHandle(outW)
		logPty(fmt.Sprintf("CreatePseudoConsole failed: hr=0x%x (%v)", r0, e1))
		return "", fmt.Errorf("CreatePseudoConsole failed: hr=0x%x (%v)", r0, e1)
	}

	// Prepare STARTUPINFOEX with pseudoconsole attribute.
	siEx, attrList, err := _startupInfoExForConPTY(&hPC)
	if err != nil {
		procClosePseudoConsole.Call(uintptr(hPC))
		syscall.CloseHandle(inR); syscall.CloseHandle(inW)
		syscall.CloseHandle(outR); syscall.CloseHandle(outW)
		logPty(fmt.Sprintf("_startupInfoExForConPTY failed: %v", err))
		return "", fmt.Errorf("startup info: %w", err)
	}

	// Build process creation command line.
	cmdLine16, _ := syscall.UTF16PtrFromString(shell)

	var procInfo _processInformation
	creationFlags := uint32(_EXTENDED_STARTUPINFO_PRESENT)

	procCreateProcessW := modkernel32.NewProc("CreateProcessW")
	r0, _, e1 = procCreateProcessW.Call(
		0,                                             // lpApplicationName (nil, use cmdLine)
		uintptr(unsafe.Pointer(cmdLine16)),           // lpCommandLine
		0,                                             // lpProcessAttributes
		0,                                             // lpThreadAttributes
		0,                                             // bInheritHandles (FALSE, ConPTY handles I/O via HPCON)
		uintptr(creationFlags),                        // dwCreationFlags
		0,                                             // lpEnvironment
		0,                                             // lpCurrentDirectory
		uintptr(unsafe.Pointer(siEx)),                 // lpStartupInfo
		uintptr(unsafe.Pointer(&procInfo)),            // lpProcessInformation
	)

	// Now we can close the ConPTY-owned ends.
	syscall.CloseHandle(inR)
	syscall.CloseHandle(outW)

	// Free the attribute list.
	procDeleteProcThreadAttributeList.Call(uintptr(unsafe.Pointer(attrList)))
	_localFree(unsafe.Pointer(attrList))

	if r0 == 0 {
		procClosePseudoConsole.Call(uintptr(hPC))
		syscall.CloseHandle(inW)
		syscall.CloseHandle(outR)
		logPty(fmt.Sprintf("CreateProcessW failed: %v", e1))
		return "", fmt.Errorf("CreateProcess %s failed: %v", shell, e1)
	}

	logPty(fmt.Sprintf("Successfully started %s (PID: %d)", shell, procInfo.dwProcessId))

	// Close thread handle.
	syscall.CloseHandle(syscall.Handle(procInfo.hThread))

	// Wrap handles in os.File for easy reading/writing.
	fileInW := os.NewFile(uintptr(inW), "pty_in")
	fileOutR := os.NewFile(uintptr(outR), "pty_out")

	session := &ptySession{
		ptmx:   fileOutR, // read from this = console output
		ptyOut: fileInW,  // write to this = console input
		hPC:    uintptr(hPC),
		pid:    int(procInfo.dwProcessId),
		cmd:    nil,
	}

	s.mu.Lock()
	s.sessions[tabId] = session
	s.mu.Unlock()

	// Read goroutine — pumps console output to xterm.js.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := fileOutR.Read(buf)
			if n > 0 {
				s.emitFn(s.ctx, "pty:output", PTYOutputEvent{
					TabID: tabId,
					Data:  string(buf[:n]),
				})
			}
			if err != nil {
				logPty(fmt.Sprintf("Read goroutine exiting for %s: %v", tabId, err))
				s.mu.Lock()
				delete(s.sessions, tabId)
				s.mu.Unlock()
				fileOutR.Close()
				fileInW.Close()
				procClosePseudoConsole.Call(uintptr(hPC))
				s.emitFn(s.ctx, "pty:closed", map[string]string{"tabId": tabId})
				return
			}
		}
	}()

	return tabId, nil
}

// _startupInfoExForConPTY allocates a STARTUPINFOEXW with the ConPTY HPCON
// in its PROC_THREAD_ATTRIBUTE_LIST. Caller must free the returned pointers.
func _startupInfoExForConPTY(hPC *syscall.Handle) (*_startupInfoExW, *_procThreadAttributeList, error) {
	// First pass: get required attribute list size.
	var size uintptr
	procInitializeProcThreadAttributeList.Call(
		0, 1, 0, uintptr(unsafe.Pointer(&size)),
	)

	buf := _localAlloc(size)
	if buf == nil {
		return nil, nil, fmt.Errorf("LocalAlloc for attribute list failed")
	}

	r0, _, e1 := procInitializeProcThreadAttributeList.Call(
		uintptr(unsafe.Pointer(buf)), 1, 0, uintptr(unsafe.Pointer(&size)),
	)
	if r0 == 0 {
		_localFree(buf)
		return nil, nil, fmt.Errorf("InitializeProcThreadAttributeList failed: %v", e1)
	}

	r0, _, e1 = procUpdateProcThreadAttribute.Call(
		uintptr(unsafe.Pointer(buf)),
		0,
		_PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		uintptr(unsafe.Pointer(hPC)),
		unsafe.Sizeof(*hPC),
		0,
		0,
	)
	if r0 == 0 {
		procDeleteProcThreadAttributeList.Call(uintptr(unsafe.Pointer(buf)))
		_localFree(buf)
		return nil, nil, fmt.Errorf("UpdateProcThreadAttribute failed: %v", e1)
	}

	siEx := &_startupInfoExW{
		StartupInfo: syscall.StartupInfo{
			Cb: uint32(unsafe.Sizeof(_startupInfoExW{})),
		},
		lpAttributeList: (*_procThreadAttributeList)(buf),
	}
	return siEx, (*_procThreadAttributeList)(buf), nil
}

// Windows structs for ConPTY process creation.
type _startupInfoExW struct {
	syscall.StartupInfo
	lpAttributeList *_procThreadAttributeList
}

type _procThreadAttributeList struct{}

type _processInformation struct {
	hProcess    uintptr
	hThread     uintptr
	dwProcessId uint32
	dwThreadId  uint32
}

// _localAlloc wraps kernel32!LocalAlloc for STARTUPINFOEX allocation.
func _localAlloc(size uintptr) unsafe.Pointer {
	procLocalAlloc := modkernel32.NewProc("LocalAlloc")
	r0, _, _ := procLocalAlloc.Call(0x0040, size) // LPTR = LMEM_ZEROINIT | LMEM_FIXED
	return unsafe.Pointer(r0)
}

// _localFree wraps kernel32!LocalFree.
func _localFree(p unsafe.Pointer) {
	procLocalFree := modkernel32.NewProc("LocalFree")
	procLocalFree.Call(uintptr(p))
}

// writeConPTYInput writes data to the pseudoconsole input pipe.
func (s *PTYService) writeConPTYInput(tabId string, data string) error {
	s.mu.Lock()
	session, ok := s.sessions[tabId]
	s.mu.Unlock()
	if !ok {
		return nil
	}
	if session.ptyOut == nil {
		return nil
	}
	_, err := session.ptyOut.Write([]byte(data))
	return err
}

// resizeConPTY resizes the pseudoconsole to match xterm.js dimensions.
func (s *PTYService) resizeConPTY(tabId string, cols, rows int) error {
	s.mu.Lock()
	session, ok := s.sessions[tabId]
	s.mu.Unlock()
	if !ok || session.hPC == 0 {
		return nil
	}
	newSize := _coord{X: int16(cols), Y: int16(rows)}
	procResizePseudoConsole.Call(
		uintptr(session.hPC),
		uintptr(*(*uint32)(unsafe.Pointer(&newSize))),
	)
	return nil
}

// closeConPTY closes the pseudoconsole handle.
func (s *PTYService) closeConPTY(hPC uintptr) {
	procClosePseudoConsole.Call(hPC)
}
