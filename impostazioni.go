package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/platform"
	"github.com/shad272/diskseer/internal/report"
	"github.com/shad272/diskseer/internal/settings"
	"github.com/shad272/diskseer/internal/tuning"
)

// Le impostazioni, viste da chi le cambia.
//
// Ogni modifica viene salvata subito, tranne la lingua: quella si applica
// all'istante ma chiede se deve valere anche le volte successive. È voluto —
// la lingua è l'unica impostazione che si cambia spesso per un momento solo,
// per mostrare un referto a qualcuno che non parla la tua.
//
// Le scelte con più di due valori — la lingua, la grafica — si fanno da un
// elenco con la spiegazione accanto, non premendo lo stesso numero finché non
// esce il valore giusto: un interruttore a tre posizioni costringe a indovinare
// quale sia la successiva, e a leggere un'etichetta abbreviata per capire cosa
// si è appena scelto.

// impostazione è una riga del menu: cosa si vede e cosa succede se la si sceglie.
type impostazione struct {
	etichetta string
	valore    string
	cambia    func(*sessione)
}

func apriImpostazioni(s *sessione) bool {
	for {
		elenco := s.impostazioni()
		s.mostraImpostazioni(elenco)

		scelta, ok := s.chiediNumero()
		if !ok {
			return true // ingresso chiuso: si chiude tutto, non solo questo menu
		}
		if scelta == "" {
			continue
		}
		if scelta == "0" {
			return false
		}

		i, ok := indiceVoce(scelta, len(elenco))
		if !ok {
			s.nessunaOpzione(scelta)
			continue
		}
		elenco[i].cambia(s)
	}
}

// impostazioni elenca le righe del menu. Sta separata dalla stampa per poterla
// collaudare: le etichette devono stare nella loro colonna in tutte e due le
// lingue, e in italiano sono sistematicamente più lunghe.
func (s *sessione) impostazioni() []impostazione {
	l := s.lingua
	p := s.stampante()
	nonImpostato := p.C(report.Dim, l.S("(not set)", "(non impostato)"))

	colori := l.S("on", "accesi")
	if !s.cfg.Colors {
		colori = l.S("off", "spenti")
	}
	cartella := l.S("next to diskseer.exe", "accanto a diskseer.exe")
	if s.cfg.ReportDir != "" {
		cartella = s.cfg.ReportDir
	}

	return []impostazione{
		{l.S("Language", "Lingua"), l.S("English", "Italiano"), (*sessione).cambiaLingua},
		{l.S("Live refresh", "Aggiornamento dal vivo"), s.ogniQuanto(), (*sessione).cambiaIntervallo},
		{l.S("Technician name", "Nome del tecnico"), oppure(s.cfg.Technician, nonImpostato),
			func(s *sessione) { s.cambiaTesto(&s.cfg.Technician, s.lingua.S("Technician name", "Nome del tecnico")) }},
		{l.S("Technician contact", "Contatto del tecnico"), oppure(s.cfg.Contact, nonImpostato),
			func(s *sessione) {
				s.cambiaTesto(&s.cfg.Contact, s.lingua.S("Technician contact", "Contatto del tecnico"))
			}},
		{l.S("Customer name", "Nome del cliente"), oppure(s.cfg.Customer, nonImpostato),
			func(s *sessione) { s.cambiaTesto(&s.cfg.Customer, s.lingua.S("Customer name", "Nome del cliente")) }},
		{l.S("Colours", "Colori"), colori, (*sessione).cambiaColori},
		{l.S("Report folder", "Cartella dei referti"), cartella, (*sessione).cambiaCartella},
		{l.S("Graphics", "Grafica"), s.descriviGrafica(), (*sessione).cambiaGrafica},
		{l.S("Startup terminal", "Terminale di avvio"), s.descriviTerminale(), (*sessione).cambiaTerminale},
		{l.S("Performance profile", "Profilo prestazioni"), s.descriviProfilo(), (*sessione).cambiaProfilo},
	}
}

// terminaliDiAvvio sono le scelte del terminale, nell'ordine in cui compaiono.
// Il primo è il predefinito, ed è scritto per primo perché è quello che quasi
// tutti devono tenere.
var terminaliDiAvvio = []string{settings.TerminaleQuestaFinestra, "auto", "wt", "pwsh", "powershell", "cmd"}

// descriviTerminale scrive per esteso dove si apre diskseer. "direct" e "auto"
// sono i valori del file delle impostazioni, non parole da mostrare a chi usa
// il programma.
func (s *sessione) descriviTerminale() string {
	l := s.lingua
	switch s.cfg.Terminal {
	case "", settings.TerminaleQuestaFinestra:
		return l.S("this window", "questa finestra")
	case "auto":
		return l.S("Windows Terminal if available", "Terminale di Windows, se c'è")
	case "wt":
		return l.S("Windows Terminal", "Terminale di Windows")
	case "pwsh":
		return "PowerShell 7"
	case "powershell":
		return "Windows PowerShell"
	case "cmd":
		return l.S("Command Prompt", "Prompt dei comandi")
	}
	return s.cfg.Terminal
}

func (s *sessione) cambiaTerminale() {
	l := s.lingua
	i := s.sottomenu(l.S("STARTUP TERMINAL", "TERMINALE DI AVVIO"), [][2]string{
		{l.S("This window", "Questa finestra"), l.S("diskseer stays where it opens (default)",
			"diskseer resta dove si apre (predefinito)")},
		{l.S("Automatic", "Automatico"), l.S("reopens in Windows Terminal if available; some antivirus programs flag it",
			"si riapre in Terminale di Windows se c'è; alcuni antivirus lo segnalano")},
		{l.S("Windows Terminal", "Terminale di Windows"), l.S("when installed", "se è installato")},
		{"PowerShell 7", l.S("Windows 10 and 11 only", "solo Windows 10 e 11")},
		{"Windows PowerShell", l.S("also on older Windows", "anche su Windows vecchi")},
		{l.S("Command Prompt", "Prompt dei comandi"), "cmd"},
	})
	if i < 0 {
		return
	}
	s.cfg.Terminal = terminaliDiAvvio[i]
	s.salva(l.S("From the next launch diskseer opens in: ", "Dal prossimo avvio diskseer si apre in: ") +
		s.descriviTerminale() + ".")
}

// descriviProfilo scrive il profilo per esteso, per la stessa ragione di
// descriviTerminale.
func (s *sessione) descriviProfilo() string {
	l := s.lingua
	switch s.cfg.Profile {
	case "conservative":
		return l.S("conservative", "conservativo")
	case "balanced":
		return l.S("balanced", "bilanciato")
	case "fast":
		return l.S("fast", "veloce")
	}
	return l.S("automatic", "automatico")
}

func (s *sessione) cambiaProfilo() {
	values := []string{"auto", "conservative", "balanced", "fast"}
	l := s.lingua
	i := s.sottomenu(l.S("PERFORMANCE", "PRESTAZIONI"), [][2]string{
		{l.S("Automatic", "Automatico"), l.S("adapt to available memory and CPUs", "adatta a memoria disponibile e CPU")},
		{l.S("Conservative", "Conservativo"), l.S("one device at a time", "un dispositivo alla volta")},
		{l.S("Balanced", "Bilanciato"), l.S("up to two devices", "fino a due dispositivi")},
		{l.S("Fast", "Veloce"), l.S("up to four devices, with resource limits", "fino a quattro dispositivi, con limiti di risorse")},
	})
	if i < 0 {
		return
	}
	s.cfg.Profile = values[i]
	activeProfile, _ = tuning.Select(platform.Detect(), s.cfg.Profile, s.cfg.Workers)
	activeProfile.Apply()
	s.salva(l.S("Performance profile updated.", "Profilo prestazioni aggiornato."))
}

func (s *sessione) mostraImpostazioni(elenco []impostazione) {
	p := s.stampante()
	l := s.lingua

	fmt.Fprintf(s.out(), "\n  %s\n\n", p.C(report.Bold, l.S("SETTINGS", "IMPOSTAZIONI")))

	righe := make([][2]string, len(elenco))
	for i, imp := range elenco {
		righe[i] = [2]string{imp.etichetta, imp.valore}
	}
	s.mostraVoci(righe, l.S("Back", "Indietro"))

	fmt.Fprintf(s.out(), "\n  %s\n", p.C(report.Dim, s.doveSonoSalvate()))
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

// ogniQuanto scrive l'intervallo per esteso e con il plurale giusto: "3s" è
// una sigla da programmatori, "ogni 1 secondi" un errore.
func (s *sessione) ogniQuanto() string {
	secondi := uint64(s.cfg.Durata().Seconds())
	return s.lingua.S("every ", "ogni ") +
		s.lingua.N(secondi, "second", "seconds", "secondo", "secondi")
}

// descriviGrafica dice quale grafica è in uso e, se la sceglie il programma,
// quale ha scelto per questa finestra: "automatica" da sola non dice cosa si
// sta guardando.
func (s *sessione) descriviGrafica() string {
	l := s.lingua
	switch s.cfg.Symbols {
	case settings.SimboliRicchi:
		return l.S("full", "completa")
	case settings.SimboliPiani:
		return l.S("simple", "semplice")
	}
	if s.consolaRicca {
		return l.S("automatic (full in this window)", "automatica (completa in questa finestra)")
	}
	return l.S("automatic (simple in this window)", "automatica (semplice in questa finestra)")
}

func oppure(valore, seVuoto string) string {
	if strings.TrimSpace(valore) == "" {
		return seVuoto
	}
	return valore
}

// sottomenu mostra un elenco di scelte con la sua intestazione e restituisce la
// posizione scelta, oppure -1 per "indietro" o per una risposta non valida.
func (s *sessione) sottomenu(titolo string, righe [][2]string) int {
	p := s.stampante()
	fmt.Fprintf(s.out(), "\n  %s\n\n", p.C(report.Bold, titolo))

	attenuate := make([][2]string, len(righe))
	for i, r := range righe {
		attenuate[i] = [2]string{r[0], p.C(report.Dim, r[1])}
	}
	s.mostraVoci(attenuate, s.lingua.S("Back", "Indietro"))

	scelta, ok := s.chiediNumero()
	if !ok || scelta == "" || scelta == "0" {
		return -1
	}
	i, ok := indiceVoce(scelta, len(righe))
	if !ok {
		s.nessunaOpzione(scelta)
		return -1
	}
	return i
}

// cambiaLingua applica subito la scelta e poi chiede se deve valere anche le
// volte successive.
//
// La domanda arriva già nella lingua appena scelta: è la conferma immediata che
// il cambio ha avuto effetto, prima ancora che l'utente risponda.
func (s *sessione) cambiaLingua() {
	lingue := []i18n.Lingua{i18n.EN, i18n.IT}
	i := s.sottomenu(s.lingua.S("LANGUAGE", "LINGUA"), [][2]string{
		{"English", ""},
		{"Italiano", ""},
	})
	if i < 0 {
		return
	}

	s.lingua = lingue[i]
	// I verdetti contengono frasi già tradotte: senza rifarli, il menu
	// parlerebbe una lingua e il referto un'altra.
	s.ricalcola()

	l := s.lingua
	fmt.Fprintln(s.out())
	if s.conferma(l.S("Use English next time too? Type y for yes or n for no:",
		"Usare l'italiano anche le prossime volte? Scrivi s per sì o n per no:")) {
		s.cfg.Language = l.String()
		s.salva(l.S("From now on diskseer starts in English.", "Da ora in poi diskseer parte in italiano."))
		return
	}
	s.fatto(l.S("English only for this time.", "Italiano solo per questa volta."))
}

func (s *sessione) cambiaIntervallo() {
	l := s.lingua

	risposta, ok := s.leggi("\n  " + l.F(
		"Every how many seconds should live mode update? (now %d): ",
		"Ogni quanti secondi aggiornare la modalità dal vivo? (ora %d): ",
		int(s.cfg.Durata().Seconds())))
	if !ok || risposta == "" {
		return
	}

	// Si accetta sia "5" sia "5s": chi risponde con il numero secco a una
	// domanda sui secondi ha ragione lui.
	if _, err := strconv.Atoi(risposta); err == nil {
		risposta += "s"
	}
	d, err := time.ParseDuration(risposta)
	if err != nil || d < time.Second {
		s.problema(l.S("That is not a valid number of seconds: it must be 1 or more.",
			"Non è un numero di secondi valido: deve essere 1 o più."))
		return
	}
	s.cfg.Interval = d.String()
	s.salva(l.S("Live mode will update ", "La modalità dal vivo si aggiornerà ") + s.ogniQuanto() + ".")
}

func (s *sessione) cambiaTesto(campo *string, etichetta string) {
	l := s.lingua

	fmt.Fprintf(s.out(), "\n  %s\n", s.stampante().C(report.Dim, l.S(
		"Write the new value and press ENTER. ENTER alone leaves it as it is, a single - deletes it.",
		"Scrivi il nuovo valore e premi INVIO. Solo INVIO lo lascia com'è, un trattino - lo cancella.")))

	risposta, ok := s.leggi("  " + etichetta + ": ")
	if !ok || risposta == "" {
		return
	}
	if risposta == "-" {
		*campo = ""
		s.salva(etichetta + l.S(" deleted.", " cancellato."))
		return
	}
	*campo = risposta
	s.salva(etichetta + ": " + risposta + ".")
}

func (s *sessione) cambiaColori() {
	l := s.lingua
	s.cfg.Colors = !s.cfg.Colors
	if s.cfg.Colors {
		s.salva(l.S("Colours on.", "Colori accesi."))
		return
	}
	s.salva(l.S("Colours off.", "Colori spenti."))
}

func (s *sessione) cambiaCartella() {
	l := s.lingua

	fmt.Fprintf(s.out(), "\n  %s\n", s.stampante().C(report.Dim, l.S(
		"Write the folder and press ENTER. ENTER alone leaves it as it is, a single - goes back to the folder of diskseer.exe.",
		"Scrivi la cartella e premi INVIO. Solo INVIO la lascia com'è, un trattino - torna alla cartella di diskseer.exe.")))

	risposta, ok := s.leggi("  " + l.S("Report folder", "Cartella dei referti") + ": ")
	if !ok || risposta == "" {
		return
	}
	if risposta == "-" {
		s.cfg.ReportDir = ""
		s.salva(l.S("Reports will be saved next to diskseer.exe.",
			"I referti verranno salvati accanto a diskseer.exe."))
		return
	}

	// Si controlla adesso, non al momento di scrivere il referto: un errore di
	// battitura scoperto qui costa una riga, scoperto dopo costa una diagnosi.
	info, err := os.Stat(risposta)
	if err != nil || !info.IsDir() {
		s.problema(l.F("The folder %q does not exist.", "La cartella %q non esiste.", risposta))
		return
	}
	s.cfg.ReportDir = risposta
	s.salva(l.F("Reports will be saved in %s.", "I referti verranno salvati in %s.", risposta))
}

// cambiaGrafica offre le tre grafiche con la spiegazione di ciascuna.
//
// È l'impostazione più difficile da capire dal nome, e la più importante per
// chi ne ha bisogno: chi vede quadratini vuoti al posto dei simboli deve poter
// riconoscere la cura leggendo la riga, senza sapere cosa sia un font.
func (s *sessione) cambiaGrafica() {
	l := s.lingua
	valori := []string{settings.SimboliAuto, settings.SimboliRicchi, settings.SimboliPiani}

	i := s.sottomenu(l.S("GRAPHICS", "GRAFICA"), [][2]string{
		{l.S("Automatic", "Automatica"), l.S("diskseer decides by looking at this window",
			"decide diskseer guardando questa finestra")},
		{l.S("Full", "Completa"), l.S("frames, dots and symbols", "cornici, pallini e simboli")},
		{l.S("Simple", "Semplice"), l.S("plain letters only: choose it if you see empty squares",
			"solo lettere normali: sceglila se vedi quadratini vuoti")},
	})
	if i < 0 {
		return
	}

	s.cfg.Symbols = valori[i]
	s.salva(l.S("Graphics: ", "Grafica: ") + s.descriviGrafica() + ".")
}

// salva scrive le impostazioni e dice in una riga cosa è cambiato.
//
// Il messaggio descrive il risultato, non l'operazione: "Colori spenti.
// Salvato." dice all'utente cosa aspettarsi, un percorso di file no.
func (s *sessione) salva(cosaECambiato string) {
	l := s.lingua
	if _, err := s.cfg.Salva(); err != nil {
		// Non salvare è un fastidio, non un guasto: la sessione in corso
		// funziona lo stesso con l'impostazione appena cambiata.
		s.problema(cosaECambiato + " " + l.F(
			"It applies now but could not be saved: %v",
			"Vale adesso ma non è stato possibile salvarlo: %v", err))
		return
	}
	s.fatto(cosaECambiato + l.S(" Saved.", " Salvato."))
}

func (s *sessione) fatto(messaggio string) {
	fmt.Fprintf(s.out(), "  %s\n", s.stampante().C(report.Green, messaggio))
}

func (s *sessione) problema(messaggio string) {
	fmt.Fprintf(s.out(), "  %s\n", s.stampante().C(report.Yellow, messaggio))
}
