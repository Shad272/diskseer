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
)

// Config è ciò che il menu può cambiare e ritrovare al prossimo avvio.
//
// I nomi dei campi nel file sono in inglese perché il file è pubblico: chi lo
// apre per correggerlo a mano deve capire cosa sta leggendo, e la lingua del
// progetto verso l'esterno è l'inglese.
type Config struct {
	Language   string `json:"language"`
	Interval   string `json:"interval"`
	Technician string `json:"technician,omitempty"`
	Contact    string `json:"contact,omitempty"`
	Customer   string `json:"customer,omitempty"`
	Colors     bool   `json:"colors"`

	// ReportDir vuoto significa "accanto all'eseguibile", che è dove chi lancia
	// il programma con un doppio clic si aspetta di trovare i file.
	ReportDir string `json:"reportDir,omitempty"`
}

// Predefinite descrive un diskseer appena installato: inglese, tre secondi,
// colori accesi.
func Predefinite() Config {
	return Config{Language: "en", Interval: "3s", Colors: true}
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
	_ = json.Unmarshal(raw, &c)
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
	if err := os.WriteFile(percorso, append(raw, '\n'), 0o600); err != nil {
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
