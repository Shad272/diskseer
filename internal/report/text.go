// Package report trasforma i verdetti in qualcosa che una persona legge.
//
// È la metà del valore del programma. La diagnosi più accurata del mondo non
// serve se chi la riceve non capisce cosa deve fare.
package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/rules"
)

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	yellow = "\033[33m"
	green  = "\033[32m"
	blue   = "\033[36m"
)

const wrapWidth = 74

type Printer struct {
	W     io.Writer
	Color bool
	Lang  i18n.Lingua
}

func (p Printer) c(code, s string) string {
	if !p.Color {
		return s
	}
	return code + s + reset
}

func (p Printer) sevColor(s rules.Severity) string {
	switch s {
	case rules.SevCritical:
		return red
	case rules.SevWarn:
		return yellow
	case rules.SevInfo:
		return blue
	default:
		return green
	}
}

func (p Printer) Print(snap model.Snapshot, fs []rules.Finding) {
	p.header(snap)
	p.verdict(fs, snap.Elevated)
	p.findings(fs)
	p.inventory(snap)
}

func (p Printer) header(s model.Snapshot) {
	sys := s.System
	l := p.Lang
	fmt.Fprintln(p.W)
	fmt.Fprintf(p.W, "  %s  %s\n", p.c(bold, "diskseer"),
		p.c(dim, s.Time.Format(l.S("2006-01-02 15:04", "02/01/2006 15:04"))))
	fmt.Fprintln(p.W)
	fmt.Fprintf(p.W, "  %s %s · %s\n", sys.Manufacturer, sys.Model, sys.Chassis)
	fmt.Fprintf(p.W, "  %s · %s\n", sys.CPU,
		l.F("%d cores / %d threads · %.1f GB RAM", "%d core / %d thread · %.1f GB RAM",
			sys.Cores, sys.Threads, float64(sys.RAMBytes)/(1024*1024*1024)))
	fmt.Fprintf(p.W, "  %s (%s)\n", sys.OS, sys.OSVersion)
}

// Summary condensa l'esito in una frase. Vive qui, in un punto solo, perché la
// usano sia il referto a schermo sia quello HTML: due copie della stessa frase
// diventano due frasi diverse alla prima modifica.
func Summary(l i18n.Lingua, fs []rules.Finding) string {
	var crit, warn int
	for _, f := range fs {
		switch f.Severity {
		case rules.SevCritical:
			crit++
		case rules.SevWarn:
			warn++
		}
	}

	gravi := l.N(uint64(crit), "serious problem", "serious problems",
		"problema grave", "problemi gravi")
	occhio := l.N(uint64(warn), "thing to keep an eye on", "things to keep an eye on",
		"cosa da tenere d'occhio", "cose da tenere d'occhio")

	switch {
	case crit > 0 && warn > 0:
		return gravi + ", " + occhio
	case crit > 0:
		return gravi
	case warn > 0:
		return occhio
	default:
		return l.S("no problems found", "nessun problema rilevato")
	}
}

func (p Printer) verdict(fs []rules.Finding, elevato bool) {
	l := p.Lang
	overall := rules.Overall(fs)

	fmt.Fprintln(p.W)
	fmt.Fprintf(p.W, "  %s\n", p.c(dim, strings.Repeat("─", wrapWidth)))
	fmt.Fprintf(p.W, "  %s  %s\n",
		p.c(bold+p.sevColor(overall), l.S("RESULT: ", "ESITO: ")+overall.Label(l)),
		Summary(l, fs))

	// L'avvertenza sta dentro il riquadro dell'esito, non in fondo ai verdetti:
	// è il punto che chiunque guarda per primo, e spesso l'unico che legge
	// davvero. Un referto parziale scambiato per completo è peggio di nessun
	// referto, perché dà una sicurezza che non c'è.
	if !elevato {
		fmt.Fprintf(p.W, "  %s  %s\n",
			p.c(bold+yellow, l.S("PARTIAL DIAGNOSIS", "DIAGNOSI PARZIALE")),
			l.S("run without administrator privileges",
				"eseguita senza privilegi di amministratore"))
	}

	fmt.Fprintf(p.W, "  %s\n", p.c(dim, strings.Repeat("─", wrapWidth)))
}

func (p Printer) findings(fs []rules.Finding) {
	if len(fs) == 0 {
		return
	}
	for _, f := range fs {
		fmt.Fprintln(p.W)
		label := fmt.Sprintf("● %s", f.Severity.Label(p.Lang))
		fmt.Fprintf(p.W, "  %s  %s\n", p.c(bold+p.sevColor(f.Severity), label),
			p.c(dim, f.Area+" · "+f.Target))
		fmt.Fprintf(p.W, "  %s\n", p.c(bold, f.Title))
		for _, line := range wrap(f.Detail, wrapWidth-2) {
			fmt.Fprintf(p.W, "  %s\n", line)
		}
		for i, line := range wrap(f.Action, wrapWidth-4) {
			prefix := "  → "
			if i > 0 {
				prefix = "    "
			}
			fmt.Fprintf(p.W, "%s%s\n", prefix, p.c(green, line))
		}
	}
}

func (p Printer) inventory(s model.Snapshot) {
	l := p.Lang
	fmt.Fprintln(p.W)
	fmt.Fprintf(p.W, "  %s\n", p.c(dim, strings.Repeat("─", wrapWidth)))
	fmt.Fprintf(p.W, "  %s\n", p.c(bold, l.S("Drives", "Dischi")))
	for _, d := range s.Disks {
		mark := "  "
		if d.IsSystemDisk {
			mark = p.c(blue, "▸ ")
		}
		media := d.MediaType
		if media == "" || media == "Unspecified" {
			media = "?"
		}
		line := fmt.Sprintf("%s%-5s %-32s %7.1f GB  %-5s %s",
			mark, media, trunc(d.Model, 32),
			float64(d.SizeBytes)/(1024*1024*1024), d.BusType, d.HealthStatus)
		if d.TemperatureC != nil {
			line += fmt.Sprintf("  %d°C", *d.TemperatureC)
		}
		fmt.Fprintf(p.W, "  %s\n", line)
	}

	fmt.Fprintln(p.W)
	fmt.Fprintf(p.W, "  %s\n", p.c(bold, l.S("Volumes", "Volumi")))
	for _, v := range s.Volumes {
		fmt.Fprintf(p.W, "    %s:  %-6s %7.1f GB %s, %7.1f GB %s (%4.1f%%)  %s\n",
			v.DriveLetter, v.FileSystem,
			float64(v.SizeBytes)/(1024*1024*1024), l.S("total", "totali"),
			float64(v.FreeBytes)/(1024*1024*1024), l.S("free", "liberi"),
			v.FreePercent(), v.HealthStatus)
	}
	fmt.Fprintln(p.W)
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// wrap manda a capo sui limiti di parola. Volutamente semplice: il testo dei
// verdetti lo scriviamo noi, non arriva dall'esterno.
func wrap(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		if len(cur)+1+len(w) > width {
			lines = append(lines, cur)
			cur = w
			continue
		}
		cur += " " + w
	}
	return append(lines, cur)
}
