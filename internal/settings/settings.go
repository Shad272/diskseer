// Package settings conserva le preferenze dell'utente fra un avvio e l'altro.
//
// diskseer nasce come programma da riga di comando, dove ogni scelta si passa
// come opzione e non c'è niente da ricordare. Con il menu interattivo il
// bisogno cambia: chi ripara computer imposta il proprio nome una volta e se lo
// ritrova su ogni referto, e chi lavora in italiano non vuole ripetere --lang
// it ogni volta che apre il programma.
//
// Il file sta nella cartella di configurazione dell'utente, non accanto
// all'eseguibile: diskseer viene portato in giro su chiavette e lanciato da
// cartelle di sola lettura, e un programma che non riesce a salvare le proprie
// impostazioni proprio dove è stato copiato è un programma che sembra rotto.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/shad272/diskseer/internal/safefile"
)

// Config è ciò che il menu può cambiare e ritrovare al prossimo avvio.
//
// I nomi dei campi nel file sono in inglese perché il file è pubblico: chi lo
// apre per correggerlo a mano deve capire cosa sta leggendo, e la lingua del
// progetto verso l'esterno è l'inglese.
type Config struct {
	Terminal   string `json:"terminal,omitempty"`
	Profile    string `json:"profile,omitempty"`
	Workers    int    `json:"workers,omitempty"`
	Language   string `json:"language"`
	Interval   string `json:"interval"`
	Technician string `json:"technician,omitempty"`
	Contact    string `json:"contact,omitempty"`
	Customer   string `json:"customer,omitempty"`
	Colors     bool   `json:"colors"`

	// Symbols vale "auto", "full" o "plain".
	//
	// Tre stati e non due perche' il riconoscimento automatico puo' sbagliare in
	// entrambe le direzioni: c'e' la console classica di Windows che non disegna
	// i caratteri decorativi, e c'e' quella che li disegna benissimo e si vede
	// comunque offrire la versione essenziale, perche' non ha modo di
	// annunciarsi. Con due soli stati una delle due categorie resta senza
	// rimedio.
	Symbols string `json:"symbols"`

	// ReportDir vuoto significa "accanto all'eseguibile", che è dove chi lancia
	// il programma con un doppio clic si aspetta di trovare i file.
	ReportDir string `json:"reportDir,omitempty"`
}

// Predefinite descrive un diskseer appena installato: inglese, tre secondi,
// colori accesi, e nessun rilancio in un altro terminale.
//
// Il rilancio resta disponibile ma va chiesto. Per trovare Terminale di Windows
// il programma avvia PowerShell, scorre le app installate e poi riapre se
// stesso in un'altra finestra: è una sequenza che gli antivirus guardano con
// sospetto, e Bitdefender ne ha già messo in quarantena la verifica. Un
// programma di diagnosi che l'antivirus di chi lo scarica cancella al primo
// doppio clic non diagnostica niente. Chi vuole Terminale di Windows lo
// sceglie dalle impostazioni sapendo cosa succede.
func Predefinite() Config {
	return Config{Language: "en", Interval: "3s", Colors: true, Symbols: SimboliAuto, Terminal: TerminaleQuestaFinestra}
}

// TerminaleQuestaFinestra è il valore di Terminal che tiene diskseer nella
// finestra in cui è stato aperto.
const TerminaleQuestaFinestra = "direct"

// I tre valori di Symbols.
const (
	SimboliAuto   = "auto"  // decide il programma, guardando il terminale
	SimboliRicchi = "full"  // sempre i caratteri decorativi
	SimboliPiani  = "plain" // sempre la versione essenziale
)

// Piani dice se stampare con i soli caratteri essenziali, dato il verdetto del
// riconoscimento automatico.
//
// Un valore che non riconosce vale "auto": un file scritto a mano con un
// refuso non deve cambiare il comportamento in modo inspiegabile.
func (c Config) Piani(consolaRicca bool) bool {
	switch c.Symbols {
	case SimboliPiani:
		return true
	case SimboliRicchi:
		return false
	default:
		return !consolaRicca
	}
}

// Percorso indica dove vivono le impostazioni.
func Percorso() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "diskseer", "settings.json"), nil
}

// Carica legge le impostazioni e non fallisce mai.
//
// Un file mancante, illeggibile o scritto male non deve impedire una diagnosi:
// le preferenze sono una comodità, non un requisito. In tutti questi casi si
// riparte dai valori predefiniti e il programma funziona lo stesso.
func Carica() Config {
	c := Predefinite()

	percorso, err := Percorso()
	if err != nil {
		return c
	}
	raw, err := os.ReadFile(percorso)
	if err != nil {
		return c
	}

	// Si legge *sopra* i valori predefiniti, non su una struttura vuota. È la
	// differenza fra aggiungere un'impostazione nuova senza rompere niente e
	// vedersi spegnere i colori a tutti quelli che hanno un file salvato prima
	// che quell'impostazione esistesse: le chiavi assenti restano al valore di
	// partenza invece di diventare zero.
	if err := json.Unmarshal(raw, &c); err != nil {
		return Predefinite()
	}
	return c
}

// Salva scrive le impostazioni e restituisce dove le ha scritte, così il menu
// può dirlo all'utente invece di lasciarlo indovinare.
func (c Config) Salva() (string, error) {
	percorso, err := Percorso()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(percorso), 0o700); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	if err := safefile.Write(percorso, append(raw, '\n')); err != nil {
		return "", err
	}
	return percorso, nil
}

// Durata converte l'intervallo scritto nel file in qualcosa di utilizzabile.
//
// Un valore illeggibile o troppo breve torna al predefinito invece di fermare
// il programma: sotto il secondo la lettura dei dischi non farebbe in tempo a
// finire prima del giro successivo, e il risultato sarebbe un terminale che
// lampeggia mostrando sempre gli stessi numeri.
func (c Config) Durata() time.Duration {
	d, err := time.ParseDuration(c.Interval)
	if err != nil || d < time.Second {
		return 3 * time.Second
	}
	return d
}
