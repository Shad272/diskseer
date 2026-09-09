package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shad272/diskseer/internal/model"
)

// Vista tecnica: i contatori grezzi, disco per disco.
//
// Il referto normale nasconde questi numeri di proposito — chi porta il
// computer in assistenza non deve leggere una tabella SMART per sapere se il
// disco va sostituito. Ma il numero grezzo serve a due persone: a chi vuole
// controllare su cosa si basa il verdetto, e a chi deve confrontare la stessa
// macchina a distanza di mesi.
//
// Sono gli stessi dati che il programma ha già letto per decidere: qui non si
// interroga niente di nuovo, si mostra ciò che è già in memoria.

const larghezzaEtichetta = 24

func (p Printer) PrintDetails(s model.Snapshot) {
	l := p.Lang

	fmt.Fprintln(p.W)
	fmt.Fprintf(p.W, "  %s\n", p.c(bold, l.S("DRIVE DETAILS", "DETTAGLI DEI DISCHI")))

	if len(s.Disks) == 0 {
		fmt.Fprintf(p.W, "  %s\n\n", p.c(dim, l.S("no drives detected", "nessun disco rilevato")))
		return
	}

	for _, d := range s.Disks {
		p.intestazioneDisco(d)
		switch {
		case d.NVMe != nil:
			p.dettagliNVMe(*d.NVMe)
		case d.SMART != nil:
			p.dettagliSMART(*d.SMART)
		default:
			p.spiegaPercheMancano(d, s.Elevated)
		}
	}
	fmt.Fprintln(p.W)
}

func (p Printer) intestazioneDisco(d model.Disk) {
	l := p.Lang
	media := d.MediaType
	if media == "" || media == "Unspecified" {
		media = "?"
	}

	segno := "  "
	if d.IsSystemDisk {
		segno = p.c(blue, "▸ ")
	}

	fmt.Fprintln(p.W)
	fmt.Fprintf(p.W, "  %s%s\n", segno, p.c(bold, d.Model))
	fmt.Fprintf(p.W, "    %s\n", p.c(dim, fmt.Sprintf("%s · %s · %.1f GB · %s %s · %s",
		media, d.BusType, float64(d.SizeBytes)/(1024*1024*1024),
		l.S("device", "dispositivo"), d.DeviceID, d.HealthStatus)))
	fmt.Fprintln(p.W)
}

// riga stampa un'etichetta e il suo valore incolonnati.
func (p Printer) riga(etichetta, valore string) {
	fmt.Fprintf(p.W, "      %-*s %s\n", larghezzaEtichetta, etichetta, valore)
}

func (p Printer) dettagliNVMe(h model.NVMeHealth) {
	l := p.Lang
	fmt.Fprintf(p.W, "    %s\n", p.c(dim, l.S("NVMe health log (page 0x02)",
		"registro di salute NVMe (pagina 0x02)")))

	p.riga(l.S("Critical warning", "Allarme critico"), p.allarmiNVMe(h))
	p.riga(l.S("Temperature", "Temperatura"), fmt.Sprintf("%d °C", h.CompositeTempC))
	p.riga(l.S("Available spare", "Settori di scorta"),
		fmt.Sprintf("%d %% (%s %d %%)", h.AvailableSparePct,
			l.S("threshold", "soglia"), h.AvailableSpareThreshPct))
	p.riga(l.S("Life used", "Vita consumata"), fmt.Sprintf("%d %%", h.PercentageUsedPct))
	p.riga(l.S("Power-on hours", "Ore di accensione"), fmt.Sprintf("%d", h.PowerOnHours))
	p.riga(l.S("Power cycles", "Accensioni"), fmt.Sprintf("%d", h.PowerCycles))
	p.riga(l.S("Unsafe shutdowns", "Spegnimenti bruschi"), fmt.Sprintf("%d", h.UnsafeShutdowns))

	// Gli errori di integrità sono l'unico contatore di questo elenco che non
	// dovrebbe mai muoversi: se non è zero si evidenzia, perché è il dato che
	// dice "questo disco ha perso qualcosa".
	errori := fmt.Sprintf("%d", h.MediaErrors)
	if h.MediaErrors > 0 {
		errori = p.c(red, errori)
	}
	p.riga(l.S("Media errors", "Errori di integrità"), errori)
	p.riga(l.S("Error log entries", "Voci nel registro errori"), fmt.Sprintf("%d", h.ErrorLogEntries))

	p.riga(l.S("Data written", "Dati scritti"), fmt.Sprintf("%.2f TB", h.TerabyteScritti()))
	p.riga(l.S("Data read", "Dati letti"),
		fmt.Sprintf("%.2f TB", float64(h.DataUnitsRead)*512*1000/1e12))

	p.riga(l.S("Above warning temp", "Sopra la soglia di avviso"),
		p.minuti(uint64(h.WarningTempTimeMin), yellow))
	p.riga(l.S("Above critical temp", "Sopra la soglia critica"),
		p.minuti(uint64(h.CriticalTempTimeMin), red))
}

// minuti evidenzia il tempo passato in sovratemperatura: zero è normale,
// qualunque altro valore è la prova che il disco ha già rallentato per scaldarsi.
func (p Printer) minuti(m uint64, colore string) string {
	s := fmt.Sprintf("%d min", m)
	if m > 0 {
		return p.c(colore, s)
	}
	return s
}

func (p Printer) allarmiNVMe(h model.NVMeHealth) string {
	l := p.Lang
	var attivi []string
	if h.SpareBelowThreshold() {
		attivi = append(attivi, l.S("spare below threshold", "scorta sotto soglia"))
	}
	if h.TemperatureAlarm() {
		attivi = append(attivi, l.S("temperature", "temperatura"))
	}
	if h.ReliabilityDegraded() {
		attivi = append(attivi, l.S("reliability degraded", "affidabilità degradata"))
	}
	if h.ReadOnly() {
		attivi = append(attivi, l.S("read-only", "sola lettura"))
	}
	if h.BackupFailed() {
		attivi = append(attivi, l.S("backup failed", "salvataggio interno fallito"))
	}
	if len(attivi) == 0 {
		return p.c(green, l.S("none", "nessuno"))
	}
	return p.c(red, strings.Join(attivi, ", "))
}

// gliAttributiCheContano sono quelli il cui conteggio grezzo dovrebbe restare a
// zero per tutta la vita del disco. Non è un giudizio — quello lo danno le
// regole — ma dice a chi legge dove guardare per primo.
var gliAttributiCheContano = map[uint8]bool{
	model.SMARTReallocatedSectors: true,
	model.SMARTPendingSectors:     true,
	model.SMARTOfflineUncorrect:   true,
	model.SMARTReportedUncorrect:  true,
	model.SMARTSpinRetry:          true,
}

func (p Printer) dettagliSMART(s model.SMARTData) {
	l := p.Lang
	fmt.Fprintf(p.W, "    %s\n", p.c(dim, l.S("SMART attributes", "attributi SMART")))
	fmt.Fprintf(p.W, "      %s\n", p.c(dim, fmt.Sprintf("%-4s %-30s %5s %5s %14s",
		"ID", l.S("Attribute", "Attributo"), l.S("Cur", "Att"),
		l.S("Wst", "Peg"), l.S("Raw count", "Conteggio"))))

	// Ordinati per identificativo: la stessa macchina riletta fra sei mesi deve
	// produrre le righe nello stesso ordine, altrimenti confrontare due letture
	// diventa un lavoro manuale.
	attributi := append([]model.SMARTAttribute(nil), s.Attributes...)
	sort.Slice(attributi, func(i, j int) bool { return attributi[i].ID < attributi[j].ID })

	for _, a := range attributi {
		grezzo := fmt.Sprintf("%14d", a.Raw)
		if gliAttributiCheContano[a.ID] && a.Raw > 0 {
			grezzo = p.c(red, grezzo)
		}
		fmt.Fprintf(p.W, "      %-4d %-30s %5d %5d %s\n",
			a.ID, trunc(a.Name, 30), a.Current, a.Worst, grezzo)
	}
}

// spiegaPercheMancano dice cosa fare quando un disco non ha restituito niente.
//
// "Nessun dato" da solo sembra un difetto del programma. Quasi sempre invece è
// una delle due cause qui sotto, ed entrambe hanno un rimedio.
func (p Printer) spiegaPercheMancano(d model.Disk, elevato bool) {
	l := p.Lang
	if !elevato {
		fmt.Fprintf(p.W, "    %s\n", p.c(yellow, l.S(
			"no raw counters: reading them requires administrator privileges",
			"nessun contatore grezzo: leggerli richiede i privilegi di amministratore")))
		return
	}
	if d.BusType == "USB" {
		fmt.Fprintf(p.W, "    %s\n", p.c(dim, l.S(
			"no raw counters: this USB enclosure does not forward the drive's own commands",
			"nessun contatore grezzo: questo box USB non inoltra i comandi propri del disco")))
		return
	}
	fmt.Fprintf(p.W, "    %s\n", p.c(dim, l.S(
		"no raw counters: this drive did not answer the health query",
		"nessun contatore grezzo: questo disco non ha risposto alla richiesta di salute")))
}
