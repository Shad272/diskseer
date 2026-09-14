//go:build !windows

package terminal

import "github.com/shad272/diskseer/internal/platform"

func DiscoverDetailed(c platform.Capabilities) ([]Candidate, []string) {
	return nil, []string{"native terminal discovery is Windows-only"}
}

func Discover(c platform.Capabilities) []Candidate                                { return nil }
func Launch(c []Candidate, args []string, own bool, log func(string)) (bool, int) { return false, 0 }
