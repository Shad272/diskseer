package model

import (
	"strings"
	"testing"
	"time"
)

// La proprietà che conta: dopo l'anonimizzazione non deve restare traccia
// della macchina, ma nemmeno una misura deve cambiare. Se un numero cambiasse,
// i verdetti calcolati su un campione anonimizzato sarebbero diversi da quelli
// dell'originale, e il campione non varrebbe più come materiale di collaudo.
func TestAnonimizzaTogliLIdentitaNonLeMisure(t *testing.T) {
	ore := uint64(1833)
	temp := 30
	usura := 7

	s := Snapshot{
		Time:     time.Now(),
		Elevated: true,
		System: System{
			Manufacturer: "MARCAFINTA", Model: "MODELLO-XY99",
			CPU: "Processore Inventato 9000", Cores: 6, Threads: 12,
			RAMBytes: 34136850432, Chassis: "Desktop",
			OS: "Microsoft Windows 11 Pro", OSVersion: "10.0.26100",
			LastBoot: time.Now().Add(-48 * time.Hour),
		},
		Disks: []Disk{{
			DeviceID: "0", Model: "DISCOFINTO ZZ1234",
			MediaType: "HDD", BusType: "SATA", HealthStatus: "Healthy",
			SizeBytes: 500107862016, PowerOnHours: &ore, TemperatureC: &temp,
			WearPercent: &usura,
			SMART:       &SMARTData{Attributes: []SMARTAttribute{{ID: 197, Raw: 4}}},
		}},
		Volumes:  []Volume{{DriveLetter: "C", FileSystem: "NTFS", SizeBytes: 1000, FreeBytes: 20}},
		Thermals: []Thermal{{Name: `ACPI ThermalZone TZ00_0`, Celsius: 27.9}},
	}

	s.Anonimizza()

	t.Run("l'identita' della macchina sparisce", func(t *testing.T) {
		reso := s.System.Manufacturer + s.System.Model + s.System.CPU +
			s.Disks[0].Model + s.Thermals[0].Name
		// Traccianti inventati apposta: se anche uno solo sopravvive,
		// l'anonimizzazione ha lasciato passare un dato identificativo.
		for _, tracciante := range []string{
			"MARCAFINTA", "MODELLO-XY99", "Inventato", "DISCOFINTO", "ACPI",
		} {
			if strings.Contains(reso, tracciante) {
				t.Errorf("%q sopravvive all'anonimizzazione", tracciante)
			}
		}
		if !s.Time.IsZero() || !s.System.LastBoot.IsZero() {
			t.Error("gli orari vanno azzerati: dicono quando la macchina viene usata")
		}
	})

	t.Run("nessuna misura cambia", func(t *testing.T) {
		d := s.Disks[0]
		if *d.PowerOnHours != 1833 || *d.TemperatureC != 30 || *d.WearPercent != 7 {
			t.Error("i contatori del disco sono stati alterati")
		}
		if n, ok := d.SMART.Raw(SMARTPendingSectors); !ok || n != 4 {
			t.Error("gli attributi SMART sono stati alterati")
		}
		if d.SizeBytes != 500107862016 || d.HealthStatus != "Healthy" {
			t.Error("le caratteristiche del disco sono state alterate")
		}
		if s.Volumes[0].FreeBytes != 20 || s.Volumes[0].DriveLetter != "C" {
			t.Error("i volumi sono stati alterati")
		}
		if s.System.Cores != 6 || s.System.RAMBytes != 34136850432 {
			t.Error("la configurazione utile alla diagnosi è stata alterata")
		}
		if !s.Elevated {
			t.Error("il livello di privilegi della cattura è stato alterato")
		}
	})

	t.Run("il disco resta riconoscibile per tipo", func(t *testing.T) {
		if !strings.Contains(s.Disks[0].Model, "HDD") ||
			!strings.Contains(s.Disks[0].Model, "SATA") {
			t.Errorf("nome %q: senza tipo e collegamento il referto diventa illeggibile",
				s.Disks[0].Model)
		}
	})
}

// Anonimizzare una copia non deve toccare l'originale.
//
// È il presupposto dell'esportazione dal menu: si esporta una versione anonima
// mentre a schermo resta la diagnosi vera, con marca e modello al loro posto.
// Senza Clona le due cose sarebbero lo stesso oggetto, e l'esportazione
// cancellerebbe i dati che l'utente sta guardando.
func TestAnonimizzareUnaCopiaLasciaStareLOriginale(t *testing.T) {
	originale := Snapshot{
		System:   System{Manufacturer: "MARCAFINTA", Model: "MODELLO-XY99"},
		Disks:    []Disk{{DeviceID: "0", Model: "DISCOFINTO 500", MediaType: "SSD", BusType: "NVMe"}},
		Thermals: []Thermal{{Name: "ACPI ThermalZone TZ00", Celsius: 42}},
		Battery:  &Battery{Name: "Batteria Inventata"},
	}

	copia := originale.Clona()
	copia.Anonimizza()

	if originale.System.Manufacturer != "MARCAFINTA" {
		t.Error("la marca dell'originale è stata cancellata dall'anonimizzazione della copia")
	}
	if originale.Disks[0].Model != "DISCOFINTO 500" {
		t.Error("il modello del disco dell'originale è cambiato: la fetta dei dischi è condivisa")
	}
	if originale.Thermals[0].Name != "ACPI ThermalZone TZ00" {
		t.Error("il nome della zona termica dell'originale è cambiato")
	}
	if originale.Battery.Name != "Batteria Inventata" {
		t.Error("il nome della batteria dell'originale è cambiato: il puntatore è condiviso")
	}
	if copia.Disks[0].Model == originale.Disks[0].Model {
		t.Error("la copia non è stata anonimizzata affatto")
	}
}
