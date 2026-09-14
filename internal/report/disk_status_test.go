package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/rules"
)

func TestDriveStatusUsesDiagnosticFindings(t *testing.T) {
	cases := []struct {
		name string
		disk model.Disk
		sev  rules.Severity
	}{
		{"pending sectors despite Windows Healthy", model.Disk{HealthStatus: "Healthy", SMART: &model.SMARTData{Attributes: []model.SMARTAttribute{{ID: model.SMARTPendingSectors, Raw: 3}}}}, rules.SevCritical},
		{"NVMe media errors without warning bits", model.Disk{HealthStatus: "Healthy", NVMe: &model.NVMeHealth{MediaErrors: 5}}, rules.SevCritical},
		{"missing counters", model.Disk{HealthStatus: "Healthy"}, rules.SevInfo},
		{"empty SMART response", model.Disk{HealthStatus: "Healthy", SMART: &model.SMARTData{}}, rules.SevInfo},
		{"clean SMART", model.Disk{HealthStatus: "Healthy", SMART: &model.SMARTData{Attributes: []model.SMARTAttribute{{ID: model.SMARTPendingSectors}}}}, rules.SevOK},
	}
	for _, c := range cases {
		for _, l := range []i18n.Lingua{i18n.EN, i18n.IT} {
			t.Run(c.name+l.S("/en", "/it"), func(t *testing.T) {
				label, sev := statoDisco(c.disk, l)
				if sev != c.sev {
					t.Fatalf("status = %q (%v), want %v", label, sev, c.sev)
				}
				p := Printer{Lang: l}
				if got := p.saluteDisco(c.disk); got != label {
					t.Errorf("live status = %q, expected %q", got, label)
				}
				snap := model.Snapshot{Elevated: true, Disks: []model.Disk{c.disk}}
				path := filepath.Join(t.TempDir(), "report.html")
				if err := WriteHTMLLang(path, l, snap, rules.Run(snap, l), HTMLOptions{}); err != nil {
					t.Fatal(err)
				}
				out, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				want := `class="disk-status ` + sev.Slug() + `">` + label + `</b>`
				if !strings.Contains(string(out), want) {
					t.Errorf("HTML drive card missing %s", want)
				}
			})
		}
	}
}

func TestStaleCriticalReadingsStayCriticalAndAreLabelled(t *testing.T) {
	d := model.Disk{ReadError: "unavailable", NVMe: &model.NVMeHealth{MediaErrors: 5}}
	label, sev := statoDisco(d, i18n.EN)
	if sev != rules.SevCritical || !strings.Contains(label, "not refreshed") {
		t.Fatalf("misleading stale status %q %v", label, sev)
	}
	fs := rules.Run(model.Snapshot{Disks: []model.Disk{d}}, i18n.EN)
	if len(fs) == 0 || !fs[0].SuiLimiti {
		t.Fatal("stale readings limitation must precede diagnosis")
	}
}
