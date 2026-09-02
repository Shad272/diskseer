//go:build windows

package collect

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/shad272/diskseer/internal/model"
)

//go:embed probe_windows.ps1
var probeScript string

// Lo script viene scritto su file temporaneo ed eseguito con -File.
//
// L'alternativa ovvia sarebbe -EncodedCommand, che eviterebbe il file: è
// esattamente ciò che NON si deve fare in un programma destinato a girare
// sui PC altrui. PowerShell con comando codificato in base64 è una delle
// firme più note del malware, e antivirus ed EDR lo bloccano o lo segnalano.
// Un tool di diagnostica che fa scattare l'antivirus del cliente non verrà
// mai più aperto.
func collect() (model.Snapshot, error) {
	dir, err := os.MkdirTemp("", "diskseer-")
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("cartella temporanea: %w", err)
	}
	defer os.RemoveAll(dir)

	script := filepath.Join(dir, "probe.ps1")
	if err := os.WriteFile(script, []byte(probeScript), 0o600); err != nil {
		return model.Snapshot{}, fmt.Errorf("scrittura script: %w", err)
	}

	cmd := exec.Command("powershell.exe",
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-File", script)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("esecuzione raccoglitore: %w", err)
	}

	var snap model.Snapshot
	if err := json.Unmarshal(out, &snap); err != nil {
		return model.Snapshot{}, fmt.Errorf("json non valido dal raccoglitore: %w", err)
	}
	enrichNVMe(&snap)
	enrichSMART(&snap)
	snap.Time = time.Now()
	return snap, nil
}
