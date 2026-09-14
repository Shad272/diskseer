//go:build windows

package collect

import (
	"context"
	"syscall"
	"unsafe"

	"github.com/shad272/diskseer/internal/model"
	"github.com/shad272/diskseer/internal/platform"
	"github.com/shad272/diskseer/internal/tuning"
)

// Aggiornamento rapido dei soli valori che cambiano nel tempo.
//
// Collect impiega un paio di secondi perché avvia PowerShell per farsi dare
// l'inventario della macchina. In un ciclo che si ripete ogni pochi secondi
// sarebbe insostenibile, e per giunta inutile: marca, modello e capacità dei
// dischi non cambiano mentre il programma è acceso.
//
// Cambiano tre cose: le temperature, i contatori interni dei dischi e lo
// spazio libero sui volumi. Tutte e tre si leggono con chiamate diritte al
// sistema, in millisecondi. Questo file legge quelle e lascia stare il resto.

var (
	procGetDiskFreeSpaceExW = kernel32Refresh.NewProc("GetDiskFreeSpaceExW")
	procGetDriveTypeW       = kernel32Refresh.NewProc("GetDriveTypeW")
	kernel32Refresh         = syscall.NewLazyDLL("kernel32.dll")
)

// Refresh riporta uno snapshot già raccolto al valore attuale.
//
// Modifica lo snapshot sul posto invece di restituirne uno nuovo: chi lo
// chiama ha già in mano l'inventario, e ricostruirlo da zero significherebbe
// pagare di nuovo i due secondi che stiamo cercando di evitare.
func Refresh(s *model.Snapshot) {
	p, _ := tuning.Select(platform.Detect(), "auto", 0)
	_ = RefreshContext(context.Background(), s, p)
}

func RefreshContext(ctx context.Context, s *model.Snapshot, p tuning.Profile) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	aggiornaSpazioLibero(ctx, s)

	// Le due letture dirette dei dischi rifanno tutto il lavoro, ma sono
	// chiamate di sistema che costano microsecondi: non vale la pena
	// distinguere quali campi siano cambiati.
	hdd := false
	for _, d := range s.Disks {
		if d.IsSystemDisk && (d.MediaType == "HDD" || d.MediaType == "Unspecified") {
			hdd = true
		}
	}
	current, _ := tuning.Select(platform.Detect(), p.Name, p.Workers)
	parallel(ctx, len(s.Disks), current.DiskWorkers(len(s.Disks), hdd), func(i int) {
		d := &s.Disks[i]
		d.ReadError = ""
		if d.BusType == "NVMe" {
			if h, err := readNVMeHealth(d.DeviceID); err == nil {
				aggiornaDaNVMe(d, h)
			} else {
				d.ReadError = "unavailable"
			}
			return
		}
		sm, err := readSMART(d.DeviceID)
		if err != nil {
			sm, err = readSMARTViaSAT(d.DeviceID)
		}
		if err == nil {
			aggiornaDaSMART(d, sm)
		} else {
			d.ReadError = "unavailable"
		}
	})

	Normalize(s)
	return ctx.Err()
}

// aggiornaDaNVMe sovrascrive, mentre enrichNVMe riempiva solo i campi vuoti.
//
// La differenza è voluta. In fase di raccolta il dubbio è "questo dato lo
// abbiamo già da Windows?", e non si sovrascrive per non perdere informazioni.
// In fase di aggiornamento il dubbio non esiste: il valore appena letto dal
// disco è quello di adesso, e quello vecchio è, per definizione, vecchio.
func aggiornaDaNVMe(d *model.Disk, h *model.NVMeHealth) {
	d.NVMe = h
	if h.CompositeTempC > 0 {
		t := h.CompositeTempC
		d.TemperatureC = &t
	}
	if h.PowerOnHours > 0 {
		v := h.PowerOnHours
		d.PowerOnHours = &v
	}
	u := h.PercentageUsedPct
	d.WearPercent = &u
}

func aggiornaDaSMART(d *model.Disk, s *model.SMARTData) {
	d.SMART = s
	if t, ok := temperaturaDaSMART(*s); ok {
		d.TemperatureC = &t
	}
	if v, ok := s.Raw(model.SMARTPowerOnHours); ok && v > 0 {
		d.PowerOnHours = &v
	}
	if v, ok := s.Raw(model.SMARTStartStopCount); ok && v > 0 {
		d.StartStopCycles = &v
	}
}

// aggiornaSpazioLibero chiede a Windows quanto spazio resta su ogni volume.
//
// È l'unico dato di questo ciclo che cambia da un secondo all'altro per opera
// dell'utente, ed è anche il più economico da leggere: una chiamata per
// volume, senza processi da avviare né dispositivi da aprire.
func aggiornaSpazioLibero(ctx context.Context, s *model.Snapshot) {
	for i := range s.Volumes {
		if ctx.Err() != nil {
			return
		}
		v := &s.Volumes[i]
		v.ReadError = ""
		if v.DriveLetter == "" {
			continue
		}
		if len(v.DriveLetter) != 1 || !((v.DriveLetter[0] >= 'A' && v.DriveLetter[0] <= 'Z') || (v.DriveLetter[0] >= 'a' && v.DriveLetter[0] <= 'z')) {
			v.ReadError = "unavailable"
			continue
		}
		percorso, err := syscall.UTF16PtrFromString(v.DriveLetter + `:\`)
		if err != nil {
			continue
		}
		typeID, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(percorso)))
		if typeID != 2 && typeID != 3 {
			v.ReadError = "unavailable"
			continue
		}

		// Il primo valore è lo spazio disponibile all'utente corrente, che su
		// un volume con quote impostate è minore di quello libero in assoluto.
		// A noi serve il secondo e il terzo: quanto è grande il volume e
		// quanto ne resta davvero.
		var disponibileUtente, totale, libero uint64
		r, _, _ := procGetDiskFreeSpaceExW.Call(
			uintptr(unsafe.Pointer(percorso)),
			uintptr(unsafe.Pointer(&disponibileUtente)),
			uintptr(unsafe.Pointer(&totale)),
			uintptr(unsafe.Pointer(&libero)),
		)
		if r == 0 || totale == 0 {
			v.ReadError = "unavailable"
			// Volume rimosso o non pronto: si lasciano i valori precedenti
			// invece di azzerarli, altrimenti le regole sullo spazio
			// segnalerebbero un disco pieno che non esiste più.
			continue
		}
		v.SizeBytes = totale
		v.FreeBytes = libero
	}
}
