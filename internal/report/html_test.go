package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/rules"
)

func TestWriteHTMLLangCreatesInteractiveStandaloneGUI(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "snapshot-completo.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snap model.Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	findings := rules.Run(snap, i18n.IT)
	path := filepath.Join(t.TempDir(), "report.html")
	if err := WriteHTMLLang(path, i18n.IT, snap, findings, HTMLOptions{Customer: "Cliente Test"}); err != nil {
		t.Fatal(err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	for _, want := range []string{
		`<html lang="it"`, `diskseer`, `data-filter="critical"`,
		`id="theme"`, `Cliente Test`, `@media print`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("report does not contain %q", want)
		}
	}
	if strings.Contains(html, "https://") || strings.Contains(html, "http://") {
		t.Error("standalone report unexpectedly references a remote resource")
	}
}

func BenchmarkWriteHTML(b *testing.B) {
	snap := model.Snapshot{System: model.System{Manufacturer: "Test", Model: "PC"}}
	findings := rules.Run(snap, i18n.EN)
	dir := b.TempDir()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := WriteHTML(filepath.Join(dir, "report.html"), snap, findings, HTMLOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}
