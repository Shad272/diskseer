//go:build !windows

package collect

import (
	"context"
	"errors"
	"runtime"

	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/tuning"
)

// Linux è il prossimo passo: la raccolta lì si fa con smartctl e /sys,
// e il motore di regole resta identico.
func collectContext(ctx context.Context, p tuning.Profile) (model.Snapshot, error) {
	return model.Snapshot{}, errors.New("diskseer non supporta ancora " + runtime.GOOS)
}
