// Package terminal separates terminal discovery, selection and process launch.
package terminal

import (
	"fmt"
	"github.com/shad272/diskseer/internal/platform"
	"strings"
)

type Candidate struct{ Kind, Path string }
type Request struct {
	Preference                    string
	Direct, NonInteractive, Child bool
	Explicit                      bool
}

func Valid(s string) bool {
	switch s {
	case "", "auto", "direct", "wt", "pwsh", "powershell", "cmd":
		return true
	}
	return false
}

func Select(c platform.Capabilities, r Request, found []Candidate) ([]Candidate, error) {
	if !Valid(r.Preference) {
		return nil, fmt.Errorf("terminal must be auto, direct, wt, pwsh, powershell or cmd")
	}
	if r.Direct || r.Child || r.NonInteractive || r.Preference == "direct" || !c.Console || !c.InputConsole || c.InTerminal {
		return nil, nil
	}
	if !c.OwnConsole && !r.Explicit {
		return nil, nil
	}
	order := []string{"wt", "pwsh", "powershell", "cmd"}
	if r.Preference != "" && r.Preference != "auto" {
		order = append([]string{r.Preference}, order...)
	}
	var out []Candidate
	seen := map[string]bool{}
	for _, kind := range order {
		if kind == "wt" && !c.CanUseTerminal() {
			continue
		}
		// Modern pwsh is not a Windows 7/8 runtime. Windows PowerShell is the
		// built-in legacy shell even if an incompatible pwsh happens to be on PATH.
		if kind == "pwsh" && c.Major < 10 {
			continue
		}
		for _, candidate := range found {
			key := strings.ToLower(candidate.Path)
			if candidate.Kind == kind && !seen[key] && key != "" {
				seen[key] = true
				out = append(out, candidate)
			}
		}
	}
	return out, nil
}

// Try only succeeds after the child has acknowledged readiness, not merely
// because an app-execution alias returned exit code zero.
func Try(candidates []Candidate, start func(Candidate) error, log func(string)) bool {
	for _, c := range candidates {
		if err := start(c); err != nil {
			log(c.Kind + ": launch failed; trying fallback")
			continue
		}
		log(c.Kind + ": child ready")
		return true
	}
	log("using the current console")
	return false
}
