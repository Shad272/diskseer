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
		Thermals: []Thermal{{Name: `ACPI\ThermalZone\TZ00_0`, Celsius: 27.9}},
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
