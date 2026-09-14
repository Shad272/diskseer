// Package collect raccoglie i dati grezzi dalla macchina.
//
// Il resto del programma non sa da dove arrivano i dati: vede solo uno
// model.Snapshot. È ciò che permette di aggiungere il supporto Linux
// scrivendo un solo file nuovo, senza toccare una riga del motore di regole.
package collect

import (
	"context"
	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/platform"
	"github.com/shad272/diskseer/internal/tuning"
)

// Collect restituisce una fotografia dello stato della macchina.
// L'implementazione dipende dal sistema operativo (vedi i file con build tag).
func Collect() (model.Snapshot, error) {
	p, _ := tuning.Select(platform.Detect(), "auto", 0)
	return CollectContext(context.Background(), p)
}

// collect is kept for the existing internal hardware-probe tests.
func collect() (model.Snapshot, error) { return Collect() }

func CollectContext(ctx context.Context, p tuning.Profile) (model.Snapshot, error) {
	s, err := collectContext(ctx, p)
	if err != nil {
		return s, err
	}
	Normalize(&s)
	return s, nil
}
