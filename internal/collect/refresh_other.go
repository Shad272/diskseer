//go:build !windows

package collect

import (
	"context"
	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/tuning"
)

// Refresh non ha nulla da aggiornare finché non esiste un raccoglitore per
// questo sistema: senza dati raccolti non c'è niente da riportare al presente.
func Refresh(s *model.Snapshot) {}

func RefreshContext(ctx context.Context, s *model.Snapshot, p tuning.Profile) error { return ctx.Err() }
