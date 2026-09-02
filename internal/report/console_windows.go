//go:build windows

package report

import (
	"syscall"
	"unsafe"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleOutputCP = kernel32.NewProc("SetConsoleOutputCP")
	procGetConsoleMode     = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode     = kernel32.NewProc("SetConsoleMode")
)

const (
	codepageUTF8                    = 65001
	enableVirtualTerminalProcessing = 0x0004
)

// PrepareConsole predispone la console di Windows e dice se supporta i colori.
//
// Servono due cose che su Linux e macOS sono gratis:
//
//  1. la codepage UTF-8, altrimenti "è" e "più" escono come caratteri
//     illeggibili su tutte le installazioni italiane di Windows;
//  2. l'interpretazione delle sequenze ANSI, spenta di default nelle console
//     piu' vecchie.
//
// Se GetConsoleMode fallisce, stdout non e' una console: significa che
// l'output e' rediretto su file o in pipe, e in quel caso i colori vanno
// omessi. E' il motivo per cui questa funzione restituisce un valore invece
// di limitarsi ad agire.
func PrepareConsole() bool {
	procSetConsoleOutputCP.Call(uintptr(codepageUTF8))

	h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return false
	}
	var mode uint32
	if r, _, _ := procGetConsoleMode.Call(uintptr(h), uintptr(unsafe.Pointer(&mode))); r == 0 {
		return false
	}
	r, _, _ := procSetConsoleMode.Call(uintptr(h), uintptr(mode|enableVirtualTerminalProcessing))
	return r != 0
}
