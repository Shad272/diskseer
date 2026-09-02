//go:build windows

package collect

import (
	"encoding/binary"
	"fmt"
	"syscall"

	"github.com/shad272/diskseer/internal/model"
)

// Lettura diretta della pagina di salute di un disco NVMe.
//
// Perché serve: Windows, tramite Get-StorageReliabilityCounter, per gli NVMe
// non restituisce né le ore di accensione né i contatori di errore. Sono i due
// dati che dicono se un disco sta morendo, e sono proprio quelli che mancano.
// L'unico modo di averli è chiederli al disco.
//
// Come funziona: si apre il disco come se fosse un file (\\.\PhysicalDrive1),
// e gli si manda un "comando di controllo" — DeviceIoControl. Windows non ha
// una funzione per ogni cosa che un dispositivo sa fare: ha una porta di
// servizio unica, dove passi un numero che dice cosa vuoi e un blocco di byte
// costruito secondo un formato preciso. Se sbagli un byte di posizione, non
// ricevi un errore: ricevi dati sbagliati. Per questo ogni offset qui sotto è
// annotato.

const (
	// Il numero del comando. Non è arbitrario: Windows lo compone da
	// dispositivo, funzione e metodo di trasferimento. Questo vale
	// "interroga una proprietà di un dispositivo di archiviazione".
	ioctlStorageQueryProperty = 0x2D1400

	// Cosa chiediamo: dati specifici del protocollo del dispositivo.
	propStorageDeviceProtocolSpecific = 50
	queryTypeStandard                 = 0

	// Il protocollo è NVMe e vogliamo una pagina di log.
	protocolTypeNVMe    = 3
	nvmeDataTypeLogPage = 2

	// Pagina 0x02: "SMART / Health Information". Sempre 512 byte.
	nvmeLogPageHealthInfo = 0x02
	nvmeHealthLogSize     = 512

	// Dimensioni delle strutture Windows, che determinano tutti gli offset.
	sizeProtocolSpecificData   = 40 // STORAGE_PROTOCOL_SPECIFIC_DATA
	sizeProtocolDataDescriptor = 48 // STORAGE_PROTOCOL_DATA_DESCRIPTOR
)

// readNVMeHealth interroga il disco numero deviceID.
//
// Restituisce un errore, e mai una struttura mezza vuota, quando qualcosa non
// funziona: chi chiama lascia il campo a nil e il resto del programma sa che
// il dato non c'è. È lo stesso principio applicato ovunque qui — un dato
// assente non è uno zero.
func readNVMeHealth(deviceID string) (*model.NVMeHealth, error) {
	var risultato *model.NVMeHealth
	err := suDispositivo(deviceID, func(h syscall.Handle) error {
		var e error
		risultato, e = leggiPaginaSalute(h, deviceID)
		return e
	})
	if err != nil {
		return nil, err
	}
	return risultato, nil
}

func leggiPaginaSalute(h syscall.Handle, deviceID string) (*model.NVMeHealth, error) {
	path := `\\.\PhysicalDrive` + deviceID

	// Un unico blocco di memoria fa sia da domanda sia da risposta: prima
	// contiene la richiesta, dopo la chiamata contiene i dati del disco.
	buf := make([]byte, sizeProtocolDataDescriptor+nvmeHealthLogSize)

	// STORAGE_PROPERTY_QUERY: cosa vogliamo sapere.
	binary.LittleEndian.PutUint32(buf[0:], propStorageDeviceProtocolSpecific)
	binary.LittleEndian.PutUint32(buf[4:], queryTypeStandard)

	// STORAGE_PROTOCOL_SPECIFIC_DATA, che parte dal byte 8.
	binary.LittleEndian.PutUint32(buf[8:], protocolTypeNVMe)
	binary.LittleEndian.PutUint32(buf[12:], nvmeDataTypeLogPage)
	binary.LittleEndian.PutUint32(buf[16:], nvmeLogPageHealthInfo)
	binary.LittleEndian.PutUint32(buf[20:], 0) // sotto-valore, non usato qui
	// Dove Windows dovrà scrivere i 512 byte del disco, contati a partire
	// dall'inizio di questa struttura (byte 8): 8 + 40 = 48.
	binary.LittleEndian.PutUint32(buf[24:], sizeProtocolSpecificData)
	binary.LittleEndian.PutUint32(buf[28:], nvmeHealthLogSize)

	var returned uint32
	err := syscall.DeviceIoControl(
		h, ioctlStorageQueryProperty,
		&buf[0], uint32(len(buf)), // domanda
		&buf[0], uint32(len(buf)), // risposta, stesso blocco
		&returned, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("interrogazione NVMe di %s: %w", path, err)
	}

	// Windows dice dove ha messo i dati invece di darlo per scontato: si
	// rilegge da lui, non si assume che sia dove l'abbiamo chiesto.
	dataOffset := binary.LittleEndian.Uint32(buf[24:])
	dataLength := binary.LittleEndian.Uint32(buf[28:])
	start := int(sizeProtocolDataDescriptor - sizeProtocolSpecificData + dataOffset)
	end := start + int(dataLength)
	if dataLength < nvmeHealthLogSize || end > len(buf) {
		return nil, fmt.Errorf("risposta NVMe incompleta da %s: %d byte", path, dataLength)
	}

	return parseHealthLog(buf[start:end]), nil
}

// parseHealthLog legge i 512 byte della pagina di salute.
//
// Gli offset vengono dallo standard NVMe e non sono negoziabili: il byte 5 è
// la percentuale di vita consumata, punto. Ogni riga qui sotto riporta la
// posizione, perché è l'unica documentazione che conta quando un valore esce
// sbagliato.
func parseHealthLog(b []byte) *model.NVMeHealth {
	// Molti contatori sono dichiarati a 128 bit. Prendiamo i 64 bit bassi:
	// per saturarli servirebbero numeri che nessun disco raggiunge.
	u128 := func(off int) uint64 { return binary.LittleEndian.Uint64(b[off:]) }

	h := &model.NVMeHealth{
		CriticalWarning: b[0], // byte 0: guasti dichiarati dal disco

		// byte 1-2: temperatura in kelvin. Sotto lo zero assoluto non si va,
		// quindi un valore minore di 273 significa "sensore non riportato".
		AvailableSparePct:       int(b[3]), // byte 3: celle di riserva rimaste
		AvailableSpareThreshPct: int(b[4]), // byte 4: soglia di allarme
		PercentageUsedPct:       int(b[5]), // byte 5: vita consumata
		DataUnitsRead:           u128(32),  // byte 32-47
		DataUnitsWritten:        u128(48),  // byte 48-63
		PowerCycles:             u128(112), // byte 112-127
		PowerOnHours:            u128(128), // byte 128-143
		UnsafeShutdowns:         u128(144), // byte 144-159
		MediaErrors:             u128(160), // byte 160-175
		ErrorLogEntries:         u128(176), // byte 176-191
		WarningTempTimeMin:      binary.LittleEndian.Uint32(b[192:]),
		CriticalTempTimeMin:     binary.LittleEndian.Uint32(b[196:]),
	}

	if k := binary.LittleEndian.Uint16(b[1:]); k > 273 {
		h.CompositeTempC = int(k) - 273
	}
	return h
}

// enrichNVMe completa i dischi NVMe con quello che Windows non racconta.
//
// Un fallimento non è un problema: significa solo che quel disco resta
// descritto com'era prima. Un box USB che contiene un NVMe, un driver
// particolare o l'assenza di privilegi sono tutti casi normali, e nessuno di
// essi deve impedire la diagnosi del resto della macchina.
func enrichNVMe(snap *model.Snapshot) {
	for i := range snap.Disks {
		d := &snap.Disks[i]
		if d.BusType != "NVMe" {
			continue
		}

		h, err := readNVMeHealth(d.DeviceID)
		if err != nil {
			continue
		}
		d.NVMe = h

		// I campi generici, se Windows non li ha forniti, li riempiamo con
		// quelli letti dal disco: così le regole già scritte funzionano sugli
		// NVMe senza doverle duplicare.
		if d.PowerOnHours == nil && h.PowerOnHours > 0 {
			v := h.PowerOnHours
			d.PowerOnHours = &v
		}
		if d.WearPercent == nil {
			v := h.PercentageUsedPct
			d.WearPercent = &v
		}
		if d.TemperatureC == nil && h.CompositeTempC > 0 {
			v := h.CompositeTempC
			d.TemperatureC = &v
		}
	}
}
