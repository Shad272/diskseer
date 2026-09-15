package report

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
)

func puntInt(v int) *int       { return &v }
func puntOre(v uint64) *uint64 { return &v }

func dischiDiProva() model.Snapshot {
	return model.Snapshot{
		Elevated: true,
		Disks: []model.Disk{
			// Modelli inventati: i dati di prova non descrivono il computer di
			// nessuno, nemmeno quello su cui il difetto è stato visto.
			{Model: "BOX USB DI PROVA", BusType: "USB", MediaType: "Unspecified",
				SizeBytes: 500107862016, HealthStatus: "Healthy",
				TemperatureC: puntInt(35), PowerOnHours: puntOre(307)},
			{Model: "HDD SATA DI PROVA", BusType: "SATA", MediaType: "HDD",
				SizeBytes: 500107862016, HealthStatus: "Healthy",
				TemperatureC: puntInt(28), PowerOnHours: puntOre(1962)},
			// Un modello più lungo della sua colonna: è quello che veniva
			// troncato con un carattere che la grafica semplice allarga.
			{Model: "SSD NVME DI PROVA CON UN NOME MOLTO LUNGO", BusType: "NVMe", MediaType: "SSD",
				SizeBytes: 512110190592, HealthStatus: "Healthy", IsSystemDisk: true,
				TemperatureC: puntInt(29), PowerOnHours: puntOre(1574), WearPercent: puntInt(1)},
			// Un disco senza letture, per i trattini al posto dei numeri.
			{Model: "Muto", BusType: "SATA", MediaType: "HDD",
				SizeBytes: 1000204886016, HealthStatus: "Healthy"},
		},
	}
}

// tabellaDischi disegna la sola tabella dei dischi, passando dalla stessa
// uscita che userebbe il programma.
func tabellaDischi(l i18n.Lingua, piano bool) string {
	var schermo bytes.Buffer
	p := Printer{W: Uscita(&schermo, piano), Lang: l}

	var b strings.Builder
	p.liveDischi(&b, dischiDiProva())
	fmt.Fprint(p.W, b.String())
	return schermo.String()
}

// colonna restituisce la posizione, contata in caratteri e non in byte, in cui
// compare per l'ultima volta un testo in una riga.
func colonna(riga, testo string) int {
	i := strings.LastIndex(riga, testo)
	if i < 0 {
		return -1
	}
	return utf8.RuneCountInString(riga[:i])
}

// La tabella dei dischi deve restare incolonnata in entrambe le grafiche e in
// entrambe le lingue.
//
// Il collaudo guarda l'ultima colonna, quella dello stato: se una qualunque
// delle colonne precedenti fosse più larga o più stretta in una riga, lo stato
// di quella riga non cadrebbe più sotto il suo titolo.
//
// Nasce da un difetto visto a schermo: in grafica semplice il modello troncato
// e il simbolo del grado cambiavano larghezza nella conversione, e la riga
// dell'SSD usciva spostata di due colonne rispetto alle altre.
func TestLaTabellaDeiDischiRestaIncolonnata(t *testing.T) {
	for _, piano := range []bool{false, true} {
		for _, l := range []i18n.Lingua{i18n.EN, i18n.IT} {
			nome := fmt.Sprintf("[%s, grafica semplice=%v]", l, piano)

			var intestazione string
			var dischi []string
			for _, r := range strings.Split(tabellaDischi(l, piano), "\n") {
				switch {
				case strings.Contains(r, "TEMP"):
					intestazione = r
				case strings.Contains(r, " GB"):
					dischi = append(dischi, r)
				}
			}
			if intestazione == "" {
				t.Fatalf("%s: manca l'intestazione della tabella", nome)
			}
			if len(dischi) != 4 {
				t.Fatalf("%s: trovate %d righe di dischi, attese 4", nome, len(dischi))
			}

			attesa := colonna(intestazione, l.S("HEALTH", "STATO"))
			for _, r := range dischi {
				stato := l.S("unverified", "non verificato")
				if strings.Contains(r, l.S("wear", "usura")) {
					stato = l.S("wear", "usura")
				}
				if got := colonna(r, stato); got != attesa {
					t.Errorf("%s: lo stato inizia alla colonna %d invece che alla %d, sotto il titolo:\n%s\n%s",
						nome, got, attesa, intestazione, r)
				}
			}
		}
	}
}

// Il consumo del disco si chiama usura, e il numero è quanto ne è stato
// consumato.
//
// "vita 1%" su un disco praticamente nuovo si leggeva come "gli resta l'1%":
// una diagnosi rovesciata, sulla riga che più di tutte fa decidere se
// cambiare un disco.
func TestIlConsumoSiChiamaUsuraNonVita(t *testing.T) {
	for _, l := range []i18n.Lingua{i18n.EN, i18n.IT} {
		tabella := tabellaDischi(l, false)
		if !strings.Contains(tabella, l.S("wear 1%", "usura 1%")) {
			t.Errorf("[%s] manca l'usura dell'SSD:\n%s", l, tabella)
		}
		if strings.Contains(tabella, l.S("life", "vita")) {
			t.Errorf("[%s] compare ancora la parola che rovescia il significato:\n%s", l, tabella)
		}
	}
}

// Senza intestazione, "1962" accanto a un disco non dice che sono ore di
// accensione.
func TestLaTabellaDiceCosaSonoINumeri(t *testing.T) {
	tabella := tabellaDischi(i18n.IT, false)
	for _, titolo := range []string{"TIPO", "BUS", "MODELLO", "CAPACITÀ", "TEMP", "ORE ACCESO", "STATO"} {
		if !strings.Contains(tabella, titolo) {
			t.Errorf("manca il titolo di colonna %q", titolo)
		}
	}
}
