//go:build windows

// Package elevate riguarda i privilegi di amministratore: capire se ci sono e,
// quando serve, chiederli.
//
// Serve perché senza elevazione Windows non lascia interrogare i dischi
// collegati via SATA e USB: la diagnosi resta possibile ma parziale, e su una
// macchina con un disco che sta cedendo la parte mancante è proprio quella
// che conta.
package elevate

import (
	"strings"
	"syscall"
	"unsafe"
)

var (
	shell32           = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

// tokenElevation è l'informazione da chiedere al sistema per sapere se il
// processo sta girando con privilegi elevati.
const tokenElevation = 20

// ShellExecuteW restituisce un valore maggiore di 32 solo in caso di successo.
// È una convenzione che risale a Windows a 16 bit: il valore restituito era un
// vero riferimento a un modulo, e i numeri bassi erano riservati ai codici di
// errore. Il riferimento non esiste più da trent'anni, la convenzione sì.
const soglaSuccessoShellExecute = 32

// Elevato dice se il processo sta già girando come amministratore.
func Elevato() bool {
	processo, err := syscall.GetCurrentProcess()
	if err != nil {
		return false
	}

	var token syscall.Token
	if err := syscall.OpenProcessToken(processo, syscall.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()

	var elevato uint32
	var restituiti uint32
	err = syscall.GetTokenInformation(token, tokenElevation,
		(*byte)(unsafe.Pointer(&elevato)), uint32(unsafe.Sizeof(elevato)), &restituiti)
	if err != nil {
		return false
	}
	return elevato != 0
}

// Richiedi riavvia il programma chiedendo i privilegi di amministratore, e
// dice se il nuovo processo è partito.
//
// Restituisce false anche quando l'utente rifiuta la richiesta, e non è un
// errore: è una risposta. Chi chiama deve proseguire con la diagnosi parziale
// invece di interrompersi, perché una diagnosi incompleta vale comunque più di
// nessuna diagnosi — a patto di dichiarare che è incompleta.
func Richiedi(eseguibile string, argomenti []string) bool {
	verbo, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return false
	}
	file, err := syscall.UTF16PtrFromString(eseguibile)
	if err != nil {
		return false
	}
	parametri, err := syscall.UTF16PtrFromString(componiArgomenti(argomenti))
	if err != nil {
		return false
	}

	const mostraNormale = 1
	r, _, _ := procShellExecuteW.Call(
		0, // nessuna finestra padre
		uintptr(unsafe.Pointer(verbo)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(parametri)),
		0, // cartella di lavoro: quella corrente
		mostraNormale,
	)
	return r > soglaSuccessoShellExecute
}

// componiArgomenti rimette insieme gli argomenti in un'unica stringa, che è il
// formato che ShellExecuteW si aspetta.
//
// Quelli che contengono spazi vanno racchiusi fra virgolette, altrimenti il
// nuovo processo li riceverebbe spezzati: --cliente "Mario Rossi" diventerebbe
// due argomenti separati e il referto uscirebbe intestato a "Mario".
func componiArgomenti(argomenti []string) string {
	pezzi := make([]string, 0, len(argomenti))
	for _, a := range argomenti {
		if strings.ContainsAny(a, " \t") {
			a = `"` + strings.ReplaceAll(a, `"`, `\"`) + `"`
		}
		pezzi = append(pezzi, a)
	}
	return strings.Join(pezzi, " ")
}
