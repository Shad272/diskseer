// Package gui apre il referto interattivo nell'applicazione predefinita del
// sistema. Il report resta un normale file HTML: nessun server locale, porta
// aperta o processo in background rallenta l'avvio o espone dati diagnostici.
package gui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Open apre path nel browser predefinito. Il comando viene avviato senza
// attenderlo, così diskseer può terminare appena il report è pronto.
func Open(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("percorso del referto: %w", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", abs)
	case "darwin":
		cmd = exec.Command("open", abs)
	default:
		cmd = exec.Command("xdg-open", abs)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("apertura interfaccia: %w", err)
	}
	return nil
}
