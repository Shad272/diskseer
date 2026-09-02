//go:build !windows

package report

// PrepareConsole non ha nulla da fare fuori da Windows: i terminali Unix
// gestiscono UTF-8 e sequenze ANSI senza chiedere il permesso a nessuno.
func PrepareConsole() bool { return true }
