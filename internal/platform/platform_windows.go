//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")

func Detect() Capabilities {
	c := base()
	c.InTerminal = os.Getenv("WT_SESSION") != "" || os.Getenv("WT_PROFILE_ID") != ""
	version := struct {
		Size, Major, Minor, Build, Platform uint32
		Text                                [128]uint16
	}{}
	version.Size = uint32(unsafe.Sizeof(version))
	rtl := syscall.NewLazyDLL("ntdll.dll").NewProc("RtlGetVersion")
	if rtl.Find() == nil {
		if r, _, _ := rtl.Call(uintptr(unsafe.Pointer(&version))); r == 0 {
			c.Major, c.Minor, c.Build = version.Major, version.Minor, version.Build
		}
	}
	mem := struct {
		Size, Load                                                       uint32
		Total, Available, Page, FreePage, Virtual, FreeVirtual, Extended uint64
	}{}
	mem.Size = uint32(unsafe.Sizeof(mem))
	p := kernel.NewProc("GlobalMemoryStatusEx")
	if p.Find() == nil {
		if r, _, _ := p.Call(uintptr(unsafe.Pointer(&mem))); r != 0 {
			c.PhysicalMemory, c.AvailableMemory, c.MemoryLoad = mem.Total, mem.Available, mem.Load
		}
	}
	// SYSTEM_INFO has a processor architecture WORD at offset zero on all targets.
	var info [64]byte
	p = kernel.NewProc("GetNativeSystemInfo")
	if p.Find() == nil {
		p.Call(uintptr(unsafe.Pointer(&info[0])))
		switch info[0] {
		case 0:
			c.NativeArchitecture = "386"
		case 9:
			c.NativeArchitecture = "amd64"
		case 12:
			c.NativeArchitecture = "arm64"
		}
	}
	p = kernel.NewProc("IsProcessorFeaturePresent")
	if p.Find() == nil {
		r, _, _ := p.Call(10)
		c.SSE2 = r != 0
	}
	mode := kernel.NewProc("GetConsoleMode")
	if mode.Find() == nil {
		var value uint32
		r, _, _ := mode.Call(os.Stdout.Fd(), uintptr(unsafe.Pointer(&value)))
		c.Console = r != 0
		r, _, _ = mode.Call(os.Stdin.Fd(), uintptr(unsafe.Pointer(&value)))
		c.InputConsole = r != 0
	}
	c.VTAvailable = kernel.NewProc("CreatePseudoConsole").Find() == nil
	p = kernel.NewProc("GetConsoleProcessList")
	if c.Console && c.InputConsole && p.Find() == nil {
		var pids [2]uint32
		r, _, _ := p.Call(uintptr(unsafe.Pointer(&pids[0])), 2)
		c.OwnConsole = r == 1 && !c.InTerminal
	}
	return c
}

// SystemExecutable obtains the system directory from Windows, not PATH or a
// fixed drive letter. The returned name is used only for trusted OS utilities.
func SystemExecutable(name string) string {
	var buf [32768]uint16
	p := kernel.NewProc("GetSystemDirectoryW")
	if p.Find() != nil {
		return ""
	}
	n, _, _ := p.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 || n >= uintptr(len(buf)) {
		return ""
	}
	return filepath.Join(syscall.UTF16ToString(buf[:n]), name)
}
