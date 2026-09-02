//go:build !windows

package collect

import (
	"errors"
	"runtime"

	"github.com/shad272/diskseer/internal/model"
)

// Linux è il prossimo passo: la raccolta lì si fa con smartctl e /sys,
// e il motore di regole resta identico.
func collect() (model.Snapshot, error) {
	return model.Snapshot{}, errors.New("diskseer non supporta ancora " + runtime.GOOS)
}
