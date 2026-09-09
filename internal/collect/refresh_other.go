//go:build !windows

package collect

import "github.com/shad272/diskseer/internal/model"

// Refresh non ha nulla da aggiornare finché non esiste un raccoglitore per
// questo sistema: senza dati raccolti non c'è niente da riportare al presente.
func Refresh(s *model.Snapshot) {}
