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
//
// È il segnale che precede la morte di un disco meccanico, e compare mesi
// prima che l'utente si accorga di qualcosa.
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
			Area:     "Disco",
			Target:   d.Model,
			Title:    conta(n, "settore illeggibile in attesa di rimappatura", "settori illeggibili in attesa di rimappatura"),
			Detail: "Il disco ha trovato settori che non riesce più a leggere e sta " +
				"tentando di recuperarli. Finché restano in questo stato, i dati che " +
				"contenevano non sono accessibili: se lì c'erano dei file, quei file " +
				"sono già danneggiati. Questo contatore è il segnale che precede il " +
				"guasto di un disco meccanico, e compare molto prima che l'utente si " +
				"accorga di qualcosa.",
			Action: "Copiare i dati oggi, dando la precedenza a ciò che è insostituibile. Non eseguire deframmentazione né scansioni complete: aggiungono sollecitazione a un disco che sta già cedendo.",
			Evidence: map[string]string{
				"attributo197": fmt.Sprint(n),
			},
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
			Area:     "Disco",
			Target:   d.Model,
			Title:    conta(n, "settore illeggibile e non recuperabile", "settori illeggibili e non recuperabili"),
			Detail: "A differenza dei settori in attesa, questi il disco ha già " +
				"smesso di provare a recuperarli. I dati che contenevano sono persi " +
				"in modo definitivo.",
			Action:   "Sostituire il disco. Verificare quali file risultano illeggibili prima di fidarsi di una copia fatta ora.",
			Evidence: map[string]string{"attributo198": fmt.Sprint(n)},
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
		dettaglio := "Il disco ha trovato settori difettosi e li ha sostituiti con " +
			"quelli di scorta. Il meccanismo funziona come previsto e i dati non " +
			"sono andati persi, ma i settori difettosi non compaiono da soli: è " +
			"l'inizio di un deterioramento, e il numero va confrontato nel tempo."
		azione := "Annotare il valore e ricontrollarlo fra qualche settimana. Se cresce, il disco va sostituito."

		if n >= reallocatedCritCount {
			sev = SevCritical
			dettaglio = fmt.Sprintf("Il disco ha già sostituito %d settori difettosi. "+
				"A questo livello il deterioramento non è più un episodio isolato: "+
				"la superficie si sta degradando e la riserva di settori di scorta "+
				"non è infinita.", n)
			azione = "Copiare i dati e pianificare la sostituzione. Non aspettare che il disco smetta di funzionare."
		}

		b.add(Finding{
			Severity: sev,
			Area:     "Disco",
			Target:   d.Model,
			Title:    conta(n, "settore riallocato", "settori riallocati"),
			Detail:   dettaglio,
			Action:   azione,
			Evidence: map[string]string{"attributo5": fmt.Sprint(n)},
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
		dettaglio := "I dati si sono corrotti nel tragitto fra il disco e la scheda " +
			"madre, non dentro il disco. Il numero è però basso: qualche errore " +
			"isolato capita normalmente dopo uno scollegamento a caldo o un " +
			"riavvio brusco, e da solo non indica un guasto."
		azione := "Nessun intervento necessario ora. Se il numero cresce nel tempo, il cavo va sostituito."

		if n >= crcErroriRilevanti {
			sev = SevWarn
			dettaglio = "I dati si sono corrotti nel tragitto fra il disco e la scheda " +
				"madre, non dentro il disco. La causa è quasi sempre il cavo o il " +
				"connettore. Il sintomo — blocchi, file corrotti, il disco che sparisce " +
				"dal sistema — è identico a quello di un disco in avaria, ed è per " +
				"questo che di solito si finisce per sostituire un disco perfettamente sano."
			azione = "Sostituire il cavo dati e verificare che sia ben inserito da entrambi i lati, prima di considerare la sostituzione del disco."
		}

		b.add(Finding{
			Severity: sev,
			Area:     "Collegamento",
			Target:   d.Model,
			Title:    conta(n, "errore di trasmissione sul cavo", "errori di trasmissione sul cavo"),
			Detail:   dettaglio,
			Action:   azione,
			Evidence: map[string]string{"attributo199": fmt.Sprint(n)},
		})
	}
}

// ruleSMARTSpinRetry riguarda il motore, non la superficie.
//
// Un tentativo fallito di avviare la rotazione significa che il motore non è
// riuscito a portare i piatti alla velocità di lavoro al primo colpo. Nei
// dischi meccanici è uno dei pochi segnali che precede un guasto totale e
// improvviso, quello in cui il disco un giorno semplicemente non parte più.
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
			Area:     "Disco",
			Target:   d.Model,
			Title:    conta(n, "tentativo fallito di avvio della rotazione", "tentativi falliti di avvio della rotazione"),
			Detail: "Il motore non è riuscito a portare i piatti alla velocità di " +
				"lavoro al primo tentativo. È un problema meccanico, non di superficie, " +
				"e precede il tipo di guasto peggiore: quello in cui il disco un giorno " +
				"non parte più e i dati non sono più raggiungibili senza un laboratorio.",
			Action:   "Copiare tutto adesso, mentre il disco ancora parte. Non spegnerlo finché la copia non è completa.",
			Evidence: map[string]string{"attributo10": fmt.Sprint(n)},
		})
	}
}

// ruleSMARTLoadCycles misura il parcheggio delle testine con il contatore
// giusto.
//
// La regola generica usa i cicli di avvio/arresto forniti da Windows, che sono
// un'approssimazione. L'attributo 193 conta esattamente i parcheggi, ed è il
// numero su cui i costruttori dichiarano il limite di progetto.
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
				Area:     "Disco",
				Target:   d.Model,
				Title:    fmt.Sprintf("%d parcheggi delle testine: limite di progetto raggiunto", n),
				Detail:   "Il meccanismo che solleva e riappoggia le testine ha superato il numero di cicli per cui è stato costruito. Continua a funzionare, ma senza più alcun margine dichiarato.",
				Action:   "Non affidare a questo disco l'unica copia di dati importanti.",
				Evidence: map[string]string{"attributo193": fmt.Sprint(n)},
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
			Area:     "Disco",
			Target:   d.Model,
			Title:    fmt.Sprintf("Testine parcheggiate %.1f volte l'ora", perOra),
			Detail: fmt.Sprintf("%d parcheggi in %d ore di funzionamento. Il risparmio "+
				"energetico solleva le testine dopo pochi secondi di inattività: ogni "+
				"ciclo è usura meccanica, e a questo ritmo il limite di progetto "+
				"(circa %d cicli) arriva molto prima della fine della vita del disco.",
				n, *d.PowerOnHours, loadCycleRated),
			Action: "Ridurre l'aggressività del risparmio energetico del disco (impostazione APM).",
			Evidence: map[string]string{
				"attributo193": fmt.Sprint(n),
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
			Area:     "Disco",
			Target:   d.Model,
			Title:    conta(n, "comando scaduto senza risposta", "comandi scaduti senza risposta"),
			Detail: "Il disco ha smesso di rispondere abbastanza a lungo da far " +
				"scadere l'attesa del sistema. È il momento in cui l'utente vede il " +
				"computer bloccarsi per qualche secondo senza motivo apparente. Le " +
				"cause vanno dall'alimentazione insufficiente al cavo, fino al disco stesso.",
			Action:   "Verificare cavo di alimentazione e cavo dati. Se il numero cresce, sostituire il disco.",
			Evidence: map[string]string{"attributo188": fmt.Sprint(n)},
		})
	}
}

const (
	powerLossWarnPct  = 10.0
	powerLossInfoPct  = 3.0
	powerLossMinCount = 5
)

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

		dettaglio := fmt.Sprintf("Su %d accensioni, %d volte il disco ha perso "+
			"corrente mentre era ancora in funzione (%.0f%%). ", accensioni, perse, pct)
		azione := "Individuare la causa: alimentatore, blocchi di sistema, spegnimenti forzati."

		if d.BusType == "USB" {
			dettaglio += "Su un disco esterno la causa è quasi sempre una sola: " +
				"viene staccato senza usare la rimozione sicura. Il disco non si " +
				"rompe, ma le scritture ancora in corso restano a metà, ed è così " +
				"che i file system si corrompono e i file spariscono."
			azione = "Usare sempre \"Rimozione sicura dell'hardware\" prima di staccarlo. È l'unico modo di evitare la corruzione, e non ha alternative software."
		} else {
			dettaglio += "Il disco regge senza danni, ma le scritture interrotte a " +
				"metà sono la causa più comune di file system corrotti."
		}

		if corrotti := volumiDaRiparare(s); corrotti != "" {
			dettaglio += " Su questa macchina risulta infatti da riparare: " + corrotti + "."
		}

		b.add(Finding{
			Severity: sev,
			Area:     "Sistema",
			Target:   d.Model,
			Title:    fmt.Sprintf("%d interruzioni di corrente impreviste su %d accensioni", perse, accensioni),
			Detail:   dettaglio,
			Action:   azione,
			Evidence: map[string]string{
				"attributo174": fmt.Sprint(perse),
				"attributo12":  fmt.Sprint(accensioni),
			},
		})
	}
}
