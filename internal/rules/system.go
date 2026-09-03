package rules

import (
	"fmt"
	"strings"
	"time"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
)

const (
	freeSpaceWarnPct  = 10.0
	freeSpaceCritPct  = 5.0
	freeSpaceCritGB   = 10.0
	thermalWarnC      = 80.0
	thermalCritC      = 90.0
	batteryWarnPct    = 80.0
	batteryCritPct    = 60.0
	batteryCyclesInfo = 500
	uptimeInfoDays    = 14
)

const gb = 1024 * 1024 * 1024

// ruleVolumeFreeSpace: sotto una certa soglia Windows non ha più spazio per
// file di paging, aggiornamenti e file temporanei. Il sintomo che l'utente
// riferisce non è "disco pieno", è "il PC si è rotto".
func ruleVolumeFreeSpace(s model.Snapshot, b *builder) {
	for _, v := range s.Volumes {
		if v.SizeBytes == 0 {
			continue
		}
		pct := v.FreePercent()
		freeGB := float64(v.FreeBytes) / gb
		if pct >= freeSpaceWarnPct {
			continue
		}

		sev := SevWarn
		detail := b.s(
			"Free space is below the safety minimum. Windows uses the system "+
				"volume for temporary files and updates: when it runs out, the "+
				"machine slows down and updates start failing.",
			"Lo spazio libero è sotto il minimo di sicurezza. Windows usa il disco "+
				"di sistema per file temporanei e aggiornamenti: quando finisce, "+
				"rallenta e gli aggiornamenti iniziano a fallire.")
		action := b.s(
			"Free up space: disk cleanup, temporary folders, move personal files to another volume.",
			"Liberare spazio: pulizia disco, cartelle temporanee, spostamento dei file personali su un altro volume.")

		if pct < freeSpaceCritPct || freeGB < freeSpaceCritGB {
			sev = SevCritical
			detail = b.s(
				"Free space is effectively exhausted. At this level the page file "+
					"cannot grow, updates fail, and some programs close without "+
					"explanation. It is one of the most frequent and most "+
					"misdiagnosed causes of \"the computer stopped working\".",
				"Lo spazio libero è praticamente esaurito. A questo livello il file "+
					"di paging non può crescere, gli aggiornamenti falliscono e alcuni "+
					"programmi si chiudono senza spiegazione. È una delle cause più "+
					"frequenti e più fraintese di \"PC che non funziona più\".")
			action = b.s(
				"Free space immediately, before any other diagnosis: many symptoms will disappear on their own.",
				"Liberare spazio subito, prima di qualsiasi altra diagnosi: molti sintomi spariranno da soli.")
		}

		b.add(Finding{
			Severity: sev,
			Area:     areaSpazio(b.l),
			Target:   v.DriveLetter + ":",
			Title: b.f("Only %.1f GB free out of %.0f GB (%.1f%%)",
				"Solo %.1f GB liberi su %.0f GB (%.1f%%)",
				freeGB, float64(v.SizeBytes)/gb, pct),
			Detail: detail,
			Action: action,
			Evidence: map[string]string{
				"freeBytes": fmt.Sprint(v.FreeBytes),
				"sizeBytes": fmt.Sprint(v.SizeBytes),
			},
		})
	}
}

// repairAction traduce lo stato operativo di Windows nell'intervento giusto.
//
// Windows ha due campi: HealthStatus dice *che* c'è un problema, e vale
// "Warning" tanto per un graffio quanto per una corruzione seria.
// OperationalStatus dice *quale* riparazione serve. Guardare solo il primo,
// come fa quasi ogni tool, significa allarmare senza saper dire cosa fare.
func repairAction(l i18n.Lingua, operational string) (Severity, string, string) {
	switch operational {
	case "Scan Needed":
		return SevWarn,
			l.S("Windows detected anomalies and is asking for a volume scan.",
				"Windows ha rilevato anomalie e chiede una scansione del volume."),
			l.S("Run a read-only scan: chkdsk <letter>: /scan",
				"Eseguire una scansione di sola lettura: chkdsk <letter>: /scan")
	case "Spot Fix Needed":
		return SevWarn,
			l.S("Windows found localised errors, repairable in seconds without scanning the whole volume.",
				"Windows ha individuato errori circoscritti, riparabili in pochi secondi senza analizzare tutto il volume."),
			l.S("Run the targeted repair: chkdsk <letter>: /spotfix",
				"Eseguire la riparazione mirata: chkdsk <letter>: /spotfix")
	case "Full Repair Needed":
		return SevCritical,
			l.S("This is the highest repair level Windows defines: the corruption is not localised and requires a full check of the volume.",
				"È il livello di riparazione più alto previsto da Windows: la corruzione non è circoscritta e richiede un controllo completo del volume."),
			l.S("Copy the important data elsewhere BEFORE repairing, then run: chkdsk <letter>: /f — repairing a corrupted volume sometimes recovers and sometimes makes it worse, so the copy comes first.",
				"Copiare altrove i dati importanti PRIMA di riparare, poi eseguire: chkdsk <letter>: /f — una riparazione su un volume corrotto a volte recupera e a volte peggiora, quindi la copia viene prima.")
	default:
		return SevWarn,
			l.S("The file system shows anomalies Windows did not classify.",
				"Il file system presenta anomalie che Windows non ha classificato."),
			l.S("Scan the volume after putting the data somewhere safe.",
				"Eseguire una scansione del volume dopo aver messo al sicuro i dati.")
	}
}

func ruleVolumeHealth(s model.Snapshot, b *builder) {
	for _, v := range s.Volumes {
		if v.HealthStatus == "" || v.HealthStatus == "Healthy" {
			continue
		}
		sev, detail, action := repairAction(b.l, v.OperationalStatus)

		title := b.f("%s file system needs repair", "File system %s da riparare", v.FileSystem)
		if v.OperationalStatus != "" {
			title = b.f("%s file system: %s", "File system %s: %s", v.FileSystem, v.OperationalStatus)
		}

		// exFAT e FAT32 non tengono un giornale delle operazioni: se una
		// scrittura si interrompe, nessuno la ripara da solo. È il motivo per
		// cui i dischi esterni si corrompono e quelli di sistema quasi mai.
		if v.FileSystem == "exFAT" || strings.HasPrefix(v.FileSystem, "FAT") {
			detail += b.f(
				" On %s this typically follows a sudden power loss or an unsafe "+
					"removal: unlike NTFS, this file system keeps no journal of "+
					"pending operations and cannot put itself back in order.",
				" Su %s succede tipicamente dopo uno spegnimento improvviso o una "+
					"rimozione non sicura: a differenza di NTFS, questo file system "+
					"non tiene un giornale delle operazioni e non sa rimettersi in "+
					"ordine da solo.",
				v.FileSystem)
		}

		b.add(Finding{
			Severity: sev,
			Area:     areaVolume(b.l),
			Target:   v.DriveLetter + ":",
			Title:    title,
			Detail:   detail,
			Action:   strings.ReplaceAll(action, "<letter>", v.DriveLetter),
			Evidence: map[string]string{
				"healthStatus":      v.HealthStatus,
				"operationalStatus": v.OperationalStatus,
				"fileSystem":        v.FileSystem,
			},
		})
	}
}

func ruleThermalHigh(s model.Snapshot, b *builder) {
	for _, t := range s.Thermals {
		if t.Celsius < thermalWarnC {
			continue
		}
		sev := SevWarn
		if t.Celsius >= thermalCritC {
			sev = SevCritical
		}
		b.add(Finding{
			Severity: sev,
			Area:     areaTermica(b.l),
			Target:   t.Name,
			Title:    b.f("Thermal zone at %.0f °C", "Zona termica a %.0f °C", t.Celsius),
			Detail: b.s(
				"At these temperatures the processor lowers its clock to protect itself: the machine feels slow exactly when power is needed, yet looks perfectly healthy in any idle test.",
				"A queste temperature il processore riduce la frequenza per proteggersi: il PC risulta lento proprio quando serve potenza, ma nei test a riposo sembra perfettamente sano."),
			Action: b.s(
				"Clean fans and heatsinks, check the thermal paste and the airflow.",
				"Pulizia di ventole e dissipatori, verifica della pasta termica e del flusso d'aria."),
			Evidence: map[string]string{"celsius": fmt.Sprintf("%.1f", t.Celsius)},
		})
	}
}

func ruleBatteryWear(s model.Snapshot, b *builder) {
	if s.Battery == nil {
		return
	}
	bat := *s.Battery
	if h, ok := bat.HealthPercent(); ok && h < batteryWarnPct {
		sev := SevWarn
		if h < batteryCritPct {
			sev = SevCritical
		}
		b.add(Finding{
			Severity: sev,
			Area:     areaBatteria(b.l),
			Target:   bat.Name,
			Title: b.f("Battery at %.0f%% of its original capacity",
				"Batteria al %.0f%% della capacità originale", h),
			Detail: b.s(
				"The battery holds far less charge than when new. Runtime drops, and sudden voltage sags can switch the laptop off without warning.",
				"La batteria trattiene molta meno carica di quando era nuova. L'autonomia percepita cala e i cali di tensione improvvisi possono spegnere il portatile senza preavviso."),
			Action: b.s("Consider replacing the battery.", "Valutare la sostituzione della batteria."),
			Evidence: map[string]string{
				"designCapacity": fmt.Sprint(*bat.DesignCapacity),
				"fullCapacity":   fmt.Sprint(*bat.FullCapacity),
			},
		})
	}
	if bat.CycleCount != nil && *bat.CycleCount > batteryCyclesInfo {
		b.add(Finding{
			Severity: SevInfo,
			Area:     areaBatteria(b.l),
			Target:   bat.Name,
			Title: b.n(uint64(*bat.CycleCount), "charge cycle", "charge cycles",
				"ciclo di ricarica", "cicli di ricarica"),
			Detail: b.s(
				"Past 500 cycles most manufacturers consider a battery to be at the end of its useful life.",
				"Oltre i 500 cicli la maggior parte delle batterie è considerata a fine vita utile dal costruttore."),
			Action: b.s(
				"No action needed now: a figure useful for estimating how much longer it will last.",
				"Nessuna azione immediata: dato utile per stimare quanto durerà ancora."),
			Evidence: map[string]string{"cycleCount": fmt.Sprint(*bat.CycleCount)},
		})
	}
}

func ruleUptime(s model.Snapshot, b *builder) {
	if s.System.LastBoot.IsZero() {
		return
	}
	d := time.Since(s.System.LastBoot)
	if d < uptimeInfoDays*24*time.Hour {
		return
	}
	b.add(Finding{
		Severity: SevInfo,
		Area:     areaSistema(b.l),
		Target:   "Windows",
		Title: b.f("Running for %d days without a restart",
			"Acceso da %d giorni senza riavvio", int(d.Hours()/24)),
		Detail: b.s(
			"Many updates only finish applying after a restart, and some slowdowns clear up on their own once the machine reboots.",
			"Molti aggiornamenti si completano solo al riavvio, e alcuni rallentamenti spariscono da soli riavviando."),
		Action: b.s(
			"Restart before investigating further. It is trivial, and it is the first thing to rule out.",
			"Riavviare prima di indagare oltre. È banale, ed è la prima cosa da escludere."),
		Evidence: map[string]string{"lastBoot": s.System.LastBoot.Format(time.RFC3339)},
	})
}

// ruleNotElevated non descrive un guasto: descrive un limite della diagnosi.
// Dire cosa NON si è potuto controllare è parte dell'onestà del referto.
//
// L'avviso però deve restare vero. Da quando i dischi NVMe vengono interrogati
// direttamente, senza passare da Windows, per quelli i dati ci sono anche
// senza privilegi: dichiararli mancanti sarebbe falso quanto inventarli. Qui
// si nominano solo i dischi rimasti effettivamente al buio, e se non ce ne
// sono l'avviso non compare affatto.
func ruleNotElevated(s model.Snapshot, b *builder) {
	if s.Elevated {
		return
	}

	var alBuio []string
	for _, d := range s.Disks {
		if d.NVMe == nil && d.TemperatureC == nil && d.PowerOnHours == nil {
			alBuio = append(alBuio, d.Model)
		}
	}
	if len(alBuio) == 0 {
		return
	}

	b.add(Finding{
		Severity:  SevInfo,
		Area:      areaDiagnosi(b.l),
		SuiLimiti: true,
		Target:    "diskseer",
		Title: b.f("Partial analysis: %s could not be examined without privileges",
			"Analisi parziale: %s non analizzabili senza privilegi",
			b.n(uint64(len(alBuio)), "drive", "drives", "disco", "dischi")),
		Detail: b.f(
			"Without administrator privileges Windows does not expose the internal "+
				"health of drives attached over SATA and USB: temperatures, powered "+
				"hours and error counters. Left out of this diagnosis: %s. A drive "+
				"close to failure may have gone undetected.",
			"Senza privilegi di amministratore Windows non espone lo stato di "+
				"salute interno dei dischi collegati via SATA e USB: temperature, "+
				"ore di funzionamento e contatori di errore. Restano fuori dalla "+
				"diagnosi: %s. Un disco prossimo al guasto potrebbe non essere stato "+
				"rilevato.",
			strings.Join(alBuio, ", ")),
		Action: b.s(
			"Run diskseer again from a terminal opened as administrator for the complete diagnosis.",
			"Rilanciare diskseer da un terminale aperto come amministratore per la diagnosi completa."),
		Evidence: map[string]string{
			"elevated":      "false",
			"drivesNotRead": fmt.Sprint(len(alBuio)),
		},
	})
}
