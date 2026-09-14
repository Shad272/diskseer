//go:build !windows

package report

// PrepareConsole non ha nulla da fare fuori da Windows: i terminali Unix
// gestiscono UTF-8 e sequenze ANSI senza chiedere il permesso a nessuno.
func PrepareConsole() bool { return true }

// LanciatoDaEsploraRisorse riguarda solo Windows: altrove i programmi da
// terminale si avviano da un terminale, e nessuno prova ad aprirli con un
// doppio clic da un gestore di file.
func LanciatoDaEsploraRisorse() bool { return false }

// ConsolaDisegnaSimboli: fuori da Windows i terminali disegnano UTF-8 senza
// storie, e il font lo sceglie l'utente sapendo cosa sta scegliendo.
func ConsolaDisegnaSimboli() bool { return true }

// SospendiModificaRapida riguarda solo la console di Windows.
func SospendiModificaRapida() func() { return func() {} }

func consoleDimensions() (int, int) { return 80, 25 }
