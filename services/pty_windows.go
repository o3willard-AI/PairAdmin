//go:build windows
// +build windows

package services

import (
	"fmt"
	"os"
	"os/exec"
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
	// Determine shell: prefer ComSpec, fall back to cmd.exe
	shell := os.Getenv("ComSpec")
	if shell == "" {
		shell = "cmd.exe"
	}

	// Create pipes for ConPTY communication.
	// inR/inW: input pipe (PTYService reads output from the console).
	// outR/outW: output pipe (PTYService writes input to the console).
	inR, inW, errPipe := os.Pipe()
	if errPipe != nil {
		return "", fmt.Errorf("input pipe: %w", errPipe)
	}
	outR, outW, errPipe := os.Pipe()
	if errPipe != nil {
		inR.Close()
		inW.Close()
		return "", fmt.Errorf("output pipe: %w", errPipe)
	}

	// Create pseudoconsole (default 120x40, will be resized by xterm).
	var hPC syscall.Handle
	conSize := _coord{X: 120, Y: 40}
	r0, _, e1 := procCreatePseudoConsole.Call(
		uintptr(*(*uint32)(unsafe.Pointer(&conSize))),
		uintptr(inR.Fd()),
		uintptr(outW.Fd()),
		0,
		uintptr(unsafe.Pointer(&hPC)),
	)
	if r0 != 0 {
		inR.Close(); inW.Close(); outR.Close(); outW.Close()
		return "", fmt.Errorf("CreatePseudoConsole failed: hr=0x%x (%v)", r0, e1)
	}

	// Prepare STARTUPINFOEX with pseudoconsole attribute.
	// We need to allocate an attribute list with one entry for the HPCON.
	siEx, attrList, err := _startupInfoExForConPTY(hPC)
	if err != nil {
		procClosePseudoConsole.Call(uintptr(hPC))
		inR.Close(); inW.Close(); outR.Close(); outW.Close()
		return "", fmt.Errorf("startup info: %w", err)
	}

	// Build process creation flags.
	// Go's syscall.StartProcess doesn't handle STARTUPINFOEX with attribute lists,
	// so we call CreateProcessW directly.
	shell16, _ := syscall.UTF16PtrFromString(shell)
	cmdLine16, _ := syscall.UTF16PtrFromString(shell)

	var procInfo _processInformation
	creationFlags := uint32(_EXTENDED_STARTUPINFO_PRESENT)

	procCreateProcessW := modkernel32.NewProc("CreateProcessW")
	r0, _, e1 := procCreateProcessW.Call(
		uintptr(unsafe.Pointer(shell16)),            // lpApplicationName
		uintptr(unsafe.Pointer(cmdLine16)),           // lpCommandLine
		0,                                             // lpProcessAttributes
		0,                                             // lpThreadAttributes
		0,                                             // bInheritHandles (false — ConPTY manages I/O)
		uintptr(creationFlags),                        // dwCreationFlags
		0,                                             // lpEnvironment
		0,                                             // lpCurrentDirectory
		uintptr(unsafe.Pointer(siEx)),                 // lpStartupInfo
		uintptr(unsafe.Pointer(&procInfo)),            // lpProcessInformation
	)
	// Immediately free the attribute list — the process has its own copy.
	procDeleteProcThreadAttributeList.Call(uintptr(unsafe.Pointer(attrList)))
	_localFree(unsafe.Pointer(siEx))

	if r0 == 0 {
		procClosePseudoConsole.Call(uintptr(hPC))
		inR.Close(); inW.Close(); outR.Close(); outW.Close()
		return "", fmt.Errorf("CreateProcess %s failed: %v", shell, e1)
	}

	// Close the thread handle — we only need the process handle.
	syscall.CloseHandle(syscall.Handle(procInfo.hThread))

	pid := int(procInfo.dwProcessId)

	// Close the ends of the pipes that the child owns.
	// The child inherited inW (write end of input) and outR (read end of output).
	inW.Close()
	outR.Close()

	// We own inR (reads console output) and outW (writes console input).
	session := &ptySession{
		ptmx:   inR,   // read from this = console output
		ptyOut: outW,  // write to this = console input
		hPC:    hPC,
		pid:    pid,
		cmd:    nil, // no exec.Cmd — we used raw syscall
	}

	s.mu.Lock()
	s.sessions[tabId] = session
	s.mu.Unlock()

	// Read goroutine — pumps console output to xterm.js.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := inR.Read(buf)
			if n > 0 {
				s.emitFn(s.ctx, "pty:output", PTYOutputEvent{
					TabID: tabId,
					Data:  string(buf[:n]),
				})
			}
			if err != nil {
				s.mu.Lock()
				delete(s.sessions, tabId)
				s.mu.Unlock()
				inR.Close()
				outW.Close()
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
func _startupInfoExForConPTY(hPC syscall.Handle) (*_startupInfoExW, *_procThreadAttributeList, error) {
	// First pass: get required attribute list size.
	var size uintptr
	r0, _, _ := procInitializeProcThreadAttributeList.Call(
		0, 1, 0, uintptr(unsafe.Pointer(&size)),
	)
	if r0 == 0 {
		// size now contains the required byte count.
	}

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
		uintptr(hPC),
		unsafe.Sizeof(hPC),
		0,
		0,
	)
	if r0 == 0 {
		procDeleteProcThreadAttributeList.Call(uintptr(unsafe.Pointer(buf)))
		_localFree(buf)
		return nil, nil, fmt.Errorf("UpdateProcThreadAttribute failed: %v", e1)
	}

	siEx := &_startupInfoExW{
		cb:         uint32(unsafe.Sizeof(_startupInfoExW{})),
		lpAttributeList: (*_procThreadAttributeList)(buf),
	}
	return siEx, (*_procThreadAttributeList)(buf), nil
}

// Windows structs for ConPTY process creation.
type _startupInfoExW struct {
	_startupInfoW
	lpAttributeList *_procThreadAttributeList
}

type _startupInfoW struct {
	cb              uint32
	_               *uint16 // lpReserved
	_               *uint16 // lpDesktop
	_               *uint16 // lpTitle
	_               uint32  // dwX
	_               uint32  // dwY
	_               uint32  // dwXSize
	_               uint32  // dwYSize
	_               uint32  // dwXCountChars
	_               uint32  // dwYCountChars
	_               uint32  // dwFillAttribute
	_               uint32  // dwFlags
	_               uint16  // wShowWindow
	_               uint16  // cbReserved2
	_               *byte   // lpReserved2
	_               syscall.Handle // hStdInput
	_               syscall.Handle // hStdOutput
	_               syscall.Handle // hStdError
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
