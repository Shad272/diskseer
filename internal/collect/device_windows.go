//go:build windows

package collect

import (
	"fmt"
	"syscall"
)

// Apertura dei dischi, condivisa fra la lettura NVMe e quella SMART.
//
// Il primo tentativo si fa chiedendo **zero** permessi. Sembra assurdo aprire
// qualcosa senza chiedere di poterci leggere, ma Windows lo prevede
// esattamente per questo: alcune interrogazioni sono considerate innocue e
// vengono concesse anche a un utente normale. Per gli NVMe funziona, ed è il
// motivo per cui la loro diagnosi non richiede privilegi.
//
// Il comando SMART dei dischi SATA invece viene rifiutato a quel livello: non
// all'apertura, ma dopo, quando lo si manda. È un errore facile da diagnosticare
// male, perché l'apertura riesce e sembra che il problema sia altrove. Per
// questo il tentativo va rifatto per intero con i permessi pieni, non solo
// riaperto.
var livelliDiAccesso = []uint32{
	0, // nessun permesso: sufficiente per molte interrogazioni
	syscall.GENERIC_READ | syscall.GENERIC_WRITE, // richiede l'amministratore
}

func openDevice(path string, access uint32) (syscall.Handle, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	const condivisione = syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE

	h, err := syscall.CreateFile(p, access, condivisione, nil,
		syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return syscall.InvalidHandle, fmt.Errorf("apertura di %s: %w", path, err)
	}
	return h, nil
}

// suDispositivo apre il disco e affida la maniglia alla funzione data,
// riprovando con permessi più alti se il tentativo leggero non basta.
//
// La maniglia viene sempre chiusa, anche quando la funzione fallisce: un
// programma che gira su decine di macchine e dimentica maniglie aperte le
// blocca finché non viene chiuso.
func suDispositivo(deviceID string, fn func(syscall.Handle) error) error {
	path := `\\.\PhysicalDrive` + deviceID

	var ultimo error
	for _, access := range livelliDiAccesso {
		h, err := openDevice(path, access)
		if err != nil {
			ultimo = err
			continue
		}

		err = fn(h)
		syscall.CloseHandle(h)
		if err == nil {
			return nil
		}
		ultimo = err
	}
	return ultimo
}
