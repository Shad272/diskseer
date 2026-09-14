//go:build windows

package collect

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/platform"
	"github.com/shad272/diskseer/internal/tuning"
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
func collectContext(ctx context.Context, p tuning.Profile) (model.Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "diskseer-")
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("cartella temporanea: %w", err)
	}
	defer os.RemoveAll(dir)

	script := filepath.Join(dir, "probe.ps1")
	// Windows PowerShell 2/5 reads UTF-8 scripts correctly with an explicit BOM.
	if err := os.WriteFile(script, append([]byte{0xef, 0xbb, 0xbf}, []byte(probeScript)...), 0o600); err != nil {
		return model.Snapshot{}, fmt.Errorf("scrittura script: %w", err)
	}

	ps := platform.SystemExecutable(filepath.Join("WindowsPowerShell", "v1.0", "powershell.exe"))
	if _, err := os.Stat(ps); err != nil {
		ps, err = exec.LookPath("powershell.exe")
		if err != nil {
			return model.Snapshot{}, fmt.Errorf("Windows PowerShell unavailable; inventory cannot be collected")
		}
	}
	cmd := exec.CommandContext(ctx, ps,
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-File", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.WaitDelay = time.Second
	limit := p.OutputLimit
	if limit < 1 {
		limit = 4 << 20
	}
	out := &boundedBuffer{limit: limit}
	cmd.Stdout = out
	err = cmd.Run()
	if ctx.Err() != nil {
		return model.Snapshot{}, fmt.Errorf("collection cancelled or timed out: %w", ctx.Err())
	}
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("esecuzione raccoglitore: %w", err)
	}

	var snap model.Snapshot
	if err := json.Unmarshal(bytes.TrimPrefix(out.Bytes(), []byte{0xef, 0xbb, 0xbf}), &snap); err != nil {
		return model.Snapshot{}, fmt.Errorf("json non valido dal raccoglitore: %w", err)
	}
	// Le letture NVMe e SMART parlano a famiglie di dispositivi diverse e non
	// dipendono l'una dall'altra. Eseguirle insieme evita che il tempo di I/O
	// di una famiglia si sommi a quello dell'altra sui PC con più dischi.
	if len(snap.Disks) == 0 && len(snap.Volumes) == 0 {
		return snap, fmt.Errorf("inventory unavailable: storage and WMI returned no disks or volumes")
	}
	hdd := false
	for _, d := range snap.Disks {
		if d.IsSystemDisk && d.MediaType == "HDD" {
			hdd = true
		}
	}
	parallel(ctx, len(snap.Disks), p.DiskWorkers(len(snap.Disks), hdd), func(i int) {
		one := model.Snapshot{Disks: []model.Disk{snap.Disks[i]}}
		if one.Disks[0].BusType == "Unspecified" || one.Disks[0].BusType == "" {
			identifyDisk(&one.Disks[0])
		}
		if one.Disks[0].BusType == "NVMe" {
			enrichNVMe(&one)
		} else {
			enrichSMART(&one)
		}
		snap.Disks[i] = one.Disks[0]
	})
	if ctx.Err() != nil {
		return snap, ctx.Err()
	}
	snap.Time = time.Now()
	return snap, nil
}
