package collect

import "github.com/shad272/diskseer/internal/model"

// Normalize elimina i falsi dati dai contatori grezzi.
//
// Windows non ha un modo per dire "questo contatore non lo so": per i valori
// che il driver non fornisce restituisce zero, esattamente come farebbe per
// un valore realmente pari a zero. Se lo prendessimo alla lettera, il
// programma direbbe "usura 0%, temperatura massima 0 C" su dischi che quei
// dati non li hanno mai comunicati — cioè proprio la bugia che questo
// progetto esiste per non raccontare.
//
// Qui si recupera l'informazione dove è recuperabile con certezza. Dove non
// lo è, il valore resta com'è e il limite va documentato: inventarsi una
// regola per indovinare sarebbe peggio del dato mancante.
func Normalize(s *model.Snapshot) {
	for i := range s.Disks {
		d := &s.Disks[i]

		// Una temperatura massima registrata di 0 C non esiste: nessun disco
		// in funzione è mai stato a zero gradi. È un contatore non fornito.
		if d.TemperatureMaxC != nil && *d.TemperatureMaxC == 0 {
			d.TemperatureMaxC = nil
		}

		// L'usura misura il consumo delle celle di memoria flash: su un disco
		// meccanico non significa niente.
		if d.WearPercent != nil && !isFlash(*d) {
			d.WearPercent = nil
		}

		// Dietro un ponte USB il valore va scartato anche quando il disco è a
		// stato solido. Il motivo è che quel numero non arriva dal disco:
		// arriva dai contatori che Windows raccoglie sul *ponte*, e il ponte
		// non sa nulla dell'usura delle celle che ha dietro. Restituisce zero
		// perché è il riempitivo, non perché il disco sia nuovo.
		//
		// È una sottigliezza che il codice aveva perso nel momento in cui
		// isFlash ha imparato a riconoscere gli SSD dietro i ponti: da lì uno
		// zero privo di significato ha ricominciato a passare per una misura.
		if d.BusType == "USB" {
			d.WearPercent = nil
		}
	}
}

// isFlash distingue i dischi a stato solido, gli unici per cui l'usura ha
// senso. Il tipo dichiarato da Windows è inaffidabile sui box esterni, dove
// arriva spesso come "Unspecified": in quel caso non si tira a indovinare.
func isFlash(d model.Disk) bool {
	if d.MediaType == "SSD" || d.BusType == "NVMe" {
		return true
	}
	// Dietro un ponte USB, Windows dichiara il tipo come "Unspecified" perché
	// vede il ponte, non il disco. Gli attributi SMART però lo tradiscono:
	// scritture fallite su cella e livellamento dell'usura esistono solo sulla
	// memoria flash, un disco meccanico non ha nulla del genere.
	if d.SMART != nil {
		for _, id := range []uint8{171, 172, 173, 174} {
			if _, ok := d.SMART.Raw(id); ok {
				return true
			}
		}
	}
	return false
}
