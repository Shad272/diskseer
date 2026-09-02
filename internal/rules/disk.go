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
			Area:     "Prestazioni",
			Target:   d.Model,
			Title:    "Il sistema operativo gira su un disco meccanico",
			Detail: "Windows è installato su un hard disk tradizionale. " +
				"Un disco meccanico regge circa 100 operazioni al secondo, un SSD " +
				"decine di migliaia: è la causa più comune di un PC che impiega " +
				"minuti ad avviarsi e si blocca aprendo i programmi. Nessun " +
				"intervento software recupera questa differenza.",
			Action:   "Sostituire con un SSD e migrare il sistema. È l'intervento con il rapporto costo/beneficio più alto su questa macchina.",
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
			Area:     "Disco",
			Target:   d.Model,
			Title:    fmt.Sprintf("Windows segnala il disco come %q", d.HealthStatus),
			Detail: "Il sistema ha declassato lo stato di salute del disco. " +
				"È una segnalazione che Windows alza solo quando ha già visto " +
				"errori, non una previsione prudenziale.",
			Action:   "Fare subito una copia dei dati, poi approfondire con i contatori SMART completi.",
			Evidence: map[string]string{"healthStatus": d.HealthStatus},
		})
	}
}

// ruleDiskUncorrectedErrors: gli errori NON corretti sono la spia più seria.
// Gli errori corretti sono normali e non allarmano nessuno; quelli che il
// disco non è riuscito a correggere significano dati già persi.
// Quando gli attributi SMART sono disponibili, questa regola tace: le regole
// SMART dicono la stessa cosa in modo più preciso, distinguendo i settori già
// sostituiti da quelli illeggibili adesso. Due verdetti sullo stesso guasto,
// con parole diverse, fanno dubitare di entrambi.
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
			Area:     "Disco",
			Target:   d.Model,
			Title:    conta(n, "errore che il disco non è riuscito a correggere", "errori che il disco non è riuscito a correggere"),
			Detail: "Ogni errore non corretto è un blocco di dati che il disco " +
				"ha provato a rileggere e non ha recuperato. Non è un'avvisaglia: " +
				"è un guasto già in corso, e tende ad accelerare.",
			Action:   "Copiare i dati oggi, prima di qualsiasi altra operazione. Non deframmentare e non eseguire scansioni lunghe: aggiungono sollecitazione a un disco che sta cedendo.",
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
				Area:     "Disco",
				Target:   d.Model,
				Title:    fmt.Sprintf("SSD al %d%% del consumo previsto", w),
				Detail:   "Le celle di memoria hanno quasi esaurito i cicli di scrittura garantiti. Oltre questa soglia molti SSD passano in sola lettura senza preavviso.",
				Action:   "Pianificare la sostituzione adesso e verificare che i backup siano funzionanti.",
				Evidence: map[string]string{"wearPercent": fmt.Sprint(w)},
			})
		case w >= wearWarnPct:
			b.add(Finding{
				Severity: SevWarn,
				Area:     "Disco",
				Target:   d.Model,
				Title:    fmt.Sprintf("SSD al %d%% del consumo previsto", w),
				Detail:   "L'usura è avanzata ma il disco lavora ancora nei parametri.",
				Action:   "Tenerlo monitorato e non usarlo per carichi con molte scritture.",
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
			Area:     "Termica",
			Target:   d.Model,
			Title:    fmt.Sprintf("Disco a %d C", t),
			Detail: "Il calore accorcia la vita del disco e, sugli SSD, fa " +
				"intervenire la riduzione automatica delle prestazioni.",
			Action:   "Verificare il flusso d'aria nel case e la posizione del disco. Sugli NVMe controllare che il dissipatore sia presente e a contatto.",
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
			Area:     "Disco",
			Target:   d.Model,
			Title:    fmt.Sprintf("Disco acceso da %d ore (circa %d anni)", h, h/8760),
			Detail:   "Il disco funziona ma ha superato la vita utile tipica di un meccanico. Non è un guasto, è un dato di rischio.",
			Action:   "Non affidargli l'unica copia di dati importanti.",
			Evidence: map[string]string{"powerOnHours": fmt.Sprint(h)},
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
// Molti dischi da portatile hanno un risparmio energetico che ferma le
// testine dopo pochi secondi di inattività. Ogni parcheggio è usura
// meccanica, e le unità sono garantite per un numero finito di cicli: un
// disco che ne accumula parecchi all'ora esaurisce quel budget in una
// frazione della vita prevista, pur risultando perfettamente sano a ogni
// controllo SMART. È un guasto annunciato che nessun altro programma dice.
func ruleDiskStartStopCycles(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		// L'attributo 193 conta i parcheggi con precisione: se c'è, se ne occupa
		// ruleSMARTLoadCycles e questa approssimazione va tolta di mezzo.
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
				Area:     "Disco",
				Target:   d.Model,
				Title:    fmt.Sprintf("%d cicli di arresto: vicino al limite di progetto", cycles),
				Detail:   "Il disco si è avvicinato al numero di cicli meccanici per cui è stato costruito. Da qui in avanti il rischio di guasto cresce indipendentemente dagli altri indicatori.",
				Action:   "Non affidargli dati di cui non esiste una seconda copia.",
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
			Area:     "Disco",
			Target:   d.Model,
			Title:    fmt.Sprintf("Testine parcheggiate %.1f volte l'ora", perHour),
			Detail: fmt.Sprintf("%d cicli di arresto in %d ore di funzionamento. "+
				"Il risparmio energetico ferma le testine troppo spesso: a questo "+
				"ritmo il disco raggiunge il limite meccanico di progetto molto "+
				"prima della fine della sua vita utile, pur restando sano a ogni "+
				"controllo.", cycles, *d.PowerOnHours),
			Action: "Ridurre l'aggressività del risparmio energetico del disco (impostazione APM) per allungarne la vita.",
			Evidence: map[string]string{
				"startStopCycles": fmt.Sprint(cycles),
				"powerOnHours":    fmt.Sprint(*d.PowerOnHours),
			},
		})
	}
}

const ssdPeakTempWarnC = 75

// ruleDiskPeakTemperature guarda la temperatura massima mai registrata, non
// quella attuale.
//
// E' la regola che smaschera il caso piu' frustrante dell'assistenza: il
// cliente dice che il PC rallenta sotto carico, il tecnico lo controlla da
// fermo, tutto risulta a posto e il problema viene archiviato come
// "impressione dell'utente". Il picco storico e' l'unica traccia che resta di
// un surriscaldamento che avviene solo quando la macchina lavora davvero.
func ruleDiskPeakTemperature(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.TemperatureMaxC == nil || !isSSD(d) {
			continue
		}
		peak := *d.TemperatureMaxC
		if peak < ssdPeakTempWarnC {
			continue
		}
		now := "sconosciuta"
		if d.TemperatureC != nil {
			now = fmt.Sprintf("%d °C", *d.TemperatureC)
		}
		b.add(Finding{
			Severity: SevWarn,
			Area:     "Termica",
			Target:   d.Model,
			Title:    fmt.Sprintf("Picco storico di %d °C (ora %s)", peak, now),
			Detail: "Il disco e' fresco adesso ma in passato ha raggiunto una " +
				"temperatura alla quale riduce da solo le prestazioni per " +
				"proteggersi. Succede sotto carico: copie di file grandi, " +
				"aggiornamenti, giochi. Un controllo fatto a macchina ferma non " +
				"lo rileva, ed e' il motivo per cui questo tipo di rallentamento " +
				"viene quasi sempre archiviato come impressione dell'utente.",
			Action: "Verificare il dissipatore dell'NVMe e il flusso d'aria nel case. Un dissipatore da pochi euro elimina il problema.",
			Evidence: map[string]string{
				"temperatureMaxC": fmt.Sprint(peak),
			},
		})
	}
}
