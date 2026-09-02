// Package model contiene i tipi condivisi fra raccolta dati e motore di regole.
//
// Regola di progetto: ogni valore che il sistema può non fornire è un
// puntatore. "Dato assente" e "dato pari a zero" sono cose diverse: un disco
// con 0 errori è sano, un disco di cui non conosciamo gli errori non lo
// sappiamo. Confonderli è il modo più rapido per dare una diagnosi falsa.
package model

import "time"

type Snapshot struct {
	Time     time.Time `json:"time"`
	Elevated bool      `json:"elevated"`
	System   System    `json:"system"`
	Disks    []Disk    `json:"disks"`
	Volumes  []Volume  `json:"volumes"`
	Battery  *Battery  `json:"battery,omitempty"`
	Thermals []Thermal `json:"thermals,omitempty"`
}

type System struct {
	Manufacturer string    `json:"manufacturer"`
	Model        string    `json:"model"`
	OS           string    `json:"os"`
	OSVersion    string    `json:"osVersion"`
	CPU          string    `json:"cpu"`
	Cores        int       `json:"cores"`
	Threads      int       `json:"threads"`
	RAMBytes     uint64    `json:"ramBytes"`
	Chassis      string    `json:"chassis"`
	LastBoot     time.Time `json:"lastBoot"`
}

type Disk struct {
	DeviceID     string `json:"deviceId"`
	Model        string `json:"model"`
	MediaType    string `json:"mediaType"` // HDD, SSD, Unspecified
	BusType      string `json:"busType"`   // SATA, NVMe, USB
	HealthStatus string `json:"healthStatus"`
	SizeBytes    uint64 `json:"sizeBytes"`
	IsSystemDisk bool   `json:"isSystemDisk"`

	// Contatori di affidabilità: richiedono privilegi di amministratore.
	// Restano nil quando il tool gira senza elevazione.
	TemperatureC     *int    `json:"temperatureC,omitempty"`
	TemperatureMaxC  *int    `json:"temperatureMaxC,omitempty"`
	WearPercent      *int    `json:"wearPercent,omitempty"`
	PowerOnHours     *uint64 `json:"powerOnHours,omitempty"`
	StartStopCycles  *uint64 `json:"startStopCycles,omitempty"`
	ReadErrorsTotal  *uint64 `json:"readErrorsTotal,omitempty"`
	ReadErrorsUncorr *uint64 `json:"readErrorsUncorrected,omitempty"`
	WriteErrTotal    *uint64 `json:"writeErrorsTotal,omitempty"`
	WriteErrUncorr   *uint64 `json:"writeErrorsUncorrected,omitempty"`

	// NVMe è popolato solo per i dischi NVMe interrogati con successo.
	NVMe *NVMeHealth `json:"nvme,omitempty"`

	// SMART è popolato solo per i dischi SATA interrogati con successo.
	SMART *SMARTData `json:"smart,omitempty"`
}

type Volume struct {
	DriveLetter  string `json:"driveLetter"`
	FileSystem   string `json:"fileSystem"`
	HealthStatus string `json:"healthStatus"`
	// OperationalStatus dice *quale* riparazione serve: e' la differenza fra
	// segnalare un problema e saper dire cosa farci.
	OperationalStatus string `json:"operationalStatus,omitempty"`
	SizeBytes         uint64 `json:"sizeBytes"`
	FreeBytes         uint64 `json:"freeBytes"`
}

func (v Volume) FreePercent() float64 {
	if v.SizeBytes == 0 {
		return 0
	}
	return float64(v.FreeBytes) / float64(v.SizeBytes) * 100
}

type Battery struct {
	Name           string `json:"name"`
	ChargePercent  *int   `json:"chargePercent,omitempty"`
	DesignCapacity *int   `json:"designCapacity,omitempty"`
	FullCapacity   *int   `json:"fullCapacity,omitempty"`
	CycleCount     *int   `json:"cycleCount,omitempty"`
}

// HealthPercent restituisce la capacità residua rispetto a quella di
// progetto, e false se i dati non bastano a calcolarla.
func (b Battery) HealthPercent() (float64, bool) {
	if b.DesignCapacity == nil || b.FullCapacity == nil || *b.DesignCapacity <= 0 {
		return 0, false
	}
	return float64(*b.FullCapacity) / float64(*b.DesignCapacity) * 100, true
}

type Thermal struct {
	Name    string  `json:"name"`
	Celsius float64 `json:"celsius"`
}

// NVMeHealth è la pagina di log 0x02 "SMART / Health Information", letta
// parlando direttamente al disco.
//
// Windows, tramite Get-StorageReliabilityCounter, per gli NVMe non fornisce
// né ore di accensione né contatori di errore: proprio i due dati che dicono
// se un disco sta morendo. Questa struttura li recupera alla fonte.
//
// Lo standard NVMe definisce molti di questi contatori a 128 bit. Nella
// pratica nessun disco arriva lontanamente a saturare 64 bit, quindi teniamo
// solo la metà bassa: l'alternativa sarebbe portarsi dietro un tipo a 128 bit
// per un caso che non si verifica.
type NVMeHealth struct {
	// CriticalWarning è una maschera di bit: ogni bit acceso è un guasto che
	// il disco sta dichiarando da solo. Vedi i metodi qui sotto.
	CriticalWarning uint8 `json:"criticalWarning"`

	CompositeTempC          int `json:"compositeTempC"`
	AvailableSparePct       int `json:"availableSparePct"`
	AvailableSpareThreshPct int `json:"availableSpareThresholdPct"`
	PercentageUsedPct       int `json:"percentageUsedPct"`

	DataUnitsRead    uint64 `json:"dataUnitsRead"`
	DataUnitsWritten uint64 `json:"dataUnitsWritten"`
	PowerCycles      uint64 `json:"powerCycles"`
	PowerOnHours     uint64 `json:"powerOnHours"`
	UnsafeShutdowns  uint64 `json:"unsafeShutdowns"`

	// MediaErrors sono errori di integrità che il disco non ha potuto
	// correggere: è il contatore che su SATA chiamiamo "errori non corretti",
	// ed è il segnale più forte che un disco sta cedendo.
	MediaErrors     uint64 `json:"mediaErrors"`
	ErrorLogEntries uint64 `json:"errorLogEntries"`

	// Minuti passati sopra la soglia di allarme termico. Un valore diverso da
	// zero è la prova che il disco ha ridotto le prestazioni per surriscaldamento,
	// anche se adesso è freddo.
	WarningTempTimeMin  uint32 `json:"warningTempTimeMinutes"`
	CriticalTempTimeMin uint32 `json:"criticalTempTimeMinutes"`
}

// I bit di CriticalWarning, come definiti dallo standard NVMe.
const (
	nvmeWarnSpareBelowThreshold = 1 << 0
	nvmeWarnTemperature         = 1 << 1
	nvmeWarnReliabilityDegraded = 1 << 2
	nvmeWarnReadOnly            = 1 << 3
	nvmeWarnBackupFailed        = 1 << 4
)

func (h NVMeHealth) SpareBelowThreshold() bool {
	return h.CriticalWarning&nvmeWarnSpareBelowThreshold != 0
}
func (h NVMeHealth) TemperatureAlarm() bool { return h.CriticalWarning&nvmeWarnTemperature != 0 }
func (h NVMeHealth) ReliabilityDegraded() bool {
	return h.CriticalWarning&nvmeWarnReliabilityDegraded != 0
}
func (h NVMeHealth) ReadOnly() bool     { return h.CriticalWarning&nvmeWarnReadOnly != 0 }
func (h NVMeHealth) BackupFailed() bool { return h.CriticalWarning&nvmeWarnBackupFailed != 0 }

// TerabyteScritti converte le "unità dati" NVMe in terabyte.
// Lo standard fissa un'unità a 1000 blocchi da 512 byte.
func (h NVMeHealth) TerabyteScritti() float64 {
	return float64(h.DataUnitsWritten) * 512 * 1000 / 1e12
}

// SMARTAttribute è un singolo attributo SMART di un disco SATA.
//
// A differenza degli NVMe, dove ogni contatore sta in una posizione fissa
// definita dallo standard, i dischi SATA espongono una lista di attributi
// numerati. Il significato del numero è standardizzato di fatto ma non di
// diritto: 197 è "settori in attesa di rimappatura" su qualunque disco in
// circolazione, ma nessun documento obbliga il costruttore a rispettarlo.
type SMARTAttribute struct {
	ID   uint8  `json:"id"`
	Name string `json:"name"`

	// Current è il valore normalizzato: parte da 100 (o 200) su disco nuovo e
	// scende avvicinandosi al guasto. È il costruttore a decidere la scala,
	// quindi confrontarlo fra dischi diversi non ha senso.
	Current uint8 `json:"current"`
	Worst   uint8 `json:"worst"`

	// Raw è il conteggio vero: quanti settori, quante ore, quanti errori.
	// È questo che serve a diagnosticare, non il valore normalizzato.
	Raw uint64 `json:"raw"`
}

type SMARTData struct {
	Attributes []SMARTAttribute `json:"attributes"`
}

// Raw restituisce il conteggio grezzo di un attributo, e false se il disco non
// espone quell'attributo. Ancora una volta: assente e zero non sono la stessa
// cosa, e qui la differenza è fra "nessun settore danneggiato" e "questo disco
// non sa dirci se ha settori danneggiati".
func (s SMARTData) Raw(id uint8) (uint64, bool) {
	for _, a := range s.Attributes {
		if a.ID == id {
			return a.Raw, true
		}
	}
	return 0, false
}

func (s SMARTData) Get(id uint8) (SMARTAttribute, bool) {
	for _, a := range s.Attributes {
		if a.ID == id {
			return a, true
		}
	}
	return SMARTAttribute{}, false
}

// Gli identificativi degli attributi che contano per la diagnosi.
const (
	SMARTReallocatedSectors = 5   // settori già sostituiti con quelli di scorta
	SMARTStartStopCount     = 4   // avvii e arresti del motore
	SMARTPowerOnHours       = 9   // ore di funzionamento
	SMARTSpinRetry          = 10  // tentativi falliti di avviare la rotazione
	SMARTPowerCycles        = 12  // accensioni
	SMARTReportedUncorrect  = 187 // errori che il disco non ha corretto
	SMARTCommandTimeout     = 188 // comandi scaduti senza risposta
	SMARTAirflowTemp        = 190 // temperatura del flusso d'aria
	SMARTLoadCycles         = 193 // parcheggi delle testine
	SMARTTemperature        = 194 // temperatura
	SMARTPendingSectors     = 197 // settori illeggibili in attesa di rimappatura
	SMARTOfflineUncorrect   = 198 // settori illeggibili e non rimappabili
	SMARTCRCErrors          = 199 // errori di trasmissione sul cavo
)
