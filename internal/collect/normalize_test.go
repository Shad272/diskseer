package collect

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/shad272/diskseer/internal/model"
)

// Il campione e' una cattura grezza fatta su una macchina reale, presa prima
// che Normalize esistesse: contiene quindi gli zeri-riempitivi cosi' come
// Windows li ha restituiti. E' esattamente il materiale che serve per
// verificare che vengano riconosciuti.
const rawFixture = "../../testdata/snapshot-admin-raw.json"

func loadRaw(t *testing.T) model.Snapshot {
	t.Helper()
	b, err := os.ReadFile(rawFixture)
	if err != nil {
		t.Skipf("campione non disponibile: %v", err)
	}
	var s model.Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("campione illeggibile: %v", err)
	}
	return s
}

func TestNormalizeScartaZeriRiempitivi(t *testing.T) {
	s := loadRaw(t)

	// Prima: Windows ha riportato zeri per contatori che non fornisce.
	var hddWearBefore, maxTempZeroBefore int
	for _, d := range s.Disks {
		if d.MediaType == "HDD" && d.WearPercent != nil {
			hddWearBefore++
		}
		if d.TemperatureMaxC != nil && *d.TemperatureMaxC == 0 {
			maxTempZeroBefore++
		}
	}
	if hddWearBefore == 0 && maxTempZeroBefore == 0 {
		t.Skip("il campione non contiene zeri-riempitivi da scartare")
	}

	Normalize(&s)

	for _, d := range s.Disks {
		if !isFlash(d) && d.WearPercent != nil {
			t.Errorf("%s: usura valorizzata su un disco non flash (%d): "+
				"su un meccanico non significa nulla", d.Model, *d.WearPercent)
		}
		if d.TemperatureMaxC != nil && *d.TemperatureMaxC == 0 {
			t.Errorf("%s: temperatura massima 0 C accettata come misura", d.Model)
		}
	}
}

// Normalize non deve toccare i valori legittimi: scartare troppo e' tanto
// dannoso quanto scartare troppo poco.
func TestNormalizePreservaDatiValidi(t *testing.T) {
	s := loadRaw(t)
	Normalize(&s)

	var withTemp int
	for _, d := range s.Disks {
		if d.TemperatureC != nil {
			withTemp++
		}
		if d.MediaType == "SSD" && d.WearPercent == nil {
			t.Errorf("%s: usura scartata su un SSD, dove invece ha senso", d.Model)
		}
	}
	if withTemp != len(s.Disks) {
		t.Errorf("temperature sopravvissute %d su %d dischi", withTemp, len(s.Disks))
	}
}

// Regressione: quando isFlash ha imparato a riconoscere gli SSD dietro un
// ponte USB, ha smesso di scartarne l'usura — che però continua ad arrivare
// dai contatori del ponte, non del disco, e vale sempre zero.
func TestNormalizeScartaUsuraDietroPonteUSB(t *testing.T) {
	zero := 0
	s := model.Snapshot{Disks: []model.Disk{{
		Model: "SSD IN BOX", BusType: "USB", MediaType: "Unspecified",
		WearPercent: &zero,
		SMART: &model.SMARTData{Attributes: []model.SMARTAttribute{
			{ID: 173, Raw: 6}, {ID: 174, Raw: 32},
		}},
	}}}

	if !isFlash(s.Disks[0]) {
		t.Fatal("gli attributi 173/174 esistono solo sulla memoria flash")
	}

	Normalize(&s)

	if s.Disks[0].WearPercent != nil {
		t.Errorf("usura %d accettata da un ponte USB, che non la conosce",
			*s.Disks[0].WearPercent)
	}
}
