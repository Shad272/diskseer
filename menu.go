package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shad272/diskseer/internal/elevate"
	"github.com/shad272/diskseer/internal/gui"
	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/report"
	"github.com/shad272/diskseer/internal/rules"
	"github.com/shad272/diskseer/internal/safefile"
	"github.com/shad272/diskseer/internal/settings"
)

// Il menu interattivo.
//
// Nasce da un difetto d'uso: chi apriva diskseer con un doppio clic riceveva un
// referto e una finestra che chiedeva INVIO per chiudersi. Tutto quello che il
// programma sa fare — la vista dal vivo, il referto HTML, i contatori grezzi —
// era raggiungibile solo scrivendo opzioni su una riga di comando che chi fa
// doppio clic non aprirà mai.
//
// Il menu compare solo quando c'è una persona davanti. Da terminale, con delle
// opzioni, diskseer resta quello di prima — stampa ed esce — perché è così che
// deve comportarsi dentro uno script.
//
// Due regole valgono per tutti i menu del programma, e sono entrambe nate da un
// errore vero:
//
//   - i numeri non si scrivono a mano. Li assegna il programma scorrendo
//     l'elenco, così due voci con lo stesso numero non possono esistere. Il
//     menu delle impostazioni ne ha avute due con il numero 8: una voce
//     aggiunta in fondo all'elenco, e il numero di "Indietro" rimasto scritto a
//     mano com'era prima;
//   - uscire è sempre 0. Un numero che cambia a seconda di quante voci ci sono
//     sopra — e la voce dell'amministratore c'è o non c'è — costringe a
//     rileggere l'elenco ogni volta per trovare l'uscita.

const larghezzaVoce = 28

// sessione è ciò che il menu tiene in mano fra una scelta e l'altra.
type sessione struct {
	snap     model.Snapshot
	findings []rules.Finding
	cfg      settings.Config
	lingua   i18n.Lingua
	anonima  bool

	// I colori dipendono da tre cose diverse, e tenerle separate è ciò che
	// evita di confondere una capacità con una preferenza:
	//
	//   - ansi: il terminale sa fare colori e sequenze di controllo. Serve
	//     anche da solo, perché la vista dal vivo si ridisegna sul posto pure
	//     quando i colori sono spenti;
	//   - coloriConsentiti: --no-color o NO_COLOR sulla riga di comando, cioè
	//     "non adesso". Non è una preferenza e non va salvata;
	//   - cfg.Colors: la preferenza vera, quella che il menu cambia e salva.
	ansi             bool
	coloriConsentiti bool

	// consolaRicca è il verdetto del riconoscimento automatico; le due bandiere
	// sono le opzioni con cui l'utente lo scavalca in una direzione o
	// nell'altra, solo per questa esecuzione.
	consolaRicca bool
	pianoDaFlag  bool
	riccoDaFlag  bool

	// grezzo è lo schermo vero. L'uscita usata per stampare ci viene costruita
	// sopra a ogni chiamata, perché dal menu si può cambiare la grafica e il
	// cambio deve vedersi subito.
	grezzo io.Writer

	in *bufio.Reader

	// referto è l'ultimo file HTML scritto in questa sessione. La modalità dal
	// vivo lo tiene aggiornato: chi apre il referto e poi va dal vivo si trova
	// la pagina che si aggiorna da sola, senza doverlo chiedere.
	referto string

	erroreRaccolta error
}

func (s *sessione) colore() bool { return s.ansi && s.coloriConsentiti && s.cfg.Colors }

// piano dice se stampare con i soli caratteri essenziali.
func (s *sessione) piano() bool {
	switch {
	case s.pianoDaFlag:
		return true
	case s.riccoDaFlag:
		return false
	default:
		return s.cfg.Piani(s.consolaRicca)
	}
}

func (s *sessione) out() io.Writer { return report.Uscita(s.grezzo, s.piano()) }

func (s *sessione) stampante() report.Printer {
	return report.Printer{W: s.out(), Color: s.colore(), Lang: s.lingua}
}

// ricalcola rifà i verdetti sui dati già in memoria.
//
// Serve dopo un cambio di lingua: i verdetti contengono frasi già tradotte, e
// senza questo passaggio il menu parlerebbe italiano sopra un referto inglese.
func (s *sessione) ricalcola() {
	if s.anonima {
		s.snap.Anonimizza()
	}
	s.findings = rules.Run(s.snap, s.lingua)
}

func (s *sessione) codiceUscita() int {
	if s.erroreRaccolta != nil {
		return 3
	}
	return codiceEsito(s.findings)
}

// Una raccolta fallita non deve diventare un referto vuoto apparentemente sano,
// né far esportare i risultati precedenti come se fossero quelli attuali.
func (s *sessione) datiDisponibili() bool {
	if s.erroreRaccolta == nil {
		return true
	}
	fmt.Fprintln(s.out(), s.lingua.S(
		"Collection failed. Run the diagnosis again before using these results.",
		"Raccolta fallita. Rifai la diagnosi prima di utilizzare questi risultati."))
	return false
}

func (s *sessione) opzioniHTML() report.HTMLOptions {
	return report.HTMLOptions{
		Technician: s.cfg.Technician,
		Contact:    s.cfg.Contact,
		Customer:   s.cfg.Customer,
		Version:    version,
	}
}

// percorsoAccanto sceglie dove scrivere un file prodotto dal menu: nella
// cartella impostata dall'utente, oppure accanto all'eseguibile — che è l'unico
// posto che conosce chi ha lanciato il programma con un doppio clic.
func (s *sessione) percorsoAccanto(nome string) string {
	if s.cfg.ReportDir != "" {
		return filepath.Join(s.cfg.ReportDir, nome)
	}
	exe, err := os.Executable()
	if err != nil {
		return nome
	}
	return filepath.Join(filepath.Dir(exe), nome)
}

// leggi chiede una riga all'utente. Il secondo valore è false quando non c'è
// più niente da leggere: succede se l'ingresso è un file invece di una
// tastiera, e senza questo controllo il menu girerebbe a vuoto per sempre.
func (s *sessione) leggi(domanda string) (string, bool) {
	fmt.Fprint(s.out(), domanda)
	riga, err := s.in.ReadString('\n')
	if err != nil && strings.TrimSpace(riga) == "" {
		fmt.Fprintln(s.out())
		return "", false
	}
	return strings.TrimSpace(riga), true
}

// chiediNumero è la domanda uguale per tutti i menu.
func (s *sessione) chiediNumero() (string, bool) {
	return s.leggi("\n  " + s.lingua.S("Type a number and press ENTER: ",
		"Scrivi un numero e premi INVIO: "))
}

// conferma accetta le risposte affermative di entrambe le lingue: chi lavora in
// italiano scrive "s", e rispondergli che non ha capito sarebbe irritante.
func (s *sessione) conferma(domanda string) bool {
	risposta, ok := s.leggi("  " + domanda + " ")
	if !ok {
		return false
	}
	switch strings.ToLower(risposta) {
	case "y", "yes", "s", "si", "sì":
		return true
	}
	return false
}

func (s *sessione) avviso(err error) {
	fmt.Fprintf(os.Stderr, "  %s %v\n", s.stampante().C(report.Yellow, "!"), err)
}

// nessunaOpzione risponde a un numero che non è nell'elenco, dicendo quale
// numero è stato scritto: "scelta non valida" da sola non aiuta a capire se si
// è sbagliato tasto o si è capito male il menu.
func (s *sessione) nessunaOpzione(scritto string) {
	fmt.Fprintf(s.out(), "  %s\n", s.stampante().C(report.Yellow, s.lingua.F(
		"There is no option %q: type one of the numbers on the list.",
		"Non c'è l'opzione %q: scrivi uno dei numeri dell'elenco.", scritto)))
}

// voce è una riga del menu. L'azione restituisce true per chiudere diskseer.
type voce struct {
	titolo string
	aiuto  string
	fai    func(*sessione) bool
}

// voci elenca le scelte del menu principale, senza numeri e senza l'uscita:
// i numeri li assegna mostraVoci, l'uscita è sempre 0.
func (s *sessione) voci() []voce {
	l := s.lingua

	v := []voce{{
		titolo: l.S("Live mode", "Modalità dal vivo"),
		aiuto: l.F("temperatures and free space, updated every %d seconds",
			"temperature e spazio libero, aggiornati ogni %d secondi", int(s.cfg.Durata().Seconds())),
		fai: vaDalVivo,
	}, {
		titolo: l.S("Open the report", "Apri il referto"),
		aiuto: l.S("save an HTML page and open it in the browser",
			"salva una pagina HTML e la apre nel browser"),
		fai: apriReferto,
	}, {
		titolo: l.S("Run the diagnosis again", "Ripeti la diagnosi"),
		aiuto:  l.S("read every drive again from scratch", "rilegge tutti i dischi da capo"),
		fai:    rifaiLaDiagnosi,
	}, {
		titolo: l.S("Drive details", "Dettagli dei dischi"),
		aiuto:  l.S("raw SMART and NVMe counters, drive by drive", "contatori SMART e NVMe, disco per disco"),
		fai:    mostraDettagli,
	}, {
		titolo: l.S("Export the data", "Esporta i dati"),
		aiuto:  l.S("anonymous JSON file, safe to send to someone", "file JSON anonimo, si può mandare a qualcuno"),
		fai:    esportaDati,
	}}

	// La voce compare solo a chi ne ha bisogno. Offrirla a un utente già
	// amministratore vorrebbe dire far riavviare il programma per niente.
	if !elevate.Elevato() {
		v = append(v, voce{
			titolo: l.S("Restart as administrator", "Riavvia come amministratore"),
			aiuto:  l.S("needed to read SATA and USB drives", "serve per leggere i dischi SATA e USB"),
			fai:    riavviaComeAmministratore,
		})
	}

	return append(v, voce{
		titolo: l.S("Settings", "Impostazioni"),
		aiuto:  l.S("language, colours, graphics, report details", "lingua, colori, grafica, dati del referto"),
		fai:    apriImpostazioni,
	})
}

// eseguiMenu è il ciclo principale della modalità interattiva.
func eseguiMenu(s *sessione) int {
	for {
		voci := s.voci()
		s.mostraMenu(voci)

		scelta, ok := s.chiediNumero()
		if !ok {
			return s.codiceUscita()
		}
		if scelta == "" {
			continue
		}
		if scelta == "0" || strings.EqualFold(scelta, "q") {
			return s.codiceUscita()
		}

		i, ok := indiceVoce(scelta, len(voci))
		if !ok {
			s.nessunaOpzione(scelta)
			continue
		}
		if voci[i].fai(s) {
			return s.codiceUscita()
		}
	}
}

// indiceVoce traduce il numero scritto dall'utente nella posizione della voce.
// Accetta solo un numero intero fra 1 e quante, senza altro intorno.
func indiceVoce(scelta string, quante int) (int, bool) {
	var n int
	var resto string
	if c, _ := fmt.Sscanf(scelta, "%d%s", &n, &resto); c != 1 {
		return 0, false
	}
	if n < 1 || n > quante {
		return 0, false
	}
	return n - 1, true
}

// mostraVoci stampa un elenco numerato e, staccata sotto, la voce 0.
//
// È l'unico punto in cui si scrivono numeri di menu, e li scrive contando:
// è ciò che rende impossibile avere due voci con lo stesso numero.
//
// La seconda colonna esce così com'è: nel menu principale è una spiegazione e
// chi chiama la attenua, nelle impostazioni è il valore attuale e deve
// leggersi bene.
func (s *sessione) mostraVoci(righe [][2]string, zero string) {
	p := s.stampante()
	for i, r := range righe {
		fmt.Fprintf(s.out(), "   %s  %-*s %s\n", p.C(report.Bold, fmt.Sprint(i+1)),
			larghezzaVoce, r[0], r[1])
	}
	fmt.Fprintf(s.out(), "\n   %s  %s\n", p.C(report.Bold, "0"), zero)
}

// mostraMenu disegna lo stato e le scelte.
//
// La riga di stato in cima si ripete a ogni giro di proposito: dopo una vista
// dal vivo o una tabella di contatori il referto è scorso via, e il menu deve
// restare leggibile senza risalire il terminale.
func (s *sessione) mostraMenu(voci []voce) {
	p := s.stampante()
	l := s.lingua
	riga := strings.Repeat("─", 74)

	fmt.Fprintln(s.out())
	fmt.Fprintf(s.out(), "  %s\n", p.C(report.Dim, riga))

	if s.erroreRaccolta != nil {
		fmt.Fprintf(s.out(), "  %s  %v\n", p.C(report.Bold+report.Red,
			l.S("COLLECTION FAILED", "RACCOLTA FALLITA")), s.erroreRaccolta)
	} else {
		complessivo := rules.Overall(s.findings)
		fmt.Fprintf(s.out(), "  %s  %s\n",
			p.C(report.Bold+p.SevColor(complessivo), "● "+complessivo.Label(l)),
			report.Summary(l, s.findings))
	}

	if !s.snap.Elevated {
		fmt.Fprintf(s.out(), "  %s  %s\n", p.C(report.Bold+report.Yellow,
			l.S("PARTIAL DIAGNOSIS", "DIAGNOSI PARZIALE")),
			l.S("run without administrator privileges", "eseguita senza privilegi di amministratore"))
	}
	fmt.Fprintf(s.out(), "  %s\n\n", p.C(report.Dim, riga))

	fmt.Fprintf(s.out(), "  %s\n\n", p.C(report.Bold, l.S("WHAT DO YOU WANT TO DO?", "COSA VUOI FARE?")))

	righe := make([][2]string, len(voci))
	for i, v := range voci {
		righe[i] = [2]string{v.titolo, p.C(report.Dim, v.aiuto)}
	}
	s.mostraVoci(righe, l.S("Close diskseer", "Chiudi diskseer"))
}

// vaDalVivo passa alla schermata che si aggiorna da sola e torna al menu quando
// l'utente la ferma.
//
// Se in questa sessione è stato aperto un referto, il ciclo riscrive anche quel
// file: la pagina nel browser si ricarica da sola e mostra gli stessi numeri
// del terminale.
func vaDalVivo(s *sessione) bool {
	if !s.datiDisponibili() {
		return false
	}
	if s.referto != "" {
		fmt.Fprintf(s.out(), "\n  %s\n", s.stampante().C(report.Dim, s.lingua.S(
			"The open report will keep updating too.",
			"Anche il referto aperto continuerà ad aggiornarsi.")))
	}
	ciclaDalVivo(s.stampante(), &s.snap, s.ansi, s.cfg.Durata(), s.referto, s.opzioniHTML())
	s.ricalcola()
	return false
}

func apriReferto(s *sessione) bool {
	if !s.datiDisponibili() {
		return false
	}
	l := s.lingua
	percorso := s.percorsoAccanto(nomeReferto())

	if err := report.WriteHTMLLang(percorso, l, s.snap, s.findings, s.opzioniHTML()); err != nil {
		s.avviso(err)
		return false
	}
	s.referto = percorso
	fmt.Fprintf(s.out(), "\n  %s %s\n", l.S("Report saved to:", "Referto salvato in:"), percorso)

	if err := gui.Open(percorso); err != nil {
		// Il file c'è comunque: si dice dov'è, invece di far sembrare fallito
		// tutto il lavoro perché non si è aperto un browser.
		s.avviso(err)
		fmt.Fprintf(s.out(), "  %s\n", l.S("Open it yourself with a double click.", "Aprilo tu con un doppio clic."))
	}
	return false
}

func rifaiLaDiagnosi(s *sessione) bool {
	l := s.lingua
	fmt.Fprintln(s.out())

	// Stessa rotella dell'avvio, perché è la stessa attesa: la raccolta completa
	// dura un paio di secondi e uno schermo fermo sembra un programma bloccato.
	var attesa *report.Attesa
	if s.ansi {
		attesa = s.stampante().Attendi(l.S("reading the drives", "lettura dei dischi"))
	}
	snap, err := raccogli()
	attesa.Ferma()

	if err != nil {
		s.erroreRaccolta = err
		s.avviso(err)
		return false
	}
	s.erroreRaccolta = nil
	s.snap = snap
	s.ricalcola()
	s.stampante().Print(s.snap, s.findings)
	return false
}

func mostraDettagli(s *sessione) bool {
	if !s.datiDisponibili() {
		return false
	}
	s.stampante().PrintDetails(s.snap)
	return false
}

// esportaDati scrive i dati grezzi in un file JSON, sempre anonimizzato.
//
// L'esportazione serve a farsi dare un secondo parere, e un secondo parere si
// chiede mandando il file a qualcuno. Se dentro ci fossero marca, modello e
// orari di accensione, chi lo manda regalerebbe la scheda di una macchina che
// spesso non è nemmeno sua. I numeri su cui si ragiona restano tutti.
func esportaDati(s *sessione) bool {
	if !s.datiDisponibili() {
		return false
	}
	l := s.lingua

	copia := s.snap.Clona()
	copia.Anonimizza()

	raw, err := json.MarshalIndent(copia, "", "  ")
	if err != nil {
		s.avviso(err)
		return false
	}
	percorso := s.percorsoAccanto("diskseer-data-" + time.Now().Format("2006-01-02-150405.000000000") + ".json")
	if err := safefile.Write(percorso, append(raw, '\n')); err != nil {
		s.avviso(err)
		return false
	}

	fmt.Fprintf(s.out(), "\n  %s %s\n", l.S("Data saved to:", "Dati salvati in:"), percorso)
	fmt.Fprintf(s.out(), "  %s\n", s.stampante().C(report.Dim, l.S(
		"Make, model and timestamps were removed. Every measurement is untouched.",
		"Marca, modello e orari sono stati rimossi. Tutte le misure sono intatte.")))
	return false
}

func riavviaComeAmministratore(s *sessione) bool {
	l := s.lingua

	eseguibile, err := os.Executable()
	if err != nil {
		s.avviso(err)
		return false
	}
	if elevate.Richiedi(eseguibile, os.Args[1:]) {
		fmt.Fprintf(s.out(), "\n  %s\n", l.S("Continuing in the new window.", "Si prosegue nella nuova finestra."))
		return true
	}

	fmt.Fprintf(s.out(), "\n  %s\n", l.S("The request was refused: continuing without privileges.",
		"Richiesta rifiutata: si prosegue senza privilegi."))
	return false
}
