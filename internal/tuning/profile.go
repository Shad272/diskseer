// Package tuning bounds diagnostic work; DiskSeer does not crawl directories.
package tuning

import (
	"fmt"
	"github.com/shad272/diskseer/internal/platform"
	"runtime/debug"
)

type Profile struct {
	Name        string `json:"profile"`
	Workers     int    `json:"workers"`
	MemoryLimit int64  `json:"goMemoryLimit"`
	OutputLimit int    `json:"collectorOutputLimit"`
}

func Select(c platform.Capabilities, requested string, workers int) (Profile, error) {
	if requested == "" {
		requested = "auto"
	}
	name := requested
	if name == "auto" {
		name = "balanced"
		if c.CPUs <= 2 || c.AvailableMemory < 1<<30 || c.MemoryLoad >= 85 || c.Architecture == "386" {
			name = "conservative"
		} else if c.CPUs >= 8 && c.AvailableMemory >= 4<<30 {
			name = "fast"
		}
	}
	p := Profile{Name: name, Workers: 2, MemoryLimit: 128 << 20, OutputLimit: 8 << 20}
	switch name {
	case "conservative":
		p.Workers, p.MemoryLimit, p.OutputLimit = 1, 64<<20, 4<<20
	case "balanced":
	case "fast":
		p.Workers, p.MemoryLimit = 4, 256<<20
	default:
		return Profile{}, fmt.Errorf("profile must be auto, conservative, balanced or fast")
	}
	if workers < 0 || workers > 4 {
		return Profile{}, fmt.Errorf("workers must be between 0 (automatic) and 4")
	}
	if workers > 0 {
		p.Workers = workers
	}
	// Hard ceilings still apply to explicit profiles on resource-starved PCs.
	if c.CPUs <= 2 || c.AvailableMemory < 512<<20 || c.MemoryLoad >= 90 {
		p.Workers = 1
		p.MemoryLimit = 64 << 20
	}
	if c.Architecture == "386" && p.Workers > 2 {
		p.Workers = 2
	}
	return p, nil
}

func (p Profile) Apply() { debug.SetMemoryLimit(p.MemoryLimit) }

func (p Profile) DiskWorkers(count int, systemHDD bool) int {
	n := p.Workers
	if n < 1 {
		n = 1
	}
	if n > 4 {
		n = 4
	}
	if systemHDD && n > 2 {
		n = 2
	}
	if count < n {
		n = count
	}
	return n
}
