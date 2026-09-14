package report

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// liveFrame uses explicit row addresses, clears old text on every row and
// leaves the last column/row unused. It never emits a newline that could
// scroll the viewport, even when a drive disappears or the window shrinks.
func liveFrame(content string, columns, rows int, overflow string) string {
	width, height := columns-1, rows-1
	if width < 1 || height < 1 {
		return cursoreAOrigine + pulisciDaQui
	}
	lines := strings.Split(strings.Trim(content, "\n"), "\n")
	if len(lines) > height {
		compact := make([]string, 0, len(lines))
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				compact = append(compact, line)
			}
		}
		lines = compact
	}
	if len(lines) > height {
		footer := lines[len(lines)-1]
		if height == 1 {
			lines = []string{footer}
		} else {
			lines = append(lines[:height-2], "  "+overflow, footer)
		}
	}
	var frame strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&frame, "\033[%d;1H\033[2K%s", i+1, fitCells(line, width))
	}
	fmt.Fprintf(&frame, "\033[%d;1H%s", len(lines)+1, pulisciDaQui)
	return frame.String()
}

// tokens separates our SGR colours from printable text. Control characters
// from device labels cannot reposition the cursor or create extra rows.
func cellTokens(s string, visit func(string, int)) {
	for len(s) > 0 {
		if s[0] == '\033' && strings.HasPrefix(s, "\033[") {
			i := 2
			for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == ';') {
				i++
			}
			if i < len(s) && s[i] == 'm' {
				visit(s[:i+1], 0)
				s = s[i+1:]
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if !unicode.IsControl(r) {
			visit(string(r), runeCells(r))
		}
	}
}

func runeCells(r rune) int {
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
		return 0
	}
	// CJK/fullwidth and emoji need two cells; ambiguous symbols use one, as
	// in the Windows fonts used for the ASCII and Unicode report variants.
	if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
		r >= 0x2e80 && r <= 0xa4cf && r != 0x303f ||
		r >= 0xac00 && r <= 0xd7a3 || r >= 0xf900 && r <= 0xfaff ||
		r >= 0xfe10 && r <= 0xfe19 || r >= 0xfe30 && r <= 0xfe6f ||
		r >= 0xff01 && r <= 0xff60 || r >= 0xffe0 && r <= 0xffe6 ||
		r >= 0x1f300 && r <= 0x1faff || r >= 0x20000 && r <= 0x3fffd) {
		return 2
	}
	return 1
}

func textCells(s string) int {
	n := 0
	cellTokens(s, func(_ string, cells int) { n += cells })
	return n
}

func fitCells(s string, width int) string {
	if width <= 0 {
		return ""
	}
	truncated := textCells(s) > width
	limit := width
	marker := ""
	if truncated {
		marker = strings.Repeat(".", minInt(3, width))
		limit -= len(marker)
	}
	var out strings.Builder
	n, stopped, ansi := 0, false, false
	cellTokens(s, func(token string, cells int) {
		if stopped {
			return
		}
		if n+cells > limit {
			stopped = true
			return
		}
		ansi = ansi || strings.HasPrefix(token, "\033[")
		out.WriteString(token)
		n += cells
	})
	out.WriteString(marker)
	if ansi {
		out.WriteString("\033[0m")
	}
	return out.String()
}

func padCells(s string, width int) string {
	s = fitCells(s, width)
	return s + strings.Repeat(" ", width-textCells(s))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
