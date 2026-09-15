package settings

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// cartellaFinta dirotta le impostazioni in una cartella temporanea, così il
// collaudo non tocca il file vero dell'utente che sta eseguendo i test.
func cartellaFinta(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AppData", dir)         // Windows
	t.Setenv("XDG_CONFIG_HOME", dir) // il resto del mondo
}

func TestSalvaERilegge(t *testing.T) {
	cartellaFinta(t)

	c := Predefinite()
	c.Language = "it"
	c.Technician = "Mario Rossi"
	c.Interval = "5s"
	if _, err := c.Salva(); err != nil {
		t.Fatal(err)
	}

	riletta := Carica()
	if riletta.Language != "it" || riletta.Technician != "Mario Rossi" || riletta.Interval != "5s" {
		t.Errorf("le impostazioni non sono tornate indietro intatte: %+v", riletta)
	}
	if riletta.Durata() != 5*time.Second {
		t.Errorf("intervallo = %v, atteso 5s", riletta.Durata())
	}
}

// Un file salvato da una versione vecchia non contiene le chiavi aggiunte
// dopo. Quelle chiavi devono restare al valore predefinito, non diventare zero.
//
// È il difetto classico di chi legge la configurazione dentro una struttura
// vuota: aggiungi un'impostazione, la rilasci, e a tutti quelli che avevano già
// un file salvato si spegne qualcosa che non hanno mai chiesto di spegnere.
func TestLeChiaviMancantiRestanoAiValoriPredefiniti(t *testing.T) {
	cartellaFinta(t)

	percorso, err := Percorso()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(percorso), 0o700); err != nil {
		t.Fatal(err)
	}
	// Un file che conosce solo la lingua: niente colori, niente intervallo.
	if err := os.WriteFile(percorso, []byte(`{"language":"it"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	c := Carica()
	if c.Language != "it" {
		t.Errorf("lingua = %q, attesa it", c.Language)
	}
	if !c.Colors {
		t.Error("i colori si sono spenti da soli: le chiavi assenti dal file " +
			"stanno sovrascrivendo i valori predefiniti invece di lasciarli stare")
	}
	if c.Durata() != 3*time.Second {
		t.Errorf("intervallo = %v, atteso il predefinito di 3s", c.Durata())
	}
}

// Impostazioni illeggibili non devono impedire una diagnosi.
func TestUnFileRottoNonFermaIlProgramma(t *testing.T) {
	cartellaFinta(t)

	percorso, err := Percorso()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(percorso), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(percorso, []byte("questo non e' JSON {{{"), 0o600); err != nil {
		t.Fatal(err)
	}

	if c := Carica(); c.Language != "en" || !c.Colors {
		t.Errorf("un file rotto non ha prodotto i valori predefiniti: %+v", c)
	}
}

func TestIntervalliAssurdiTornanoAlPredefinito(t *testing.T) {
	for _, scritto := range []string{"", "banana", "0s", "100ms", "-5s"} {
		c := Config{Interval: scritto}
		if got := c.Durata(); got != 3*time.Second {
			t.Errorf("intervallo %q = %v, atteso 3s", scritto, got)
		}
	}
}

// I tre stati dei simboli.
//
// Il valore "auto" delega al riconoscimento del terminale; gli altri due lo
// scavalcano nelle due direzioni opposte. Un valore che non si riconosce vale
// "auto": un refuso in un file scritto a mano non deve produrre un
// comportamento che nessuno riesce a spiegarsi.
func TestITreStatiDeiSimboli(t *testing.T) {
	casi := []struct {
		symbols      string
		consolaRicca bool
		vuolePiani   bool
	}{
		{SimboliAuto, true, false},
		{SimboliAuto, false, true},
		{SimboliRicchi, false, false},
		{SimboliRicchi, true, false},
		{SimboliPiani, true, true},
		{SimboliPiani, false, true},
		{"", true, false},
		{"", false, true},
		{"banana", true, false},
		{"banana", false, true},
	}
	for _, c := range casi {
		got := Config{Symbols: c.symbols}.Piani(c.consolaRicca)
		if got != c.vuolePiani {
			t.Errorf("Symbols=%q consolaRicca=%v: piani=%v, atteso %v",
				c.symbols, c.consolaRicca, got, c.vuolePiani)
		}
	}
}

func TestIPredefinitiScelgonoIlRiconoscimentoAutomatico(t *testing.T) {
	if c := Predefinite(); c.Symbols != SimboliAuto {
		t.Errorf("Symbols predefinito = %q, atteso %q", c.Symbols, SimboliAuto)
	}
}

func TestInvalidFieldDoesNotApplyPartialConfiguration(t *testing.T) {
	cartellaFinta(t)
	path, err := Percorso()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	// Unmarshal assigns valid fields even when another field has the wrong type.
	if err := os.WriteFile(path, []byte(`{"language":"it","colors":false,"interval":42}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Carica(); got != Predefinite() {
		t.Fatalf("invalid settings were partially applied: %+v", got)
	}
}

// Un diskseer appena installato resta nella finestra in cui si apre.
//
// Il rilancio in un altro terminale passa da una sequenza — PowerShell che
// scorre le app installate, poi il programma che riapre se stesso — che
// Bitdefender ha già messo in quarantena. Deve essere una scelta dell'utente,
// non il comportamento di chiunque faccia doppio clic.
//
// Il secondo controllo copre chi ha un file di impostazioni salvato prima che
// la chiave esistesse: la chiave assente deve lasciare il predefinito, non
// tornare al rilancio.
func TestIlTerminaleDiAvvioPredefinitoEQuestaFinestra(t *testing.T) {
	if got := Predefinite().Terminal; got != TerminaleQuestaFinestra {
		t.Errorf("terminale predefinito = %q, atteso %q", got, TerminaleQuestaFinestra)
	}

	cartellaFinta(t)
	percorso, err := Percorso()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(percorso), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(percorso, []byte(`{"language":"it"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Carica().Terminal; got != TerminaleQuestaFinestra {
		t.Errorf("con un file senza la chiave terminal il terminale è %q, atteso %q", got, TerminaleQuestaFinestra)
	}
}
