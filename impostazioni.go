package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/report"
	"github.com/shad272/diskseer/internal/settings"
)

// Le impostazioni, viste da chi le cambia.
//
// Ogni modifica viene salvata subito, tranne la lingua: quella si applica
// all'istante ma chiede se deve valere anche per i prossimi avvii. È voluto —
// la lingua è l'unica impostazione che si cambia spesso per un momento solo,
// per mostrare un referto a qualcuno che non parla la tua.

func apriImpostazioni(s *sessione) bool {
	for {
		s.mostraImpostazioni()

		scelta, ok := s.leggi("  > ")
		if !ok {
			return true // ingresso chiuso: si chiude tutto, non solo questo menu
		}
		switch strings.ToLower(scelta) {
		case "", "0", "9", "q", "b":
			return false
		case "1":
			s.cambiaLingua()
		case "2":
			s.cambiaIntervallo()
		case "3":
			s.cambiaTesto(&s.cfg.Technician, s.lingua.S("Technician", "Tecnico"))
		case "4":
			s.cambiaTesto(&s.cfg.Contact, s.lingua.S("Contact", "Contatto"))
		case "5":
			s.cambiaTesto(&s.cfg.Customer, s.lingua.S("Customer", "Cliente"))
		case "6":
			s.cfg.Colors = !s.cfg.Colors
			s.salvaImpostazioni()
		case "7":
			s.cambiaCartella()
		case "8":
			s.cfg.Symbols = settings.ProssimiSimboli(s.cfg.Symbols)
			s.salvaImpostazioni()
		default:
			fmt.Fprintf(s.out(), "  %s\n", s.stampante().C(report.Dim,
				s.lingua.S("type a number between 1 and 9", "digita un numero fra 1 e 9")))
		}
	}
}

type rigaImpostazione struct{ etichetta, valore string }

// righeImpostazioni descrive cosa mostrare, senza stamparlo.
//
// Sta separata dalla stampa per poterla collaudare: le etichette devono stare
// dentro la loro colonna in tutte e due le lingue, e in italiano sono
// sistematicamente più lunghe.
func (s *sessione) righeImpostazioni() []rigaImpostazione {
	p := s.stampante()
	l := s.lingua

	nonImpostato := p.C(report.Dim, l.S("(not set)", "(non impostato)"))
	acceso := l.S("on", "accesi")
	if !s.cfg.Colors {
		acceso = l.S("off", "spenti")
	}
	cartella := l.S("next to diskseer.exe", "accanto a diskseer.exe")
	if s.cfg.ReportDir != "" {
		cartella = s.cfg.ReportDir
	}

	return []rigaImpostazione{
		{l.S("Language", "Lingua"), l.S("English", "Italiano")},
		{l.S("Refresh interval", "Intervallo di aggiornamento"), s.cfg.Durata().String()},
		{l.S("Technician", "Tecnico"), oppure(s.cfg.Technician, nonImpostato)},
		{l.S("Contact", "Contatto"), oppure(s.cfg.Contact, nonImpostato)},
		{l.S("Customer", "Cliente"), oppure(s.cfg.Customer, nonImpostato)},
		{l.S("Colours", "Colori"), acceso},
		{l.S("Report folder", "Cartella dei referti"), cartella},
		{l.S("Symbols", "Simboli"), s.descriviSimboli()},
	}
}

func (s *sessione) mostraImpostazioni() {
	p := s.stampante()
	l := s.lingua

	fmt.Fprintln(s.out())
	fmt.Fprintf(s.out(), "  %s\n\n", p.C(report.Bold, l.S("SETTINGS", "IMPOSTAZIONI")))

	for i, r := range s.righeImpostazioni() {
		fmt.Fprintf(s.out(), "   %s  %-*s %s\n", p.C(report.Bold, fmt.Sprint(i+1)), larghezzaVoce, r.etichetta, r.valore)
	}
	fmt.Fprintf(s.out(), "   %s  %s\n", p.C(report.Bold, "8"), l.S("Back", "Indietro"))

	fmt.Fprintf(s.out(), "\n  %s\n", p.C(report.Dim, s.doveSonoSalvate()))
	fmt.Fprintln(s.out())
}

// doveSonoSalvate dice all'utente quale file stiamo scrivendo. Un programma che
// ricorda delle preferenze senza dire dove le mette è un programma che lascia
// tracce sul computer di qualcun altro senza dichiararlo.
func (s *sessione) doveSonoSalvate() string {
	l := s.lingua
	percorso, err := settings.Percorso()
	if err != nil {
		return l.S("Settings cannot be saved on this system.",
			"Su questo sistema le impostazioni non si possono salvare.")
	}
	return l.F("Settings file: %s", "File delle impostazioni: %s", percorso)
}

// descriviSimboli dice quali caratteri si stanno usando e, quando sono quelli
// essenziali senza che l'utente li abbia chiesti, perché.
//
// Serve a non far sembrare un difetto una scelta: chi vede un referto con gli
// asterischi al posto dei pallini deve poter capire in un colpo d'occhio che è
// la sua console a non saperli disegnare, e che si può forzare il contrario.
func (s *sessione) descriviSimboli() string {
	l := s.lingua
	switch s.cfg.Symbols {
	case settings.SimboliPiani:
		return l.S("plain text (chosen)", "solo testo (scelto)")
	case settings.SimboliRicchi:
		return l.S("full (chosen)", "completi (scelto)")
	}
	if s.consolaRicca {
		return l.S("full (automatic)", "completi (automatico)")
	}
	return l.S("plain text - this console was not recognised",
		"solo testo - questa console non e' stata riconosciuta")
}

func oppure(valore, seVuoto string) string {
	if strings.TrimSpace(valore) == "" {
		return seVuoto
	}
	return valore
}

// cambiaLingua applica subito la scelta e poi chiede se renderla permanente.
//
// La domanda arriva già nella lingua appena scelta: è la conferma immediata che
// il cambio ha avuto effetto, prima ancora che l'utente risponda.
func (s *sessione) cambiaLingua() {
	fmt.Fprintln(s.out())
	fmt.Fprintf(s.out(), "   1  English\n   2  Italiano\n\n")

	scelta, ok := s.leggi("  > ")
	if !ok {
		return
	}
	switch scelta {
	case "1":
		s.lingua = i18n.EN
	case "2":
		s.lingua = i18n.IT
	default:
		return
	}

	// I verdetti contengono frasi già tradotte: senza rifarli, il menu
	// parlerebbe una lingua e il referto un'altra.
	s.ricalcola()

	if s.conferma(s.lingua.S("Make English the default language? [y/N]",
		"Vuoi che l'italiano diventi la lingua predefinita? [s/N]")) {
		s.cfg.Language = s.lingua.String()
		s.salvaImpostazioni()
		return
	}
	fmt.Fprintf(s.out(), "  %s\n", s.stampante().C(report.Dim, s.lingua.S(
		"Kept for this session only.", "Vale solo per questa sessione.")))
}

func (s *sessione) cambiaIntervallo() {
	l := s.lingua

	risposta, ok := s.leggi("\n  " + l.F("Refresh every how many seconds? [%d] ",
		"Ogni quanti secondi aggiornare? [%d] ", int(s.cfg.Durata().Seconds())))
	if !ok || risposta == "" {
		return
	}

	// Si accetta sia "5" sia "5s": chi risponde con il numero secco a una
	// domanda che finisce con "secondi" ha ragione lui.
	if _, err := strconv.Atoi(risposta); err == nil {
		risposta += "s"
	}
	d, err := time.ParseDuration(risposta)
	if err != nil || d < time.Second {
		fmt.Fprintf(s.out(), "  %s\n", l.S("Not a valid interval: one second is the minimum.",
			"Intervallo non valido: il minimo è un secondo."))
		return
	}
	s.cfg.Interval = d.String()
	s.salvaImpostazioni()
}

func (s *sessione) cambiaTesto(campo *string, etichetta string) {
	l := s.lingua

	fmt.Fprintf(s.out(), "\n  %s\n", s.stampante().C(report.Dim, l.S(
		"ENTER to leave it as it is, - to clear it.",
		"INVIO per lasciarlo com'è, - per cancellarlo.")))

	risposta, ok := s.leggi("  " + etichetta + ": ")
	if !ok || risposta == "" {
		return
	}
	if risposta == "-" {
		*campo = ""
	} else {
		*campo = risposta
	}
	s.salvaImpostazioni()
}

func (s *sessione) cambiaCartella() {
	l := s.lingua

	fmt.Fprintf(s.out(), "\n  %s\n", s.stampante().C(report.Dim, l.S(
		"ENTER to leave it as it is, - to go back to the folder holding diskseer.exe.",
		"INVIO per lasciarla com'è, - per tornare alla cartella di diskseer.exe.")))

	risposta, ok := s.leggi("  " + l.S("Folder", "Cartella") + ": ")
	if !ok || risposta == "" {
		return
	}
	if risposta == "-" {
		s.cfg.ReportDir = ""
		s.salvaImpostazioni()
		return
	}

	// Si controlla adesso, non al momento di scrivere il referto: un errore di
	// battitura scoperto qui costa una riga, scoperto dopo costa una diagnosi.
	info, err := os.Stat(risposta)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(s.out(), "  %s\n", l.S("That folder does not exist.", "Quella cartella non esiste."))
		return
	}
	s.cfg.ReportDir = risposta
	s.salvaImpostazioni()
}

func (s *sessione) salvaImpostazioni() {
	l := s.lingua

	percorso, err := s.cfg.Salva()
	if err != nil {
		// Non salvare è un fastidio, non un guasto: la sessione in corso
		// funziona lo stesso con le impostazioni appena cambiate.
		fmt.Fprintf(s.out(), "  %s %v\n", l.S("Settings not saved:", "Impostazioni non salvate:"), err)
		return
	}
	fmt.Fprintf(s.out(), "  %s %s\n", s.stampante().C(report.Green, l.S("Saved in", "Salvate in")), percorso)
}
