package report

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/rules"
)

// A tiny screen model catches stale tails and unintended scrolling, rather
// than just checking that certain escape sequences appear in the output.
type liveScreen struct {
	cells [][]rune
	x, y  int
}

func newLiveScreen(width, height int) *liveScreen {
	s := &liveScreen{cells: make([][]rune, height)}
	for i := range s.cells {
		s.cells[i] = []rune(strings.Repeat(" ", width))
	}
	return s
}

var screenCSI = regexp.MustCompile(`^\x1b\[([0-9;]*)([A-Za-z])`)

func (s *liveScreen) draw(t *testing.T, data string) {
	t.Helper()
	for data != "" {
		if m := screenCSI.FindStringSubmatch(data); m != nil {
			data = data[len(m[0]):]
			switch m[2] {
			case "m":
			case "H":
				s.x, s.y = 0, 0
				if m[1] != "" {
					parts := strings.Split(m[1], ";")
					row, _ := strconv.Atoi(parts[0])
					col, _ := strconv.Atoi(parts[1])
					s.x, s.y = col-1, row-1
				}
				if s.y < 0 || s.y >= len(s.cells) || s.x < 0 || s.x >= len(s.cells[0]) {
					t.Fatalf("cursor outside viewport: %s", m[0])
				}
			case "K":
				if m[1] != "2" {
					t.Fatalf("unsupported line erase: %q", m[0])
				}
				for x := range s.cells[s.y] {
					s.cells[s.y][x] = ' '
				}
			case "J":
				for y := s.y; y < len(s.cells); y++ {
					for x := range s.cells[y] {
						if y > s.y || x >= s.x {
							s.cells[y][x] = ' '
						}
					}
				}
			default:
				t.Fatalf("unexpected cursor control: %q", m[0])
			}
			continue
		}
		r, size := utf8.DecodeRuneInString(data)
		data = data[size:]
		if r == '\n' || r == '\r' || r == '\033' {
			t.Fatalf("unexpected control character %q", r)
		}
		width := runeCells(r)
		if s.x+width >= len(s.cells[0]) || s.y >= len(s.cells)-1 {
			t.Fatalf("text can wrap or scroll at (%d,%d): %q", s.x, s.y, r)
		}
		if width > 0 {
			s.cells[s.y][s.x] = r
		}
		s.x += width
	}
}

func (s *liveScreen) text() string {
	var lines []string
	for _, line := range s.cells {
		lines = append(lines, string(line))
	}
	return strings.Join(lines, "\n")
}

func TestLiveRepaintRemovesOldRowTailsAndRemovedRows(t *testing.T) {
	s := newLiveScreen(100, 30)
	s.draw(t, liveFrame("old header text\nlong drive model and readings\nremoved drive\nfooter\n", 100, 30, "more"))
	s.draw(t, liveFrame("new\nshort\nfooter\n", 100, 30, "more"))
	if got := s.text(); strings.Contains(got, "old") || strings.Contains(got, "readings") || strings.Contains(got, "removed") {
		t.Fatalf("old frame leaked into new one:\n%s", got)
	}
}

func TestLiveFrameFitsViewportWithManyDisksAndAfterResize(t *testing.T) {
	snap := dischiDiProva()
	for i := 0; i < 30; i++ {
		snap.Disks = append(snap.Disks, model.Disk{Model: "日本語のディスク with a very long model", BusType: "NVMe"})
	}
	fs := []rules.Finding{{Severity: rules.SevCritical, Area: "Space", Target: "C:", Title: "A critical finding"}}
	for _, plain := range []bool{false, true} {
		for _, size := range [][2]int{{100, 32}, {80, 25}, {40, 12}, {140, 60}} {
			var out bytes.Buffer
			p := Printer{W: Uscita(&out, plain), Lang: i18n.EN, Color: true}
			p.PrintLive(snap, fs, LiveOptions{Ridisegna: true, Columns: size[0], Rows: size[1]})
			screen := newLiveScreen(size[0], size[1])
			screen.draw(t, out.String())
			got := screen.text()
			if !strings.Contains(got, "Ctrl+C") || !strings.Contains(got, "CRITICAL") {
				t.Fatalf("summary/exit instruction lost at %v:\n%s", size, got)
			}
			if size[1] < 40 && !strings.Contains(got, "More rows") {
				t.Fatalf("hidden rows not explained: %v", size)
			}
		}
	}
}

func TestLiveRedirectedOutputIsCompleteAndContainsNoCursorControls(t *testing.T) {
	var out bytes.Buffer
	p := Printer{W: &out, Lang: i18n.EN}
	p.PrintLive(dischiDiProva(), nil, LiveOptions{Columns: 8, Rows: 2})
	if strings.Contains(out.String(), "\033") || !strings.Contains(out.String(), "TOSHIBA") || !strings.Contains(out.String(), "Muto") {
		t.Fatal("redirected output was clipped or contains cursor controls")
	}
}

func TestLiveFindingColumnsAreUnaffectedByColours(t *testing.T) {
	fs := []rules.Finding{{Area: "Space", Target: "C:", Title: "first title"}, {Area: "Diagnosis", Target: "a longer disk", Title: "second title"}}
	for _, color := range []bool{false, true} {
		var out strings.Builder
		p := Printer{Lang: i18n.EN, Color: color}
		p.liveVerdetti(&out, fs)
		var positions []int
		for _, line := range strings.Split(out.String(), "\n") {
			for _, title := range []string{"first title", "second title"} {
				if i := strings.Index(line, title); i >= 0 {
					positions = append(positions, textCells(line[:i]))
				}
			}
		}
		if fmt.Sprint(positions) != "[27 27]" {
			t.Fatalf("columns moved (colour=%v): %v", color, positions)
		}
	}
}

func TestCellClippingPreservesColourAndBoundsUnicodeAndASCII(t *testing.T) {
	for _, text := range []string{"\033[31m日本語abcdef\033[0m", "e\u0301 and emoji 😀", "bad\xffutf8", "arrow → ellipsis …"} {
		for n := 0; n < 16; n++ {
			got := fitCells(text, n)
			if textCells(got) > n || !utf8.ValidString(got) {
				t.Fatalf("invalid clipping: %q at %d", got, n)
			}
		}
	}
	var out bytes.Buffer
	restore := PrepareLive(&out, true)
	restore()
	restore()
	if strings.Count(out.String(), schermoNormale) != 1 {
		t.Fatal("console restored more than once")
	}
}
