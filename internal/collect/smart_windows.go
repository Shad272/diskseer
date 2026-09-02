//go:build windows

package collect

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"syscall"

	"github.com/shad272/diskseer/internal/model"
)

// Lettura diretta degli attributi SMART di un disco SATA.
//
// È la stessa idea usata per gli NVMe — si parla al disco invece che a
// Windows — ma il protocollo è di un'altra epoca. Gli NVMe restituiscono una
// pagina con posizioni fisse definite dallo standard. I dischi ATA, che
// vengono dagli anni Novanta, funzionano invece riempendo i registri di un
// controllore: si scrive il comando nei registri, il disco risponde con 512
// byte, e dentro ci sono trenta attributi numerati la cui numerazione è
// standard soltanto per abitudine consolidata.
//
// La ricchezza però è dall'altra parte: un NVMe dice quanti errori ha avuto,
// un disco SATA dice quanti settori sono già stati sostituiti, quanti sono
// illeggibili in questo momento, e quanti errori sono avvenuti sul cavo. Sono
// diagnosi molto più precise, ed è per questo che vale la pena implementarle.

const (
	// Il comando di Windows per farsi restituire i dati SMART di un disco.
	smartRcvDriveData = 0x0007C088

	// I valori da mettere nei registri del disco. Questi numeri sono fissati
	// dallo standard ATA e non hanno alternative: 0xB0 è "comando SMART",
	// 0xD0 è "leggi gli attributi".
	ataCmdSMART            = 0xB0
	smartReadAttributes    = 0xD0
	smartCylinderLow       = 0x4F // firma obbligatoria: senza questi due valori
	smartCylinderHigh      = 0xC2 // il disco rifiuta il comando
	ataDriveHeadDefault    = 0xA0
	smartAttributeDataSize = 512

	// Dimensioni delle strutture Windows. La sottrazione di 1 non è un errore:
	// entrambe finiscono con un array dichiarato di un solo byte, che in realtà
	// ne contiene molti di più. È un modo di scrivere strutture a lunghezza
	// variabile che risale a prima che il C avesse un modo pulito di farlo, e
	// Windows lo usa ancora perché non può rompere la compatibilità.
	sizeSendCmdInParams  = 36 // SENDCMDINPARAMS
	sizeSendCmdOutHeader = 16 // parte fissa di SENDCMDOUTPARAMS, poi i dati
)

// nomiAttributi traduce gli identificativi nei loro significati.
//
// Sono qui e non nelle regole perché servono anche a chi legge il referto
// grezzo: un numero senza nome non aiuta nessuno.
var nomiAttributi = map[uint8]string{
	1:   "Tasso di errori in lettura",
	2:   "Prestazioni di trasferimento",
	3:   "Tempo di avvio della rotazione",
	4:   "Cicli di avvio/arresto",
	5:   "Settori riallocati",
	7:   "Tasso di errori di posizionamento",
	8:   "Prestazioni di posizionamento",
	9:   "Ore di funzionamento",
	10:  "Tentativi falliti di avvio rotazione",
	12:  "Cicli di accensione",
	171: "Scritture fallite su cella",
	172: "Cancellazioni fallite su cella",
	173: "Livellamento dell'usura",
	174: "Interruzioni di corrente impreviste",
	180: "Blocchi di riserva ancora disponibili",
	183: "Riduzioni di velocità del collegamento",
	184: "Errori end-to-end",
	187: "Errori non corretti riportati",
	188: "Comandi scaduti",
	190: "Temperatura del flusso d'aria",
	191: "Urti rilevati",
	192: "Parcheggi di emergenza",
	193: "Cicli di parcheggio testine",
	194: "Temperatura",
	196: "Eventi di riallocazione",
	197: "Settori in attesa di rimappatura",
	198: "Settori illeggibili non rimappabili",
	199: "Errori di trasmissione sul cavo",
	200: "Tasso di errori in scrittura",
	220: "Spostamento dei piatti",
	222: "Ore con testine caricate",
	223: "Tentativi ripetuti di caricamento testine",
	224: "Attrito di caricamento",
	226: "Tempo di caricamento",
	240: "Ore di volo delle testine",
	246: "Settori totali scritti dal sistema",
}

// Gli identificativi sopra 200 sono in larga parte definiti dal costruttore
// del controllore, non da uno standard. Quelli elencati qui sono concordi su
// quasi tutti i dischi in circolazione; gli altri (202, 206, 210, 247-253 e
// simili) cambiano significato da un produttore all'altro, e restano
// volutamente senza nome.
//
// Dare un nome sbagliato a un attributo è peggio che lasciarlo anonimo: un
// numero senza etichetta si va a verificare, un'etichetta sbagliata la si
// crede.

// readSMART interroga il disco numero deviceID.
func readSMART(deviceID string) (*model.SMARTData, error) {
	var risultato *model.SMARTData
	err := suDispositivo(deviceID, func(h syscall.Handle) error {
		var e error
		risultato, e = leggiAttributiSMART(h, deviceID)
		return e
	})
	if err != nil {
		return nil, err
	}
	return risultato, nil
}

func leggiAttributiSMART(h syscall.Handle, deviceID string) (*model.SMARTData, error) {
	path := `\\.\PhysicalDrive` + deviceID

	num, err := strconv.Atoi(deviceID)
	if err != nil {
		return nil, fmt.Errorf("numero disco non valido: %q", deviceID)
	}

	// SENDCMDINPARAMS: la domanda, cioè i registri da caricare nel disco.
	in := make([]byte, sizeSendCmdInParams)
	binary.LittleEndian.PutUint32(in[0:], smartAttributeDataSize) // quanti byte vogliamo
	in[4] = smartReadAttributes                                   // registro Features
	in[5] = 1                                                     // SectorCount
	in[6] = 1                                                     // SectorNumber
	in[7] = smartCylinderLow                                      // CylinderLow
	in[8] = smartCylinderHigh                                     // CylinderHigh
	in[9] = ataDriveHeadDefault                                   // DriveHead
	in[10] = ataCmdSMART                                          // Command
	in[12] = byte(num)                                            // bDriveNumber

	// SENDCMDOUTPARAMS: 16 byte di intestazione, poi i 512 del disco.
	out := make([]byte, sizeSendCmdOutHeader+smartAttributeDataSize)

	var returned uint32
	err = syscall.DeviceIoControl(
		h, smartRcvDriveData,
		&in[0], uint32(len(in)-1),
		&out[0], uint32(len(out)),
		&returned, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("lettura SMART di %s: %w", path, err)
	}

	// L'intestazione riporta l'esito visto dal driver e dal disco. Windows può
	// restituire "operazione riuscita" pur avendo ricevuto un rifiuto dal
	// disco: se questi due byte non sono zero, i 512 byte che seguono non
	// vogliono dire niente e vanno buttati, non interpretati.
	if driverErr, ideErr := out[4], out[5]; driverErr != 0 || ideErr != 0 {
		return nil, fmt.Errorf("il disco %s ha rifiutato il comando SMART (driver=%d, ide=%d)",
			path, driverErr, ideErr)
	}

	attrs := parseSMARTAttributes(out[sizeSendCmdOutHeader:])
	if len(attrs) == 0 {
		return nil, fmt.Errorf("nessun attributo SMART leggibile da %s", path)
	}
	return &model.SMARTData{Attributes: attrs}, nil
}

// parseSMARTAttributes estrae gli attributi dai 512 byte restituiti dal disco.
//
// Il formato: 2 byte di versione, poi 30 attributi da 12 byte ciascuno.
// Ogni attributo è
//
//	byte 0     identificativo
//	byte 1-2   flag di stato
//	byte 3     valore normalizzato attuale
//	byte 4     valore normalizzato peggiore mai raggiunto
//	byte 5-10  conteggio grezzo, 48 bit
//	byte 11    riservato
//
// Un identificativo pari a zero significa "posizione non usata": i dischi
// riempiono solo gli attributi che supportano e lasciano vuoto il resto.
func parseSMARTAttributes(b []byte) []model.SMARTAttribute {
	const (
		inizio     = 2  // i primi due byte sono la versione della tabella
		dimAttr    = 12 // ogni attributo occupa 12 byte
		numMaxAttr = 30 // la tabella ne contiene sempre 30, anche se vuoti
		rawInizio  = 5  // il conteggio grezzo comincia qui
		rawFine    = 10 // e finisce qui: sei byte in tutto
	)

	var out []model.SMARTAttribute
	for i := 0; i < numMaxAttr; i++ {
		off := inizio + i*dimAttr
		if off+dimAttr > len(b) {
			break
		}
		id := b[off]
		if id == 0 {
			continue
		}

		// Il conteggio grezzo occupa i byte 5-10, NON i primi sei: davanti ci
		// sono identificativo, flag e valori normalizzati. Sbagliare questo
		// scostamento non produce alcun errore — produce numeri plausibili e
		// completamente falsi, perché i byte di intestazione finiscono dentro
		// il conteggio. Ci sono cascato alla prima stesura, e me ne sono
		// accorto solo perché "431174464261 settori riallocati" era assurdo:
		// con un attributo meno evidente non l'avrei notato.
		//
		// Non esiste un modo diretto di leggere 48 bit, quindi si ricompone
		// byte per byte dal più significativo al meno.
		var raw uint64
		for j := rawFine; j >= rawInizio; j-- {
			raw = raw<<8 | uint64(b[off+j])
		}

		nome := nomiAttributi[id]
		if nome == "" {
			nome = fmt.Sprintf("Attributo %d", id)
		}

		out = append(out, model.SMARTAttribute{
			ID:      id,
			Name:    nome,
			Current: b[off+3],
			Worst:   b[off+4],
			Raw:     raw,
		})
	}
	return out
}

// enrichSMART completa i dischi non NVMe con quello che Windows nasconde.
//
// Come per gli NVMe, un fallimento è normale e silenzioso: i box USB spesso
// non traducono i comandi ATA, e alcuni controller li rifiutano. Quel disco
// resta descritto com'era, e la regola che dichiara l'analisi parziale se ne
// accorgerà da sola.
func enrichSMART(snap *model.Snapshot) {
	for i := range snap.Disks {
		d := &snap.Disks[i]
		if d.BusType == "NVMe" || d.NVMe != nil {
			continue
		}

		// Prima il comando ATA diretto, che vale per i dischi collegati
		// internamente. Se fallisce, il disco potrebbe essere dentro un box
		// esterno: si riprova imbustando il comando dentro uno SCSI.
		s, err := readSMART(d.DeviceID)
		if err != nil {
			s, err = readSMARTViaSAT(d.DeviceID)
		}
		if err != nil {
			continue
		}
		d.SMART = s

		// I campi generici, quando Windows non li ha forniti, li prendiamo dal
		// disco: così le regole già scritte funzionano anche senza privilegi.
		if d.PowerOnHours == nil {
			if v, ok := s.Raw(model.SMARTPowerOnHours); ok && v > 0 {
				d.PowerOnHours = &v
			}
		}
		if d.TemperatureC == nil {
			if t, ok := temperaturaDaSMART(*s); ok {
				d.TemperatureC = &t
			}
		}
		if d.StartStopCycles == nil {
			// Attributo 4, non 12: il 12 conta le accensioni, il 4 gli avvii del
			// motore. Sono numeri diversi e solo il secondo misura usura meccanica.
			if v, ok := s.Raw(model.SMARTStartStopCount); ok && v > 0 {
				d.StartStopCycles = &v
			}
		}
	}
}

// temperaturaDaSMART estrae i gradi dal conteggio grezzo.
//
// L'attributo temperatura è il caso peggiore di standardizzazione informale:
// alcuni dischi mettono i gradi nel byte più basso e usano gli altri per
// minimo e massimo storici, altri scrivono direttamente il numero. Prendere il
// valore così com'è darebbe temperature di milioni di gradi, quindi si tiene
// solo il byte basso e si scarta ciò che non è fisicamente plausibile.
func temperaturaDaSMART(s model.SMARTData) (int, bool) {
	for _, id := range []uint8{model.SMARTTemperature, model.SMARTAirflowTemp} {
		raw, ok := s.Raw(id)
		if !ok {
			continue
		}
		if t := int(raw & 0xFF); t > 0 && t < 120 {
			return t, true
		}
	}
	return 0, false
}
