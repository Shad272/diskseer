//go:build windows

package collect

import (
	"testing"

	"github.com/shad272/diskseer/internal/model"
)

// scriviAttributo compone i 12 byte di un attributo così come li manda un
// disco, per poterli rileggere e verificare che lo scostamento sia giusto.
func scriviAttributo(b []byte, off int, id byte, current, worst byte, raw uint64) {
	b[off] = id
	b[off+1] = 0x33 // flag: valore qualsiasi, serve solo a occupare lo spazio
	b[off+2] = 0x00
	b[off+3] = current
	b[off+4] = worst
	for j := 0; j < 6; j++ {
		b[off+5+j] = byte(raw >> (8 * j))
	}
}

// Questo test esiste per un bug vero: la prima stesura leggeva il conteggio
// grezzo dal byte 0 dell'attributo invece che dal byte 5, infilandoci dentro
// identificativo, flag e valori normalizzati. Non produceva alcun errore —
// produceva numeri enormi e plausibili come "431174464261 settori riallocati".
//
// I valori qui sotto vengono da un disco reale reale: 1832 ore
// di funzionamento, un errore di trasmissione sul cavo, nessun settore
// riallocato.
func TestParseSMARTLeggeIlConteggioGrezzoDalByteGiusto(t *testing.T) {
	b := make([]byte, 512)
	b[0], b[1] = 0x10, 0x00 // versione della tabella

	scriviAttributo(b, 2, 9, 96, 96, 1832)   // ore di funzionamento
	scriviAttributo(b, 14, 199, 200, 200, 1) // errori sul cavo
	scriviAttributo(b, 26, 5, 100, 100, 0)   // settori riallocati
	scriviAttributo(b, 38, 193, 99, 99, 68113)

	attrs := parseSMARTAttributes(b)
	if len(attrs) != 4 {
		t.Fatalf("attributi letti %d, attesi 4", len(attrs))
	}

	casi := []struct {
		id             uint8
		raw            uint64
		current, worst uint8
	}{
		{9, 1832, 96, 96},
		{199, 1, 200, 200},
		{5, 0, 100, 100},
		{193, 68113, 99, 99},
	}

	for i, c := range casi {
		a := attrs[i]
		if a.ID != c.id {
			t.Errorf("posizione %d: identificativo %d, atteso %d", i, a.ID, c.id)
			continue
		}
		if a.Raw != c.raw {
			t.Errorf("attributo %d: conteggio %d, atteso %d — lo scostamento del "+
				"valore grezzo è sbagliato", a.ID, a.Raw, c.raw)
		}
		if a.Current != c.current || a.Worst != c.worst {
			t.Errorf("attributo %d: normalizzati %d/%d, attesi %d/%d",
				a.ID, a.Current, a.Worst, c.current, c.worst)
		}
		if a.Name == "" {
			t.Errorf("attributo %d senza nome", a.ID)
		}
	}
}

// Le posizioni non usate hanno identificativo zero e vanno saltate, non
// riportate come attributi validi con conteggio nullo.
func TestParseSMARTSaltaLePosizioniVuote(t *testing.T) {
	b := make([]byte, 512)
	scriviAttributo(b, 2, 5, 100, 100, 0)
	// tutto il resto resta a zero

	attrs := parseSMARTAttributes(b)
	if len(attrs) != 1 {
		t.Fatalf("attributi letti %d, atteso 1: le posizioni vuote non sono attributi", len(attrs))
	}
}

// La temperatura sta nel byte basso del conteggio: molti dischi usano gli
// altri byte per minimo e massimo storici, e prendere il valore intero darebbe
// temperature di milioni di gradi.
func TestTemperaturaSoloDalByteBasso(t *testing.T) {
	b := make([]byte, 512)
	// 30 gradi attuali, con 22 e 45 (minimo e massimo) nei byte successivi
	scriviAttributo(b, 2, 194, 100, 100, 30|(22<<16)|(45<<32))

	s := modelDataFrom(parseSMARTAttributes(b))
	got, ok := temperaturaDaSMART(s)
	if !ok {
		t.Fatal("temperatura non riconosciuta")
	}
	if got != 30 {
		t.Errorf("temperatura %d, attesa 30", got)
	}
}

func modelDataFrom(attrs []model.SMARTAttribute) model.SMARTData {
	return model.SMARTData{Attributes: attrs}
}
