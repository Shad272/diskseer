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
