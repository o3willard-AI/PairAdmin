//go:build windows
// +build windows

// Vendored and trimmed from github.com/UserExistsError/conpty v0.1.4 (MIT
// licensed), with one change: CREATE_NEW_PROCESS_GROUP is added to the
// child process's creation flags. Without it, the spawned shell shares a
// console process group with PairAdmin itself; when the shell exits (e.g.
// the user types "exit"), Windows can broadcast a console close event to
// the whole process group, which kills PairAdmin along with it.
package services

import (
	"context"
	"fmt"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	conptyModKernel32                        = windows.NewLazySystemDLL("kernel32.dll")
	conptyCreatePseudoConsole               = conptyModKernel32.NewProc("CreatePseudoConsole")
	conptyResizePseudoConsole               = conptyModKernel32.NewProc("ResizePseudoConsole")
	conptyClosePseudoConsole                = conptyModKernel32.NewProc("ClosePseudoConsole")
	conptyInitializeProcThreadAttributeList = conptyModKernel32.NewProc("InitializeProcThreadAttributeList")
	conptyUpdateProcThreadAttribute         = conptyModKernel32.NewProc("UpdateProcThreadAttribute")
)

func conptyIsAvailable() bool {
	return conptyCreatePseudoConsole.Find() == nil &&
		conptyResizePseudoConsole.Find() == nil &&
		conptyClosePseudoConsole.Find() == nil &&
		conptyInitializeProcThreadAttributeList.Find() == nil &&
		conptyUpdateProcThreadAttribute.Find() == nil
}

const (
	conptyStillActive                  uint32  = 259
	conptySOk                          uintptr = 0
	conptyAttrPseudoconsole            uintptr = 0x20016
	conptyDefaultConsoleWidth                  = 80
	conptyDefaultConsoleHeight                 = 40
)

type conptyCoord struct {
	X, Y int16
}

func (c *conptyCoord) pack() uintptr {
	return uintptr((int32(c.Y) << 16) | int32(c.X))
}

type conptyHPCON windows.Handle

type conptyHandleIO struct {
	handle windows.Handle
}

func (h *conptyHandleIO) Read(p []byte) (int, error) {
	var numRead uint32 = 0
	err := windows.ReadFile(h.handle, p, &numRead, nil)
	return int(numRead), err
}

func (h *conptyHandleIO) Write(p []byte) (int, error) {
	var numWritten uint32 = 0
	err := windows.WriteFile(h.handle, p, &numWritten, nil)
	return int(numWritten), err
}

// WinConPty wraps a Windows ConPTY pseudoconsole and its attached process.
type WinConPty struct {
	hpc                          conptyHPCON
	pi                           *windows.ProcessInformation
	ptyIn, ptyOut, cmdIn, cmdOut *conptyHandleIO
}

func conptyWin32ClosePseudoConsole(hPc conptyHPCON) {
	if conptyClosePseudoConsole.Find() != nil {
		return
	}
	conptyClosePseudoConsole.Call(uintptr(hPc))
}

func conptyWin32ResizePseudoConsole(hPc conptyHPCON, coord *conptyCoord) error {
	if conptyResizePseudoConsole.Find() != nil {
		return fmt.Errorf("ResizePseudoConsole not found")
	}
	ret, _, _ := conptyResizePseudoConsole.Call(uintptr(hPc), coord.pack())
	if ret != conptySOk {
		return fmt.Errorf("ResizePseudoConsole failed with status 0x%x", ret)
	}
	return nil
}

func conptyWin32CreatePseudoConsole(c *conptyCoord, hIn, hOut windows.Handle) (conptyHPCON, error) {
	if conptyCreatePseudoConsole.Find() != nil {
		return 0, fmt.Errorf("CreatePseudoConsole not found")
	}
	var hPc conptyHPCON
	ret, _, _ := conptyCreatePseudoConsole.Call(
		c.pack(),
		uintptr(hIn),
		uintptr(hOut),
		0,
		uintptr(unsafe.Pointer(&hPc)))
	if ret != conptySOk {
		return 0, fmt.Errorf("CreatePseudoConsole() failed with status 0x%x", ret)
	}
	return hPc, nil
}

type conptyStartupInfoEx struct {
	startupInfo   windows.StartupInfo
	attributeList []byte
}

func conptyGetStartupInfoExForPTY(hpc conptyHPCON) (*conptyStartupInfoEx, error) {
	if conptyInitializeProcThreadAttributeList.Find() != nil {
		return nil, fmt.Errorf("InitializeProcThreadAttributeList not found")
	}
	if conptyUpdateProcThreadAttribute.Find() != nil {
		return nil, fmt.Errorf("UpdateProcThreadAttribute not found")
	}
	var siEx conptyStartupInfoEx
	siEx.startupInfo.Cb = uint32(unsafe.Sizeof(windows.StartupInfo{}) + unsafe.Sizeof(&siEx.attributeList[0]))
	siEx.startupInfo.Flags |= windows.STARTF_USESTDHANDLES
	var size uintptr

	conptyInitializeProcThreadAttributeList.Call(0, 1, 0, uintptr(unsafe.Pointer(&size)))
	siEx.attributeList = make([]byte, size)
	ret, _, err := conptyInitializeProcThreadAttributeList.Call(
		uintptr(unsafe.Pointer(&siEx.attributeList[0])),
		1,
		0,
		uintptr(unsafe.Pointer(&size)))
	if ret != 1 {
		return nil, fmt.Errorf("InitializeProcThreadAttributeList: %v", err)
	}

	ret, _, err = conptyUpdateProcThreadAttribute.Call(
		uintptr(unsafe.Pointer(&siEx.attributeList[0])),
		0,
		conptyAttrPseudoconsole,
		uintptr(hpc),
		unsafe.Sizeof(hpc),
		0,
		0)
	if ret != 1 {
		return nil, fmt.Errorf("UpdateProcThreadAttribute: %v", err)
	}
	return &siEx, nil
}

func conptyCreateConsoleProcessAttachedToPTY(hpc conptyHPCON, commandLine, workDir string, env []string) (*windows.ProcessInformation, error) {
	cmdLine, err := windows.UTF16PtrFromString(commandLine)
	if err != nil {
		return nil, err
	}
	var currentDirectory *uint16
	if workDir != "" {
		currentDirectory, err = windows.UTF16PtrFromString(workDir)
		if err != nil {
			return nil, err
		}
	}
	var envBlock *uint16
	// CREATE_NEW_PROCESS_GROUP keeps the spawned shell out of PairAdmin's own
	// console process group, so a console close/Ctrl signal generated when
	// the shell exits doesn't propagate up and kill PairAdmin itself.
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_NEW_PROCESS_GROUP)
	if env != nil {
		flags |= uint32(windows.CREATE_UNICODE_ENVIRONMENT)
		envBlock = conptyCreateEnvBlock(env)
	}
	siEx, err := conptyGetStartupInfoExForPTY(hpc)
	if err != nil {
		return nil, err
	}
	var pi windows.ProcessInformation
	err = windows.CreateProcess(
		nil,
		cmdLine,
		nil,
		nil,
		false,
		flags,
		envBlock,
		currentDirectory,
		&siEx.startupInfo,
		&pi)
	if err != nil {
		return nil, err
	}
	return &pi, nil
}

func conptyCreateEnvBlock(envv []string) *uint16 {
	if len(envv) == 0 {
		return &utf16.Encode([]rune("\x00\x00"))[0]
	}
	length := 0
	for _, s := range envv {
		length += len(s) + 1
	}
	length += 1

	b := make([]byte, length)
	i := 0
	for _, s := range envv {
		l := len(s)
		copy(b[i:i+l], []byte(s))
		copy(b[i+l:i+l+1], []byte{0})
		i = i + l + 1
	}
	copy(b[i:i+1], []byte{0})

	return &utf16.Encode([]rune(string(b)))[0]
}

func conptyCloseHandles(handles ...windows.Handle) error {
	var err error
	for _, h := range handles {
		if h != windows.InvalidHandle {
			if err == nil {
				err = windows.CloseHandle(h)
			} else {
				windows.CloseHandle(h)
			}
		}
	}
	return err
}

// Close closes all open handles and terminates the attached process.
func (cpty *WinConPty) Close() error {
	conptyWin32ClosePseudoConsole(cpty.hpc)
	return conptyCloseHandles(
		cpty.pi.Process,
		cpty.pi.Thread,
		cpty.ptyIn.handle,
		cpty.ptyOut.handle,
		cpty.cmdIn.handle,
		cpty.cmdOut.handle)
}

// Wait blocks until the attached process exits and returns its exit code.
func (cpty *WinConPty) Wait(ctx context.Context) (uint32, error) {
	var exitCode uint32 = conptyStillActive
	for {
		if err := ctx.Err(); err != nil {
			return conptyStillActive, fmt.Errorf("wait canceled: %v", err)
		}
		ret, _ := windows.WaitForSingleObject(cpty.pi.Process, 1000)
		if ret != uint32(windows.WAIT_TIMEOUT) {
			err := windows.GetExitCodeProcess(cpty.pi.Process, &exitCode)
			return exitCode, err
		}
	}
}

func (cpty *WinConPty) Resize(width, height int) error {
	coords := conptyCoord{int16(width), int16(height)}
	return conptyWin32ResizePseudoConsole(cpty.hpc, &coords)
}

func (cpty *WinConPty) Read(p []byte) (int, error) {
	return cpty.cmdOut.Read(p)
}

func (cpty *WinConPty) Write(p []byte) (int, error) {
	return cpty.cmdIn.Write(p)
}

func (cpty *WinConPty) Pid() int {
	return int(cpty.pi.ProcessId)
}

// WinConPtyStart starts commandLine attached to a new ConPTY pseudoconsole
// of the given dimensions.
func WinConPtyStart(commandLine string, width, height int) (*WinConPty, error) {
	if !conptyIsAvailable() {
		return nil, fmt.Errorf("ConPTY is not available on this version of Windows")
	}
	coords := conptyCoord{int16(width), int16(height)}

	var cmdIn, cmdOut, ptyIn, ptyOut windows.Handle
	if err := windows.CreatePipe(&ptyIn, &cmdIn, nil, 0); err != nil {
		return nil, fmt.Errorf("CreatePipe: %v", err)
	}
	if err := windows.CreatePipe(&cmdOut, &ptyOut, nil, 0); err != nil {
		conptyCloseHandles(ptyIn, cmdIn)
		return nil, fmt.Errorf("CreatePipe: %v", err)
	}

	hPc, err := conptyWin32CreatePseudoConsole(&coords, ptyIn, ptyOut)
	if err != nil {
		conptyCloseHandles(ptyIn, ptyOut, cmdIn, cmdOut)
		return nil, err
	}

	pi, err := conptyCreateConsoleProcessAttachedToPTY(hPc, commandLine, "", nil)
	if err != nil {
		conptyCloseHandles(ptyIn, ptyOut, cmdIn, cmdOut)
		conptyWin32ClosePseudoConsole(hPc)
		return nil, fmt.Errorf("failed to create console process: %v", err)
	}

	return &WinConPty{
		hpc:    hPc,
		pi:     pi,
		ptyIn:  &conptyHandleIO{ptyIn},
		ptyOut: &conptyHandleIO{ptyOut},
		cmdIn:  &conptyHandleIO{cmdIn},
		cmdOut: &conptyHandleIO{cmdOut},
	}, nil
}
