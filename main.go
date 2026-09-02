// diskseer — diagnostica dischi che dà un verdetto, non una tabella di numeri.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shad272/diskseer/internal/collect"
	"github.com/shad272/diskseer/internal/report"
	"github.com/shad272/diskseer/internal/rules"
)

const version = "0.1.0"

// main non fa altro che decidere quando uscire.
//
// Il lavoro sta in esegui(), che restituisce il codice di uscita invece di
// chiamare os.Exit da dentro: os.Exit termina il processo all'istante,
// saltando qualunque cosa venga dopo. Con la pausa finale da gestire, un
// os.Exit sparso nel mezzo del programma chiuderebbe la finestra proprio nel
// caso in cui volevamo tenerla aperta.
func main() {
	codice := esegui()

	if report.LanciatoDaEsploraRisorse() {
		fmt.Fprint(os.Stderr, "\n  Premi INVIO per chiudere questa finestra... ")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}

	os.Exit(codice)
}

func esegui() int {
	var (
		asJSON      = flag.Bool("json", false, "stampa i dati grezzi in JSON invece del referto")
		noColor     = flag.Bool("no-color", false, "disattiva i colori ANSI")
		showVersion = flag.Bool("version", false, "stampa la versione ed esce")
		htmlPath    = flag.String("html", "", "salva il referto come pagina HTML nel percorso indicato")
		tecnico     = flag.String("tecnico", "", "nome di chi esegue la diagnosi, stampato sul referto")
		contatto    = flag.String("contatto", "", "recapito di chi esegue la diagnosi")
		cliente     = flag.String("cliente", "", "nome del cliente, stampato sul referto")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("diskseer", version)
		return 0
	}

	ansiOK := report.PrepareConsole()

	snap, err := collect.Collect()
	if err != nil {
		fmt.Fprintln(os.Stderr, "diskseer: raccolta dati fallita:", err)
		return 3
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(snap); err != nil {
			fmt.Fprintln(os.Stderr, "diskseer:", err)
			return 3
		}
		return 0
	}

	findings := rules.Run(snap)

	// Chi apre il programma con un doppio clic non ha modo di passare
	// opzioni, quindi non otterrebbe mai un file da consegnare: vedrebbe il
	// referto scorrere a schermo e finirebbe lì. In quel caso il file lo
	// salviamo da soli, accanto all'eseguibile.
	percorsoHTML := *htmlPath
	if percorsoHTML == "" && report.LanciatoDaEsploraRisorse() {
		percorsoHTML = percorsoRefertoPredefinito()
	}

	if percorsoHTML != "" {
		opts := report.HTMLOptions{Tecnico: *tecnico, Contatto: *contatto, Cliente: *cliente}
		if err := report.WriteHTML(percorsoHTML, snap, findings, opts); err != nil {
			// Un referto non salvato non deve far perdere la diagnosi appena
			// fatta: si segnala e si continua a stamparla a schermo.
			fmt.Fprintln(os.Stderr, "diskseer: referto HTML non salvato:", err)
			percorsoHTML = ""
		}
	}

	report.Printer{
		W:     os.Stdout,
		Color: ansiOK && !*noColor && os.Getenv("NO_COLOR") == "",
	}.Print(snap, findings)

	if percorsoHTML != "" {
		fmt.Printf("  Referto salvato in: %s\n\n", percorsoHTML)
	}

	// Codice di uscita utilizzabile negli script: permette di far girare
	// diskseer su più macchine e raccogliere solo quelle che hanno problemi.
	switch rules.Overall(findings) {
	case rules.SevCritical:
		return 2
	case rules.SevWarn:
		return 1
	}
	return 0
}

// percorsoRefertoPredefinito sceglie dove salvare il referto quando nessuno
// l'ha indicato.
//
// Accanto all'eseguibile, non nella cartella di lavoro: chi avvia il programma
// con un doppio clic non sa nemmeno quale sia la cartella di lavoro, mentre
// sa benissimo dov'è il file che ha appena cliccato. Se il percorso
// dell'eseguibile non è ricavabile — caso raro ma possibile — si ripiega sulla
// cartella corrente invece di rinunciare al referto.
//
// Il nome porta data e ora perché su una macchina si fanno più controlli, e
// un referto che sovrascrive il precedente cancella proprio il confronto che
// serve a capire se un disco sta peggiorando.
func percorsoRefertoPredefinito() string {
	nome := "referto-" + time.Now().Format("2006-01-02-1504") + ".html"

	exe, err := os.Executable()
	if err != nil {
		return nome
	}
	return filepath.Join(filepath.Dir(exe), nome)
}
