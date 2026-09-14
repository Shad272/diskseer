package tuning

import (
	"github.com/shad272/diskseer/internal/platform"
	"testing"
)

func TestResourceProfiles(t *testing.T) {
	for _, c := range []struct {
		caps    platform.Capabilities
		name    string
		workers int
	}{
		{platform.Capabilities{}, "conservative", 1},
		{platform.Capabilities{CPUs: 2, AvailableMemory: 512 << 20}, "conservative", 1},
		{platform.Capabilities{CPUs: 16, AvailableMemory: 8 << 30, Architecture: "amd64"}, "fast", 4},
		{platform.Capabilities{CPUs: 4, AvailableMemory: 2 << 30, Architecture: "arm64"}, "balanced", 2},
		{platform.Capabilities{CPUs: 16, AvailableMemory: 8 << 30, MemoryLoad: 95}, "conservative", 1},
		{platform.Capabilities{CPUs: 8, AvailableMemory: 3 << 30, Architecture: "386"}, "conservative", 1},
	} {
		p, err := Select(c.caps, "auto", 0)
		if err != nil || p.Name != c.name || p.Workers != c.workers {
			t.Fatalf("%+v: %+v %v", c.caps, p, err)
		}
	}
	p, _ := Select(platform.Capabilities{}, "fast", 4)
	if p.Workers != 1 {
		t.Fatal("explicit mode bypassed resource ceiling")
	}
	if _, err := Select(platform.Capabilities{}, "invalid", 0); err == nil {
		t.Fatal("invalid profile accepted")
	}
	if _, err := Select(platform.Capabilities{}, "auto", 100); err == nil {
		t.Fatal("unbounded workers accepted")
	}
	p = Profile{Workers: 4}
	if p.DiskWorkers(50, true) != 2 || p.DiskWorkers(1, false) != 1 {
		t.Fatal("disk limits ignored")
	}
}
