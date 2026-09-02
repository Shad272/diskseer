//go:build windows

package collect

import (
	"encoding/binary"
	"fmt"
	"syscall"

	"github.com/shad272/diskseer/internal/model"
)

// Lettura SMART attraverso un ponte USB-SATA (traduzione SCSI-ATA, "SAT").
//
// Un disco dentro un box esterno non è raggiungibile con i comandi ATA usati
// per i dischi interni: fra il computer e il disco c'è un ponte che parla
// SCSI da un lato e ATA dall'altro. Il comando ATA va quindi imbustato dentro
// un comando SCSI — è letteralmente una busta dentro un'altra busta — e il
// ponte lo srotola e lo consegna al disco.
//
// Funziona solo se il ponte implementa la traduzione. Molti box economici non
// la implementano affatto e restituiscono un rifiuto, alcuni fingono di
// accettarla e rispondono con dati vuoti: per questo il risultato va sempre
// controllato prima di crederci.
//
// Esistono due formati di busta, da 12 e da 16 byte. Nessuno dei due è
// universale: alcuni ponti supportano solo il primo, altri solo il secondo.
// Si provano entrambi.

const (
	// Comando di Windows per inoltrare un comando SCSI a un dispositivo.
	//
	// Esiste anche una variante "DIRECT", più efficiente perché il buffer dei
	// dati resta dove sta. Qui si usa quella normale, che porta i dati dentro
	// lo stesso blocco: la variante diretta richiederebbe di mettere un
	// puntatore a memoria Go dentro una struttura passata al sistema, cosa che
	// le regole di Go sui puntatori sconsigliano.
	ioctlScsiPassThrough = 0x0004D004

	sizeScsiPassThrough = 56 // dimensione della struttura su sistemi a 64 bit
	senseBufferSize     = 32 // dove il dispositivo scrive il motivo di un rifiuto
	senseBufferOffset   = sizeScsiPassThrough
	satDataOffset       = sizeScsiPassThrough + senseBufferSize
	satDataSize         = 512

	scsiDataIn    = 1 // i dati vengono dal dispositivo verso di noi
	satTimeoutSec = 10

	// I due formati di busta previsti dallo standard SAT.
	opAtaPassThrough16 = 0x85
	opAtaPassThrough12 = 0xA1
)

// cdbAtaPassThrough16 costruisce la busta da 16 byte.
//
// I campi sono sparsi in modo poco intuitivo perché lo standard prevede
// registri a 16 bit per la compatibilità con i dischi moderni, e ne mette la
// metà alta prima della metà bassa. Per un comando SMART la metà alta è sempre
// zero, ma va comunque riservata.
func cdbAtaPassThrough16() []byte {
	cdb := make([]byte, 16)
	cdb[0] = opAtaPassThrough16
	cdb[1] = 4 << 1 // protocollo 4: il disco ci manda dati
	// Direzione dal dispositivo, trasferimento a blocchi, lunghezza indicata
	// dal contatore di settori.
	cdb[2] = 0x08 | 0x04 | 0x02
	cdb[4] = smartReadAttributes // registro Features, metà bassa
	cdb[6] = 1                   // un settore
	cdb[8] = 1                   // LBA basso
	cdb[10] = smartCylinderLow   // firma obbligatoria del comando SMART
	cdb[12] = smartCylinderHigh
	cdb[13] = ataDriveHeadDefault
	cdb[14] = ataCmdSMART
	return cdb
}

// cdbAtaPassThrough12 costruisce la busta da 12 byte, che ha gli stessi campi
// senza le metà alte dei registri.
func cdbAtaPassThrough12() []byte {
	cdb := make([]byte, 12)
	cdb[0] = opAtaPassThrough12
	cdb[1] = 4 << 1
	cdb[2] = 0x08 | 0x04 | 0x02
	cdb[3] = smartReadAttributes
	cdb[4] = 1
	cdb[5] = 1
	cdb[6] = smartCylinderLow
	cdb[7] = smartCylinderHigh
	cdb[8] = ataDriveHeadDefault
	cdb[9] = ataCmdSMART
	return cdb
}

// readSMARTViaSAT interroga un disco dietro un ponte USB.
func readSMARTViaSAT(deviceID string) (*model.SMARTData, error) {
	var risultato *model.SMARTData

	err := suDispositivo(deviceID, func(h syscall.Handle) error {
		var ultimo error
		// I due formati non sono intercambiabili e nessuno è universale:
		// si prova il più moderno e si ripiega sull'altro.
		for _, cdb := range [][]byte{cdbAtaPassThrough16(), cdbAtaPassThrough12()} {
			s, err := inviaComandoSAT(h, cdb)
			if err == nil {
				risultato = s
				return nil
			}
			ultimo = err
		}
		return ultimo
	})
	if err != nil {
		return nil, err
	}
	return risultato, nil
}

func inviaComandoSAT(h syscall.Handle, cdb []byte) (*model.SMARTData, error) {
	buf := make([]byte, satDataOffset+satDataSize)

	binary.LittleEndian.PutUint16(buf[0:], sizeScsiPassThrough)
	buf[6] = byte(len(cdb))                                // CdbLength
	buf[7] = senseBufferSize                               // SenseInfoLength
	buf[8] = scsiDataIn                                    // DataIn
	binary.LittleEndian.PutUint32(buf[12:], satDataSize)   // DataTransferLength
	binary.LittleEndian.PutUint32(buf[16:], satTimeoutSec) // TimeOutValue
	// DataBufferOffset è uno scostamento dall'inizio di questa struttura, non
	// un puntatore: è il motivo per cui questa variante è più semplice da usare
	// da Go in sicurezza.
	binary.LittleEndian.PutUint64(buf[24:], satDataOffset)
	binary.LittleEndian.PutUint32(buf[32:], senseBufferOffset)
	copy(buf[36:52], cdb)

	var returned uint32
	err := syscall.DeviceIoControl(
		h, ioctlScsiPassThrough,
		&buf[0], uint32(len(buf)),
		&buf[0], uint32(len(buf)),
		&returned, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("comando SAT (%d byte) rifiutato: %w", len(cdb), err)
	}

	// ScsiStatus diverso da zero significa che il ponte o il disco hanno
	// rifiutato il comando. Windows in questo caso non segnala alcun errore:
	// la chiamata riesce, ed è la struttura a contenere il rifiuto.
	if stato := buf[2]; stato != 0 {
		return nil, fmt.Errorf("comando SAT (%d byte) respinto dal dispositivo: stato SCSI %d",
			len(cdb), stato)
	}

	attrs := parseSMARTAttributes(buf[satDataOffset:])
	if len(attrs) == 0 {
		// Alcuni ponti accettano il comando e restituiscono un blocco vuoto
		// invece di dichiarare che non sanno tradurlo. Va trattato come un
		// fallimento, non come "disco senza attributi".
		return nil, fmt.Errorf("il ponte ha accettato il comando SAT (%d byte) ma non ha restituito dati",
			len(cdb))
	}
	return &model.SMARTData{Attributes: attrs}, nil
}
