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
	"github.com/shad272/diskseer/internal/elevate"
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
	codice, delegato := esegui()

	// Se il lavoro è passato a un processo elevato, questa finestra non ha
	// più niente da mostrare: farla aspettare un INVIO lascerebbe l'utente
	// davanti a due finestre, una delle quali chiede qualcosa senza motivo.
	if !delegato && report.LanciatoDaEsploraRisorse() {
		fmt.Fprint(os.Stderr, "\n  Premi INVIO per chiudere questa finestra... ")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}

	os.Exit(codice)
}

// esegui restituisce il codice di uscita e se il lavoro è stato affidato a un
// altro processo.
func esegui() (int, bool) {
	var (
		asJSON      = flag.Bool("json", false, "stampa i dati grezzi in JSON invece del referto")
		noColor     = flag.Bool("no-color", false, "disattiva i colori ANSI")
		showVersion = flag.Bool("version", false, "stampa la versione ed esce")
		htmlPath    = flag.String("html", "", "salva il referto come pagina HTML nel percorso indicato")
		tecnico     = flag.String("tecnico", "", "nome di chi esegue la diagnosi, stampato sul referto")
		contatto    = flag.String("contatto", "", "recapito di chi esegue la diagnosi")
		cliente     = flag.String("cliente", "", "nome del cliente, stampato sul referto")
		noElevate   = flag.Bool("no-elevate", false, "non chiedere i privilegi di amministratore all'avvio")
		anonimo     = flag.Bool("anonimo", false, "rimuove marca, modello e orari della macchina dai dati")
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
		fmt.Fprintln(os.Stderr, "diskseer: raccolta dati fallita:", err)
		return 3, false
	}

	// L'anonimizzazione va fatta subito dopo la raccolta, prima che i dati
	// vengano usati da qualunque cosa: così nessun percorso del programma può
	// far uscire un dato identificativo per distrazione.
	if *anonimo {
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
	nome := "referto-" + time.Now().Format("2006-01-02-1504") + ".html"

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

	fmt.Println("\n  Richiesta dei privilegi di amministratore in corso...")
	fmt.Println("  Servono per leggere lo stato di salute dei dischi SATA e USB.")

	return elevate.Richiedi(eseguibile, os.Args[1:])
}
