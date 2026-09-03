package rules

import (
	"strings"
	"testing"

	"github.com/shad272/diskseer/internal/i18n"
	"github.com/shad272/diskseer/internal/model"
)

// macchinaMalandata attiva quante più regole possibile in un colpo solo, così
// il controllo sulle traduzioni copre l'intero catalogo invece di un campione.
func macchinaMalandata() model.Snapshot {
	ore := uint64(20000)
	temp := 85
	picco := 90
	usura := 95
	cicli := uint64(50000)

	return model.Snapshot{
		Elevated: false,
		System:   model.System{Manufacturer: "X", Model: "Y", CPU: "Z"},
		Disks: []model.Disk{
			{
				DeviceID: "0", Model: "HDD", MediaType: "HDD", BusType: "SATA",
				HealthStatus: "Unhealthy", IsSystemDisk: true,
				PowerOnHours: &ore, TemperatureC: &temp, StartStopCycles: &cicli,
				SMART: &model.SMARTData{Attributes: []model.SMARTAttribute{
					{ID: 5, Raw: 200}, {ID: 10, Raw: 3}, {ID: 12, Raw: 100},
					{ID: 174, Raw: 60}, {ID: 188, Raw: 7}, {ID: 193, Raw: 700000},
					{ID: 197, Raw: 9}, {ID: 198, Raw: 2}, {ID: 199, Raw: 50},
				}},
			},
			{
				DeviceID: "1", Model: "NVME", MediaType: "SSD", BusType: "NVMe",
				TemperatureC: &temp, TemperatureMaxC: &picco, WearPercent: &usura,
				NVMe: &model.NVMeHealth{
					CriticalWarning: 0x1F, AvailableSparePct: 5, AvailableSpareThreshPct: 10,
					PercentageUsedPct: 95, PowerOnHours: 20000, PowerCycles: 100,
					UnsafeShutdowns: 40, MediaErrors: 12,
					WarningTempTimeMin: 30, CriticalTempTimeMin: 5,
				},
			},
			{DeviceID: "2", Model: "IGNOTO", BusType: "USB"},
		},
		Volumes: []model.Volume{
			{DriveLetter: "C", FileSystem: "NTFS", HealthStatus: "Healthy",
				SizeBytes: 1000000000000, FreeBytes: 1000000},
			{DriveLetter: "E", FileSystem: "exFAT", HealthStatus: "Warning",
				OperationalStatus: "Full Repair Needed", SizeBytes: 1000, FreeBytes: 900},
		},
		Thermals: []model.Thermal{{Name: "TZ", Celsius: 95}},
	}
}

// Il difetto classico di un programma bilingue è la traduzione dimenticata:
// il codice compila, i test passano, e in una delle due lingue compare una
// frase nell'altra. Qui si confrontano i due referti verdetto per verdetto: se
// un testo è identico in entrambe le lingue, o ne manca una versione o è stata
// copiata senza tradurla.
func TestOgniVerdettoEsisteInEntrambeLeLingue(t *testing.T) {
	s := macchinaMalandata()
	en := Run(s, i18n.EN)
	it := Run(s, i18n.IT)

	if len(en) != len(it) {
		t.Fatalf("verdetti: %d in inglese, %d in italiano — le regole non "+
			"devono dipendere dalla lingua", len(en), len(it))
	}
	if len(en) < 15 {
		t.Fatalf("solo %d verdetti: il campione non attiva abbastanza regole "+
			"per rendere significativo il confronto", len(en))
	}

	for i := range en {
		if en[i].Severity != it[i].Severity {
			t.Errorf("verdetto %d: gravità diversa fra le due lingue", i)
		}
		for _, c := range []struct{ nome, a, b string }{
			{"titolo", en[i].Title, it[i].Title},
			{"dettaglio", en[i].Detail, it[i].Detail},
			{"azione", en[i].Action, it[i].Action},
		} {
			if c.a == "" || c.b == "" {
				t.Errorf("verdetto %q: %s vuoto in una delle due lingue", en[i].Title, c.nome)
				continue
			}
			// Un titolo di soli numeri e simboli può legittimamente coincidere
			// (es. "50 °C"): si confrontano solo i testi con delle parole.
			if c.a == c.b && contieneParole(c.a) {
				t.Errorf("verdetto %q: %s identico nelle due lingue — traduzione mancante:\n  %s",
					en[i].Title, c.nome, c.a)
			}
		}
	}
}

func contieneParole(s string) bool {
	for _, p := range strings.Fields(s) {
		if len(p) > 4 && strings.IndexFunc(p, func(r rune) bool {
			return r >= 'a' && r <= 'z'
		}) >= 0 {
			return true
		}
	}
	return false
}
