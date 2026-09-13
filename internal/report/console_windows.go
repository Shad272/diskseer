//go:build windows

package report

import (
	"os"
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

	// Modalità dell'ingresso della console. La modifica rapida si può
	// cambiare solo dichiarando anche i flag estesi: senza, SetConsoleMode
	// ignora il bit in silenzio.
	enableQuickEditMode = 0x0040
	enableExtendedFlags = 0x0080
)

// SospendiModificaRapida spegne la modifica rapida della console e restituisce
// la funzione che la rimette com'era.
//
// La modifica rapida è l'opzione di Windows per cui un clic nella finestra
// comincia a selezionare testo. Nella vista dal vivo fa due danni, ed entrambi
// sembrano guasti del programma:
//
//   - finché c'è una selezione la console sospende l'uscita, e i valori
//     smettono di aggiornarsi senza nessun motivo apparente;
//   - con del testo selezionato il primo Ctrl+C copia la selezione invece di
//     arrivare al programma, e per fermare la vista serve premerlo due volte.
//
// La modifica viene rimessa com'era all'uscita: è un'impostazione della
// finestra, non di diskseer, e chi ha lanciato il programma dal proprio
// terminale deve ritrovarselo come l'aveva lasciato.
//
// Se l'ingresso non è una console, o la modifica rapida era già spenta, non si
// tocca niente e la funzione restituita non fa niente.
func SospendiModificaRapida() func() {
	h, err := syscall.GetStdHandle(syscall.STD_INPUT_HANDLE)
	if err != nil {
		return func() {}
	}
	var modo uint32
	if r, _, _ := procGetConsoleMode.Call(uintptr(h), uintptr(unsafe.Pointer(&modo))); r == 0 {
		return func() {}
	}
	if modo&enableQuickEditMode == 0 {
		return func() {}
	}

	senza := (modo | enableExtendedFlags) &^ enableQuickEditMode
	if r, _, _ := procSetConsoleMode.Call(uintptr(h), uintptr(senza)); r == 0 {
		return func() {}
	}
	return func() {
		procSetConsoleMode.Call(uintptr(h), uintptr(modo|enableExtendedFlags))
	}
}

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

// ConsolaDisegnaSimboli dice se conviene usare i caratteri decorativi del
// referto: pallini, frecce, barre, il grado delle temperature, i mezzi blocchi
// del banner.
//
// La domanda vera sarebbe "questo font contiene quei caratteri?", e non esiste
// un modo di chiederlo. Il primo tentativo guardava se il font della console
// fosse vettoriale o raster, sul presupposto che un font vettoriale li
// disegnasse tutti. Il presupposto e' falso, e l'ha smentito una macchina vera:
// console con font Consolas, vettoriale, codepage UTF-8, e il simbolo del grado
// che esce come due quadratini.
//
// Quindi si e' ribaltato il criterio. Invece di cercare la prova che la console
// NON sappia disegnare, si cerca la prova che sappia: un terminale moderno si
// annuncia da solo con una variabile d'ambiente, e quelli che si annunciano
// disegnano tutto. La console classica di Windows non si annuncia, e su quella
// si sta prudenti.
//
// Il caso dell'uscita rediretta e' l'opposto: i caratteri finiscono in un file
// UTF-8 dove si leggono benissimo, e impoverirli sarebbe un danno gratuito.
func ConsolaDisegnaSimboli() bool {
	if terminaleCheSiAnnuncia() {
		return true
	}

	h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return true
	}
	var mode uint32
	if r, _, _ := procGetConsoleMode.Call(uintptr(h), uintptr(unsafe.Pointer(&mode))); r == 0 {
		// Non e' una console: e' un file o una pipe.
		return true
	}

	// Console classica di Windows, che non dichiara nulla di se'.
	return false
}

// terminaleCheSiAnnuncia riconosce i terminali che dicono di esserci.
//
// Nessuna di queste variabili la mette la console classica di Windows: e' il
// motivo per cui funzionano come riconoscimento. WT_SESSION la scrive Terminale
// di Windows, ConEmuANSI ConEmu, TERM_PROGRAM gli editor che ospitano un
// terminale, TERM e ANSICON il mondo Unix e i suoi portati su Windows.
func terminaleCheSiAnnuncia() bool {
	for _, nome := range []string{
		"WT_SESSION", "WT_PROFILE_ID", "ConEmuANSI",
		"TERM_PROGRAM", "TERM", "ANSICON",
	} {
		if os.Getenv(nome) != "" {
			return true
		}
	}
	return false
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
