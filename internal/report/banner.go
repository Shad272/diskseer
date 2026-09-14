package report

import "strings"

// bannerLines riprende il marchio: riquadro blu notte, disco chiaro e una sola
// onda ECG arancione. Ogni riga del riquadro occupa esattamente 28 colonne:
// tenere qui una larghezza fissa evita che i tre colori spezzino l'allineamento.
//
// Resta un ornamento: nessuna informazione del referto vive qui, e il
// programma si comporta in modo identico se questo file sparisse. Va stampata
// solo fuori dalla modalità JSON — un banner in mezzo a un output pensato per
// uno script lo romperebbe.
type bannerSegment struct {
	text  string
	style bannerStyle
}

type bannerStyle uint8

const (
	bannerNavy bannerStyle = iota
	bannerLight
	bannerPulse
)

var bannerLines = [][]bannerSegment{
	{{text: `╭──────────────────────────╮`, style: bannerNavy}},
	{{text: `│       `, style: bannerNavy}, {text: `▄██████████▄`, style: bannerLight}, {text: `       │`, style: bannerNavy}},
	{{text: `│     `, style: bannerNavy}, {text: `▄██████████████▄`, style: bannerLight}, {text: `     │`, style: bannerNavy}},
	{{text: `│   `, style: bannerNavy}, {text: `▄██████████████████▄`, style: bannerLight}, {text: `   │`, style: bannerNavy}, {text: `   ___ ___ ___ _  _____ ___ ___ ___`, style: bannerNavy}},
	{{text: `│   `, style: bannerNavy}, {text: `████████`, style: bannerLight}, {text: `/\`, style: bannerPulse}, {text: `██████████`, style: bannerLight}, {text: `   │`, style: bannerNavy}, {text: `  |   \_ _/ __| |/ / __| __| __| _ \`, style: bannerNavy}},
	{{text: `│   `, style: bannerNavy}, {text: `██`, style: bannerLight}, {text: `─────/  \`, style: bannerPulse}, {text: `█████████`, style: bannerLight}, {text: `   │`, style: bannerNavy}, {text: `  | |) | |\__ \ ' <\__ \ _|| _||   /`, style: bannerNavy}},
	{{text: `│   `, style: bannerNavy}, {text: `██████████`, style: bannerLight}, {text: `\`, style: bannerPulse}, {text: `█████████`, style: bannerLight}, {text: `   │`, style: bannerNavy}, {text: `  |___/___|___/_|\_\___/___|___|_|_\`, style: bannerNavy}},
	{{text: `│   `, style: bannerNavy}, {text: `███████████`, style: bannerLight}, {text: `\  /────`, style: bannerPulse}, {text: `█`, style: bannerLight}, {text: `   │`, style: bannerNavy}},
	{{text: `│   `, style: bannerNavy}, {text: `████████████`, style: bannerLight}, {text: `\/`, style: bannerPulse}, {text: `██████`, style: bannerLight}, {text: `   │`, style: bannerNavy}},
	{{text: `│     `, style: bannerNavy}, {text: `▀██████████████▀`, style: bannerLight}, {text: `     │`, style: bannerNavy}},
	{{text: `│       `, style: bannerNavy}, {text: `▀██████████▀`, style: bannerLight}, {text: `       │`, style: bannerNavy}},
	{{text: `╰──────────────────────────╯`, style: bannerNavy}},
}

const bannerTagline = "disk diagnostics that gives you a verdict, not a spreadsheet"

// Banner restituisce l'intestazione ASCII con cui il programma si presenta.
//
// Con color a false esce identica ma senza sequenze ANSI: succede sui
// terminali che non le capiscono, e PrepareConsole se ne accorge da sola prima
// che questa funzione venga chiamata.
func Banner(color bool) string {
	var b strings.Builder
	for _, segments := range bannerLines {
		b.WriteString("  ")
		for _, seg := range segments {
			if color {
				switch seg.style {
				case bannerPulse:
					b.WriteString(bold + orange)
				case bannerLight:
					b.WriteString(bold)
				default:
					b.WriteString(bold + blue)
				}
			}
			b.WriteString(seg.text)
			if color {
				b.WriteString(reset)
			}
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
