//go:build !windows

package report

// PrepareConsole non ha nulla da fare fuori da Windows: i terminali Unix
// gestiscono UTF-8 e sequenze ANSI senza chiedere il permesso a nessuno.
func PrepareConsole() bool { return true }

// LanciatoDaEsploraRisorse riguarda solo Windows: altrove i programmi da
// terminale si avviano da un terminale, e nessuno prova ad aprirli con un
// doppio clic da un gestore di file.
func LanciatoDaEsploraRisorse() bool { return false }
