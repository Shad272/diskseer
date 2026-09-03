// Package i18n gestisce le due lingue del programma.
//
// La scelta di progetto: ogni testo è scritto nelle due lingue una accanto
// all'altra, nel punto in cui viene usato, invece che in un catalogo separato
// con delle chiavi.
//
//	Title: l.F("%d unreadable sectors", "%d settori illeggibili", n)
//
// Un catalogo con chiavi è più ordinato in teoria e peggiore in pratica per un
// progetto di questa dimensione: chi modifica una diagnosi deve ricordarsi di
// aggiornare un altro file, e quando se ne dimentica il programma non se ne
// accorge — continua a compilare e a mostrare la vecchia frase nell'altra
// lingua. Qui le due versioni sono sulla stessa riga: dimenticarne una si vede
// mentre la si scrive.
package i18n

import "fmt"

type Lingua int

const (
	EN Lingua = iota
	IT
)

// Da riconosce la lingua da una sigla. Tutto ciò che non è italiano diventa
// inglese: per un progetto pubblico è la scelta che lascia fuori meno gente.
func Da(sigla string) Lingua {
	switch sigla {
	case "it", "IT", "it-IT", "ita":
		return IT
	default:
		return EN
	}
}

func (l Lingua) String() string {
	if l == IT {
		return "it"
	}
	return "en"
}

// S sceglie fra due testi fissi.
func (l Lingua) S(en, it string) string {
	if l == IT {
		return it
	}
	return en
}

// F sceglie fra due testi con dei valori da inserire.
//
// Le due stringhe devono contenere gli stessi segnaposto nello stesso ordine.
// Non è verificabile dal compilatore, ma è verificabile da `go vet`, che
// controlla i formati: un segnaposto in più o in meno viene segnalato.
func (l Lingua) F(en, it string, args ...any) string {
	if l == IT {
		return fmt.Sprintf(it, args...)
	}
	return fmt.Sprintf(en, args...)
}

// N accorda un numero con il sostantivo che lo segue, in entrambe le lingue.
//
// Sembra una rifinitura e invece è sostanza: il referto finisce sotto gli occhi
// di un cliente, e "1 errors" fa sembrare approssimativo tutto quello che c'è
// scritto intorno, comprese le diagnosi giuste.
func (l Lingua) N(n uint64, enSing, enPlur, itSing, itPlur string) string {
	sing, plur := enSing, enPlur
	if l == IT {
		sing, plur = itSing, itPlur
	}
	if n == 1 {
		return fmt.Sprintf("%d %s", n, sing)
	}
	return fmt.Sprintf("%d %s", n, plur)
}
