package report

import (
	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/rules"
)

// statoDisco usa le regole della diagnosi: il giudizio generico di Windows
// non deve nascondere gli allarmi letti direttamente dal dispositivo.
func statoDisco(d model.Disk, l i18n.Lingua) (string, rules.Severity) {
	fs := rules.Run(model.Snapshot{Elevated: true, Disks: []model.Disk{d}}, l)
	sev := rules.Overall(fs)
	if d.ReadError != "" {
		label := l.S("not refreshed", "non aggiornato")
		if sev >= rules.SevWarn {
			return sev.Label(l) + " · " + label, sev
		}
		return label, rules.SevInfo
	}
	if sev >= rules.SevWarn {
		return sev.Label(l), sev
	}
	if d.NVMe == nil && (d.SMART == nil || len(d.SMART.Attributes) == 0) {
		return l.S("unverified", "non verificato"), rules.SevInfo
	}
	return "ok", rules.SevOK
}
