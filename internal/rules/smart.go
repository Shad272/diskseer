package rules

import (
	"fmt"

	"github.com/shad272/diskseer/internal/model"
)

const (
	reallocatedCritCount = 50
	loadCycleRated       = 600000 // cicli di parcheggio garantiti, valore tipico
	loadCyclePerHourWarn = 5.0
	loadCycleMinHours    = 100
	crcErroriRilevanti   = 10
	powerLossWarnPct     = 10.0
	powerLossInfoPct     = 3.0
	powerLossMinCount    = 5
)

// Regole per i dischi SATA, basate sugli attributi letti direttamente dal
// disco. Sono più precise di qualunque cosa Windows possa dire, perché
// distinguono cose che Windows accorpa: un settore *già sostituito* e un
// settore *illeggibile adesso* sono situazioni molto diverse, e la seconda è
// molto più urgente.

// ruleSMARTPendingSectors è la regola più importante di tutto il progetto.
//
// Un settore "in attesa" è un settore che il disco ha provato a leggere e non
// ci è riuscito. Non è ancora stato sostituito perché il disco aspetta di
// riuscire a recuperarne il contenuto. Nel frattempo quei dati non sono
// leggibili: se lì c'era un file, quel file è già danneggiato.
func ruleSMARTPendingSectors(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.SMART == nil {
			continue
		}
		n, ok := d.SMART.Raw(model.SMARTPendingSectors)
		if !ok || n == 0 {
			continue
		}
		b.add(Finding{
			Severity: SevCritical,
			Area:     areaDisco(b.l),
			Target:   d.Model,
			Title: b.n(n,
				"unreadable sector awaiting reallocation", "unreadable sectors awaiting reallocation",
				"settore illeggibile in attesa di rimappatura", "settori illeggibili in attesa di rimappatura"),
			Detail: b.s(
				"The drive found sectors it can no longer read and is still trying "+
					"to recover them. While they stay in this state the data they held "+
					"is inaccessible: if files lived there, those files are already "+
					"damaged. This counter is the signal that precedes the failure of "+
					"a mechanical drive, and it appears long before the user notices "+
					"anything.",
				"Il disco ha trovato settori che non riesce più a leggere e sta "+
					"tentando di recuperarli. Finché restano in questo stato, i dati "+
					"che contenevano non sono accessibili: se lì c'erano dei file, quei "+
					"file sono già danneggiati. Questo contatore è il segnale che "+
					"precede il guasto di un disco meccanico, e compare molto prima che "+
					"l'utente si accorga di qualcosa."),
			Action: b.s(
				"Copy the data today, starting with whatever is irreplaceable. Do not defragment and do not run full scans: they add stress to a drive that is already failing.",
				"Copiare i dati oggi, dando la precedenza a ciò che è insostituibile. Non eseguire deframmentazione né scansioni complete: aggiungono sollecitazione a un disco che sta già cedendo."),
			Evidence: map[string]string{"attribute197": fmt.Sprint(n)},
		})
	}
}

func ruleSMARTOfflineUncorrectable(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.SMART == nil {
			continue
		}
		n, ok := d.SMART.Raw(model.SMARTOfflineUncorrect)
		if !ok || n == 0 {
			continue
		}
		b.add(Finding{
			Severity: SevCritical,
			Area:     areaDisco(b.l),
			Target:   d.Model,
			Title: b.n(n, "unreadable, unrecoverable sector", "unreadable, unrecoverable sectors",
				"settore illeggibile e non recuperabile", "settori illeggibili e non recuperabili"),
			Detail: b.s(
				"Unlike pending sectors, the drive has already given up trying to recover these. The data they held is permanently gone.",
				"A differenza dei settori in attesa, questi il disco ha già smesso di provare a recuperarli. I dati che contenevano sono persi in modo definitivo."),
			Action: b.s(
				"Replace the drive. Check which files come back unreadable before trusting any copy made now.",
				"Sostituire il disco. Verificare quali file risultano illeggibili prima di fidarsi di una copia fatta ora."),
			Evidence: map[string]string{"attribute198": fmt.Sprint(n)},
		})
	}
}

func ruleSMARTReallocatedSectors(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.SMART == nil {
			continue
		}
		n, ok := d.SMART.Raw(model.SMARTReallocatedSectors)
		if !ok || n == 0 {
			continue
		}

		sev := SevWarn
		dettaglio := b.s(
			"The drive found defective sectors and replaced them with spares. The "+
				"mechanism worked as designed and no data was lost, but defective "+
				"sectors do not appear on their own: this is the start of a decline, "+
				"and the number has to be compared over time.",
			"Il disco ha trovato settori difettosi e li ha sostituiti con quelli di "+
				"scorta. Il meccanismo funziona come previsto e i dati non sono andati "+
				"persi, ma i settori difettosi non compaiono da soli: è l'inizio di un "+
				"deterioramento, e il numero va confrontato nel tempo.")
		azione := b.s(
			"Note the value and check it again in a few weeks. If it grows, replace the drive.",
			"Annotare il valore e ricontrollarlo fra qualche settimana. Se cresce, il disco va sostituito.")

		if n >= reallocatedCritCount {
			sev = SevCritical
			dettaglio = b.f(
				"The drive has already replaced %d defective sectors. At this level "+
					"the decline is no longer an isolated episode: the surface is "+
					"degrading and the reserve of spare sectors is not infinite.",
				"Il disco ha già sostituito %d settori difettosi. A questo livello il "+
					"deterioramento non è più un episodio isolato: la superficie si sta "+
					"degradando e la riserva di settori di scorta non è infinita.", n)
			azione = b.s(
				"Copy the data and plan the replacement. Do not wait for the drive to stop working.",
				"Copiare i dati e pianificare la sostituzione. Non aspettare che il disco smetta di funzionare.")
		}

		b.add(Finding{
			Severity: sev,
			Area:     areaDisco(b.l),
			Target:   d.Model,
			Title: b.n(n, "reallocated sector", "reallocated sectors",
				"settore riallocato", "settori riallocati"),
			Detail:   dettaglio,
			Action:   azione,
			Evidence: map[string]string{"attribute5": fmt.Sprint(n)},
		})
	}
}

// ruleSMARTCRCErrors indica un problema che non è nel disco.
//
// Questi errori avvengono durante il trasferimento fra disco e scheda madre:
// il disco riceve dati che non corrispondono alla loro somma di controllo. La
// causa è quasi sempre il cavo, o il connettore, non il disco.
//
// È la diagnosi che fa risparmiare più soldi in assoluto: il sintomo — blocchi,
// file corrotti, sparizioni dal sistema — assomiglia in tutto a un disco che
// muore, e viene sistematicamente curato sostituendo un disco perfettamente
// sano. Il problema si risolve con un cavo da tre euro.
func ruleSMARTCRCErrors(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.SMART == nil {
			continue
		}
		n, ok := d.SMART.Raw(model.SMARTCRCErrors)
		if !ok || n == 0 {
			continue
		}

		// Taratura fatta su hardware reale: un disco sano da 1833 ore mostrava
		// un singolo errore. Contarlo come ATTENZIONE avrebbe segnalato un
		// problema inesistente, e un programma che allarma per niente insegna a
		// ignorare anche gli allarmi veri. Sotto la decina si annota soltanto.
		sev := SevInfo
		dettaglio := b.s(
			"Data was corrupted in transit between the drive and the motherboard, not inside the drive. The count is low, though: isolated errors are normal after a hot unplug or an abrupt reboot, and on their own do not indicate a fault.",
			"I dati si sono corrotti nel tragitto fra il disco e la scheda madre, non dentro il disco. Il numero è però basso: qualche errore isolato capita normalmente dopo uno scollegamento a caldo o un riavvio brusco, e da solo non indica un guasto.")
		azione := b.s(
			"No action needed now. If the number grows over time, replace the cable.",
			"Nessun intervento necessario ora. Se il numero cresce nel tempo, il cavo va sostituito.")

		if n >= crcErroriRilevanti {
			sev = SevWarn
			dettaglio = b.s(
				"Data was corrupted in transit between the drive and the motherboard, "+
					"not inside the drive. The cause is almost always the cable or the "+
					"connector. The symptoms — freezes, corrupted files, the drive "+
					"vanishing from the system — are identical to a dying drive, which "+
					"is why perfectly healthy drives get replaced over this.",
				"I dati si sono corrotti nel tragitto fra il disco e la scheda madre, "+
					"non dentro il disco. La causa è quasi sempre il cavo o il "+
					"connettore. Il sintomo — blocchi, file corrotti, il disco che "+
					"sparisce dal sistema — è identico a quello di un disco in avaria, "+
					"ed è per questo che di solito si finisce per sostituire un disco "+
					"perfettamente sano.")
			azione = b.s(
				"Replace the data cable and check it is firmly seated at both ends, before considering replacing the drive.",
				"Sostituire il cavo dati e verificare che sia ben inserito da entrambi i lati, prima di considerare la sostituzione del disco.")
		}

		b.add(Finding{
			Severity: sev,
			Area:     areaCollegamento(b.l),
			Target:   d.Model,
			Title: b.n(n, "cable transfer error", "cable transfer errors",
				"errore di trasmissione sul cavo", "errori di trasmissione sul cavo"),
			Detail:   dettaglio,
			Action:   azione,
			Evidence: map[string]string{"attribute199": fmt.Sprint(n)},
		})
	}
}

// ruleSMARTSpinRetry riguarda il motore, non la superficie.
func ruleSMARTSpinRetry(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.SMART == nil {
			continue
		}
		n, ok := d.SMART.Raw(model.SMARTSpinRetry)
		if !ok || n == 0 {
			continue
		}
		b.add(Finding{
			Severity: SevCritical,
			Area:     areaDisco(b.l),
			Target:   d.Model,
			Title: b.n(n, "failed spin-up attempt", "failed spin-up attempts",
				"tentativo fallito di avvio della rotazione", "tentativi falliti di avvio della rotazione"),
			Detail: b.s(
				"The motor failed to bring the platters up to operating speed on the "+
					"first attempt. This is a mechanical problem, not a surface one, "+
					"and it precedes the worst kind of failure: the one where the drive "+
					"simply does not start one day and the data is out of reach without "+
					"a laboratory.",
				"Il motore non è riuscito a portare i piatti alla velocità di lavoro "+
					"al primo tentativo. È un problema meccanico, non di superficie, e "+
					"precede il tipo di guasto peggiore: quello in cui il disco un "+
					"giorno non parte più e i dati non sono più raggiungibili senza un "+
					"laboratorio."),
			Action: b.s(
				"Copy everything now, while the drive still spins up. Do not power it off until the copy is complete.",
				"Copiare tutto adesso, mentre il disco ancora parte. Non spegnerlo finché la copia non è completa."),
			Evidence: map[string]string{"attribute10": fmt.Sprint(n)},
		})
	}
}

// ruleSMARTLoadCycles misura il parcheggio delle testine con il contatore
// giusto: l'attributo 193, quello su cui i costruttori dichiarano il limite.
func ruleSMARTLoadCycles(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.SMART == nil {
			continue
		}
		n, ok := d.SMART.Raw(model.SMARTLoadCycles)
		if !ok || n == 0 {
			continue
		}

		if n >= loadCycleRated {
			b.add(Finding{
				Severity: SevWarn,
				Area:     areaDisco(b.l),
				Target:   d.Model,
				Title: b.f("%d head park cycles: design limit reached",
					"%d parcheggi delle testine: limite di progetto raggiunto", n),
				Detail: b.s(
					"The mechanism that lifts and lowers the heads has exceeded the number of cycles it was built for. It still works, but with no declared margin left.",
					"Il meccanismo che solleva e riappoggia le testine ha superato il numero di cicli per cui è stato costruito. Continua a funzionare, ma senza più alcun margine dichiarato."),
				Action: b.s(
					"Do not trust this drive with the only copy of anything important.",
					"Non affidare a questo disco l'unica copia di dati importanti."),
				Evidence: map[string]string{"attribute193": fmt.Sprint(n)},
			})
			continue
		}

		if d.PowerOnHours == nil || *d.PowerOnHours < loadCycleMinHours {
			continue
		}
		perOra := float64(n) / float64(*d.PowerOnHours)
		if perOra < loadCyclePerHourWarn {
			continue
		}
		b.add(Finding{
			Severity: SevInfo,
			Area:     areaDisco(b.l),
			Target:   d.Model,
			Title: b.f("Heads parked %.1f times per hour",
				"Testine parcheggiate %.1f volte l'ora", perOra),
			Detail: b.f(
				"%d park cycles over %d powered hours. Power saving lifts the heads "+
					"after a few seconds of inactivity: every cycle is mechanical wear, "+
					"and at this rate the design limit (around %d cycles) arrives long "+
					"before the end of the drive's life.",
				"%d parcheggi in %d ore di funzionamento. Il risparmio energetico "+
					"solleva le testine dopo pochi secondi di inattività: ogni ciclo è "+
					"usura meccanica, e a questo ritmo il limite di progetto (circa %d "+
					"cicli) arriva molto prima della fine della vita del disco.",
				n, *d.PowerOnHours, loadCycleRated),
			Action: b.s(
				"Reduce the drive's power-saving aggressiveness (APM setting).",
				"Ridurre l'aggressività del risparmio energetico del disco (impostazione APM)."),
			Evidence: map[string]string{
				"attribute193": fmt.Sprint(n),
				"powerOnHours": fmt.Sprint(*d.PowerOnHours),
			},
		})
	}
}

func ruleSMARTCommandTimeout(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.SMART == nil {
			continue
		}
		n, ok := d.SMART.Raw(model.SMARTCommandTimeout)
		if !ok || n == 0 {
			continue
		}
		b.add(Finding{
			Severity: SevWarn,
			Area:     areaDisco(b.l),
			Target:   d.Model,
			Title: b.n(n, "command timed out with no reply", "commands timed out with no reply",
				"comando scaduto senza risposta", "comandi scaduti senza risposta"),
			Detail: b.s(
				"The drive stopped responding long enough for the system to give up waiting. This is the moment when the user sees the computer freeze for a few seconds for no apparent reason. Causes range from insufficient power to the cable, to the drive itself.",
				"Il disco ha smesso di rispondere abbastanza a lungo da far scadere l'attesa del sistema. È il momento in cui l'utente vede il computer bloccarsi per qualche secondo senza motivo apparente. Le cause vanno dall'alimentazione insufficiente al cavo, fino al disco stesso."),
			Action: b.s(
				"Check the power and data cables. If the number grows, replace the drive.",
				"Verificare cavo di alimentazione e cavo dati. Se il numero cresce, sostituire il disco."),
			Evidence: map[string]string{"attribute188": fmt.Sprint(n)},
		})
	}
}

// ruleSMARTUnexpectedPowerLoss riconosce i dischi che perdono corrente mentre
// stanno ancora scrivendo.
//
// Su un disco interno significa mancanza di alimentazione, blocchi o
// spegnimenti forzati. Su un disco esterno significa quasi sempre un'altra
// cosa: che viene staccato senza usare la rimozione sicura. Il consiglio
// cambia completamente, e darlo sbagliato fa perdere credibilità — per questo
// la regola guarda come è collegato il disco prima di parlare.
func ruleSMARTUnexpectedPowerLoss(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.SMART == nil {
			continue
		}
		perse, ok := d.SMART.Raw(174)
		if !ok || perse < powerLossMinCount {
			continue
		}
		accensioni, ok := d.SMART.Raw(model.SMARTPowerCycles)
		if !ok || accensioni == 0 {
			continue
		}
		pct := float64(perse) / float64(accensioni) * 100

		var sev Severity
		switch {
		case pct >= powerLossWarnPct:
			sev = SevWarn
		case pct >= powerLossInfoPct:
			sev = SevInfo
		default:
			continue
		}

		dettaglio := b.f(
			"Out of %d power-ups, %d times the drive lost power while still running (%.0f%%). ",
			"Su %d accensioni, %d volte il disco ha perso corrente mentre era ancora in funzione (%.0f%%). ",
			accensioni, perse, pct)
		azione := b.s(
			"Find the cause: power supply, system freezes, forced shutdowns.",
			"Individuare la causa: alimentatore, blocchi di sistema, spegnimenti forzati.")

		if d.BusType == "USB" {
			dettaglio += b.s(
				"On an external drive the cause is almost always a single one: it gets unplugged without using safe removal. The drive does not break, but writes still in flight are left half done, and that is how file systems get corrupted and files disappear.",
				"Su un disco esterno la causa è quasi sempre una sola: viene staccato senza usare la rimozione sicura. Il disco non si rompe, ma le scritture ancora in corso restano a metà, ed è così che i file system si corrompono e i file spariscono.")
			azione = b.s(
				"Always use \"Safely Remove Hardware\" before unplugging it. It is the only way to avoid the corruption, and there is no software substitute.",
				"Usare sempre \"Rimozione sicura dell'hardware\" prima di staccarlo. È l'unico modo di evitare la corruzione, e non ha alternative software.")
		} else {
			dettaglio += b.s(
				"The drive survives it undamaged, but writes interrupted halfway are the most common cause of corrupted file systems.",
				"Il disco regge senza danni, ma le scritture interrotte a metà sono la causa più comune di file system corrotti.")
		}

		if corrotti := volumiDaRiparare(s); corrotti != "" {
			dettaglio += b.f(" On this machine the following indeed needs repair: %s.",
				" Su questa macchina risulta infatti da riparare: %s.", corrotti)
		}

		b.add(Finding{
			Severity: sev,
			Area:     areaSistema(b.l),
			Target:   d.Model,
			Title: b.f("%d unexpected power losses out of %d power-ups",
				"%d interruzioni di corrente impreviste su %d accensioni", perse, accensioni),
			Detail: dettaglio,
			Action: azione,
			Evidence: map[string]string{
				"attribute174": fmt.Sprint(perse),
				"attribute12":  fmt.Sprint(accensioni),
			},
		})
	}
}
