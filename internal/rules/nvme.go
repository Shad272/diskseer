package rules

import (
	"fmt"

	"github.com/shad272/diskseer/internal/model"
)

const (
	spareWarnPct           = 20 // celle di riserva rimaste
	unsafeShutdownWarnPct  = 10.0
	unsafeShutdownInfoPct  = 3.0
	unsafeShutdownMinCount = 10
	lifeProjectionMinHours = 500  // sotto questa soglia la proiezione è rumore
	lifeWarnHours          = 8760 // un anno di accensione residuo
	lifeInfoHours          = 17520
)

// ruleNVMeCriticalWarning riporta i guasti che il disco dichiara da solo.
//
// Il byte di allarme critico è l'unico posto dove un NVMe ammette apertamente
// di essere in avaria. Ogni bit acceso è una diagnosi già fatta dal
// costruttore: non c'è niente da interpretare, solo da riferire.
func ruleNVMeCriticalWarning(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.NVMe == nil || d.NVMe.CriticalWarning == 0 {
			continue
		}
		h := *d.NVMe

		add := func(sev Severity, titolo, dettaglio, azione string) {
			b.add(Finding{
				Severity: sev,
				Area:     areaDisco(b.l),
				Target:   d.Model,
				Title:    titolo,
				Detail:   dettaglio,
				Action:   azione,
				Evidence: map[string]string{
					"criticalWarning": fmt.Sprintf("0x%02X", h.CriticalWarning),
				},
			})
		}

		if h.ReadOnly() {
			add(SevCritical,
				b.s("The drive has switched to read-only", "Il disco è passato in sola lettura"),
				b.s("The drive has exhausted its ability to write and protected itself by blocking all writes. Existing data is still readable, but nothing new can be saved and the condition is not reversible.",
					"Il disco ha esaurito la propria capacità di scrivere e si è protetto bloccando ogni scrittura. I dati presenti sono ancora leggibili, ma non è possibile salvarne di nuovi e la condizione non è reversibile."),
				b.s("Copy everything off it now and replace the drive. It cannot be repaired.",
					"Copiare subito tutto il contenuto su un altro supporto e sostituire il disco. Non è riparabile."))
		}
		if h.ReliabilityDegraded() {
			add(SevCritical,
				b.s("The drive reports its own reliability as degraded",
					"Il disco dichiara la propria affidabilità compromessa"),
				b.s("The drive itself is reporting that its memory subsystem is no longer reliable. This is not a threshold-based prediction: it is a diagnosis made by the manufacturer's firmware.",
					"È il disco stesso a segnalare che il sottosistema di memoria non è più affidabile. Non è una previsione basata su soglie: è una diagnosi fatta dal firmware del costruttore."),
				b.s("Back up immediately and replace. No software fix recovers this condition.",
					"Backup immediato e sostituzione. Nessun intervento software recupera questa condizione."))
		}
		if h.SpareBelowThreshold() {
			add(SevCritical,
				b.s("Spare cells below the safety threshold", "Celle di riserva sotto la soglia di sicurezza"),
				b.s("Every SSD keeps spare cells to replace those that fail. This drive has consumed its reserve below the manufacturer's limit: from here, a failure has no replacement available.",
					"Ogni SSD tiene celle di scorta per rimpiazzare quelle che si guastano. Il disco ha consumato la riserva fin sotto il limite fissato dal costruttore: da qui in avanti un guasto non ha più rimpiazzi disponibili."),
				b.s("Replace the drive. In the meantime, avoid writing to it.",
					"Sostituire il disco. Nel frattempo evitare di scriverci sopra."))
		}
		if h.TemperatureAlarm() {
			add(SevWarn,
				b.s("The drive is raising a temperature alarm", "Il disco segnala allarme di temperatura"),
				b.s("The internal sensor is outside the intended operating range.",
					"Il sensore interno è fuori dall'intervallo di funzionamento previsto."),
				b.s("Check the heatsink and airflow before heat starts shortening the drive's life.",
					"Verificare dissipatore e flusso d'aria prima che il calore riduca la vita del disco."))
		}
		if h.BackupFailed() {
			add(SevWarn,
				b.s("Volatile memory backup device has failed",
					"Dispositivo di salvataggio della memoria volatile guasto"),
				b.s("The circuit meant to flush the cache to persistent storage on power loss is not working: a sudden outage can lose whatever is still cached.",
					"Il circuito che dovrebbe scrivere la cache su memoria permanente in caso di mancanza di corrente non funziona: un'interruzione improvvisa può far perdere i dati ancora in cache."),
				b.s("Do not use this drive for critical data until it is replaced.",
					"Non usare questo disco per dati critici finché non viene sostituito."))
		}
	}
}

// ruleNVMeMediaErrors è la regola che colma il buco lasciato da Windows.
//
// Get-StorageReliabilityCounter non riporta alcun contatore di errore per gli
// NVMe: prima di leggere il disco direttamente, il segnale più importante di
// tutti era semplicemente invisibile.
func ruleNVMeMediaErrors(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.NVMe == nil || d.NVMe.MediaErrors == 0 {
			continue
		}
		n := d.NVMe.MediaErrors
		b.add(Finding{
			Severity: SevCritical,
			Area:     areaDisco(b.l),
			Target:   d.Model,
			Title: b.n(n, "data integrity error", "data integrity errors",
				"errore di integrità dei dati", "errori di integrità dei dati"),
			Detail: b.s(
				"The drive found data that does not match what was written and could not correct it. This is not an early warning: the data is already lost, and the counter tends to grow faster over time.",
				"Il disco ha rilevato dati che non corrispondono a quanto era stato scritto e che non è riuscito a correggere. Non è un'avvisaglia: sono dati già persi, e il contatore tende a crescere sempre più in fretta."),
			Action: b.s(
				"Copy the data today and plan the replacement. Do not be reassured by the drive appearing to work normally.",
				"Copiare i dati oggi e pianificare la sostituzione. Non fidarsi del fatto che il disco sembri funzionare normalmente."),
			Evidence: map[string]string{
				"mediaErrors":     fmt.Sprint(n),
				"errorLogEntries": fmt.Sprint(d.NVMe.ErrorLogEntries),
			},
		})
	}
}

func ruleNVMeAvailableSpare(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		// Il caso "sotto soglia" lo dichiara già il byte di allarme critico:
		// qui interessa avvisare prima che ci si arrivi.
		if d.NVMe == nil || d.NVMe.SpareBelowThreshold() {
			continue
		}
		h := *d.NVMe
		if h.AvailableSparePct == 0 || h.AvailableSparePct >= spareWarnPct {
			continue
		}
		b.add(Finding{
			Severity: SevWarn,
			Area:     areaDisco(b.l),
			Target:   d.Model,
			Title:    b.f("Spare cells at %d%%", "Celle di riserva al %d%%", h.AvailableSparePct),
			Detail: b.f(
				"The spare cells used to replace failed ones are running out. The manufacturer's alarm threshold is %d%%: below that the drive has no replacements left.",
				"Le celle di scorta usate per rimpiazzare quelle guaste stanno finendo. La soglia di allarme del costruttore è al %d%%: sotto quel valore il disco non avrà più rimpiazzi.",
				h.AvailableSpareThreshPct),
			Action: b.s(
				"Plan the replacement before reaching the threshold, not after.",
				"Pianificare la sostituzione prima di raggiungere la soglia, non dopo."),
			Evidence: map[string]string{
				"availableSpare": fmt.Sprint(h.AvailableSparePct),
				"spareThreshold": fmt.Sprint(h.AvailableSpareThreshPct),
			},
		})
	}
}

// ruleNVMeUnsafeShutdowns collega un dato del disco a un sintomo di sistema.
//
// Uno spegnimento anomalo è una mancanza di corrente, un blocco o un riavvio
// forzato mentre il disco stava ancora scrivendo. Non danneggia il disco, ma è
// la causa numero uno di file system corrotti — e quella corruzione compare
// altrove, su un volume, dove nessuno la collega più allo spegnimento.
// Mettere insieme le due cose è esattamente il lavoro che un tecnico fa a
// mente e che nessun programma di diagnostica fa al posto suo.
func ruleNVMeUnsafeShutdowns(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.NVMe == nil {
			continue
		}
		h := *d.NVMe
		if h.PowerCycles == 0 || h.UnsafeShutdowns < unsafeShutdownMinCount {
			continue
		}
		pct := float64(h.UnsafeShutdowns) / float64(h.PowerCycles) * 100

		var sev Severity
		switch {
		case pct >= unsafeShutdownWarnPct:
			sev = SevWarn
		case pct >= unsafeShutdownInfoPct:
			sev = SevInfo
		default:
			continue
		}

		dettaglio := b.f(
			"Out of %d power-ups, %d times the computer shut down while the drive "+
				"was still writing (%.1f%%): power loss, freezes, or holding the "+
				"power button. The drive survives it undamaged, but this is exactly "+
				"how file systems get corrupted.",
			"Su %d accensioni, %d volte il computer si è spento mentre il disco "+
				"stava ancora scrivendo (%.1f%%): mancanza di corrente, blocchi, o "+
				"spegnimento tenendo premuto il pulsante. Il disco regge senza danni, "+
				"ma è così che i file system si corrompono.",
			h.PowerCycles, h.UnsafeShutdowns, pct)

		if corrotti := volumiDaRiparare(s); corrotti != "" {
			dettaglio += b.f(" On this machine the following indeed needs repair: %s.",
				" Su questa macchina risulta infatti da riparare: %s.", corrotti)
		}

		b.add(Finding{
			Severity: sev,
			Area:     areaSistema(b.l),
			Target:   d.Model,
			Title: b.f("%d unsafe shutdowns out of %d power-ups",
				"%d spegnimenti anomali su %d accensioni", h.UnsafeShutdowns, h.PowerCycles),
			Detail: dettaglio,
			Action: b.s(
				"Find the cause of the shutdowns: power supply, overheating, system freezes. Repairing file systems without removing the cause means doing it again and again.",
				"Individuare la causa degli spegnimenti: alimentatore, surriscaldamento, blocchi di sistema. Riparare i file system senza rimuovere la causa significa rifarlo di continuo."),
			Evidence: map[string]string{
				"unsafeShutdowns": fmt.Sprint(h.UnsafeShutdowns),
				"powerCycles":     fmt.Sprint(h.PowerCycles),
			},
		})
	}
}

func volumiDaRiparare(s model.Snapshot) string {
	var out string
	for _, v := range s.Volumes {
		if v.HealthStatus == "" || v.HealthStatus == "Healthy" {
			continue
		}
		if out != "" {
			out += ", "
		}
		out += v.DriveLetter + ":"
	}
	return out
}

// ruleNVMeThermalThrottleTime è la prova, non l'indizio.
//
// La temperatura massima registrata suggerisce che il disco si sia scaldato.
// Questo contatore dice per quanti minuti è rimasto sopra la soglia di
// allarme: è la differenza fra "potrebbe aver rallentato" e "ha rallentato,
// per questo tempo".
func ruleNVMeThermalThrottleTime(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.NVMe == nil {
			continue
		}
		h := *d.NVMe
		if h.CriticalTempTimeMin > 0 {
			b.add(Finding{
				Severity: SevCritical,
				Area:     areaTermica(b.l),
				Target:   d.Model,
				Title: b.n(uint64(h.CriticalTempTimeMin),
					"minute spent beyond the critical temperature",
					"minutes spent beyond the critical temperature",
					"minuto passato oltre la temperatura critica",
					"minuti passati oltre la temperatura critica"),
				Detail: b.s(
					"The drive exceeded the temperature beyond which the manufacturer does not guarantee operation. At that point it throttles performance drastically, and sustained heat shortens the life of the cells.",
					"Il disco ha superato la temperatura oltre la quale il costruttore non ne garantisce il funzionamento. A quel punto riduce drasticamente le prestazioni, e il calore prolungato accorcia la vita delle celle."),
				Action: b.s(
					"Fix the cooling before anything else: a heatsink on the NVMe drive and case ventilation.",
					"Intervenire sul raffreddamento prima di qualsiasi altra cosa: dissipatore sull'NVMe e ventilazione del case."),
				Evidence: map[string]string{"criticalTempTimeMinutes": fmt.Sprint(h.CriticalTempTimeMin)},
			})
			continue
		}
		if h.WarningTempTimeMin > 0 {
			b.add(Finding{
				Severity: SevWarn,
				Area:     areaTermica(b.l),
				Target:   d.Model,
				Title: b.n(uint64(h.WarningTempTimeMin),
					"minute spent above the thermal warning threshold",
					"minutes spent above the thermal warning threshold",
					"minuto passato sopra la soglia di allarme termico",
					"minuti passati sopra la soglia di allarme termico"),
				Detail: b.s(
					"This is not an estimate: the drive counted the minutes it spent too hot. During those minutes it throttled itself, which is why the machine slows down under load while passing every check performed at idle.",
					"Non è una stima: il disco ha contato i minuti in cui è stato troppo caldo. In quei momenti ha ridotto le prestazioni per proteggersi, ed è il motivo per cui la macchina rallenta sotto carico pur risultando a posto in ogni controllo fatto da ferma."),
				Action: b.s(
					"Fit or check the NVMe heatsink and the airflow in the case.",
					"Montare o verificare il dissipatore dell'NVMe e il flusso d'aria nel case."),
				Evidence: map[string]string{"warningTempTimeMinutes": fmt.Sprint(h.WarningTempTimeMin)},
			})
		}
	}
}

// ruleNVMeLifeProjection stima quanto manca, invece di limitarsi a dire quanto
// è stato consumato.
//
// "Vita consumata: 60%" non dice niente di utile da solo: dipende da quanto
// tempo c'è voluto. Sessanta per cento in dieci anni va benissimo, sessanta
// per cento in otto mesi significa che il disco muore l'anno prossimo. Il
// dato che serve al cliente è il secondo, e si ricava dai due che abbiamo.
func ruleNVMeLifeProjection(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.NVMe == nil {
			continue
		}
		h := *d.NVMe
		if h.PercentageUsedPct <= 0 || h.PowerOnHours < lifeProjectionMinHours {
			continue
		}

		totaliPrevisti := h.PowerOnHours * 100 / uint64(h.PercentageUsedPct)
		if totaliPrevisti <= h.PowerOnHours {
			continue
		}
		residue := totaliPrevisti - h.PowerOnHours

		var sev Severity
		switch {
		case residue < lifeWarnHours:
			sev = SevWarn
		case residue < lifeInfoHours:
			sev = SevInfo
		default:
			continue
		}

		b.add(Finding{
			Severity: sev,
			Area:     areaDisco(b.l),
			Target:   d.Model,
			Title: b.f("Estimated life left: about %d hours of use (%d%% already consumed)",
				"Vita residua stimata: circa %d ore di utilizzo (%d%% già consumato)",
				residue, h.PercentageUsedPct),
			Detail: b.f(
				"The drive has consumed %d%% of its life over %d powered hours. At "+
					"the same write rate, the reserve runs out within the time shown. "+
					"The estimate holds as long as usage stays the same: heavier use "+
					"shortens it.",
				"Il disco ha consumato il %d%% della propria vita in %d ore di "+
					"funzionamento. Mantenendo lo stesso ritmo di scrittura, la riserva "+
					"si esaurisce entro il tempo indicato. La stima vale finché l'uso "+
					"resta questo: un utilizzo più intenso la accorcia.",
				h.PercentageUsedPct, h.PowerOnHours),
			Action: b.s(
				"Plan the replacement in advance and verify a working backup exists.",
				"Pianificare la sostituzione con anticipo e verificare che esista un backup funzionante."),
			Evidence: map[string]string{
				"percentageUsed":   fmt.Sprint(h.PercentageUsedPct),
				"powerOnHours":     fmt.Sprint(h.PowerOnHours),
				"terabytesWritten": fmt.Sprintf("%.1f", h.TerabyteScritti()),
			},
		})
	}
}
