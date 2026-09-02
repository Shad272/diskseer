package rules

import (
	"fmt"
	"strings"
	"time"

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
		detail := "Lo spazio libero è sotto il minimo di sicurezza. Windows " +
			"usa il disco di sistema per file temporanei e aggiornamenti: " +
			"quando finisce, rallenta e gli aggiornamenti iniziano a fallire."
		action := "Liberare spazio: pulizia disco, cartelle temporanee, spostamento dei file personali su un altro volume."
		if pct < freeSpaceCritPct || freeGB < freeSpaceCritGB {
			sev = SevCritical
			detail = "Lo spazio libero è praticamente esaurito. A questo livello " +
				"il file di paging non può crescere, gli aggiornamenti falliscono " +
				"e alcuni programmi si chiudono senza spiegazione. È una delle " +
				"cause più frequenti e più fraintese di \"PC che non funziona più\"."
			action = "Liberare spazio subito, prima di qualsiasi altra diagnosi: molti sintomi spariranno da soli."
		}
		b.add(Finding{
			Severity: sev,
			Area:     "Spazio",
			Target:   v.DriveLetter + ":",
			Title:    fmt.Sprintf("Solo %.1f GB liberi su %.0f GB (%.1f%%)", freeGB, float64(v.SizeBytes)/gb, pct),
			Detail:   detail,
			Action:   action,
			Evidence: map[string]string{
				"freeBytes": fmt.Sprint(v.FreeBytes),
				"sizeBytes": fmt.Sprint(v.SizeBytes),
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
			Area:     "Termica",
			Target:   t.Name,
			Title:    fmt.Sprintf("Zona termica a %.0f C", t.Celsius),
			Detail: "A queste temperature il processore riduce la frequenza per " +
				"proteggersi: il PC risulta lento proprio quando serve potenza, " +
				"ma nei test a riposo sembra perfettamente sano.",
			Action:   "Pulizia di ventole e dissipatori, verifica della pasta termica e del flusso d'aria.",
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
			Area:     "Batteria",
			Target:   bat.Name,
			Title:    fmt.Sprintf("Batteria al %.0f%% della capacità originale", h),
			Detail:   "La batteria trattiene molta meno carica di quando era nuova. L'autonomia percepita cala e i cali di tensione improvvisi possono spegnere il portatile senza preavviso.",
			Action:   "Valutare la sostituzione della batteria.",
			Evidence: map[string]string{
				"designCapacity": fmt.Sprint(*bat.DesignCapacity),
				"fullCapacity":   fmt.Sprint(*bat.FullCapacity),
			},
		})
	}
	if bat.CycleCount != nil && *bat.CycleCount > batteryCyclesInfo {
		b.add(Finding{
			Severity: SevInfo,
			Area:     "Batteria",
			Target:   bat.Name,
			Title:    fmt.Sprintf("%d cicli di ricarica", *bat.CycleCount),
			Detail:   "Oltre i 500 cicli la maggior parte delle batterie è considerata a fine vita utile dal costruttore.",
			Action:   "Nessuna azione immediata: dato utile per stimare quanto durerà ancora.",
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
		Area:     "Sistema",
		Target:   "Windows",
		Title:    fmt.Sprintf("Acceso da %d giorni senza riavvio", int(d.Hours()/24)),
		Detail:   "Molti aggiornamenti si completano solo al riavvio, e alcuni rallentamenti spariscono da soli riavviando.",
		Action:   "Riavviare prima di indagare oltre. È banale, ed è la prima cosa da escludere.",
		Evidence: map[string]string{"lastBoot": s.System.LastBoot.Format(time.RFC3339)},
	})
}

// repairAction traduce lo stato operativo di Windows nell'intervento giusto.
//
// Windows ha due campi: HealthStatus dice *che* c'è un problema, e vale
// "Warning" tanto per un graffio quanto per una corruzione seria.
// OperationalStatus dice *quale* riparazione serve. Guardare solo il primo,
// come fa quasi ogni tool, significa allarmare senza saper dire cosa fare.
func repairAction(operational string) (Severity, string, string) {
	switch operational {
	case "Scan Needed":
		return SevWarn,
			"Windows ha rilevato anomalie e chiede una scansione del volume.",
			"Eseguire una scansione di sola lettura: chkdsk <lettera>: /scan"
	case "Spot Fix Needed":
		return SevWarn,
			"Windows ha individuato errori circoscritti, riparabili in pochi secondi senza analizzare tutto il volume.",
			"Eseguire la riparazione mirata: chkdsk <lettera>: /spotfix"
	case "Full Repair Needed":
		return SevCritical,
			"È il livello di riparazione più alto previsto da Windows: la corruzione non è circoscritta e richiede un controllo completo del volume.",
			"Copiare altrove i dati importanti PRIMA di riparare, poi eseguire: chkdsk <lettera>: /f — una riparazione su un volume corrotto a volte recupera e a volte peggiora, quindi la copia viene prima."
	default:
		return SevWarn,
			"Il file system presenta anomalie che Windows non ha classificato.",
			"Eseguire una scansione del volume dopo aver messo al sicuro i dati."
	}
}

func ruleVolumeHealth(s model.Snapshot, b *builder) {
	for _, v := range s.Volumes {
		if v.HealthStatus == "" || v.HealthStatus == "Healthy" {
			continue
		}
		sev, detail, action := repairAction(v.OperationalStatus)

		title := fmt.Sprintf("File system %s da riparare", v.FileSystem)
		if v.OperationalStatus != "" {
			title = fmt.Sprintf("File system %s: %s", v.FileSystem, v.OperationalStatus)
		}

		// exFAT e FAT32 non tengono un giornale delle operazioni: se una
		// scrittura si interrompe, nessuno la ripara da solo. È il motivo per
		// cui i dischi esterni si corrompono e quelli di sistema quasi mai.
		if v.FileSystem == "exFAT" || strings.HasPrefix(v.FileSystem, "FAT") {
			detail += " Su " + v.FileSystem + " succede tipicamente dopo uno " +
				"spegnimento improvviso o una rimozione non sicura: a differenza " +
				"di NTFS, questo file system non tiene un giornale delle " +
				"operazioni e non sa rimettersi in ordine da solo."
		}

		b.add(Finding{
			Severity: sev,
			Area:     "Volume",
			Target:   v.DriveLetter + ":",
			Title:    title,
			Detail:   detail,
			Action:   strings.ReplaceAll(action, "<lettera>", v.DriveLetter),
			Evidence: map[string]string{
				"healthStatus":      v.HealthStatus,
				"operationalStatus": v.OperationalStatus,
				"fileSystem":        v.FileSystem,
			},
		})
	}
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
		Severity: SevInfo,
		Area:     "Diagnosi",
		Target:   "diskseer",
		Title: fmt.Sprintf("Analisi parziale: %s non analizzabili senza privilegi",
			pluralizeDischi(len(alBuio))),
		Detail: fmt.Sprintf("Senza privilegi di amministratore Windows non espone lo "+
			"stato di salute interno dei dischi collegati via SATA e USB: temperature, "+
			"ore di funzionamento e contatori di errore. Restano fuori dalla diagnosi: "+
			"%s. Un disco prossimo al guasto potrebbe non essere stato rilevato.",
			strings.Join(alBuio, ", ")),
		Action: "Rilanciare diskseer da un terminale aperto come amministratore per la diagnosi completa.",
		Evidence: map[string]string{
			"elevated":       "false",
			"dischiNonLetti": fmt.Sprint(len(alBuio)),
		},
	})
}

func pluralizeDischi(n int) string {
	if n == 1 {
		return "1 disco"
	}
	return fmt.Sprintf("%d dischi", n)
}
