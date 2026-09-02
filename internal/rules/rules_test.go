package rules

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/shad272/diskseer/internal/collect"
	"github.com/shad272/diskseer/internal/model"
)

// Il motore di regole e' una funzione pura: snapshot dentro, verdetti fuori.
// E' cio' che permette di collaudarlo su macchine che non abbiamo sottomano.
// Ogni volta che un cliente porta un guasto vero, il suo snapshot entra qui e
// il caso non potra' piu' tornare a passare inosservato.
func loadSnapshot(t *testing.T, path string) model.Snapshot {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("campione non disponibile: %v", err)
	}
	var s model.Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("campione illeggibile: %v", err)
	}
	collect.Normalize(&s)
	return s
}

func findByTitle(fs []Finding, substr string) *Finding {
	for i := range fs {
		if strings.Contains(fs[i].Title, substr) {
			return &fs[i]
		}
	}
	return nil
}

func TestSnapshotReale(t *testing.T) {
	s := loadSnapshot(t, "../../testdata/snapshot-admin-raw.json")
	fs := Run(s)

	t.Run("spazio esaurito su C e' critico", func(t *testing.T) {
		f := findByTitle(fs, "2.2 GB liberi")
		if f == nil {
			t.Fatalf("nessun verdetto sullo spazio di C:")
		}
		if f.Severity != SevCritical {
			t.Errorf("severita' %v, attesa CRITICO", f.Severity)
		}
		if f.Action == "" {
			t.Error("verdetto senza azione: e' cio' che gia' fanno tutti gli altri tool")
		}
	})

	t.Run("volume exFAT degradato", func(t *testing.T) {
		if f := findByTitle(fs, "File system exFAT"); f == nil || f.Severity != SevWarn {
			t.Errorf("atteso ATTENZIONE sul volume E:, ottenuto %v", f)
		}
	})

	t.Run("parcheggio testine rilevato", func(t *testing.T) {
		f := findByTitle(fs, "Testine parcheggiate")
		if f == nil {
			t.Fatal("6982 cicli in 1828 ore devono produrre un verdetto")
		}
		if !strings.Contains(f.Detail, "6982") {
			t.Error("il dettaglio deve riportare i numeri su cui si basa")
		}
	})

	t.Run("picco termico storico sull NVMe", func(t *testing.T) {
		f := findByTitle(fs, "Picco storico")
		if f == nil {
			t.Fatal("84 gradi registrati devono produrre un verdetto")
		}
		if f.Severity != SevWarn {
			t.Errorf("severita' %v, attesa ATTENZIONE", f.Severity)
		}
	})

	t.Run("nessun avviso di analisi parziale se elevati", func(t *testing.T) {
		if f := findByTitle(fs, "Analisi parziale"); f != nil {
			t.Error("lo snapshot e' elevato, l'avviso non deve comparire")
		}
	})

	t.Run("disco di sistema SSD non segnalato", func(t *testing.T) {
		if f := findByTitle(fs, "disco meccanico"); f != nil {
			t.Error("il sistema gira su NVMe: falso positivo")
		}
	})

	t.Run("esito complessivo critico", func(t *testing.T) {
		if got := Overall(fs); got != SevCritical {
			t.Errorf("esito %v, atteso CRITICO", got)
		}
	})
}

func u64(v uint64) *uint64 { return &v }

// Il principio che regge tutto il progetto: un dato assente non e' uno zero.
// Un disco con zero errori e' sano; un disco di cui non conosciamo gli errori
// e' semplicemente sconosciuto, e dichiararlo sano sarebbe una diagnosi falsa.
func TestDatoAssenteNonEZero(t *testing.T) {
	tests := []struct {
		nome        string
		uncorrected *uint64
		vuoleAllarm bool
	}{
		{"contatore non fornito dal driver", nil, false},
		{"contatore fornito e pari a zero", u64(0), false},
		{"errori realmente presenti", u64(3), true},
	}

	for _, tc := range tests {
		t.Run(tc.nome, func(t *testing.T) {
			s := model.Snapshot{
				Elevated: true,
				Disks: []model.Disk{{
					Model:            "DISCO DI PROVA",
					MediaType:        "HDD",
					ReadErrorsUncorr: tc.uncorrected,
				}},
			}
			f := findByTitle(Run(s), "non è riuscito a correggere")
			if tc.vuoleAllarm && f == nil {
				t.Error("errori non corretti presenti ma nessun allarme")
			}
			if !tc.vuoleAllarm && f != nil {
				t.Errorf("allarme su dato assente o nullo: %q", f.Title)
			}
		})
	}
}

// Lo stato operativo di Windows deve tradursi nell'intervento giusto, non in
// un generico "c'è un problema". Un "Spot Fix" si risolve in dieci secondi,
// un "Full Repair" richiede di salvare prima i dati: dare la stessa risposta
// a entrambi significa o allarmare per niente o far perdere dei file.
func TestStatoOperativoDetermina1Intervento(t *testing.T) {
	tests := []struct {
		operational string
		vuoleSev    Severity
		azioneHa    string
	}{
		{"Scan Needed", SevWarn, "/scan"},
		{"Spot Fix Needed", SevWarn, "/spotfix"},
		{"Full Repair Needed", SevCritical, "/f"},
		{"", SevWarn, "scansione"},
	}

	for _, tc := range tests {
		t.Run(tc.operational, func(t *testing.T) {
			s := model.Snapshot{
				Elevated: true,
				Volumes: []model.Volume{{
					DriveLetter:       "E",
					FileSystem:        "exFAT",
					HealthStatus:      "Warning",
					OperationalStatus: tc.operational,
					SizeBytes:         1000,
					FreeBytes:         900,
				}},
			}
			fs := Run(s)
			f := findByTitle(fs, "File system exFAT")
			if f == nil {
				t.Fatal("nessun verdetto sul volume")
			}
			if f.Severity != tc.vuoleSev {
				t.Errorf("severità %v, attesa %v", f.Severity, tc.vuoleSev)
			}
			if !strings.Contains(f.Action, tc.azioneHa) {
				t.Errorf("azione %q: manca %q", f.Action, tc.azioneHa)
			}
			if !strings.Contains(f.Action, "E:") && tc.operational != "" {
				t.Errorf("azione %q: deve contenere la lettera del volume", f.Action)
			}
			if !strings.Contains(f.Detail, "giornale") {
				t.Error("su exFAT il dettaglio deve spiegare l'assenza del giornale")
			}
		})
	}
}

func TestSenzaElevazioneNominaISoliDischiAlBuio(t *testing.T) {
	s := model.Snapshot{
		Elevated: false,
		Disks: []model.Disk{
			{Model: "SATA SENZA DATI", MediaType: "HDD", BusType: "SATA"},
			{Model: "NVME LETTO DIRETTAMENTE", MediaType: "SSD", BusType: "NVMe",
				NVMe: &model.NVMeHealth{PercentageUsedPct: 1, PowerOnHours: 1000}},
		},
	}
	f := findByTitle(Run(s), "Analisi parziale")
	if f == nil {
		t.Fatal("con un disco non leggibile l'avviso deve comparire")
	}
	if f.Severity != SevInfo {
		t.Errorf("severità %v: è un limite della diagnosi, non un guasto", f.Severity)
	}
	if !strings.Contains(f.Detail, "SATA SENZA DATI") {
		t.Error("il disco non analizzato va nominato")
	}
	if strings.Contains(f.Detail, "NVME LETTO DIRETTAMENTE") {
		t.Error("un disco letto direttamente non è al buio: dichiararlo tale è falso")
	}
}

// Se ogni disco è stato interrogato direttamente, non c'è alcun limite da
// dichiarare: un avviso che non corrisponde a niente insegna a ignorare tutti
// gli avvisi.
func TestSenzaElevazioneMaTuttoLettoNessunAvviso(t *testing.T) {
	s := model.Snapshot{
		Elevated: false,
		Disks: []model.Disk{
			{Model: "SOLO NVME", MediaType: "SSD", BusType: "NVMe",
				NVMe: &model.NVMeHealth{PercentageUsedPct: 1, PowerOnHours: 1000}},
		},
	}
	if f := findByTitle(Run(s), "Analisi parziale"); f != nil {
		t.Errorf("nessun disco è al buio, l'avviso non deve comparire: %q", f.Title)
	}
}

func TestRegoleNVMe(t *testing.T) {
	base := func(h model.NVMeHealth) model.Snapshot {
		return model.Snapshot{
			Elevated: true,
			Disks: []model.Disk{{
				Model: "NVME DI PROVA", MediaType: "SSD", BusType: "NVMe", NVMe: &h,
			}},
		}
	}

	t.Run("errori di integrità sono critici", func(t *testing.T) {
		f := findByTitle(Run(base(model.NVMeHealth{MediaErrors: 4})), "errori di integrità")
		if f == nil || f.Severity != SevCritical {
			t.Fatalf("atteso CRITICO, ottenuto %v", f)
		}
	})

	t.Run("zero errori non allarma", func(t *testing.T) {
		if f := findByTitle(Run(base(model.NVMeHealth{MediaErrors: 0})), "errori di integrità"); f != nil {
			t.Error("falso positivo su un disco sano")
		}
	})

	t.Run("sola lettura è irreversibile", func(t *testing.T) {
		f := findByTitle(Run(base(model.NVMeHealth{CriticalWarning: 0x08})), "sola lettura")
		if f == nil || f.Severity != SevCritical {
			t.Fatalf("atteso CRITICO, ottenuto %v", f)
		}
	})

	// Il punto della proiezione: la stessa percentuale consumata significa
	// cose opposte a seconda di quanto tempo ci è voluto.
	t.Run("consumo lento non allarma", func(t *testing.T) {
		f := findByTitle(Run(base(model.NVMeHealth{PercentageUsedPct: 1, PowerOnHours: 1437})), "Vita residua")
		if f != nil {
			t.Errorf("1%% in 1437 ore proietta decenni: %q", f.Title)
		}
	})

	t.Run("consumo rapido allarma", func(t *testing.T) {
		f := findByTitle(Run(base(model.NVMeHealth{PercentageUsedPct: 60, PowerOnHours: 6000})), "Vita residua")
		if f == nil || f.Severity != SevWarn {
			t.Fatalf("60%% in 6000 ore lascia meno di un anno: atteso ATTENZIONE, ottenuto %v", f)
		}
	})

	t.Run("minuti sopra soglia termica sono una prova", func(t *testing.T) {
		f := findByTitle(Run(base(model.NVMeHealth{WarningTempTimeMin: 45})), "sopra la soglia")
		if f == nil || f.Severity != SevWarn {
			t.Fatalf("atteso ATTENZIONE, ottenuto %v", f)
		}
	})

	t.Run("spegnimenti anomali citano i volumi corrotti", func(t *testing.T) {
		s := base(model.NVMeHealth{PowerCycles: 100, UnsafeShutdowns: 15})
		s.Volumes = []model.Volume{{
			DriveLetter: "E", FileSystem: "exFAT",
			HealthStatus: "Warning", OperationalStatus: "Full Repair Needed",
			SizeBytes: 1000, FreeBytes: 900,
		}}
		f := findByTitle(Run(s), "spegnimenti anomali")
		if f == nil {
			t.Fatal("15 spegnimenti su 100 devono produrre un verdetto")
		}
		if !strings.Contains(f.Detail, "E:") {
			t.Error("il collegamento col volume corrotto è il valore della regola")
		}
	})
}

func TestRegoleSMART(t *testing.T) {
	disco := func(attrs ...model.SMARTAttribute) model.Snapshot {
		ore := uint64(20000)
		return model.Snapshot{
			Elevated: true,
			Disks: []model.Disk{{
				Model: "HDD DI PROVA", MediaType: "HDD", BusType: "SATA",
				PowerOnHours: &ore,
				SMART:        &model.SMARTData{Attributes: attrs},
			}},
		}
	}
	attr := func(id uint8, raw uint64) model.SMARTAttribute {
		return model.SMARTAttribute{ID: id, Raw: raw, Current: 100, Worst: 100}
	}

	t.Run("settori in attesa sono critici", func(t *testing.T) {
		f := findByTitle(Run(disco(attr(197, 8))), "in attesa di rimappatura")
		if f == nil || f.Severity != SevCritical {
			t.Fatalf("atteso CRITICO, ottenuto %v", f)
		}
	})

	t.Run("pochi settori riallocati avvisano senza allarmare", func(t *testing.T) {
		f := findByTitle(Run(disco(attr(5, 3))), "settori riallocati")
		if f == nil || f.Severity != SevWarn {
			t.Fatalf("atteso ATTENZIONE, ottenuto %v", f)
		}
	})

	t.Run("molti settori riallocati diventano critici", func(t *testing.T) {
		f := findByTitle(Run(disco(attr(5, 120))), "settori riallocati")
		if f == nil || f.Severity != SevCritical {
			t.Fatalf("atteso CRITICO, ottenuto %v", f)
		}
	})

	// La regola che fa risparmiare un disco: il colpevole è il cavo.
	t.Run("errori CRC accusano il cavo non il disco", func(t *testing.T) {
		f := findByTitle(Run(disco(attr(199, 14))), "errori di trasmissione")
		if f == nil {
			t.Fatal("nessun verdetto sugli errori di trasmissione")
		}
		if f.Area != "Collegamento" {
			t.Errorf("area %q: il problema non è nel disco", f.Area)
		}
		if !strings.Contains(f.Action, "cavo") {
			t.Error("l'azione deve indicare il cavo prima della sostituzione del disco")
		}
	})

	t.Run("zero errori non produce verdetti", func(t *testing.T) {
		fs := Run(disco(attr(5, 0), attr(197, 0), attr(199, 0)))
		for _, f := range fs {
			if f.Area == "Disco" || f.Area == "Collegamento" {
				t.Errorf("falso positivo su un disco sano: %q", f.Title)
			}
		}
	})

	// Precedenza: quando i dati diretti ci sono, la regola generica basata sui
	// contatori di Windows deve tacere per non dire due volte la stessa cosa.
	t.Run("i dati diretti hanno la precedenza sui contatori Windows", func(t *testing.T) {
		s := disco(attr(197, 4))
		n := uint64(9)
		s.Disks[0].ReadErrorsUncorr = &n
		if f := findByTitle(Run(s), "non è riuscito a correggere"); f != nil {
			t.Errorf("verdetto duplicato dai contatori Windows: %q", f.Title)
		}
		if f := findByTitle(Run(s), "in attesa di rimappatura"); f == nil {
			t.Error("il verdetto SMART, più preciso, deve restare")
		}
	})
}

// Campione reale di un disco SATA sano (un disco meccanico SATA con 1833 ore).
// Serve soprattutto a bloccare i falsi positivi: su una macchina senza guasti
// il programma deve tacere, ed è la proprietà più facile da perdere man mano
// che si aggiungono regole.
func TestSnapshotSATAReale(t *testing.T) {
	s := loadSnapshot(t, "../../testdata/snapshot-smart.json")
	fs := Run(s)

	t.Run("nessun falso allarme sulla superficie", func(t *testing.T) {
		for _, titolo := range []string{
			"in attesa di rimappatura",
			"settori riallocati",
			"illeggibili e non recuperabili",
			"avvio della rotazione",
		} {
			if f := findByTitle(fs, titolo); f != nil {
				t.Errorf("falso positivo su disco sano: %q", f.Title)
			}
		}
	})

	// Un solo errore sul cavo su 1833 ore: va annotato, non trattato come guasto.
	t.Run("un singolo errore sul cavo resta informativo", func(t *testing.T) {
		f := findByTitle(fs, "di trasmissione sul cavo")
		if f == nil {
			t.Fatal("l'errore va comunque riportato")
		}
		if f.Severity != SevInfo {
			t.Errorf("severità %v: un errore isolato non è un guasto", f.Severity)
		}
	})

	// 15871 parcheggi in 1833 ore: l'attributo 193 rivela un ritmo quasi doppio
	// rispetto a quanto suggerivano i cicli di avvio/arresto di Windows (7018).
	// È la ragione per cui vale la pena leggere il disco invece del sistema.
	t.Run("il parcheggio testine usa l'attributo preciso", func(t *testing.T) {
		f := findByTitle(fs, "Testine parcheggiate")
		if f == nil {
			t.Fatal("8,7 parcheggi l'ora devono produrre un verdetto")
		}
		if !strings.Contains(f.Detail, "15871") {
			t.Errorf("il dettaglio deve citare l'attributo 193, non l'approssimazione: %q", f.Detail)
		}
	})

	t.Run("la regola approssimata tace quando c'è quella precisa", func(t *testing.T) {
		var conteggio int
		for _, f := range fs {
			if strings.Contains(f.Title, "Testine parcheggiate") ||
				strings.Contains(f.Title, "cicli di arresto") {
				conteggio++
			}
		}
		if conteggio > 1 {
			t.Errorf("%d verdetti sullo stesso fenomeno", conteggio)
		}
	})
}

// Lo stesso dato porta a consigli opposti a seconda di come è collegato il
// disco: su un box esterno significa "usa la rimozione sicura", su uno interno
// "controlla l'alimentatore". Sbagliare il consiglio fa perdere credibilità
// più che non darlo affatto.
func TestInterruzioniDiCorrenteDipendonoDalCollegamento(t *testing.T) {
	disco := func(bus string) model.Snapshot {
		return model.Snapshot{
			Elevated: true,
			Disks: []model.Disk{{
				Model: "SSD DI PROVA", MediaType: "SSD", BusType: bus,
				SMART: &model.SMARTData{Attributes: []model.SMARTAttribute{
					{ID: 174, Raw: 32}, {ID: 12, Raw: 33},
				}},
			}},
		}
	}

	t.Run("disco esterno: rimozione sicura", func(t *testing.T) {
		f := findByTitle(Run(disco("USB")), "interruzioni di corrente")
		if f == nil {
			t.Fatal("32 interruzioni su 33 accensioni devono produrre un verdetto")
		}
		if !strings.Contains(f.Action, "Rimozione sicura") {
			t.Errorf("azione %q: su USB la causa è lo scollegamento a caldo", f.Action)
		}
	})

	t.Run("disco interno: alimentazione", func(t *testing.T) {
		f := findByTitle(Run(disco("SATA")), "interruzioni di corrente")
		if f == nil {
			t.Fatal("verdetto mancante sul disco interno")
		}
		if strings.Contains(f.Action, "Rimozione sicura") {
			t.Error("un disco interno non si scollega a caldo: consiglio sbagliato")
		}
		if !strings.Contains(f.Action, "limentatore") {
			t.Errorf("azione %q: dovrebbe indagare l'alimentazione", f.Action)
		}
	})

	t.Run("poche interruzioni non allarmano", func(t *testing.T) {
		s := disco("USB")
		s.Disks[0].SMART.Attributes = []model.SMARTAttribute{{ID: 174, Raw: 1}, {ID: 12, Raw: 500}}
		if f := findByTitle(Run(s), "interruzioni di corrente"); f != nil {
			t.Errorf("falso positivo: %q", f.Title)
		}
	})
}

// Campione di riferimento: una macchina con tutti e tre i tipi di disco letti
// direttamente, ognuno con un protocollo diverso. È il caso più completo che
// abbiamo, e serve a verificare l'insieme: che le regole non si pestino i
// piedi, che i numeri siano accordati e che ogni verdetto dica cosa fare.
func TestSnapshotCompleto(t *testing.T) {
	s := loadSnapshot(t, "../../testdata/snapshot-completo.json")
	fs := Run(s)

	t.Run("tutti e tre i dischi sono stati letti", func(t *testing.T) {
		for _, d := range s.Disks {
			if d.SMART == nil && d.NVMe == nil {
				t.Errorf("%s [%s]: nessuna lettura diretta", d.Model, d.BusType)
			}
		}
		if f := findByTitle(fs, "Analisi parziale"); f != nil {
			t.Error("nessun disco è al buio: l'avviso non deve comparire")
		}
	})

	t.Run("il consiglio sul disco esterno parla di rimozione sicura", func(t *testing.T) {
		f := findByTitle(fs, "interruzioni di corrente")
		if f == nil {
			t.Fatal("32 interruzioni su 33 accensioni devono produrre un verdetto")
		}
		if !strings.Contains(f.Action, "Rimozione sicura") {
			t.Errorf("azione %q: il disco è su USB", f.Action)
		}
	})

	t.Run("i numeri sono accordati col sostantivo", func(t *testing.T) {
		for _, f := range fs {
			if strings.HasPrefix(f.Title, "1 ") {
				for _, plurale := range []string{"errori", "settori", "minuti", "comandi", "tentativi", "cicli"} {
					if strings.Contains(f.Title, "1 "+plurale) {
						t.Errorf("accordo sbagliato: %q", f.Title)
					}
				}
			}
		}
	})

	t.Run("ogni verdetto dice cosa fare", func(t *testing.T) {
		for _, f := range fs {
			if f.Action == "" {
				t.Errorf("%q non indica alcuna azione: è ciò che già fanno gli altri tool", f.Title)
			}
			if f.Detail == "" {
				t.Errorf("%q non spiega su cosa si basa", f.Title)
			}
		}
	})

	t.Run("nessun verdetto duplicato sullo stesso bersaglio", func(t *testing.T) {
		visti := map[string]bool{}
		for _, f := range fs {
			chiave := f.Target + "|" + f.Title
			if visti[chiave] {
				t.Errorf("verdetto ripetuto: %s — %s", f.Target, f.Title)
			}
			visti[chiave] = true
		}
	})
}
