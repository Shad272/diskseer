package report

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/rules"
)

// Vista dal vivo: tutto quello che cambia, in una schermata sola.
//
// È l'opposto del referto normale, e di proposito. Il referto spiega: ogni
// verdetto ha il suo perché e la sua azione, e va letto una volta. Questa
// vista invece si guarda mentre si lavora sulla macchina — si stacca un cavo,
// si copia un file, si carica la CPU — e serve a vedere subito cosa si muove.
// Perciò niente spiegazioni: numeri, e tutti insieme.
//
// Sta in una schermata perché viene ridisegnata sul posto. Se non ci stesse,
// il terminale scorrerebbe e la parte alta — l'esito, le temperature —
// sparirebbe proprio mentre la si guarda.

const (
	// Sequenze per ridisegnare senza far scorrere il terminale.
	cursoreAOrigine = "\033[H"
	pulisciDaQui    = "\033[J"
	nascondiCursore = "\033[?25l"
	mostraCursore   = "\033[?25h"

	maxVerdettiMostrati = 8
	larghezzaBarra      = 20
)

// LiveOptions raccoglie ciò che la vista dal vivo sa e il referto no.
type LiveOptions struct {
	Intervallo time.Duration
	Elevato    bool

	// Ridisegna dice se l'uscita è un terminale capace di riposizionare il
	// cursore. È una cosa diversa dall'avere i colori accesi: chi lancia con
	// --no-color in un terminale vero vuole comunque una schermata che si
	// aggiorna sul posto, non un fiume di ripetizioni che scorre.
	//
	// Quando l'uscita è rediretta su file va invece lasciata scorrere: le
	// sequenze di controllo finirebbero dentro il file come caratteri strani.
	Ridisegna bool
}

// PrepareLive prepara il terminale e restituisce la funzione che lo rimette
// com'era.
//
// Il cursore va nascosto perché altrimenti lampeggia in mezzo ai dati a ogni
// ridisegno. Va però rimesso: un terminale lasciato senza cursore resta senza
// cursore anche dopo che il programma è finito, e all'utente sembra bloccato.
func PrepareLive(w io.Writer, ridisegna bool) func() {
	if !ridisegna {
		return func() {}
	}
	fmt.Fprint(w, nascondiCursore)
	return func() { fmt.Fprint(w, mostraCursore) }
}

// PrintLive disegna una schermata intera di stato.
func (p Printer) PrintLive(snap model.Snapshot, fs []rules.Finding, opt LiveOptions) {
	l := p.Lang
	var b strings.Builder

	if opt.Ridisegna {
		b.WriteString(cursoreAOrigine)
	}

	p.liveIntestazione(&b, snap, opt)
	p.liveEsito(&b, fs, opt.Elevato)
	p.liveDischi(&b, snap)
	p.liveVolumi(&b, snap)
	p.liveVerdetti(&b, fs)

	b.WriteString("\n  ")
	b.WriteString(p.c(dim, l.S("Ctrl+C to stop", "Ctrl+C per fermare")))
	b.WriteString("\n")

	if opt.Ridisegna {
		// Cancella ciò che restava della schermata precedente: senza,
		// passando da un ciclo con sei verdetti a uno con quattro,
		// resterebbero a schermo le due righe vecchie.
		b.WriteString(pulisciDaQui)
	}
	fmt.Fprint(p.W, b.String())
}

func (p Printer) liveIntestazione(b *strings.Builder, snap model.Snapshot, opt LiveOptions) {
	l := p.Lang
	stato := l.F("live · %s · every %s", "dal vivo · %s · ogni %s",
		time.Now().Format("15:04:05"), formattaIntervallo(opt.Intervallo))

	fmt.Fprintf(b, "\n  %s  %s\n", p.c(bold, "diskseer"), p.c(dim, stato))
	fmt.Fprintf(b, "  %s\n", p.c(dim, trunc(snap.System.Manufacturer+" "+snap.System.Model+" · "+snap.System.CPU, 72)))
}

func (p Printer) liveEsito(b *strings.Builder, fs []rules.Finding, elevato bool) {
	l := p.Lang
	var crit, warn, info int
	for _, f := range fs {
		switch f.Severity {
		case rules.SevCritical:
			crit++
		case rules.SevWarn:
			warn++
		default:
			info++
		}
	}
	complessivo := rules.Overall(fs)

	conteggi := fmt.Sprintf("%s · %s · %s",
		p.c(red, fmt.Sprintf("%d %s", crit, l.S("critical", "critici"))),
		p.c(yellow, fmt.Sprintf("%d %s", warn, l.S("warning", "avvisi"))),
		p.c(blue, fmt.Sprintf("%d %s", info, "info")))

	fmt.Fprintf(b, "\n  %s  %s\n",
		p.c(bold+p.sevColor(complessivo), "● "+complessivo.Label(l)), conteggi)

	if !elevato {
		fmt.Fprintf(b, "  %s %s\n", p.c(yellow, "!"),
			p.c(dim, l.S("partial: not running as administrator",
				"parziale: non in esecuzione come amministratore")))
	}
}

// Larghezze delle colonne della tabella dei dischi.
//
// Stanno in un posto solo perché intestazione e righe devono usare le stesse:
// una colonna più larga in una delle due sposta a destra tutto quello che
// viene dopo, e i valori finiscono sotto il titolo sbagliato.
const (
	colTipo     = 4
	colBus      = 5
	colModello  = 26
	colCapacita = 10
	colTemp     = 4
	colOre      = 10
)

func (p Printer) liveDischi(b *strings.Builder, snap model.Snapshot) {
	l := p.Lang
	fmt.Fprintf(b, "\n  %s  %s\n", p.c(bold, l.S("DRIVES", "DISCHI")),
		p.c(dim, l.S("(▸ = the drive Windows starts from)", "(▸ = il disco da cui parte Windows)")))

	// L'intestazione c'è perché senza i numeri della tabella sono solo numeri:
	// "1962" accanto a un disco non dice da solo che sono ore di accensione.
	fmt.Fprintf(b, "    %s\n", p.c(dim, fmt.Sprintf("%-*s %-*s %-*s %*s  %*s  %*s  %s",
		colTipo, l.S("TYPE", "TIPO"),
		colBus, "BUS",
		colModello, l.S("MODEL", "MODELLO"),
		colCapacita, l.S("SIZE", "CAPACITÀ"),
		colTemp, "TEMP",
		colOre, l.S("HOURS ON", "ORE ACCESO"),
		l.S("HEALTH", "STATO"))))

	for _, d := range snap.Disks {
		segno := "  "
		if d.IsSystemDisk {
			segno = p.c(blue, "▸ ")
		}
		tipo := d.MediaType
		if tipo == "" || tipo == "Unspecified" {
			tipo = "?"
		}

		temp := p.c(dim, fmt.Sprintf("%*s", colTemp, "—"))
		if d.TemperatureC != nil {
			t := *d.TemperatureC
			colore := green
			switch {
			case t >= 65:
				colore = red
			case t >= 50:
				colore = yellow
			}
			// La C senza il simbolo del grado. Il grado è l'unico carattere di
			// questa tabella che la grafica semplice toglie invece di
			// sostituire: la colonna delle temperature diventerebbe più corta
			// di un carattere dell'intestazione, e tutto ciò che la segue
			// scivolerebbe a sinistra.
			temp = p.c(colore, fmt.Sprintf("%*s", colTemp, fmt.Sprintf("%dC", t)))
		}

		ore := p.c(dim, fmt.Sprintf("%*s", colOre, "—"))
		if d.PowerOnHours != nil {
			ore = fmt.Sprintf("%*d", colOre, *d.PowerOnHours)
		}

		capacita := fmt.Sprintf("%.1f GB", float64(d.SizeBytes)/(1024*1024*1024))

		fmt.Fprintf(b, "  %s%-*s %-*s %-*s %*s  %s  %s  %s\n",
			segno, colTipo, tipo, colBus, d.BusType,
			colModello, troncaColonna(d.Model, colModello),
			colCapacita, capacita, temp, ore, p.saluteDisco(d))
	}
}

// troncaColonna accorcia un testo a una larghezza fissa usando tre punti veri.
//
// Il troncamento del resto del programma usa il carattere dei puntini di
// sospensione, che occupa una colonna. In grafica semplice diventa tre punti,
// cioè due colonne in più: nella tabella il modello troncato spingeva a destra
// capacità, temperatura e ore di quella riga soltanto. Qui si usano i tre
// punti fin dall'inizio, così la larghezza è la stessa in entrambe le grafiche.
func troncaColonna(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-3]) + "..."
}

// saluteDisco riassume in poche parole lo stato del disco: quanto è consumato,
// se il disco lo dichiara, altrimenti il giudizio di Windows.
//
// Il consumo si chiama "usura", non "vita". È lo stesso numero che lo
// standard NVMe chiama "percentuale usata", e parte da zero su un disco nuovo:
// scritto "vita 1%" si leggeva come "gli resta l'1%", cioè il contrario, su un
// disco praticamente nuovo.
func (p Printer) saluteDisco(d model.Disk) string {
	l := p.Lang
	if d.NVMe != nil && d.NVMe.CriticalWarning != 0 {
		return p.c(red, l.S("FAULT", "GUASTO"))
	}
	if d.WearPercent != nil {
		v := *d.WearPercent
		colore := green
		switch {
		case v >= 90:
			colore = red
		case v >= 80:
			colore = yellow
		}
		return p.c(colore, l.F("wear %d%%", "usura %d%%", v))
	}
	if d.HealthStatus != "" && d.HealthStatus != "Healthy" {
		return p.c(yellow, d.HealthStatus)
	}
	return p.c(green, "ok")
}

func (p Printer) liveVolumi(b *strings.Builder, snap model.Snapshot) {
	l := p.Lang
	fmt.Fprintf(b, "\n  %s\n", p.c(bold, l.S("VOLUMES", "VOLUMI")))

	for _, v := range snap.Volumes {
		libero := v.FreePercent()
		colore := green
		switch {
		case libero < 5:
			colore = red
		case libero < 10:
			colore = yellow
		}

		nota := ""
		if v.OperationalStatus != "" && v.OperationalStatus != "OK" {
			nota = "  " + p.c(yellow, v.OperationalStatus)
		}

		fmt.Fprintf(b, "    %s:  %-6s %s %s %s%s\n",
			v.DriveLetter, v.FileSystem,
			p.barra(100-libero, colore),
			p.c(colore, fmt.Sprintf("%5.1f%% %s", libero, l.S("free", "liberi"))),
			p.c(dim, fmt.Sprintf("%8.1f GB", float64(v.FreeBytes)/(1024*1024*1024))),
			nota)
	}
}

// barra disegna quanto è occupato un volume.
func (p Printer) barra(usatoPct float64, colore string) string {
	if usatoPct < 0 {
		usatoPct = 0
	}
	if usatoPct > 100 {
		usatoPct = 100
	}
	pieni := int(usatoPct/100*larghezzaBarra + 0.5)
	return "[" + p.c(colore, strings.Repeat("█", pieni)) +
		p.c(dim, strings.Repeat("░", larghezzaBarra-pieni)) + "]"
}

func (p Printer) liveVerdetti(b *strings.Builder, fs []rules.Finding) {
	l := p.Lang
	fmt.Fprintf(b, "\n  %s\n", p.c(bold, l.S("FINDINGS", "SEGNALAZIONI")))

	if len(fs) == 0 {
		fmt.Fprintf(b, "    %s\n", p.c(green, l.S("nothing to report", "niente da segnalare")))
		return
	}

	for i, f := range fs {
		if i >= maxVerdettiMostrati {
			fmt.Fprintf(b, "    %s\n", p.c(dim,
				l.F("and %d more — run without --watch for the full report",
					"e altri %d — esegui senza --watch per il referto completo",
					len(fs)-maxVerdettiMostrati)))
			return
		}
		fmt.Fprintf(b, "  %s %-22s %s\n",
			p.c(p.sevColor(f.Severity), "●"),
			p.c(dim, trunc(f.Area+" · "+f.Target, 22)),
			trunc(f.Title, 46))
	}
}

func formattaIntervallo(d time.Duration) string {
	if d%time.Second == 0 {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return d.String()
}
