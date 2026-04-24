package terminal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

type conPty struct {
	hPC       windows.Handle
	readPipe  *os.File
	writePipe *os.File
	cmd       *exec.Cmd
}

func startPty(cmd *exec.Cmd) (io.ReadWriteCloser, error) {
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("conpty: input pipe: %w", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		inR.Close()
		inW.Close()
		return nil, fmt.Errorf("conpty: output pipe: %w", err)
	}

	size := uint32(120) | uint32(40)<<16
	var hPC windows.Handle
	r, _, e := procCreatePseudoConsole.Call(
		uintptr(size),
		inR.Fd(),
		outW.Fd(),
		0,
		uintptr(unsafe.Pointer(&hPC)),
	)
	inR.Close()
	outW.Close()
	if r != 0 {
		inW.Close()
		outR.Close()
		return nil, fmt.Errorf("conpty: CreatePseudoConsole: %v", e)
	}

	attrSize := uintptr(0)
	procInitializeProcThreadAttributeList.Call(0, 1, 0, uintptr(unsafe.Pointer(&attrSize)))
	attrBuf := make([]byte, attrSize)
	attrList := uintptr(unsafe.Pointer(&attrBuf[0]))

	r, _, e = procInitializeProcThreadAttributeList.Call(attrList, 1, 0, uintptr(unsafe.Pointer(&attrSize)))
	if r == 0 {
		inW.Close()
		outR.Close()
		procClosePseudoConsole.Call(uintptr(hPC))
		return nil, fmt.Errorf("conpty: InitAttributeList: %v", e)
	}
	defer procDeleteProcThreadAttributeList.Call(attrList)

	r, _, e = procUpdateProcThreadAttribute.Call(
		attrList, 0,
		0x00020016,
		uintptr(unsafe.Pointer(&hPC)),
		unsafe.Sizeof(hPC),
		0, 0,
	)
	if r == 0 {
		inW.Close()
		outR.Close()
		procClosePseudoConsole.Call(uintptr(hPC))
		return nil, fmt.Errorf("conpty: UpdateAttribute: %v", e)
	}

	exe, err := exec.LookPath(cmd.Path)
	if err != nil {
		inW.Close()
		outR.Close()
		procClosePseudoConsole.Call(uintptr(hPC))
		return nil, fmt.Errorf("conpty: lookup: %w", err)
	}

	cmdLine := exe
	for _, a := range cmd.Args[1:] {
		cmdLine += " " + a
	}
	cmdLineUTF16, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		inW.Close()
		outR.Close()
		procClosePseudoConsole.Call(uintptr(hPC))
		return nil, fmt.Errorf("conpty: cmdline: %w", err)
	}

	var cwdUTF16 *uint16
	if cmd.Dir != "" {
		cwdUTF16, _ = windows.UTF16PtrFromString(cmd.Dir)
	}

	var envUTF16 *uint16
	if len(cmd.Env) > 0 {
		envUTF16 = createEnvBlock(cmd.Env)
	}

	type startupInfoEx struct {
		cb              uint32
		lpReserved      *uint16
		lpDesktop       *uint16
		lpTitle         *uint16
		dwX             uint32
		dwY             uint32
		dwXSize         uint32
		dwYSize         uint32
		dwXCountChars   uint32
		dwYCountChars   uint32
		dwFillAttribute uint32
		dwFlags         uint32
		wShowWindow     uint16
		cbReserved2     uint16
		lpReserved2     *byte
		hStdInput       windows.Handle
		hStdOutput      windows.Handle
		hStdError       windows.Handle
		lpAttributeList uintptr
	}

	si := startupInfoEx{}
	si.cb = uint32(unsafe.Sizeof(si))
	si.lpAttributeList = attrList

	var pi windows.ProcessInformation

	r, _, e = procCreateProcessW.Call(
		0,
		uintptr(unsafe.Pointer(cmdLineUTF16)),
		0, 0,
		0,
		uintptr(createUnicodeEnvironment|createNewProcessGroup|extendedStartupinfoPresent),
		uintptr(unsafe.Pointer(envUTF16)),
		uintptr(unsafe.Pointer(cwdUTF16)),
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if r == 0 {
		inW.Close()
		outR.Close()
		procClosePseudoConsole.Call(uintptr(hPC))
		return nil, fmt.Errorf("conpty: CreateProcess: %v", e)
	}

	windows.CloseHandle(pi.Process)
	windows.CloseHandle(pi.Thread)

	cmd.Process = &os.Process{Pid: int(pi.ProcessId)}

	return &conPty{
		hPC:       hPC,
		readPipe:  outR,
		writePipe: inW,
		cmd:       cmd,
	}, nil
}

func (c *conPty) Read(b []byte) (int, error)  { return c.readPipe.Read(b) }
func (c *conPty) Write(b []byte) (int, error) { return c.writePipe.Write(b) }
func (c *conPty) Close() error {
	c.readPipe.Close()
	c.writePipe.Close()
	procClosePseudoConsole.Call(uintptr(c.hPC))
	return nil
}

var (
	kernel32 = windows.NewLazyDLL("kernel32.dll")

	procCreatePseudoConsole              = kernel32.NewProc("CreatePseudoConsole")
	procClosePseudoConsole               = kernel32.NewProc("ClosePseudoConsole")
	procInitializeProcThreadAttributeList = kernel32.NewProc("InitializeProcThreadAttributeList")
	procUpdateProcThreadAttribute         = kernel32.NewProc("UpdateProcThreadAttribute")
	procDeleteProcThreadAttributeList     = kernel32.NewProc("DeleteProcThreadAttributeList")
	procCreateProcessW                    = kernel32.NewProc("CreateProcessW")
)

const (
	extendedStartupinfoPresent = 0x00080000
	createUnicodeEnvironment   = 0x00000400
	createNewProcessGroup      = 0x00000200
)

func createEnvBlock(env []string) *uint16 {
	total := ""
	for _, e := range env {
		total += e + "\x00"
	}
	total += "\x00"
	encoded := utf16.Encode([]rune(total))
	buf := make([]uint16, len(encoded))
	copy(buf, encoded)
	return &buf[0]
}
