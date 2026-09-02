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

var procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")

// LanciatoDaEsploraRisorse dice se la finestra del terminale è stata creata
// apposta per questo programma, cioè se l'utente ha fatto doppio clic
// sull'eseguibile invece di scrivere il comando in un terminale già aperto.
//
// Serve a risolvere un problema che sembra un guasto e non lo è: un programma
// da terminale avviato con doppio clic stampa il suo referto e finisce, e nel
// momento in cui finisce la finestra creata per lui non ha più motivo di
// esistere e si chiude. Il risultato appare per un decimo di secondo. Chi
// guarda conclude che il programma non funziona.
//
// Il modo di accorgersene è contare quanti processi sono attaccati alla
// console: se siamo soli, la console è nata con noi e morirà con noi. Se ce
// n'è un altro, è la shell da cui siamo stati lanciati, e la finestra resterà
// aperta anche dopo.
func LanciatoDaEsploraRisorse() bool {
	var pids [2]uint32
	n, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	return n == 1
}
