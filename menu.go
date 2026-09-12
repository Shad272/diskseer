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

	"github.com/shad272/diskseer/internal/collect"
	"github.com/shad272/diskseer/internal/elevate"
	"github.com/shad272/diskseer/internal/gui"
	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/report"
	"github.com/shad272/diskseer/internal/rules"
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
// Da qui in poi la diagnosi non è la fine del programma, è il suo inizio: si
// legge il referto e poi si decide cosa farci.
//
// Il menu compare solo quando c'è una persona davanti. Da terminale, con delle
// opzioni, diskseer resta quello di prima — stampa ed esce — perché è così che
// deve comportarsi dentro uno script.

const larghezzaVoce = 28

// sessione è ciò che il menu tiene in mano fra una scelta e l'altra.
type sessione struct {
	snap     model.Snapshot
	findings []rules.Finding
	cfg      settings.Config
	lingua   i18n.Lingua

	// I colori dipendono da tre cose diverse, e tenerle separate è ciò che
	// evita di confondere una capacità con una preferenza:
	//
	//   - ansi: il terminale sa fare colori e sequenze di controllo. Serve
	//     anche da solo, perché la vista dal vivo si ridisegna sul posto pure
	//     quando i colori sono spenti;
	//   - coloriConsentiti: --no-color o NO_COLOR sulla riga di comando, cioè
	//     "non adesso". Non è una preferenza e non va salvata: chi reindirizza
	//     l'uscita una volta non sta scegliendo un diskseer in bianco e nero
	//     per sempre;
	//   - cfg.Colors: la preferenza vera, quella che il menu cambia e salva.
	ansi             bool
	coloriConsentiti bool

	// consolaRicca e pianoDaFlag decidono, insieme all'impostazione salvata,
	// se stampare i caratteri decorativi o le loro versioni essenziali.
	consolaRicca bool
	pianoDaFlag  bool

	// grezzo e' lo schermo vero. L'uscita usata per stampare ci viene
	// costruita sopra a ogni chiamata, perche' dal menu si puo' cambiare
	// l'impostazione dei caratteri e il cambio deve vedersi subito.
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
func (s *sessione) piano() bool { return s.pianoDaFlag || s.cfg.PlainSymbols || !s.consolaRicca }

func (s *sessione) out() io.Writer { return report.Uscita(s.grezzo, s.piano()) }

func (s *sessione) stampante() report.Printer {
	return report.Printer{W: s.out(), Color: s.colore(), Lang: s.lingua}
}

// ricalcola rifà i verdetti sui dati già in memoria.
//
// Serve dopo un cambio di lingua: i verdetti contengono frasi già tradotte, e
// senza questo passaggio il menu parlerebbe italiano sopra un referto inglese.
func (s *sessione) ricalcola() { s.findings = rules.Run(s.snap, s.lingua) }

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
func (s *sessione) leggi(prompt string) (string, bool) {
	fmt.Fprint(s.out(), prompt)
	riga, err := s.in.ReadString('\n')
	if err != nil && strings.TrimSpace(riga) == "" {
		fmt.Fprintln(s.out())
		return "", false
	}
	return strings.TrimSpace(riga), true
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

// voce è una riga del menu. L'azione restituisce true per chiudere diskseer.
type voce struct {
	titolo string
	aiuto  string
	fai    func(*sessione) bool
}

func (s *sessione) voci() []voce {
	l := s.lingua

	v := []voce{{
		titolo: l.S("Live mode", "Modalità dal vivo"),
		aiuto: l.F("temperatures and free space, refreshed every %s",
			"temperature e spazio libero, aggiornati ogni %s", s.cfg.Durata()),
		fai: vaDalVivo,
	}, {
		titolo: l.S("Open the report", "Apri il referto"),
		aiuto: l.S("write an HTML report and open it in the browser",
			"scrive un referto HTML e lo apre nel browser"),
		fai: apriReferto,
	}, {
		titolo: l.S("Run the diagnosis again", "Ripeti la diagnosi"),
		aiuto:  l.S("re-read every drive from scratch", "rilegge tutti i dischi da zero"),
		fai:    rifaiLaDiagnosi,
	}, {
		titolo: l.S("Drive details", "Dettagli dei dischi"),
		aiuto:  l.S("raw SMART and NVMe counters", "contatori grezzi SMART e NVMe"),
		fai:    mostraDettagli,
	}, {
		titolo: l.S("Export the data", "Esporta i dati"),
		aiuto:  l.S("anonymised JSON, safe to share", "JSON anonimizzato, si può condividere"),
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
		aiuto:  l.S("language, refresh rate, report details", "lingua, frequenza, dati del referto"),
		fai:    apriImpostazioni,
	}, voce{
		titolo: l.S("Close diskseer", "Chiudi diskseer"),
		fai:    func(*sessione) bool { return true },
	})
}

// eseguiMenu è il ciclo principale della modalità interattiva.
func eseguiMenu(s *sessione) int {
	for {
		voci := s.voci()
		s.mostra(voci)

		scelta, ok := s.leggi("  > ")
		if !ok {
			return codiceEsito(s.findings)
		}
		switch strings.ToLower(scelta) {
		case "":
			continue
		case "q", "quit", "exit", "0":
			return codiceEsito(s.findings)
		}

		i, err := indiceVoce(scelta, len(voci))
		if err != nil {
			fmt.Fprintf(s.out(), "  %s\n", s.stampante().C(report.Dim, s.lingua.F(
				"type a number between 1 and %d", "digita un numero fra 1 e %d", len(voci))))
			continue
		}
		if voci[i].fai(s) {
			return codiceEsito(s.findings)
		}
	}
}

func indiceVoce(scelta string, quante int) (int, error) {
	var n int
	if _, err := fmt.Sscanf(scelta, "%d", &n); err != nil {
		return 0, err
	}
	if n < 1 || n > quante {
		return 0, fmt.Errorf("scelta %d fuori dall'intervallo 1-%d", n, quante)
	}
	return n - 1, nil
}

// mostra disegna lo stato e le scelte.
//
// La riga di stato in cima si ripete a ogni giro di proposito: dopo una vista
// dal vivo o una tabella di contatori il referto è scorso via, e il menu deve
// restare leggibile senza risalire il terminale.
func (s *sessione) mostra(voci []voce) {
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

	for i, v := range voci {
		fmt.Fprintf(s.out(), "   %s  %-*s %s\n", p.C(report.Bold, fmt.Sprint(i+1)),
			larghezzaVoce, v.titolo, p.C(report.Dim, v.aiuto))
	}
	fmt.Fprintln(s.out())
}

// vaDalVivo passa alla schermata che si aggiorna da sola e torna al menu quando
// l'utente la ferma.
//
// Se in questa sessione è stato aperto un referto, il ciclo riscrive anche quel
// file: la pagina nel browser si ricarica da sola e mostra gli stessi numeri
// del terminale.
func vaDalVivo(s *sessione) bool {
	if s.referto != "" {
		fmt.Fprintf(s.out(), "\n  %s\n", s.stampante().C(report.Dim, s.lingua.S(
			"the open report will keep updating too",
			"anche il referto aperto continuerà ad aggiornarsi")))
	}
	ciclaDalVivo(s.stampante(), &s.snap, s.ansi, s.cfg.Durata(), s.referto, s.opzioniHTML())
	s.ricalcola()
	return false
}

func apriReferto(s *sessione) bool {
	l := s.lingua
	percorso := s.percorsoAccanto("diskseer-report-" + time.Now().Format("2006-01-02-1504") + ".html")

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
	fmt.Fprintf(s.out(), "\n  %s\n", l.S("Reading the drives...", "Lettura dei dischi in corso..."))

	snap, err := collect.Collect()
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
	l := s.lingua

	copia := s.snap.Clona()
	copia.Anonimizza()

	raw, err := json.MarshalIndent(copia, "", "  ")
	if err != nil {
		s.avviso(err)
		return false
	}
	percorso := s.percorsoAccanto("diskseer-data-" + time.Now().Format("2006-01-02-1504") + ".json")
	if err := os.WriteFile(percorso, append(raw, '\n'), 0o600); err != nil {
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
