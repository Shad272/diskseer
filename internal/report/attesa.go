package report

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// La rotella che gira durante l'attesa.
//
// Serve a un momento preciso: la raccolta dei dati impiega un paio di secondi,
// perché avvia PowerShell per farsi dare l'inventario della macchina. In quei
// due secondi il programma ha già stampato il banner e poi non stampa più
// niente, e uno schermo fermo non si distingue da un programma bloccato.
//
// I quattro fotogrammi sono - \ | / , che nello stesso carattere disegnano
// un'asta in quattro inclinazioni: sostituendoli sul posto sembra che ruoti.
// Sono tutti nell'alfabeto di base, quindi girano anche sulle console che non
// disegnano nient'altro — e sono proprio quelle a cui serve di più un segno di
// vita.
//
// I puntini dopo la scritta crescono da uno a tre: la rotella dice "sto
// lavorando", i puntini dicono "il tempo passa". Insieme distinguono un
// programma lento da un programma fermo, che era il punto.

const (
	fotogrammiRotella = `-\|/`
	passoRotella      = 110 * time.Millisecond

	// Ogni quanti fotogrammi cambia il numero di puntini. La rotella deve
	// girare in fretta per sembrare fluida, i puntini piano per sembrare un
	// conteggio: allo stesso ritmo diventerebbero un tremolio.
	fotogrammiPerPuntino = 4
)

// Attesa è una rotella in corso. Si ferma con Ferma, che cancella anche la
// riga: quello che viene stampato dopo non deve trovarsi davanti i resti.
type Attesa struct {
	p     Printer
	testo string
	stop  chan struct{}
	fine  chan struct{}
}

// Attendi comincia a far girare la rotella e restituisce il modo di fermarla.
//
// Va chiamata solo quando l'uscita è un terminale vero: il ritorno a capo
// senza avanzamento che riscrive la riga, dentro un file, lascerebbe una scia
// di rotelle sovrapposte. Chi chiama lo sa, questa funzione no, e per questo la
// decisione resta a chi chiama.
func (p Printer) Attendi(testo string) *Attesa {
	a := &Attesa{
		p:     p,
		testo: testo,
		stop:  make(chan struct{}),
		fine:  make(chan struct{}),
	}

	go func() {
		defer close(a.fine)

		battito := time.NewTicker(passoRotella)
		defer battito.Stop()

		// Il primo fotogramma si disegna subito, senza attendere il ticchettio:
		// su una macchina veloce la raccolta potrebbe finire prima, e una
		// rotella che non è mai comparsa è peggio di nessuna rotella.
		for giro := 0; ; giro++ {
			a.disegna(giro)
			select {
			case <-a.stop:
				return
			case <-battito.C:
			}
		}
	}()

	return a
}

func (a *Attesa) disegna(giro int) {
	fotogrammi := []rune(fotogrammiRotella)
	asta := string(fotogrammi[giro%len(fotogrammi)])
	punti := strings.Repeat(".", 1+(giro/fotogrammiPerPuntino)%3)

	// I due spazi in coda cancellano i puntini del giro precedente quando il
	// conteggio torna da tre a uno: senza, resterebbero a schermo.
	fmt.Fprintf(a.p.W, "\r  %s  %s%s  ",
		a.p.c(bold, asta), a.p.c(dim, a.testo), a.p.c(dim, punti))
}

// Ferma interrompe la rotella e ripulisce la riga.
//
// Aspetta che il disegno sia davvero finito prima di cancellare: senza
// l'attesa, l'ultimo fotogramma potrebbe uscire dopo la pulizia e restare lì.
//
// Funziona anche su un'Attesa nulla, così chi non ha avviato la rotella —
// perché l'uscita non era un terminale — non deve ricordarsi di controllarlo.
func (a *Attesa) Ferma() {
	if a == nil {
		return
	}
	close(a.stop)
	<-a.fine

	larghezza := utf8.RuneCountInString(a.testo) + 12
	fmt.Fprintf(a.p.W, "\r%s\r", strings.Repeat(" ", larghezza))
}
