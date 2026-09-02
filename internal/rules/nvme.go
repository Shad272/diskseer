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
				Area:     "Disco",
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
				"Il disco è passato in sola lettura",
				"Il disco ha esaurito la propria capacità di scrivere e si è protetto "+
					"bloccando ogni scrittura. I dati presenti sono ancora leggibili, ma "+
					"non è possibile salvarne di nuovi e la condizione non è reversibile.",
				"Copiare subito tutto il contenuto su un altro supporto e sostituire il disco. Non è riparabile.")
		}
		if h.ReliabilityDegraded() {
			add(SevCritical,
				"Il disco dichiara la propria affidabilità compromessa",
				"È il disco stesso a segnalare che il sottosistema di memoria non è più "+
					"affidabile. Non è una previsione basata su soglie: è una diagnosi "+
					"fatta dal firmware del costruttore.",
				"Backup immediato e sostituzione. Nessun intervento software recupera questa condizione.")
		}
		if h.SpareBelowThreshold() {
			add(SevCritical,
				"Celle di riserva sotto la soglia di sicurezza",
				"Ogni SSD tiene celle di scorta per rimpiazzare quelle che si guastano. "+
					"Il disco ha consumato la riserva fin sotto il limite fissato dal "+
					"costruttore: da qui in avanti un guasto non ha più rimpiazzi disponibili.",
				"Sostituire il disco. Nel frattempo evitare di scriverci sopra.")
		}
		if h.TemperatureAlarm() {
			add(SevWarn,
				"Il disco segnala allarme di temperatura",
				"Il sensore interno è fuori dall'intervallo di funzionamento previsto.",
				"Verificare dissipatore e flusso d'aria prima che il calore riduca la vita del disco.")
		}
		if h.BackupFailed() {
			add(SevWarn,
				"Dispositivo di salvataggio della memoria volatile guasto",
				"Il circuito che dovrebbe scrivere la cache su memoria permanente in caso "+
					"di mancanza di corrente non funziona: un'interruzione improvvisa può "+
					"far perdere i dati ancora in cache.",
				"Non usare questo disco per dati critici finché non viene sostituito.")
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
			Area:     "Disco",
			Target:   d.Model,
			Title:    conta(n, "errore di integrità dei dati", "errori di integrità dei dati"),
			Detail: "Il disco ha rilevato dati che non corrispondono a quanto era " +
				"stato scritto e che non è riuscito a correggere. Non è un'avvisaglia: " +
				"sono dati già persi, e il contatore tende a crescere sempre più in fretta.",
			Action: "Copiare i dati oggi e pianificare la sostituzione. Non fidarsi del fatto che il disco sembri funzionare normalmente.",
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
			Area:     "Disco",
			Target:   d.Model,
			Title:    fmt.Sprintf("Celle di riserva al %d%%", h.AvailableSparePct),
			Detail: fmt.Sprintf("Le celle di scorta usate per rimpiazzare quelle guaste "+
				"stanno finendo. La soglia di allarme del costruttore è al %d%%: sotto "+
				"quel valore il disco non avrà più rimpiazzi.", h.AvailableSpareThreshPct),
			Action: "Pianificare la sostituzione prima di raggiungere la soglia, non dopo.",
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

		dettaglio := fmt.Sprintf("Su %d accensioni, %d volte il computer si è spento "+
			"mentre il disco stava ancora scrivendo (%.1f%%): mancanza di corrente, "+
			"blocchi, o spegnimento tenendo premuto il pulsante. Il disco regge senza "+
			"danni, ma è così che i file system si corrompono.",
			h.PowerCycles, h.UnsafeShutdowns, pct)

		if corrotti := volumiDaRiparare(s); corrotti != "" {
			dettaglio += " Su questa macchina risulta infatti da riparare: " + corrotti + "."
		}

		b.add(Finding{
			Severity: sev,
			Area:     "Sistema",
			Target:   d.Model,
			Title:    fmt.Sprintf("%d spegnimenti anomali su %d accensioni", h.UnsafeShutdowns, h.PowerCycles),
			Detail:   dettaglio,
			Action:   "Individuare la causa degli spegnimenti: alimentatore, surriscaldamento, blocchi di sistema. Riparare i file system senza rimuovere la causa significa rifarlo di continuo.",
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
				Area:     "Termica",
				Target:   d.Model,
				Title:    conta(uint64(h.CriticalTempTimeMin), "minuto passato oltre la temperatura critica", "minuti passati oltre la temperatura critica"),
				Detail:   "Il disco ha superato la temperatura oltre la quale il costruttore non ne garantisce il funzionamento. A quel punto riduce drasticamente le prestazioni, e il calore prolungato accorcia la vita delle celle.",
				Action:   "Intervenire sul raffreddamento prima di qualsiasi altra cosa: dissipatore sull'NVMe e ventilazione del case.",
				Evidence: map[string]string{"criticalTempTimeMinutes": fmt.Sprint(h.CriticalTempTimeMin)},
			})
			continue
		}
		if h.WarningTempTimeMin > 0 {
			b.add(Finding{
				Severity: SevWarn,
				Area:     "Termica",
				Target:   d.Model,
				Title:    conta(uint64(h.WarningTempTimeMin), "minuto passato sopra la soglia di allarme termico", "minuti passati sopra la soglia di allarme termico"),
				Detail: "Non è una stima: il disco ha contato i minuti in cui è stato " +
					"troppo caldo. In quei momenti ha ridotto le prestazioni per " +
					"proteggersi, ed è il motivo per cui la macchina rallenta sotto " +
					"carico pur risultando a posto in ogni controllo fatto da ferma.",
				Action:   "Montare o verificare il dissipatore dell'NVMe e il flusso d'aria nel case.",
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
			Area:     "Disco",
			Target:   d.Model,
			Title: fmt.Sprintf("Vita residua stimata: circa %d ore di utilizzo (%d%% già consumato)",
				residue, h.PercentageUsedPct),
			Detail: fmt.Sprintf("Il disco ha consumato il %d%% della propria vita in %d ore "+
				"di funzionamento. Mantenendo lo stesso ritmo di scrittura, la riserva si "+
				"esaurisce entro il tempo indicato. La stima vale finché l'uso resta questo: "+
				"un utilizzo più intenso la accorcia.",
				h.PercentageUsedPct, h.PowerOnHours),
			Action: "Pianificare la sostituzione con anticipo e verificare che esista un backup funzionante.",
			Evidence: map[string]string{
				"percentageUsed":  fmt.Sprint(h.PercentageUsedPct),
				"powerOnHours":    fmt.Sprint(h.PowerOnHours),
				"terabyteScritti": fmt.Sprintf("%.1f", h.TerabyteScritti()),
			},
		})
	}
}
