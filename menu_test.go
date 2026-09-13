package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"testing"
	"unicode/utf8"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/settings"
)

func sessioneDiProva(l i18n.Lingua, schermo *bytes.Buffer) *sessione {
	return &sessione{
		lingua:           l,
		cfg:              settings.Predefinite(),
		grezzo:           schermo,
		consolaRicca:     true,
		coloriConsentiti: true,
	}
}

// numeroDiVoce riconosce le righe di un elenco: tre spazi, il numero, due spazi.
var numeroDiVoce = regexp.MustCompile(`(?m)^   (\d+)  `)

// numeriMostrati estrae, nell'ordine, i numeri che un menu ha davvero stampato.
func numeriMostrati(t *testing.T, schermo string) []int {
	t.Helper()
	var numeri []int
	for _, m := range numeroDiVoce.FindAllStringSubmatch(schermo, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatal(err)
		}
		numeri = append(numeri, n)
	}
	return numeri
}

// verificaNumerazione controlla che un menu mostri 1, 2, … n e poi 0, ognuno una
// volta sola.
func verificaNumerazione(t *testing.T, nome string, numeri []int) {
	t.Helper()
	if len(numeri) < 2 {
		t.Fatalf("%s: trovate solo %d voci numerate, il menu non è stato disegnato", nome, len(numeri))
	}

	visti := map[int]int{}
	for _, n := range numeri {
		visti[n]++
	}
	for n, volte := range visti {
		if volte > 1 {
			t.Errorf("%s: il numero %d compare %d volte — l'utente non può sapere quale delle due voci sceglie",
				nome, n, volte)
		}
	}

	ultima := len(numeri) - 1
	for i := 0; i < ultima; i++ {
		if numeri[i] != i+1 {
			t.Errorf("%s: la voce in posizione %d ha il numero %d, atteso %d (elenco: %v)",
				nome, i+1, numeri[i], i+1, numeri)
		}
	}
	if numeri[ultima] != 0 {
		t.Errorf("%s: l'ultima voce ha il numero %d, deve essere 0 (elenco: %v)", nome, numeri[ultima], numeri)
	}
}

// I menu mostrano i numeri 1, 2, … n e poi 0, ognuno una volta sola.
//
// Il collaudo disegna i menu veri e legge cosa è uscito, invece di controllare
// le strutture da cui vengono disegnati. È la differenza che conta: il doppio 8
// delle impostazioni non stava nell'elenco delle voci, che era giusto, ma nella
// riga di "Indietro" scritta a mano sotto l'elenco. Un collaudo sull'elenco
// sarebbe passato.
func TestNessunMenuHaDueVociConLoStessoNumero(t *testing.T) {
	for _, l := range []i18n.Lingua{i18n.EN, i18n.IT} {
		var schermo bytes.Buffer
		s := sessioneDiProva(l, &schermo)

		s.mostraMenu(s.voci())
		verificaNumerazione(t, fmt.Sprintf("[%s] menu principale", l), numeriMostrati(t, schermo.String()))

		schermo.Reset()
		s.mostraImpostazioni(s.impostazioni())
		verificaNumerazione(t, fmt.Sprintf("[%s] impostazioni", l), numeriMostrati(t, schermo.String()))
	}
}

// Le etichette dei menu devono stare dentro la loro colonna in tutte e due le
// lingue.
//
// Le voci sono incolonnate con una larghezza fissa, e una sola etichetta più
// lunga sposta a destra la seconda colonna di quella riga soltanto, scalinando
// l'elenco. Succede quasi sempre in italiano, dove le stesse parole sono più
// lunghe.
func TestOgniEtichettaStaNellaSuaColonna(t *testing.T) {
	for _, l := range []i18n.Lingua{i18n.EN, i18n.IT} {
		var schermo bytes.Buffer
		s := sessioneDiProva(l, &schermo)

		var etichette []string
		for _, v := range s.voci() {
			etichette = append(etichette, v.titolo)
		}
		for _, imp := range s.impostazioni() {
			etichette = append(etichette, imp.etichetta)
		}

		for _, e := range etichette {
			if n := utf8.RuneCountInString(e); n > larghezzaVoce {
				t.Errorf("[%s] l'etichetta %q è lunga %d caratteri, la colonna ne tiene %d",
					l, e, n, larghezzaVoce)
			}
		}
	}
}

// I menu devono esistere per intero in entrambe le lingue: una voce dimenticata
// resterebbe in inglese in mezzo a un menu italiano.
func TestIMenuEsistonoInEntrambeLeLingue(t *testing.T) {
	var a, b bytes.Buffer
	en, it := sessioneDiProva(i18n.EN, &a), sessioneDiProva(i18n.IT, &b)

	voceEN, voceIT := en.voci(), it.voci()
	if len(voceEN) != len(voceIT) {
		t.Fatalf("il menu ha %d voci in inglese e %d in italiano", len(voceEN), len(voceIT))
	}
	for i := range voceEN {
		if voceEN[i].titolo == voceIT[i].titolo {
			t.Errorf("la voce %q non è tradotta", voceEN[i].titolo)
		}
	}

	impEN, impIT := en.impostazioni(), it.impostazioni()
	if len(impEN) != len(impIT) {
		t.Fatalf("le impostazioni sono %d in inglese e %d in italiano", len(impEN), len(impIT))
	}
	for i := range impEN {
		if impEN[i].etichetta == impIT[i].etichetta {
			t.Errorf("l'impostazione %q non è tradotta", impEN[i].etichetta)
		}
	}
}

// Ogni voce deve fare qualcosa: una voce senza azione manderebbe il programma
// in errore proprio mentre l'utente la sceglie.
func TestOgniVoceHaUnAzione(t *testing.T) {
	var schermo bytes.Buffer
	s := sessioneDiProva(i18n.EN, &schermo)

	for i, v := range s.voci() {
		if v.fai == nil {
			t.Errorf("la voce %d (%q) non ha un'azione", i+1, v.titolo)
		}
	}
	for i, imp := range s.impostazioni() {
		if imp.cambia == nil {
			t.Errorf("l'impostazione %d (%q) non ha un'azione", i+1, imp.etichetta)
		}
	}
}

// Il plurale dell'intervallo: "ogni 1 secondi" è il genere di errore che fa
// sembrare approssimativo tutto il resto.
func TestLIntervalloHaIlPluraleGiusto(t *testing.T) {
	casi := []struct {
		intervallo string
		lingua     i18n.Lingua
		vuole      string
	}{
		{"1s", i18n.IT, "ogni 1 secondo"},
		{"3s", i18n.IT, "ogni 3 secondi"},
		{"1s", i18n.EN, "every 1 second"},
		{"5s", i18n.EN, "every 5 seconds"},
	}
	for _, c := range casi {
		var schermo bytes.Buffer
		s := sessioneDiProva(c.lingua, &schermo)
		s.cfg.Interval = c.intervallo
		if got := s.ogniQuanto(); got != c.vuole {
			t.Errorf("%s in %s = %q, atteso %q", c.intervallo, c.lingua, got, c.vuole)
		}
	}
}

func TestIndiceVoce(t *testing.T) {
	casi := []struct {
		scelta string
		quante int
		vuole  int
		valida bool
	}{
		{"1", 8, 0, true},
		{"8", 8, 7, true},
		{"0", 8, 0, false},
		{"9", 8, 0, false},
		{"-1", 8, 0, false},
		{"banana", 8, 0, false},
		{"", 8, 0, false},
		{"8abc", 8, 0, false},
		{"3 4", 8, 0, false},
	}
	for _, c := range casi {
		i, ok := indiceVoce(c.scelta, c.quante)
		if ok != c.valida {
			t.Errorf("scelta %q: valida=%v, atteso %v", c.scelta, ok, c.valida)
			continue
		}
		if ok && i != c.vuole {
			t.Errorf("scelta %q = indice %d, atteso %d", c.scelta, i, c.vuole)
		}
	}
}
