package rules

import "github.com/shad272/diskseer/internal/model"

func ruleCollectionNotes(s model.Snapshot, b *builder) {
	for _, d := range s.Disks {
		if d.ReadError != "" {
			staleFinding(d.Model, b)
		}
	}
	for _, v := range s.Volumes {
		if v.ReadError != "" {
			staleFinding(v.DriveLetter+":", b)
		}
	}
	if len(s.CollectionNotes) == 0 {
		return
	}
	b.add(Finding{Severity: SevInfo, SuiLimiti: true, Area: areaDiagnosi(b.l), Target: "diskseer",
		Title:  b.s("Compatibility inventory: some checks are unavailable", "Inventario compatibile: alcuni controlli non sono disponibili"),
		Detail: b.s("Windows Storage APIs were unavailable. WMI supplies basic inventory and free space; drive type, filesystem health and some reliability counters may be unknown. Direct SMART access is attempted when permitted.", "Le API Storage di Windows non erano disponibili. WMI fornisce inventario e spazio libero; tipo del disco, salute del filesystem e alcuni contatori possono restare sconosciuti. La lettura SMART diretta viene tentata quando consentita."),
		Action: b.s("Use the available measurements and verify missing checks separately. Missing data does not indicate a healthy disk.", "Usa le misure disponibili e verifica separatamente i controlli mancanti. L'assenza di dati non indica un disco sano.")})
}

func staleFinding(target string, b *builder) {
	b.add(Finding{Severity: SevInfo, SuiLimiti: true, Area: areaDiagnosi(b.l), Target: target,
		Title:  b.s("Refresh failed: previous readings retained", "Aggiornamento fallito: restano le letture precedenti"),
		Detail: b.s("This device did not return fresh data. Displayed counters and findings may refer to an earlier reading.", "Il dispositivo non ha restituito dati aggiornati. I contatori e le segnalazioni mostrati possono riferirsi a una lettura precedente."),
		Action: b.s("Check the connection and run the diagnosis again after reconnecting the device.", "Controlla il collegamento e ripeti la diagnosi dopo aver ricollegato il dispositivo.")})
}
