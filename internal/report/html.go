package report

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/rules"
	"github.com/shad272/diskseer/internal/safefile"
)

//go:embed report.html.tmpl
var htmlTemplate string

var parsedHTMLTemplate = template.Must(template.New("referto").Parse(htmlTemplate))

// HTMLOptions sono i dati che il programma non puo' sapere da solo: chi ha
// fatto la diagnosi e per chi. Senza questi il referto resta un file tecnico;
// con questi diventa un documento che si consegna.
type HTMLOptions struct {
	Technician string
	Contact    string
	Customer   string

	// Version è la versione del programma, stampata in fondo al referto.
	//
	// Arriva da chi chiama invece di stare qui: scritta in due posti si
	// disallinea alla prima modifica, e il referto finirebbe per dichiarare
	// una versione diversa da quella che --version stampa. È già successo.
	Version string
}

// Il template riceve solo stringhe gia' pronte. Tutta la logica — conversioni,
// arrotondamenti, scelta delle etichette — sta qui in Go, dove si puo'
// leggere e collaudare. Un template pieno di calcoli e' codice che nessun
// test raggiunge.
type htmlView struct {
	// RicaricaOgni, se maggiore di zero, sono i secondi dopo i quali la pagina
	// si ricarica da sola. Vale solo in modalità dal vivo.
	RicaricaOgni int

	Logo      template.URL
	Lang      string
	Version   string
	Data      string
	T         etichette
	Opts      HTMLOptions
	Sys       model.System
	RAM       string
	Esito     string
	EsitoCSS  string
	Riepilogo string
	Elevated  bool
	Findings  []htmlFinding
	Disks     []htmlDisk
	Volumes   []htmlVolume
	Criticals int
	Warnings  int
	Infos     int
}

type htmlFinding struct {
	Severita  string
	CSS       string
	Area      string
	Target    string
	Titolo    string
	Dettaglio string
	Azione    string
	Dati      string
}

type htmlDisk struct {
	Tipo      string
	Modello   string
	Capacita  string
	Bus       string
	Stato     string
	StatoCSS  string
	Windows   string
	Temp      string
	Vita      string
	Scritti   string
	DiSistema bool
}

type htmlVolume struct {
	Lettera  string
	FS       string
	Capacita string
	Libero   string
	Percento string
	Stato    string
	Critico  bool
	UsedPct  string
}

const gigabyte = 1024 * 1024 * 1024

func gb(b uint64) string { return fmt.Sprintf("%.1f GB", float64(b)/gigabyte) }

// WriteHTML genera il referto come pagina autonoma: nessun foglio di stile
// esterno, nessuna immagine remota, niente da scaricare. Deve aprirsi con un
// doppio clic sul PC di un cliente che magari non ha internet, e stamparsi
// senza sorprese.
func WriteHTML(path string, snap model.Snapshot, fs []rules.Finding, opts HTMLOptions) error {
	return WriteHTMLLang(path, i18n.EN, snap, fs, opts)
}

// etichette raccoglie i testi fissi del referto HTML. Stanno qui e non nel
// template perché un template pieno di condizioni sulla lingua diventa
// illeggibile, e perché la logica in Go si può collaudare.
type etichette struct {
	Titolo, Sottotitolo, Esito, Rilevato       string
	Dischi, Volumi, Generato                   string
	ColTipo, ColModello, ColCapacita, ColBus   string
	ColStato, ColTemp, ColVita, ColScritti     string
	ColUnita, ColFS, ColLibero, ColPerc        string
	Produttore, Modello, Tipo, Processore      string
	CoreThread, Memoria, SistemaOperativo, Ver string
	Sistema, CosaFare, NotaTitolo, NotaTesto   string
	Cliente                                    string
	Panoramica, Problemi, Filtra, Tutti        string
	Stampa, NessunProblema, Tema, Analisi      string
	Critici, Avvisi, Evidenze, StatoWindows    string
}

func etichetteDi(l i18n.Lingua) etichette {
	return etichette{
		Titolo:           l.S("Diagnostic report", "Referto diagnostico"),
		Sottotitolo:      l.S("Analysis performed on", "Analisi eseguita il"),
		Cliente:          l.S("Customer", "Cliente"),
		Esito:            l.S("OVERALL RESULT", "ESITO COMPLESSIVO"),
		Rilevato:         l.S("What was found", "Cosa è stato rilevato"),
		Dischi:           l.S("Installed drives", "Dischi installati"),
		Volumi:           l.S("Volumes", "Volumi"),
		Generato:         l.S("Report generated with diskseer", "Referto generato con diskseer"),
		CosaFare:         l.S("What to do", "Cosa fare"),
		ColTipo:          l.S("Type", "Tipo"),
		ColModello:       l.S("Model", "Modello"),
		ColCapacita:      l.S("Capacity", "Capacità"),
		ColBus:           l.S("Connection", "Collegamento"),
		ColStato:         l.S("Status", "Stato"),
		StatoWindows:     l.S("Windows status", "Stato Windows"),
		Critici:          l.S("Critical", "Critici"),
		Avvisi:           l.S("Warning", "Avvisi"),
		Evidenze:         l.S("Evidence", "Dati rilevati"),
		ColTemp:          l.S("Temp.", "Temp."),
		ColVita:          l.S("Life used", "Vita usata"),
		ColScritti:       l.S("Written", "Scritti"),
		ColUnita:         l.S("Drive", "Unità"),
		ColFS:            l.S("File system", "File system"),
		ColLibero:        l.S("Free space", "Spazio libero"),
		ColPerc:          "%",
		Produttore:       l.S("Manufacturer", "Produttore"),
		Modello:          l.S("Model", "Modello"),
		Tipo:             l.S("Form factor", "Tipo"),
		Processore:       l.S("Processor", "Processore"),
		CoreThread:       l.S("Cores / threads", "Core / thread"),
		Memoria:          l.S("Memory", "Memoria"),
		SistemaOperativo: l.S("Operating system", "Sistema operativo"),
		Ver:              l.S("Version", "Versione"),
		Sistema:          l.S("SYSTEM", "SISTEMA"),
		NotaTitolo:       l.S("Partial analysis.", "Analisi parziale."),
		NotaTesto: l.S(
			"This check ran without administrator privileges. Some drive data may be unavailable; NVMe health can still be accessible. Review the findings and each drive's status before drawing conclusions.",
			"Il controllo è stato eseguito senza privilegi di amministratore. Alcuni dati potrebbero non essere disponibili; lo stato NVMe può essere comunque accessibile. Consulta le segnalazioni e lo stato di ciascun disco prima di trarre conclusioni."),
		Panoramica:     l.S("Overview", "Panoramica"),
		Problemi:       l.S("Findings", "Segnalazioni"),
		Filtra:         l.S("Filter findings", "Filtra segnalazioni"),
		Tutti:          l.S("All", "Tutte"),
		Stampa:         l.S("Print / save PDF", "Stampa / salva PDF"),
		NessunProblema: l.S("No actionable problems found", "Nessun problema che richieda intervento"),
		Tema:           l.S("Switch theme", "Cambia tema"),
		Analisi:        l.S("Disk health analysis", "Analisi salute dischi"),
	}
}

func WriteHTMLLang(path string, l i18n.Lingua, snap model.Snapshot, fs []rules.Finding, opts HTMLOptions) error {
	return scriviHTML(path, l, snap, fs, opts, 0)
}

// WriteHTMLLive scrive un referto che si aggiorna da solo.
//
// La pagina si ricarica a intervalli invece di andare a cercare i dati da
// sola: un file aperto dal disco non può interrogare nient'altro — i browser
// lo vietano — e l'alternativa sarebbe un server locale in ascolto su una
// porta, che questo programma non vuole essere.
//
// Il risultato è lo stesso e il prezzo è modesto: il file viene riscritto per
// intero a ogni giro, ma sono venti kilobyte.
func WriteHTMLLive(path string, l i18n.Lingua, snap model.Snapshot, fs []rules.Finding,
	opts HTMLOptions, ogni time.Duration) error {
	secondi := int(ogni.Seconds())
	if secondi < 1 {
		secondi = 1
	}
	return scriviHTML(path, l, snap, fs, opts, secondi)
}

func scriviHTML(path string, l i18n.Lingua, snap model.Snapshot, fs []rules.Finding,
	opts HTMLOptions, ricaricaOgni int) error {
	overall := rules.Overall(fs)
	view := htmlView{
		// template.URL dice al motore dei template che questo indirizzo è
		// nostro e non va disinnescato. Senza, Go lo sostituirebbe con
		// "#ZgotmplZ" — è la protezione che impedisce a un indirizzo arrivato
		// da fuori di iniettare codice nella pagina, e qui va disattivata di
		// proposito perché il contenuto lo produciamo noi.
		RicaricaOgni: ricaricaOgni,
		Logo:         template.URL(logoDataURI()),
		Lang:         l.S("en", "it"),
		Version:      opts.Version,
		Data:         time.Now().Format(l.S("2006-01-02 at 15:04", "02/01/2006 alle 15:04")),
		T:            etichetteDi(l),
		Opts:         opts,
		Sys:          snap.System,
		RAM:          fmt.Sprintf("%.0f GB", float64(snap.System.RAMBytes)/gigabyte),
		Esito:        overall.Label(l),
		EsitoCSS:     overall.Slug(),
		Riepilogo:    Summary(l, fs),
		Elevated:     snap.Elevated,
	}

	for _, f := range fs {
		switch f.Severity {
		case rules.SevCritical:
			view.Criticals++
		case rules.SevWarn:
			view.Warnings++
		default:
			view.Infos++
		}
		view.Findings = append(view.Findings, htmlFinding{
			Severita:  f.Severity.Label(l),
			CSS:       f.Severity.Slug(),
			Area:      f.Area,
			Target:    f.Target,
			Titolo:    f.Title,
			Dettaglio: f.Detail,
			Azione:    f.Action,
			Dati:      formatEvidence(f.Evidence),
		})
	}

	for _, d := range snap.Disks {
		tipo := d.MediaType
		if tipo == "" || tipo == "Unspecified" {
			tipo = l.S("unknown", "sconosciuto")
		}
		vita, scritti := "—", "—"
		if d.NVMe != nil {
			vita = fmt.Sprintf("%d%%", d.NVMe.PercentageUsedPct)
			scritti = fmt.Sprintf("%.1f TB", d.NVMe.TerabyteScritti())
		} else if d.WearPercent != nil {
			vita = fmt.Sprintf("%d%%", *d.WearPercent)
		}
		temp := "—"
		if d.TemperatureC != nil {
			temp = fmt.Sprintf("%d °C", *d.TemperatureC)
		}
		stato, sev := statoDisco(d, l)
		view.Disks = append(view.Disks, htmlDisk{
			Tipo: tipo, Modello: d.Model, Capacita: gb(d.SizeBytes),
			Bus: d.BusType, Stato: stato, StatoCSS: sev.Slug(), Windows: d.HealthStatus, Temp: temp,
			Vita: vita, Scritti: scritti,
			DiSistema: d.IsSystemDisk,
		})
	}

	for _, v := range snap.Volumes {
		stato := v.HealthStatus
		if v.OperationalStatus != "" && v.OperationalStatus != "OK" {
			stato = v.OperationalStatus
		}
		if v.ReadError != "" {
			stato = l.S("not refreshed", "non aggiornato")
		}
		view.Volumes = append(view.Volumes, htmlVolume{
			Lettera: v.DriveLetter, FS: v.FileSystem,
			Capacita: gb(v.SizeBytes), Libero: gb(v.FreeBytes),
			Percento: fmt.Sprintf("%.1f%%", v.FreePercent()),
			Stato:    stato, Critico: v.FreePercent() < freeSpaceLowPct,
			UsedPct: fmt.Sprintf("%.1f", 100-v.FreePercent()),
		})
	}

	var out bytes.Buffer
	if err := parsedHTMLTemplate.Execute(&out, view); err != nil {
		return fmt.Errorf("generazione referto: %w", err)
	}
	if err := safefile.Write(path, out.Bytes()); err != nil {
		return fmt.Errorf("creazione referto: %w", err)
	}
	return nil
}

const freeSpaceLowPct = 10.0

// formatEvidence mette i dati grezzi in ordine alfabetico. Senza ordinamento
// Go percorre le mappe in ordine casuale, e due referti della stessa macchina
// uscirebbero diversi: un documento che cambia da solo non e' credibile.
func formatEvidence(ev map[string]string) string {
	if len(ev) == 0 {
		return ""
	}
	keys := make([]string, 0, len(ev))
	for k := range ev {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+" = "+ev[k])
	}
	return strings.Join(parts, " · ")
}
