package report

import "strings"

// bannerLines è l'intestazione stampata una volta sola all'avvio: il disco a
// sinistra, il nome e il tracciato del battito a destra.
//
// Il disco è un cerchio vero, rasterizzato con i mezzi blocchi ▀▄█ invece che
// disegnato con i caratteri a scatola: ogni cella di testo vale due pixel in
// verticale, e quel raddoppio è ciò che permette a una circonferenza di
// sembrare tonda in un terminale, dove le celle sono alte il doppio di quanto
// sono larghe. Con ╭─╯ si ottengono solo ottagoni.
//
// Il tracciato sta sotto il nome invece che dentro il disco: alla risoluzione
// di una riga di terminale le due forme sovrapposte si mangiano a vicenda,
// mentre separate restano leggibili entrambe. Sono due battiti identici e non
// uno perché è la ripetizione a farlo leggere come un monitor cardiaco: un
// picco isolato sembra un disturbo del segnale, due uguali a distanza regolare
// sembrano un ritmo. La linea di base cade esattamente su una riga di pixel,
// così esce continua invece che seghettata, e ogni battito è una salita netta
// seguita da una discesa sotto la linea — le due cose che rendono
// riconoscibile un elettrocardiogramma quando lo spazio verticale è cinque
// righe di testo.
//
// Il segmento del tracciato è marcato pulse così esce in giallo mentre disco e
// nome restano blu.
//
// Resta un ornamento: nessuna informazione del referto vive qui, e il
// programma si comporta in modo identico se questo file sparisse. Va stampata
// solo fuori dalla modalità JSON — un banner in mezzo a un output pensato per
// uno script lo romperebbe.
type bannerSegment struct {
	text  string
	pulse bool // true = colorato come il battito, false = come il disco e il nome
}

var bannerLines = [][]bannerSegment{
	{{text: `      ▄▄████████▄▄        `}, {text: ` ___ ___ ___ _  _____ ___ ___ ___`}},
	{{text: `    ▄██████████████▄      `}, {text: `|   \_ _/ __| |/ / __| __| __| _ \`}},
	{{text: `   ▄████████████████▄     `}, {text: `| |) | |\__ \ ' <\__ \ _|| _||   /`}},
	{{text: `   ███████▀██▀███████     `}, {text: `|___/___|___/_|\_\___/___|___|_|_\`}},
	{{text: `   ██████████████████     `}, {text: `          ▄▄           ▄▄`, pulse: true}},
	{{text: `   ███████▄██▄███████     `}, {text: `          ██           ██`, pulse: true}},
	{{text: `   ▀████████████████▀     `}, {text: `          ██           ██`, pulse: true}},
	{{text: `    ▀██████████████▀      `}, {text: `▀▀▀▀▀▀▀▀▀▀▀██▀▀▀▀▀▀▀▀▀▀▀██▀▀▀▀▀▀▀▀`, pulse: true}},
	{{text: `      ▀▀████████▀▀        `}, {text: `           ▀▀           ▀▀`, pulse: true}},
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
