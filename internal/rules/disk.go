package rules

import (
	"fmt"

	"github.com/shad272/diskseer/internal/model"
)

// Soglie. Sono il cuore del progetto e vanno difese con dati reali, non
// copiate da un forum: ogni volta che una diagnosi si rivela sbagliata sul
// campo, la correzione si fa qui.
const (
	hddTempWarnC = 50 // i meccanici soffrono molto prima degli SSD
	hddTempCritC = 60
	ssdTempWarnC = 70
	ssdTempCritC = 80
	wearWarnPct  = 80
	wearCritPct  = 90
	hddOldHours  = 30000 // ~3,4 anni acceso in continuazione
)

func isHDD(d model.Disk) bool { return d.MediaType == "HDD" }
func isSSD(d model.Disk) bool { return d.MediaType == "SSD" }

// ruleSystemDiskIsHDD è la regola più redditizia di tutte: nella grande
// maggioranza dei "PC lento" portati in assistenza la causa è questa, e
// nessun programma di diagnostica la dice in chiaro.
func ruleSystemDiskIsHDD(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if !d.IsSystemDisk || !isHDD(d) {
			continue
		}
		b.add(Finding{
			Severity: SevWarn,
			Area:     areaPrestazioni(b.l),
			Target:   d.Model,
			Title: b.s("The operating system runs on a mechanical drive",
				"Il sistema operativo gira su un disco meccanico"),
			Detail: b.s(
				"Windows is installed on a spinning hard drive. A mechanical disk "+
					"sustains roughly 100 operations per second, an SSD tens of "+
					"thousands: this is the single most common cause of a computer "+
					"that takes minutes to boot and freezes when opening programs. "+
					"No software fix recovers that difference.",
				"Windows è installato su un hard disk tradizionale. Un disco "+
					"meccanico regge circa 100 operazioni al secondo, un SSD decine "+
					"di migliaia: è la causa più comune di un PC che impiega minuti "+
					"ad avviarsi e si blocca aprendo i programmi. Nessun intervento "+
					"software recupera questa differenza."),
			Action: b.s(
				"Replace it with an SSD and migrate the system. On this machine it is the single highest-value change you can make.",
				"Sostituire con un SSD e migrare il sistema. È l'intervento con il rapporto costo/beneficio più alto su questa macchina."),
			Evidence: map[string]string{"mediaType": d.MediaType, "busType": d.BusType},
		})
	}
}

func ruleDiskHealthStatus(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.HealthStatus == "" || d.HealthStatus == "Healthy" {
			continue
		}
		sev := SevWarn
		if d.HealthStatus == "Unhealthy" {
			sev = SevCritical
		}
		b.add(Finding{
			Severity: sev,
			Area:     areaDisco(b.l),
			Target:   d.Model,
			Title: b.f("Windows reports this drive as %q", "Windows segnala il disco come %q",
				d.HealthStatus),
			Detail: b.s(
				"The system has downgraded the drive's health status. Windows only "+
					"raises this after it has already seen errors — it is not a "+
					"precautionary guess.",
				"Il sistema ha declassato lo stato di salute del disco. È una "+
					"segnalazione che Windows alza solo quando ha già visto errori, "+
					"non una previsione prudenziale."),
			Action: b.s(
				"Copy the data now, then investigate with the full S.M.A.R.T. counters.",
				"Fare subito una copia dei dati, poi approfondire con i contatori SMART completi."),
			Evidence: map[string]string{"healthStatus": d.HealthStatus},
		})
	}
}

// ruleDiskUncorrectedErrors tace quando gli attributi SMART sono disponibili:
// le regole SMART dicono la stessa cosa in modo più preciso, distinguendo i
// settori già sostituiti da quelli illeggibili adesso. Due verdetti sullo
// stesso guasto, con parole diverse, fanno dubitare di entrambi.
func ruleDiskUncorrectedErrors(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.SMART != nil {
			continue
		}
		var n uint64
		var known bool
		if d.ReadErrorsUncorr != nil {
			n += *d.ReadErrorsUncorr
			known = true
		}
		if d.WriteErrUncorr != nil {
			n += *d.WriteErrUncorr
			known = true
		}
		if !known || n == 0 {
			continue
		}
		b.add(Finding{
			Severity: SevCritical,
			Area:     areaDisco(b.l),
			Target:   d.Model,
			Title: b.n(n,
				"error the drive could not correct", "errors the drive could not correct",
				"errore che il disco non è riuscito a correggere",
				"errori che il disco non è riuscito a correggere"),
			Detail: b.s(
				"Every uncorrected error is a block of data the drive tried to "+
					"re-read and failed to recover. This is not an early warning: it "+
					"is a failure already in progress, and it tends to accelerate.",
				"Ogni errore non corretto è un blocco di dati che il disco ha provato "+
					"a rileggere e non ha recuperato. Non è un'avvisaglia: è un guasto "+
					"già in corso, e tende ad accelerare."),
			Action: b.s(
				"Copy the data today, before anything else. Do not defragment and do not run long scans: they add stress to a drive that is already failing.",
				"Copiare i dati oggi, prima di qualsiasi altra operazione. Non deframmentare e non eseguire scansioni lunghe: aggiungono sollecitazione a un disco che sta cedendo."),
			Evidence: map[string]string{"uncorrectedTotal": fmt.Sprint(n)},
		})
	}
}

func ruleDiskWear(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.WearPercent == nil {
			continue
		}
		w := *d.WearPercent
		switch {
		case w >= wearCritPct:
			b.add(Finding{
				Severity: SevCritical,
				Area:     areaDisco(b.l),
				Target:   d.Model,
				Title:    b.f("SSD at %d%% of its rated write life", "SSD al %d%% del consumo previsto", w),
				Detail: b.s(
					"The memory cells have nearly exhausted their guaranteed write cycles. Past this point many SSDs switch to read-only without warning.",
					"Le celle di memoria hanno quasi esaurito i cicli di scrittura garantiti. Oltre questa soglia molti SSD passano in sola lettura senza preavviso."),
				Action: b.s(
					"Plan the replacement now and verify the backups actually work.",
					"Pianificare la sostituzione adesso e verificare che i backup siano funzionanti."),
				Evidence: map[string]string{"wearPercent": fmt.Sprint(w)},
			})
		case w >= wearWarnPct:
			b.add(Finding{
				Severity: SevWarn,
				Area:     areaDisco(b.l),
				Target:   d.Model,
				Title:    b.f("SSD at %d%% of its rated write life", "SSD al %d%% del consumo previsto", w),
				Detail: b.s(
					"Wear is advanced but the drive is still within specification.",
					"L'usura è avanzata ma il disco lavora ancora nei parametri."),
				Action: b.s(
					"Keep it monitored and avoid write-heavy workloads on it.",
					"Tenerlo monitorato e non usarlo per carichi con molte scritture."),
				Evidence: map[string]string{"wearPercent": fmt.Sprint(w)},
			})
		}
	}
}

func ruleDiskTemperature(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.TemperatureC == nil {
			continue
		}
		t := *d.TemperatureC
		warn, crit := ssdTempWarnC, ssdTempCritC
		if isHDD(d) {
			warn, crit = hddTempWarnC, hddTempCritC
		}
		if t < warn {
			continue
		}
		sev := SevWarn
		if t >= crit {
			sev = SevCritical
		}
		b.add(Finding{
			Severity: sev,
			Area:     areaTermica(b.l),
			Target:   d.Model,
			Title:    b.f("Drive at %d °C", "Disco a %d °C", t),
			Detail: b.s(
				"Heat shortens a drive's life and, on SSDs, triggers automatic performance throttling.",
				"Il calore accorcia la vita del disco e, sugli SSD, fa intervenire la riduzione automatica delle prestazioni."),
			Action: b.s(
				"Check airflow inside the case and where the drive sits. On NVMe drives, confirm the heatsink is present and making contact.",
				"Verificare il flusso d'aria nel case e la posizione del disco. Sugli NVMe controllare che il dissipatore sia presente e a contatto."),
			Evidence: map[string]string{"temperatureC": fmt.Sprint(t)},
		})
	}
}

func ruleDiskAge(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.PowerOnHours == nil || !isHDD(d) || *d.PowerOnHours < hddOldHours {
			continue
		}
		h := *d.PowerOnHours
		b.add(Finding{
			Severity: SevInfo,
			Area:     areaDisco(b.l),
			Target:   d.Model,
			Title: b.f("Powered on for %d hours (about %d years)",
				"Disco acceso da %d ore (circa %d anni)", h, h/8760),
			Detail: b.s(
				"The drive works, but it has outlived the typical service life of a mechanical disk. That is not a fault — it is a risk figure.",
				"Il disco funziona ma ha superato la vita utile tipica di un meccanico. Non è un guasto, è un dato di rischio."),
			Action: b.s(
				"Do not let it hold the only copy of anything important.",
				"Non affidargli l'unica copia di dati importanti."),
			Evidence: map[string]string{"powerOnHours": fmt.Sprint(h)},
		})
	}
}

const ssdPeakTempWarnC = 75

// ruleDiskPeakTemperature guarda la temperatura massima mai registrata, non
// quella attuale.
//
// È la regola che smaschera il caso più frustrante dell'assistenza: il cliente
// dice che il PC rallenta sotto carico, il tecnico lo controlla da fermo,
// tutto risulta a posto e il problema viene archiviato come "impressione
// dell'utente". Il picco storico è l'unica traccia che resta di un
// surriscaldamento che avviene solo quando la macchina lavora davvero.
func ruleDiskPeakTemperature(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.TemperatureMaxC == nil || !isSSD(d) {
			continue
		}
		peak := *d.TemperatureMaxC
		if peak < ssdPeakTempWarnC {
			continue
		}
		now := b.s("unknown", "sconosciuta")
		if d.TemperatureC != nil {
			now = fmt.Sprintf("%d °C", *d.TemperatureC)
		}
		b.add(Finding{
			Severity: SevWarn,
			Area:     areaTermica(b.l),
			Target:   d.Model,
			Title: b.f("All-time peak of %d °C (now %s)",
				"Picco storico di %d °C (ora %s)", peak, now),
			Detail: b.s(
				"The drive is cool right now, but it has previously reached a "+
					"temperature at which it throttles itself to avoid damage. That "+
					"happens under load: large file copies, updates, games. A check "+
					"performed on an idle machine never sees it, which is why this "+
					"kind of slowdown is almost always dismissed as the user "+
					"imagining things.",
				"Il disco è fresco adesso ma in passato ha raggiunto una temperatura "+
					"alla quale riduce da solo le prestazioni per proteggersi. Succede "+
					"sotto carico: copie di file grandi, aggiornamenti, giochi. Un "+
					"controllo fatto a macchina ferma non lo rileva, ed è il motivo per "+
					"cui questo tipo di rallentamento viene quasi sempre archiviato "+
					"come impressione dell'utente."),
			Action: b.s(
				"Check the NVMe heatsink and the airflow in the case. A heatsink costing a few euros removes the problem.",
				"Verificare il dissipatore dell'NVMe e il flusso d'aria nel case. Un dissipatore da pochi euro elimina il problema."),
			Evidence: map[string]string{"temperatureMaxC": fmt.Sprint(peak)},
		})
	}
}

const (
	startStopRatedCycles = 45000 // limite tipico dichiarato dai costruttori
	startStopPerHourWarn = 3.0
	startStopMinHours    = 100 // sotto questa soglia il rapporto è rumore
)

// ruleDiskStartStopCycles riconosce il parcheggio aggressivo delle testine.
//
// Molti dischi da portatile hanno un risparmio energetico che ferma le testine
// dopo pochi secondi di inattività. Ogni parcheggio è usura meccanica, e le
// unità sono garantite per un numero finito di cicli: un disco che ne accumula
// parecchi all'ora esaurisce quel budget in una frazione della vita prevista,
// pur risultando perfettamente sano a ogni controllo SMART.
func ruleDiskStartStopCycles(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		// L'attributo 193 conta i parcheggi con precisione: se c'è, se ne
		// occupa ruleSMARTLoadCycles e questa approssimazione va tolta di mezzo.
		if d.SMART != nil {
			if _, ok := d.SMART.Raw(model.SMARTLoadCycles); ok {
				continue
			}
		}
		if d.StartStopCycles == nil || !isHDD(d) {
			continue
		}
		cycles := *d.StartStopCycles

		if cycles >= startStopRatedCycles {
			b.add(Finding{
				Severity: SevWarn,
				Area:     areaDisco(b.l),
				Target:   d.Model,
				Title: b.f("%d stop cycles: close to the design limit",
					"%d cicli di arresto: vicino al limite di progetto", cycles),
				Detail: b.s(
					"The drive is approaching the number of mechanical cycles it was built for. From here the failure risk grows regardless of every other indicator.",
					"Il disco si è avvicinato al numero di cicli meccanici per cui è stato costruito. Da qui in avanti il rischio di guasto cresce indipendentemente dagli altri indicatori."),
				Action: b.s(
					"Do not trust it with data that has no second copy.",
					"Non affidargli dati di cui non esiste una seconda copia."),
				Evidence: map[string]string{"startStopCycles": fmt.Sprint(cycles)},
			})
			continue
		}

		if d.PowerOnHours == nil || *d.PowerOnHours < startStopMinHours {
			continue
		}
		perHour := float64(cycles) / float64(*d.PowerOnHours)
		if perHour < startStopPerHourWarn {
			continue
		}
		b.add(Finding{
			Severity: SevInfo,
			Area:     areaDisco(b.l),
			Target:   d.Model,
			Title: b.f("Heads parked %.1f times per hour",
				"Testine parcheggiate %.1f volte l'ora", perHour),
			Detail: b.f(
				"%d stop cycles over %d powered hours. Power saving parks the heads "+
					"too often: at this rate the drive reaches its mechanical design "+
					"limit long before the end of its useful life, while passing every "+
					"health check along the way.",
				"%d cicli di arresto in %d ore di funzionamento. Il risparmio "+
					"energetico ferma le testine troppo spesso: a questo ritmo il disco "+
					"raggiunge il limite meccanico di progetto molto prima della fine "+
					"della sua vita utile, pur restando sano a ogni controllo.",
				cycles, *d.PowerOnHours),
			Action: b.s(
				"Reduce the drive's power-saving aggressiveness (APM setting) to extend its life.",
				"Ridurre l'aggressività del risparmio energetico del disco (impostazione APM) per allungarne la vita."),
			Evidence: map[string]string{
				"startStopCycles": fmt.Sprint(cycles),
				"powerOnHours":    fmt.Sprint(*d.PowerOnHours),
			},
		})
	}
}
