// Package collect raccoglie i dati grezzi dalla macchina.
//
// Il resto del programma non sa da dove arrivano i dati: vede solo uno
// model.Snapshot. È ciò che permette di aggiungere il supporto Linux
// scrivendo un solo file nuovo, senza toccare una riga del motore di regole.
package collect

import "github.com/shad272/diskseer/internal/model"

// Collect restituisce una fotografia dello stato della macchina.
// L'implementazione dipende dal sistema operativo (vedi i file con build tag).
func Collect() (model.Snapshot, error) {
	s, err := collect()
	if err != nil {
		return s, err
	}
	Normalize(&s)
	return s, nil
}
