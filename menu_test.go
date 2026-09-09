package main

import (
	"testing"
	"unicode/utf8"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/settings"
)

func sessioneDiProva(l i18n.Lingua) *sessione {
	return &sessione{lingua: l, cfg: settings.Predefinite()}
}

// Le etichette del menu devono stare dentro la loro colonna in tutte e due le
// lingue.
//
// Non è pignoleria tipografica: le voci sono incolonnate con una larghezza
// fissa, e una sola etichetta più lunga sposta a destra la spiegazione di
// quella riga soltanto, scalinando l'elenco. Succede solo in italiano, dove le
// stesse parole sono più lunghe — cioè nella lingua che chi scrive il codice
// guarda per ultima.
func TestOgniEtichettaStaNellaSuaColonna(t *testing.T) {
	for _, l := range []i18n.Lingua{i18n.EN, i18n.IT} {
		s := sessioneDiProva(l)

		for _, v := range s.voci() {
			if n := utf8.RuneCountInString(v.titolo); n > larghezzaVoce {
				t.Errorf("[%s] la voce %q è lunga %d caratteri, la colonna ne tiene %d",
					l, v.titolo, n, larghezzaVoce)
			}
		}
		for _, r := range s.righeImpostazioni() {
			if n := utf8.RuneCountInString(r.etichetta); n > larghezzaVoce {
				t.Errorf("[%s] l'impostazione %q è lunga %d caratteri, la colonna ne tiene %d",
					l, r.etichetta, n, larghezzaVoce)
			}
		}
	}
}

// Il menu deve esistere per intero in entrambe le lingue: una voce dimenticata
// resterebbe in inglese in mezzo a un menu italiano.
func TestIlMenuEsisteInEntrambeLeLingue(t *testing.T) {
	en := sessioneDiProva(i18n.EN).voci()
	it := sessioneDiProva(i18n.IT).voci()

	if len(en) != len(it) {
		t.Fatalf("il menu ha %d voci in inglese e %d in italiano", len(en), len(it))
	}
	for i := range en {
		if en[i].titolo == it[i].titolo {
			t.Errorf("la voce %q non è tradotta", en[i].titolo)
		}
		if en[i].titolo == "" || it[i].titolo == "" {
			t.Errorf("la voce numero %d non ha titolo", i+1)
		}
	}
}

// L'ultima voce chiude il programma, e deve restare l'ultima: è quella che
// l'utente cerca per uscire, e in un elenco che cambia lunghezza a seconda dei
// privilegi è l'unico punto fermo.
func TestChiudereEUltimaVoce(t *testing.T) {
	s := sessioneDiProva(i18n.EN)
	voci := s.voci()

	ultima := voci[len(voci)-1]
	if !ultima.fai(s) {
		t.Error("l'ultima voce del menu non chiude diskseer")
	}
	for i, v := range voci[:len(voci)-1] {
		if v.fai == nil {
			t.Errorf("la voce numero %d non fa niente", i+1)
		}
	}
}

func TestIndiceVoce(t *testing.T) {
	casi := []struct {
		scelta string
		quante int
		vuole  int
		errore bool
	}{
		{"1", 8, 0, false},
		{"8", 8, 7, false},
		{"0", 8, 0, true},
		{"9", 8, 0, true},
		{"-1", 8, 0, true},
		{"banana", 8, 0, true},
		{"", 8, 0, true},
	}
	for _, c := range casi {
		i, err := indiceVoce(c.scelta, c.quante)
		if c.errore {
			if err == nil {
				t.Errorf("scelta %q accettata, doveva essere rifiutata", c.scelta)
			}
			continue
		}
		if err != nil {
			t.Errorf("scelta %q rifiutata: %v", c.scelta, err)
			continue
		}
		if i != c.vuole {
			t.Errorf("scelta %q = indice %d, atteso %d", c.scelta, i, c.vuole)
		}
	}
}
