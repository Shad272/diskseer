package report

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shad272/diskseer/internal/i18n"
)

// foglioSicuro raccoglie ciò che viene scritto da un'altra goroutine.
//
// La rotella disegna da sola, in parallelo a chi l'ha avviata: un
// bytes.Buffer nudo verrebbe scritto e letto da due goroutine diverse, che è
// esattamente la situazione che il rilevatore di corse segnala.
type foglioSicuro struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (f *foglioSicuro) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.b.Write(p)
}

func (f *foglioSicuro) String() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.b.String()
}

func TestLaRotellaPassaPerTutteQuattroLePosizioni(t *testing.T) {
	var foglio foglioSicuro

	attesa := Printer{W: &foglio, Lang: i18n.EN}.Attendi("loading")
	time.Sleep(passoRotella * 6)
	attesa.Ferma()

	uscita := foglio.String()
	for _, fotogramma := range []string{`-`, `\`, `|`, `/`} {
		if !strings.Contains(uscita, "\r  "+fotogramma+"  ") {
			t.Errorf("manca il fotogramma %q: la rotella non sembrerebbe girare", fotogramma)
		}
	}
	if !strings.Contains(uscita, "loading.") {
		t.Error("manca la scritta con i puntini")
	}
}

// Ferma deve ripulire la riga, altrimenti la prima riga del referto si trova
// davanti i resti dell'ultimo fotogramma.
func TestFermaCancellaLaRiga(t *testing.T) {
	var foglio foglioSicuro

	attesa := Printer{W: &foglio, Lang: i18n.EN}.Attendi("loading")
	time.Sleep(passoRotella * 2)
	attesa.Ferma()

	uscita := foglio.String()
	if !strings.HasSuffix(uscita, "\r") {
		t.Error("la pulizia finale non riporta il cursore a inizio riga")
	}

	// Dopo l'ultimo ritorno a capo ci devono essere solo spazi: è quello che
	// copre il disegno precedente.
	pezzi := strings.Split(strings.TrimSuffix(uscita, "\r"), "\r")
	ultimo := pezzi[len(pezzi)-1]
	if strings.TrimSpace(ultimo) != "" {
		t.Errorf("l'ultima scrittura prima della pulizia non è fatta di spazi: %q", ultimo)
	}
}

// Chi non ha avviato la rotella — perché l'uscita non era un terminale — non
// deve ricordarsi di controllare se esiste prima di fermarla.
func TestFermareUnaRotellaMaiAvviataNonRompeNiente(t *testing.T) {
	var mai *Attesa
	mai.Ferma()
}

// I fotogrammi stanno nell'alfabeto di base: la rotella deve girare anche
// sulle console che non disegnano nient'altro, che sono quelle a cui serve di
// più un segno di vita.
func TestLaRotellaFunzionaAncheInModalitaPiana(t *testing.T) {
	var foglio foglioSicuro

	attesa := Printer{W: Uscita(&foglio, true), Lang: i18n.IT}.Attendi("caricamento")
	time.Sleep(passoRotella * 2)
	attesa.Ferma()

	uscita := foglio.String()
	for i, r := range uscita {
		if r > 127 {
			t.Fatalf("carattere %q (U+%04X) alla posizione %d", r, r, i)
		}
	}
	if !strings.Contains(uscita, "caricamento") {
		t.Error("la scritta è sparita nella conversione")
	}
}
