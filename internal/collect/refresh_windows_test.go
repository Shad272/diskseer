//go:build windows

package collect

import (
	"os"
	"testing"
	"time"
)

// Refresh esiste per una sola ragione: essere abbastanza veloce da poter
// girare in un ciclo di pochi secondi. Se non lo fosse, tanto varrebbe
// richiamare Collect e la funzione non avrebbe motivo di esistere.
//
// Spento di default perché tocca l'hardware di questa macchina:
//
//	DISKSEER_PROBE=1 go test ./internal/collect/ -run Velocita -v
func TestVelocitaDiRefresh(t *testing.T) {
	if os.Getenv("DISKSEER_PROBE") == "" {
		t.Skip("diagnostico disattivato: impostare DISKSEER_PROBE=1")
	}

	inizio := time.Now()
	snap, err := Collect()
	if err != nil {
		t.Fatalf("raccolta iniziale: %v", err)
	}
	raccolta := time.Since(inizio)

	inizio = time.Now()
	Refresh(&snap)
	aggiornamento := time.Since(inizio)

	t.Logf("Collect: %v", raccolta.Round(time.Millisecond))
	t.Logf("Refresh: %v", aggiornamento.Round(time.Millisecond))
	t.Logf("rapporto: %.0fx più veloce", float64(raccolta)/float64(aggiornamento))

	if aggiornamento > time.Second {
		t.Errorf("Refresh ha impiegato %v: troppo per un ciclo di pochi secondi",
			aggiornamento.Round(time.Millisecond))
	}
}
