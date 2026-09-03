package rules

import (
	"sort"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
)

// Rule esamina uno snapshot e aggiunge zero o più verdetti.
// Ogni regola è indipendente dalle altre: si aggiunge una regola nuova
// scrivendo una funzione e mettendola in questa lista.
type Rule func(model.Snapshot, *builder)

var registry = []Rule{
	ruleSystemDiskIsHDD,
	ruleDiskHealthStatus,
	ruleDiskUncorrectedErrors,
	ruleDiskWear,
	ruleDiskTemperature,
	ruleDiskPeakTemperature,
	ruleDiskAge,
	ruleNVMeCriticalWarning,
	ruleNVMeMediaErrors,
	ruleNVMeAvailableSpare,
	ruleNVMeUnsafeShutdowns,
	ruleNVMeThermalThrottleTime,
	ruleNVMeLifeProjection,
	ruleDiskStartStopCycles,
	ruleSMARTPendingSectors,
	ruleSMARTOfflineUncorrectable,
	ruleSMARTReallocatedSectors,
	ruleSMARTCRCErrors,
	ruleSMARTSpinRetry,
	ruleSMARTLoadCycles,
	ruleSMARTCommandTimeout,
	ruleSMARTUnexpectedPowerLoss,
	ruleVolumeFreeSpace,
	ruleVolumeHealth,
	ruleThermalHigh,
	ruleBatteryWear,
	ruleUptime,
	ruleNotElevated,
}

// Run applica tutte le regole e restituisce i verdetti dal più grave al meno.
func Run(s model.Snapshot, l i18n.Lingua) []Finding {
	b := &builder{l: l}
	for _, r := range registry {
		r(s, b)
	}

	// I limiti della diagnosi vanno prima dei suoi risultati.
	//
	// L'avviso "analisi parziale" è marcato INFO perché non descrive un
	// guasto, e ordinando per sola gravità finirebbe in fondo — letto per
	// ultimo, quando chi legge si è già fatto un'idea sulla macchina. Ma
	// sapere che mezzo controllo non è stato fatto cambia il peso di tutto
	// ciò che viene dopo, e quindi va detto prima.
	sort.SliceStable(b.out, func(i, j int) bool {
		if b.out[i].SuiLimiti != b.out[j].SuiLimiti {
			return b.out[i].SuiLimiti
		}
		return b.out[i].Severity > b.out[j].Severity
	})
	return b.out
}

// Overall riassume l'esito complessivo nella severità più alta trovata.
func Overall(fs []Finding) Severity {
	worst := SevOK
	for _, f := range fs {
		if f.Severity > worst {
			worst = f.Severity
		}
	}
	return worst
}

// Le aree sono etichette mostrate all'utente, quindi vanno tradotte. Restano
// funzioni e non costanti perché dipendono dalla lingua scelta a ogni
// esecuzione.
func areaDisco(l i18n.Lingua) string       { return l.S("Drive", "Disco") }
func areaTermica(l i18n.Lingua) string     { return l.S("Thermal", "Termica") }
func areaSpazio(l i18n.Lingua) string      { return l.S("Space", "Spazio") }
func areaVolume(l i18n.Lingua) string      { return l.S("Volume", "Volume") }
func areaSistema(l i18n.Lingua) string     { return l.S("System", "Sistema") }
func areaBatteria(l i18n.Lingua) string    { return l.S("Battery", "Batteria") }
func areaPrestazioni(l i18n.Lingua) string { return l.S("Performance", "Prestazioni") }
func areaCollegamento(l i18n.Lingua) string {
	return l.S("Connection", "Collegamento")
}

func areaDiagnosi(l i18n.Lingua) string { return l.S("Diagnosis", "Diagnosi") }
