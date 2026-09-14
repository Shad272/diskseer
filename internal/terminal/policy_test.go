package terminal

import (
	"errors"
	"github.com/shad272/diskseer/internal/platform"
	"reflect"
	"testing"
)

func modern() platform.Capabilities {
	return platform.Capabilities{OS: "windows", Major: 10, Build: 26100, VTAvailable: true, Console: true, InputConsole: true, OwnConsole: true}
}
func TestSelectionMatrix(t *testing.T) {
	found := []Candidate{{"wt", "stable"}, {"wt", "preview"}, {"pwsh", "ps7"}, {"powershell", "ps5"}, {"cmd", "cmd"}}
	cases := []struct {
		name      string
		c         platform.Capabilities
		r         Request
		available []Candidate
		want      []string
	}{
		{"modern", modern(), Request{}, found, []string{"stable", "preview", "ps7", "ps5", "cmd"}},
		{"explicit", modern(), Request{Preference: "cmd"}, found, []string{"cmd", "stable", "preview", "ps7", "ps5"}},
		{"no PATH alias, Preview registered", modern(), Request{}, found[1:], []string{"preview", "ps7", "ps5", "cmd"}},
		{"no WT", modern(), Request{}, found[2:], []string{"ps7", "ps5", "cmd"}},
		{"only cmd", modern(), Request{}, found[4:], []string{"cmd"}},
		{"Windows 7", platform.Capabilities{OS: "windows", Major: 6, Minor: 1, Console: true, InputConsole: true, OwnConsole: true}, Request{}, found, []string{"ps5", "cmd"}},
		{"Windows 8.1", platform.Capabilities{OS: "windows", Major: 6, Minor: 3, Console: true, InputConsole: true, OwnConsole: true}, Request{}, found, []string{"ps5", "cmd"}},
		{"direct", modern(), Request{Direct: true}, found, nil},
		{"child prevents loop", modern(), Request{Child: true}, found, nil},
		{"JSON/script", modern(), Request{NonInteractive: true}, found, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Select(c.c, c.r, c.available)
			if err != nil {
				t.Fatal(err)
			}
			var paths []string
			for _, v := range got {
				paths = append(paths, v.Path)
			}
			if !reflect.DeepEqual(paths, c.want) {
				t.Fatalf("got %v want %v", paths, c.want)
			}
		})
	}
	for _, mutate := range []func(*platform.Capabilities){func(c *platform.Capabilities) { c.InTerminal = true }, func(c *platform.Capabilities) { c.OwnConsole = false }, func(c *platform.Capabilities) { c.Console = false }, func(c *platform.Capabilities) { c.InputConsole = false }} {
		c := modern()
		mutate(&c)
		got, _ := Select(c, Request{}, found)
		if len(got) != 0 {
			t.Fatalf("unexpected relaunch: %+v", c)
		}
	}
}

func TestFallbacksAreAttemptedUntilAcknowledged(t *testing.T) {
	var visited []string
	got := Try([]Candidate{{"wt", "broken"}, {"pwsh", "broken"}, {"cmd", "works"}}, func(c Candidate) error {
		visited = append(visited, c.Kind)
		if c.Kind == "cmd" {
			return nil
		}
		return errors.New("failed")
	}, func(string) {})
	if !got || !reflect.DeepEqual(visited, []string{"wt", "pwsh", "cmd"}) {
		t.Fatal(visited)
	}
	if Try(nil, func(Candidate) error { return nil }, func(string) {}) {
		t.Fatal("empty candidate list succeeded")
	}
}
