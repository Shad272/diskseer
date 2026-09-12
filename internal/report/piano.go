package report

import (
	"io"
	"strings"
)

// Uscita a caratteri semplici, per le console che non sanno disegnare gli altri.
//
// Il programma usa qualche carattere fuori dall'alfabeto inglese: i pallini
// delle gravità, le frecce delle azioni, i separatori, le barre dello spazio
// occupato, il grado delle temperature, e i mezzi blocchi con cui è disegnato
// il disco nel banner.
//
// Su una console con un font raster — il vecchio "Terminal" di Windows, che si
// trova ancora su parecchie macchine — quei caratteri non esistono: escono
// come quadratini, o non escono affatto. Il risultato è un referto pieno di
// buchi, e una temperatura scritta "84 ▫▫" invece di "84 °C" fa sembrare rotto
// il programma proprio mentre sta dando la diagnosi giusta.
//
// La sostituzione avviene qui, in uscita, invece che nei punti in cui il testo
// viene composto. Il motivo è che il carattere del grado non sta solo nelle
// decorazioni: sta dentro le frasi dei verdetti, che sono scritte una volta
// sola e servono anche al referto HTML, dove invece si vedono benissimo. Un
// solo passaggio finale evita di duplicare ogni frase in due versioni.

// versionePiana traduce ciò che una console essenziale non sa disegnare.
//
// Gli accenti ci sono perché il programma parla anche italiano: sulle stesse
// console che perdono i pallini, "più" diventa un quadratino in mezzo a una
// parola. "piu" è brutto, ma è leggibile — ed è un compromesso che si paga
// solo dove l'alternativa sarebbe peggio.
var versionePiana = strings.NewReplacer(
	// Decorazioni e punteggiatura tipografica. I trattini lunghi non sono un
	// vezzo: sono sparsi dentro le frasi dei verdetti, ed è il collaudo ad
	// averli trovati, non la memoria di chi scrive.
	"─", "-",
	"—", "-",
	"–", "-",
	"·", "-",
	"“", "\"",
	"”", "\"",
	"‘", "'",
	"’", "'",
	"«", "\"",
	"»", "\"",
	"×", "x",
	"≥", ">=",
	"≤", "<=",
	"●", "*",
	"▸", ">",
	"→", "->",
	"…", "...",
	"°", "",

	// Mezzi blocchi: il disco del banner e le barre dello spazio occupato.
	// Pieno, metà sopra e metà sotto diventano tre altezze diverse, così il
	// cerchio resta un cerchio e il tracciato del battito resta una linea.
	"█", "#",
	"▀", "-",
	"▄", "_",
	"░", ".",

	// Lettere accentate.
	"à", "a", "á", "a", "è", "e", "é", "e", "ì", "i", "í", "i",
	"ò", "o", "ó", "o", "ù", "u", "ú", "u",
	"À", "A", "È", "E", "É", "E", "Ì", "I", "Ò", "O", "Ù", "U",
)

// Uscita restituisce il flusso su cui stampare.
//
// Con piano a false è esattamente w, senza alcun costo: la stragrande
// maggioranza delle console moderne disegna tutto e non ha bisogno di niente.
func Uscita(w io.Writer, piano bool) io.Writer {
	if !piano {
		return w
	}
	return scrittorePiano{w}
}

type scrittorePiano struct{ w io.Writer }

// Write sostituisce i caratteri e dichiara di aver scritto tutto l'originale.
//
// Il conteggio restituito è quello dei byte ricevuti, non di quelli scritti
// davvero: "→" ne occupa tre e diventa "->" che ne occupa due, e chi ha
// chiamato Write si aspetta indietro la lunghezza di quello che ha passato,
// altrimenti la considera una scrittura incompleta e segnala un errore che non
// c'è.
//
// La sostituzione lavora su ogni singola scrittura. Un carattere spezzato a
// metà fra due chiamate resterebbe intradotto, ma non succede: il programma
// compone ogni riga per intero e poi la stampa in un colpo solo.
func (s scrittorePiano) Write(p []byte) (int, error) {
	if _, err := versionePiana.WriteString(s.w, string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}
