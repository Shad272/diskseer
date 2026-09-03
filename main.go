// diskseer — diagnostica dischi che dà un verdetto, non una tabella di numeri.
//
// Copyright (C) 2026 Shad272
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version. This program is distributed in the hope that it will be
// useful, but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General
// Public License for more details. You should have received a copy of the
// licence along with this program; if not, see <https://www.gnu.org/licenses/>.
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
	"github.com/shad272/diskseer/internal/elevate"
	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/report"
	"github.com/shad272/diskseer/internal/rules"
)

const version = "1.0.0"

// main non fa altro che decidere quando uscire.
//
// Il lavoro sta in esegui(), che restituisce il codice di uscita invece di
// chiamare os.Exit da dentro: os.Exit termina il processo all'istante,
// saltando qualunque cosa venga dopo. Con la pausa finale da gestire, un
// os.Exit sparso nel mezzo del programma chiuderebbe la finestra proprio nel
// caso in cui volevamo tenerla aperta.
func main() {
	codice, delegato := esegui()

	// Se il lavoro è passato a un processo elevato, questa finestra non ha
	// più niente da mostrare: farla aspettare un INVIO lascerebbe l'utente
	// davanti a due finestre, una delle quali chiede qualcosa senza motivo.
	if !delegato && report.LanciatoDaEsploraRisorse() {
		fmt.Fprint(os.Stderr, "\n  Press ENTER to close this window... ")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}

	os.Exit(codice)
}

// esegui restituisce il codice di uscita e se il lavoro è stato affidato a un
// altro processo.
func esegui() (int, bool) {
	var (
		lang        = flag.String("lang", "en", "report language: en or it")
		asJSON      = flag.Bool("json", false, "print raw data as JSON instead of the report")
		noColor     = flag.Bool("no-color", false, "disable ANSI colours")
		showVersion = flag.Bool("version", false, "print the version and exit")
		htmlPath    = flag.String("html", "", "save the report as an HTML page at the given path")
		technician  = flag.String("technician", "", "name of whoever ran the diagnosis, printed on the report")
		contact     = flag.String("contact", "", "contact details of whoever ran the diagnosis")
		customer    = flag.String("customer", "", "customer name, printed on the report")
		noElevate   = flag.Bool("no-elevate", false, "do not request administrator privileges at startup")
		anonymous   = flag.Bool("anonymous", false, "strip make, model and timestamps from the machine data")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("diskseer", version)
		return 0, false
	}

	ansiOK := report.PrepareConsole()

	if chiediPrivilegi(*noElevate, *asJSON) {
		return 0, true
	}

	snap, err := collect.Collect()
	if err != nil {
		fmt.Fprintln(os.Stderr, "diskseer: data collection failed:", err)
		return 3, false
	}

	// L'anonimizzazione va fatta subito dopo la raccolta, prima che i dati
	// vengano usati da qualunque cosa: così nessun percorso del programma può
	// far uscire un dato identificativo per distrazione.
	if *anonymous {
		snap.Anonimizza()
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(snap); err != nil {
			fmt.Fprintln(os.Stderr, "diskseer:", err)
			return 3, false
		}
		return 0, false
	}

	l := i18n.Da(*lang)
	findings := rules.Run(snap, l)

	// Chi apre il programma con un doppio clic non ha modo di passare
	// opzioni, quindi non otterrebbe mai un file da consegnare: vedrebbe il
	// referto scorrere a schermo e finirebbe lì. In quel caso il file lo
	// salviamo da soli, accanto all'eseguibile.
	percorsoHTML := *htmlPath
	if percorsoHTML == "" && report.LanciatoDaEsploraRisorse() {
		percorsoHTML = percorsoRefertoPredefinito()
	}

	if percorsoHTML != "" {
		opts := report.HTMLOptions{Technician: *technician, Contact: *contact, Customer: *customer}
		if err := report.WriteHTMLLang(percorsoHTML, l, snap, findings, opts); err != nil {
			// Un referto non salvato non deve far perdere la diagnosi appena
			// fatta: si segnala e si continua a stamparla a schermo.
			fmt.Fprintln(os.Stderr, "diskseer: HTML report not saved:", err)
			percorsoHTML = ""
		}
	}

	report.Printer{
		W:     os.Stdout,
		Color: ansiOK && !*noColor && os.Getenv("NO_COLOR") == "",
		Lang:  l,
	}.Print(snap, findings)

	if percorsoHTML != "" {
		fmt.Printf("  %s %s\n\n", l.S("Report saved to:", "Referto salvato in:"), percorsoHTML)
	}

	// Codice di uscita utilizzabile negli script: permette di far girare
	// diskseer su più macchine e raccogliere solo quelle che hanno problemi.
	switch rules.Overall(findings) {
	case rules.SevCritical:
		return 2, false
	case rules.SevWarn:
		return 1, false
	}
	return 0, false
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
	nome := "diskseer-report-" + time.Now().Format("2006-01-02-1504") + ".html"

	exe, err := os.Executable()
	if err != nil {
		return nome
	}
	return filepath.Join(filepath.Dir(exe), nome)
}

// chiediPrivilegi rilancia il programma come amministratore quando serve, e
// dice se il lavoro è stato passato al nuovo processo.
//
// Le condizioni sono tre, e ognuna esclude un caso in cui l'elevazione
// automatica farebbe danno:
//
//   - solo se non siamo già elevati, altrimenti si rilancerebbe all'infinito;
//   - solo con doppio clic. Da un terminale il processo elevato aprirebbe una
//     finestra tutta sua, e nel terminale d'origine non comparirebbe più
//     nulla: output perso, codice di uscita perso, script rotti. Chi lavora
//     da terminale sa già aprirlo come amministratore;
//   - mai in modalità JSON, che serve agli script: una finestra di richiesta
//     privilegi in mezzo a una raccolta automatica la blocca e basta.
//
// Se l'utente rifiuta la richiesta non si insiste e non ci si ferma: il
// programma prosegue con quel che riesce a leggere e lo dichiara apertamente
// nel referto. Una diagnosi parziale vale più di nessuna diagnosi, purché sia
// dichiarata parziale.
func chiediPrivilegi(disattivato, modalitaJSON bool) bool {
	if disattivato || modalitaJSON {
		return false
	}
	if elevate.Elevato() || !report.LanciatoDaEsploraRisorse() {
		return false
	}

	eseguibile, err := os.Executable()
	if err != nil {
		return false
	}

	fmt.Println("\n  Requesting administrator privileges...")
	fmt.Println("  They are needed to read the health of SATA and USB drives.")

	return elevate.Richiedi(eseguibile, os.Args[1:])
}
