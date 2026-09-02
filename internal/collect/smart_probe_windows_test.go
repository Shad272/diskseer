//go:build windows

package collect

import (
	"os"
	"testing"

	"github.com/shad272/diskseer/internal/model"
)

// Diagnostico, non collaudo: interroga i dischi veri di questa macchina e
// riporta cosa risponde ognuno. Serve quando una lettura fallisce e bisogna
// capire a che livello — apertura, permessi, rifiuto del disco.
//
// È spento di default perché avvia PowerShell e tocca l'hardware: un test che
// dipende dalla macchina su cui gira non può stare nella suite normale.
//
//	DISKSEER_PROBE=1 go test ./internal/collect/ -run Probe -v
func TestProbeSMART(t *testing.T) {
	if os.Getenv("DISKSEER_PROBE") == "" {
		t.Skip("diagnostico disattivato: impostare DISKSEER_PROBE=1 per eseguirlo")
	}

	snap, err := collect()
	if err != nil {
		t.Fatalf("raccolta fallita: %v", err)
	}
	vie := []struct {
		nome string
		fn   func(string) (*model.SMARTData, error)
	}{
		{"ATA diretto", readSMART},
		{"SAT (ponte USB)", readSMARTViaSAT},
	}

	for _, d := range snap.Disks {
		t.Logf("=== disco %s: %s [%s] ===", d.DeviceID, d.Model, d.BusType)
		for _, via := range vie {
			s, err := via.fn(d.DeviceID)
			if err != nil {
				t.Logf("  %-16s FALLITO: %v", via.nome, err)
				continue
			}
			t.Logf("  %-16s OK: %d attributi", via.nome, len(s.Attributes))
			for _, a := range s.Attributes {
				t.Logf("      %3d  %-42s cur=%3d worst=%3d raw=%d",
					a.ID, a.Name, a.Current, a.Worst, a.Raw)
			}
			break
		}
	}
}
