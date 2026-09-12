package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/rules"
)

func snapshotDiProva(t *testing.T) model.Snapshot {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "snapshot-completo.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snap model.Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	return snap
}

// In modalità piana non deve uscire nemmeno un carattere fuori dall'alfabeto
// di base, da nessuna parte.
//
// È il controllo che conta davvero, perché il difetto che questa modalità
// evita non si manifesta qui: si manifesta sulla console di qualcun altro,
// dove un solo carattere non disegnabile diventa un quadratino in mezzo a una
// diagnosi. Il collaudo passa quindi tutto quello che il programma sa
// stampare — banner, esito, verdetti, inventario, contatori grezzi — e non
// solo le decorazioni che ho ricordato di sostituire.
//
// La lingua è l'italiano di proposito: è quella con gli accenti, cioè quella
// che scopre le dimenticanze.
func TestInModalitaPianaEsceSoloASCII(t *testing.T) {
	snap := snapshotDiProva(t)
	findings := rules.Run(snap, i18n.IT)

	var buf bytes.Buffer
	piano := Uscita(&buf, true)

	fmt.Fprint(piano, Banner(false))
	fmt.Fprint(piano, Banner(true)) // anche con i colori accesi

	p := Printer{W: piano, Lang: i18n.IT}
	p.Print(snap, findings)
	p.PrintDetails(snap)
	p.PrintLive(snap, findings, LiveOptions{Intervallo: 3e9})

	for i, r := range buf.String() {
		if r > 127 {
			inizio := i - 40
			if inizio < 0 {
				inizio = 0
			}
			fine := i + 40
			if fine > buf.Len() {
				fine = buf.Len()
			}
			t.Fatalf("carattere %q (U+%04X) alla posizione %d: una console essenziale "+
				"non lo disegnerebbe.\ncontesto: %q", r, r, i, buf.String()[inizio:fine])
		}
	}
}

// Senza la modalità piana non deve cambiare niente: il flusso restituito è
// esattamente quello ricevuto, e i caratteri decorativi arrivano interi.
func TestSenzaModalitaPianaNonSiTocca(t *testing.T) {
	var buf bytes.Buffer
	w := Uscita(&buf, false)

	if w != (&buf) {
		t.Error("con piano a false Uscita deve restituire lo stesso flusso, senza incartarlo")
	}
	fmt.Fprint(w, "● 30 °C → ok")
	if got := buf.String(); got != "● 30 °C → ok" {
		t.Errorf("testo alterato senza che fosse richiesto: %q", got)
	}
}

// Chi scrive deve ricevere indietro il numero di byte che ha passato.
//
// La sostituzione cambia la lunghezza del testo — "→" occupa tre byte e
// diventa "->" che ne occupa due — e un Write che dichiarasse i byte
// realmente usciti verrebbe interpretato come scrittura incompleta.
func TestLaScritturaDichiaraIByteRicevuti(t *testing.T) {
	var buf bytes.Buffer
	w := Uscita(&buf, true)

	testo := "→ 30 °C ●"
	n, err := w.Write([]byte(testo))
	if err != nil {
		t.Fatal(err)
	}
	if n != len(testo) {
		t.Errorf("Write ha dichiarato %d byte su %d ricevuti: chi chiama lo legge "+
			"come scrittura incompleta", n, len(testo))
	}
	if got := buf.String(); got != "-> 30 C *" {
		t.Errorf("sostituzione sbagliata: %q", got)
	}
}

// Gli accenti sopravvivono come lettere: su queste console "più" diventerebbe
// un quadratino in mezzo a una parola, e "piu" almeno si legge.
func TestGliAccentiDiventanoLettere(t *testing.T) {
	var buf bytes.Buffer
	fmt.Fprint(Uscita(&buf, true), "è più però verità")

	got := buf.String()
	if got != "e piu pero verita" {
		t.Errorf("accenti = %q, atteso %q", got, "e piu pero verita")
	}
	if strings.ContainsAny(got, "àèéìòù") {
		t.Error("è rimasto un accento")
	}
}
