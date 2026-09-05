package report

import "strings"

// bannerIcon è l'icona (disco + battito) stampata accanto al nome del
// programma all'avvio, spezzata in segmenti così il bordo e il battito
// possono avere colori diversi: bordo in blu (il disco), battito in giallo
// (il segnale), agli stessi due toni del logo PNG del progetto.
//
// Resta un ornamento: nessuna informazione del referto vive qui, e il
// programma si comporta in modo identico se questo file sparisse. Va
// stampata solo fuori dalla modalità JSON — un banner in mezzo a un output
// pensato per uno script lo romperebbe.
type bannerSegment struct {
	text  string
	pulse bool // true = colorato come il battito, false = colorato come il bordo
}

var bannerIcon = [][]bannerSegment{
	{{text: `╭─────────────╮`}},
	{{text: `│             │`}},
	{{text: `│   `}, {text: `╱╲`, pulse: true}, {text: `   `}, {text: `╱╲`, pulse: true}, {text: `   │`}},
	{{text: `│  `}, {text: `╱  ╲_╱  ╲`, pulse: true}, {text: `  │`}},
	{{text: `│             │`}},
	{{text: `╰─────────────╯`}},
}

var bannerWordmark = []string{
	`██████╗ ██╗███████╗██╗  ██╗███████╗███████╗███████╗██████╗ `,
	`██╔══██╗██║██╔════╝██║ ██╔╝██╔════╝██╔════╝██╔════╝██╔══██╗`,
	`██║  ██║██║███████╗█████╔╝ ███████╗█████╗  █████╗  ██████╔╝`,
	`██║  ██║██║╚════██║██╔═██╗ ╚════██║██╔══╝  ██╔══╝  ██╔══██╗`,
	`██████╔╝██║███████║██║  ██╗███████║███████╗███████╗██║  ██║`,
	`╚═════╝ ╚═╝╚══════╝╚═╝  ╚═╝╚══════╝╚══════╝╚══════╝╚═╝  ╚═╝`,
}

const bannerTagline = "disk diagnostics that gives you a verdict, not a spreadsheet"

// Banner restituisce l'intestazione ASCII con cui il programma si presenta.
//
// Con color a false esce identica ma senza sequenze ANSI: succede sui
// terminali che non le capiscono, e PrepareConsole se ne accorge da sola
// prima che questa funzione venga chiamata.
func Banner(color bool) string {
	var b strings.Builder
	for i, segments := range bannerIcon {
		b.WriteString("  ")
		for _, seg := range segments {
			if color {
				if seg.pulse {
					b.WriteString(bold + yellow)
				} else {
					b.WriteString(bold + blue)
				}
			}
			b.WriteString(seg.text)
			if color {
				b.WriteString(reset)
			}
		}
		b.WriteString("  ")
		if color {
			b.WriteString(bold + blue)
		}
		b.WriteString(bannerWordmark[i])
		if color {
			b.WriteString(reset)
		}
		b.WriteByte('\n')
	}
	b.WriteString("\n  ")
	if color {
		b.WriteString(dim)
	}
	b.WriteString(bannerTagline)
	if color {
		b.WriteString(reset)
	}
	b.WriteByte('\n')
	return b.String()
}
