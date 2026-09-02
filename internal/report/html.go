package report

import (
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/rules"
)

//go:embed report.html.tmpl
var htmlTemplate string

// HTMLOptions sono i dati che il programma non puo' sapere da solo: chi ha
// fatto la diagnosi e per chi. Senza questi il referto resta un file tecnico;
// con questi diventa un documento che si consegna.
type HTMLOptions struct {
	Tecnico  string
	Contatto string
	Cliente  string
}

// Il template riceve solo stringhe gia' pronte. Tutta la logica — conversioni,
// arrotondamenti, scelta delle etichette — sta qui in Go, dove si puo'
// leggere e collaudare. Un template pieno di calcoli e' codice che nessun
// test raggiunge.
type htmlView struct {
	Version   string
	Data      string
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
}

const gigabyte = 1024 * 1024 * 1024

func gb(b uint64) string { return fmt.Sprintf("%.1f GB", float64(b)/gigabyte) }

// WriteHTML genera il referto come pagina autonoma: nessun foglio di stile
// esterno, nessuna immagine remota, niente da scaricare. Deve aprirsi con un
// doppio clic sul PC di un cliente che magari non ha internet, e stamparsi
// senza sorprese.
func WriteHTML(path string, snap model.Snapshot, fs []rules.Finding, opts HTMLOptions) error {
	tmpl, err := template.New("referto").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("template non valido: %w", err)
	}

	overall := rules.Overall(fs)
	view := htmlView{
		Version:   "0.1.0",
		Data:      time.Now().Format("02/01/2006 alle 15:04"),
		Opts:      opts,
		Sys:       snap.System,
		RAM:       fmt.Sprintf("%.0f GB", float64(snap.System.RAMBytes)/gigabyte),
		Esito:     overall.String(),
		EsitoCSS:  overall.Slug(),
		Riepilogo: Summary(fs),
		Elevated:  snap.Elevated,
	}

	for _, f := range fs {
		view.Findings = append(view.Findings, htmlFinding{
			Severita:  f.Severity.String(),
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
			tipo = "sconosciuto"
		}
		vita, scritti := "—", "—"
		if d.NVMe != nil {
			vita = fmt.Sprintf("%d%%", d.NVMe.PercentageUsedPct)
			scritti = fmt.Sprintf("%.1f TB", d.NVMe.TerabyteScritti())
		}
		temp := "—"
		if d.TemperatureC != nil {
			temp = fmt.Sprintf("%d °C", *d.TemperatureC)
		}
		view.Disks = append(view.Disks, htmlDisk{
			Tipo: tipo, Modello: d.Model, Capacita: gb(d.SizeBytes),
			Bus: d.BusType, Stato: d.HealthStatus, Temp: temp,
			Vita: vita, Scritti: scritti,
			DiSistema: d.IsSystemDisk,
		})
	}

	for _, v := range snap.Volumes {
		stato := v.HealthStatus
		if v.OperationalStatus != "" && v.OperationalStatus != "OK" {
			stato = v.OperationalStatus
		}
		view.Volumes = append(view.Volumes, htmlVolume{
			Lettera: v.DriveLetter, FS: v.FileSystem,
			Capacita: gb(v.SizeBytes), Libero: gb(v.FreeBytes),
			Percento: fmt.Sprintf("%.1f%%", v.FreePercent()),
			Stato:    stato, Critico: v.FreePercent() < freeSpaceLowPct,
		})
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creazione referto: %w", err)
	}
	defer f.Close()

	return tmpl.Execute(f, view)
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
