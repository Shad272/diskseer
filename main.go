// diskseer — diagnostica dischi che dà un verdetto, non una tabella di numeri.
//
// Copyright (C) 2026 Shad272. This program is free software: you can redistribute it and/or modify it
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
	"os/signal"
	"path/filepath"
	"time"

	"github.com/shad272/diskseer/internal/collect"
	"github.com/shad272/diskseer/internal/elevate"
	"github.com/shad272/diskseer/internal/gui"
	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/report"
	"github.com/shad272/diskseer/internal/rules"
	"github.com/shad272/diskseer/internal/settings"
)

const version = "1.2.0"

// main non fa altro che decidere con quale codice uscire.
//
// Il lavoro sta in esegui(), che restituisce il codice invece di chiamare
// os.Exit da dentro: os.Exit termina il processo all'istante, e un os.Exit
// sparso nel mezzo del programma salterebbe qualunque cosa venga dopo.
func main() { os.Exit(esegui()) }

func esegui() int {
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
		showGUI     = flag.Bool("gui", false, "open the report in the default browser")
		watch       = flag.Bool("watch", false, "keep running and refresh the readings continuously")
		interval    = flag.Duration("interval", 3*time.Second, "how often to refresh in watch mode (minimum 1s)")
		showMenu    = flag.Bool("menu", false, "show the interactive menu instead of printing the report once")
		plain       = flag.Bool("ascii", false, "use plain characters only, for consoles that cannot draw the rest")
		unicode     = flag.Bool("unicode", false, "use the decorated characters even if the console was not recognised")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("diskseer", version)
		return 0
	}

	ansiOK := report.PrepareConsole()

	// Le preferenze salvate valgono come punto di partenza, le opzioni scritte
	// sulla riga di comando le scavalcano. L'ordine non è arbitrario: chi
	// scrive un'opzione la sta chiedendo adesso, per questa esecuzione, e deve
	// vincere su una scelta fatta settimane fa dentro un menu.
	cfg := settings.Carica()
	if scrittoDaRigaDiComando("lang") {
		cfg.Language = *lang
	}
	if scrittoDaRigaDiComando("interval") {
		cfg.Interval = interval.String()
	}
	if *technician != "" {
		cfg.Technician = *technician
	}
	if *contact != "" {
		cfg.Contact = *contact
	}
	if *customer != "" {
		cfg.Customer = *customer
	}
	// I colori spenti dalla riga di comando restano fuori dalle impostazioni di
	// proposito: --no-color riguarda questa esecuzione, non è una preferenza da
	// ricordare. Se finisse dentro cfg, basterebbe un --no-color seguito da una
	// qualsiasi modifica dal menu per salvare un diskseer in bianco e nero che
	// nessuno ha chiesto.
	coloriConsentiti := !*noColor && os.Getenv("NO_COLOR") == ""

	l := i18n.Da(cfg.Language)
	colore := ansiOK && coloriConsentiti && cfg.Colors

	// La scelta dei caratteri ha tre livelli, dal più immediato al più lontano:
	// l'opzione scritta adesso, l'impostazione salvata, e in mancanza di
	// entrambe il riconoscimento del terminale. Le opzioni sono due e opposte
	// perché il riconoscimento può sbagliare in due direzioni.
	consolaRicca := report.ConsolaDisegnaSimboli()
	piano := cfg.Piani(consolaRicca)
	if *plain {
		piano = true
	}
	if *unicode {
		piano = false
	}
	uscita := report.Uscita(os.Stdout, piano)

	if !*asJSON {
		fmt.Fprint(uscita, report.Banner(colore))
	}

	// Il menu compare solo quando c'è una persona davanti: con un doppio clic,
	// o quando lo si chiede. Mai in modalità JSON, che serve agli script, e mai
	// insieme a --watch, che è già una schermata interattiva per conto suo.
	modalitaMenu := (*showMenu || report.LanciatoDaEsploraRisorse()) && !*asJSON && !*watch

	if chiediPrivilegi(*noElevate, *asJSON, l) {
		return 0
	}

	// La rotella gira solo se c'è uno schermo a guardarla. Con l'uscita
	// rediretta su file i ritorni a capo lascerebbero una scia di rotelle
	// sovrapposte dentro il file, e in modalità JSON romperebbero il formato.
	var attesa *report.Attesa
	if ansiOK && !*asJSON {
		attesa = report.Printer{W: uscita, Color: colore, Lang: l}.
			Attendi(l.S("loading", "caricamento"))
	}

	snap, err := collect.Collect()
	attesa.Ferma()

	if err != nil && !modalitaMenu {
		fmt.Fprintln(os.Stderr, "diskseer: data collection failed:", err)
		return 3
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
			return 3
		}
		return 0
	}

	findings := rules.Run(snap, l)
	stampante := report.Printer{W: uscita, Color: colore, Lang: l}

	// Modalità interattiva: si mostra la diagnosi e poi si lascia decidere.
	//
	// Nessun file viene scritto da solo. Prima il referto HTML compariva
	// accanto all'eseguibile a ogni doppio clic, e la cartella si riempiva di
	// pagine che nessuno aveva chiesto; adesso lo si chiede dal menu.
	if modalitaMenu {
		if err != nil {
			fmt.Fprintln(os.Stderr, "diskseer: data collection failed:", err)
		} else {
			stampante.Print(snap, findings)
		}
		return eseguiMenu(&sessione{
			snap:             snap,
			findings:         findings,
			cfg:              cfg,
			lingua:           l,
			ansi:             ansiOK,
			coloriConsentiti: coloriConsentiti,
			consolaRicca:     consolaRicca,
			pianoDaFlag:      *plain,
			riccoDaFlag:      *unicode,

			// Il flusso grezzo, non quello già confezionato: dal menu si può
			// cambiare l'impostazione dei caratteri, e la scelta deve valere
			// subito invece che al riavvio successivo.
			grezzo:         os.Stdout,
			in:             bufio.NewReader(os.Stdin),
			erroreRaccolta: err,
		})
	}

	opts := report.HTMLOptions{
		Technician: cfg.Technician,
		Contact:    cfg.Contact,
		Customer:   cfg.Customer,
		Version:    version,
	}

	percorsoHTML := *htmlPath
	if percorsoHTML == "" && *showGUI {
		percorsoHTML = percorsoRefertoPredefinito()
	}
	if percorsoHTML != "" {
		if err := report.WriteHTMLLang(percorsoHTML, l, snap, findings, opts); err != nil {
			// Un referto non salvato non deve far perdere la diagnosi appena
			// fatta: si segnala e si continua a stamparla a schermo.
			fmt.Fprintln(os.Stderr, "diskseer: HTML report not saved:", err)
			percorsoHTML = ""
		}
	}

	// In modalità dal vivo il ciclo prende il posto di tutto il resto: stampa
	// lui, riscrive lui il referto, e finisce solo quando l'utente lo ferma.
	if *watch {
		if *showGUI && percorsoHTML != "" {
			apriNelBrowser(percorsoHTML)
		}
		return ciclaDalVivo(stampante, &snap, ansiOK, cfg.Durata(), percorsoHTML, opts)
	}

	stampante.Print(snap, findings)

	if percorsoHTML != "" {
		fmt.Fprintf(uscita, "  %s %s\n\n", l.S("Report saved to:", "Referto salvato in:"), percorsoHTML)
		if *showGUI {
			apriNelBrowser(percorsoHTML)
		}
	}

	// Codice di uscita utilizzabile negli script: permette di far girare
	// diskseer su più macchine e raccogliere solo quelle che hanno problemi.
	return codiceEsito(findings)
}

// scrittoDaRigaDiComando distingue un'opzione scritta davvero dall'utente dal
// suo valore predefinito.
//
// Senza questa distinzione non si potrebbe avere una lingua predefinita nelle
// impostazioni: il valore di --lang è "en" anche quando nessuno l'ha scritto,
// e cancellerebbe a ogni avvio la scelta salvata dal menu.
func scrittoDaRigaDiComando(nome string) bool {
	trovato := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == nome {
			trovato = true
		}
	})
	return trovato
}

func apriNelBrowser(percorso string) {
	if err := gui.Open(percorso); err != nil {
		fmt.Fprintln(os.Stderr, "diskseer: GUI not opened:", err)
	}
}

func codiceEsito(findings []rules.Finding) int {
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
// nel referto, e il menu gli offre di riprovare quando vuole. Una diagnosi
// parziale vale più di nessuna diagnosi, purché sia dichiarata parziale.
func chiediPrivilegi(disattivato, modalitaJSON bool, l i18n.Lingua) bool {
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

	fmt.Printf("\n  %s\n", l.S("Requesting administrator privileges...",
		"Richiesta dei privilegi di amministratore..."))
	fmt.Printf("  %s\n", l.S("They are needed to read the health of SATA and USB drives.",
		"Servono per leggere la salute dei dischi SATA e USB."))

	return elevate.Richiedi(eseguibile, os.Args[1:])
}

// ciclaDalVivo tiene il programma acceso e ridisegna lo stato a intervalli.
//
// L'inventario della macchina si raccoglie una volta sola: marca, modello e
// capacità dei dischi non cambiano mentre il programma è in esecuzione, e
// rifarli costerebbe tre secondi a giro contro i tre millisecondi che serve
// per rileggere temperature, contatori e spazio libero.
//
// Lo snapshot arriva per puntatore perché il ciclo lo aggiorna sul posto: chi
// esce dalla vista dal vivo — il menu, per esempio — si ritrova in mano i
// valori attuali e non quelli con cui era entrato.
//
// Se è stato chiesto anche il referto HTML, viene riscritto a ogni giro: la
// pagina si ricarica da sola e mostra gli stessi valori del terminale. È il
// motivo per cui non serve un server locale — il file su disco è già il canale
// di comunicazione fra i due.
func ciclaDalVivo(stampante report.Printer, snap *model.Snapshot, ridisegna bool,
	intervallo time.Duration, percorsoHTML string, opts report.HTMLOptions) int {

	l := stampante.Lang
	ripristina := report.PrepareLive(stampante.W, ridisegna)
	defer ripristina()

	// Ctrl+C non deve limitarsi a terminare il processo: il cursore è stato
	// nascosto, e un terminale che resta senza cursore sembra bloccato anche
	// dopo che il programma è finito.
	interruzione := make(chan os.Signal, 1)
	signal.Notify(interruzione, os.Interrupt)
	defer signal.Stop(interruzione)

	ticchettio := time.NewTicker(intervallo)
	defer ticchettio.Stop()

	if ridisegna {
		fmt.Fprint(stampante.W, "\033[2J") // una pulizia sola all'avvio, poi si ridisegna sul posto
	}

	for {
		findings := rules.Run(*snap, l)
		stampante.PrintLive(*snap, findings, report.LiveOptions{
			Intervallo: intervallo,
			Elevato:    snap.Elevated,
			Ridisegna:  ridisegna,
		})

		if percorsoHTML != "" {
			// Un referto non scritto non deve fermare il ciclo: la vista a
			// terminale continua a funzionare, ed è quella che si sta guardando.
			_ = report.WriteHTMLLive(percorsoHTML, l, *snap, findings, opts, intervallo)
		}

		select {
		case <-interruzione:
			ripristina()
			fmt.Fprintln(stampante.W)
			return codiceEsito(findings)
		case <-ticchettio.C:
			collect.Refresh(snap)
		}
	}
}
