package rules

import (
	"sort"

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

// areaDiagnosi raccoglie i verdetti che parlano della diagnosi stessa invece
// che della macchina.
const areaDiagnosi = "Diagnosi"

// Run applica tutte le regole e restituisce i verdetti dal più grave al meno.
func Run(s model.Snapshot) []Finding {
	b := &builder{}
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
		if diagnosi := b.out[i].Area == areaDiagnosi; diagnosi != (b.out[j].Area == areaDiagnosi) {
			return diagnosi
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
