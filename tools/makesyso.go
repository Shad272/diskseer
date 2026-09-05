//go:build ignore

// Genera il file di risorse che dà all'eseguibile la sua icona.
//
//	go run tools/makesyso.go assets/diskseer.ico rsrc_windows_amd64.syso
//
// Perché serve un programma apposta. Windows non prende l'icona da un file
// accanto all'eseguibile: la vuole *dentro*, in una sezione di risorse. Il
// compilatore Go non sa costruirla, ma il collegatore raccoglie da solo
// qualsiasi file .syso trovi nella cartella del pacchetto principale. Un .syso
// è un file oggetto in formato COFF, lo stesso che produce un compilatore C.
//
// Di norma si usa uno strumento esterno. Qui no: aggiungerebbe una dipendenza
// di compilazione a un progetto che non ne ha nessuna, per una cosa che si
// scrive una volta e non cambia più. Il file prodotto va messo sotto controllo
// di versione, così chi clona il progetto compila l'icona senza avere questo
// generatore né sapere che esiste.
//
// Struttura di ciò che viene scritto:
//
//	intestazione COFF
//	intestazione della sezione .rsrc
//	albero delle risorse   tipo -> nome -> lingua -> descrittore
//	dati grezzi            le immagini prese dal file .ico
//	rilocazioni            una per descrittore
//	tavola dei simboli     il simbolo della sezione
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

// Identificativi dei tipi di risorsa, fissati da Windows.
const (
	rtIcon      = 3
	rtGroupIcon = 14

	// Lingua della risorsa. 1033 è l'inglese americano: per un'icona la lingua
	// non significa niente, ma il campo esiste e va riempito con qualcosa che
	// Windows si aspetti di trovare.
	langNeutral = 1033
)

// Dimensioni delle strutture dell'albero delle risorse.
const (
	dimDirectory  = 16
	dimEntry      = 8
	dimDataEntry  = 16
	allineamento  = 8
	numeroSezioni = 1
)

type immagine struct {
	larghezza, altezza byte
	colori, riservato  byte
	piani, bitPerPixel uint16
	dati               []byte
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "uso: go run tools/makesyso.go <icona.ico> <uscita.syso>")
		os.Exit(2)
	}

	grezzo, err := os.ReadFile(os.Args[1])
	if err != nil {
		esci(err)
	}
	immagini, err := leggiIco(grezzo)
	if err != nil {
		esci(err)
	}

	syso, err := componiSyso(immagini)
	if err != nil {
		esci(err)
	}
	if err := os.WriteFile(os.Args[2], syso, 0o644); err != nil {
		esci(err)
	}
	fmt.Printf("%s: %d immagini, %d byte\n", os.Args[2], len(immagini), len(syso))
}

func esci(err error) {
	fmt.Fprintln(os.Stderr, "makesyso:", err)
	os.Exit(1)
}

// leggiIco estrae le immagini dal file .ico.
//
// Il formato è semplice: sei byte di intestazione, poi una voce di indice da
// sedici byte per ogni risoluzione, poi le immagini. L'indice dice dove ognuna
// comincia e quanto è lunga.
func leggiIco(b []byte) ([]immagine, error) {
	if len(b) < 6 {
		return nil, fmt.Errorf("file troppo corto per essere un'icona")
	}
	if tipo := binary.LittleEndian.Uint16(b[2:]); tipo != 1 {
		return nil, fmt.Errorf("tipo %d: non è un file di icone", tipo)
	}
	n := int(binary.LittleEndian.Uint16(b[4:]))
	if n == 0 {
		return nil, fmt.Errorf("l'icona non contiene immagini")
	}

	out := make([]immagine, 0, n)
	for i := 0; i < n; i++ {
		off := 6 + i*16
		if off+16 > len(b) {
			return nil, fmt.Errorf("indice troncato alla voce %d", i)
		}
		lung := int(binary.LittleEndian.Uint32(b[off+8:]))
		inizio := int(binary.LittleEndian.Uint32(b[off+12:]))
		if inizio+lung > len(b) {
			return nil, fmt.Errorf("immagine %d fuori dai limiti del file", i)
		}
		out = append(out, immagine{
			larghezza:   b[off],
			altezza:     b[off+1],
			colori:      b[off+2],
			riservato:   b[off+3],
			piani:       binary.LittleEndian.Uint16(b[off+4:]),
			bitPerPixel: binary.LittleEndian.Uint16(b[off+6:]),
			dati:        b[inizio : inizio+lung],
		})
	}
	return out, nil
}

// gruppoIcone costruisce il descrittore che lega insieme le immagini.
//
// È quasi identico all'indice del file .ico, con una differenza cruciale: al
// posto della posizione nel file, ogni voce porta il *numero* della risorsa
// corrispondente. Dentro l'eseguibile le immagini non stanno più una dopo
// l'altra, e Windows le ritrova per numero.
func gruppoIcone(immagini []immagine) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, uint16(0))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(len(immagini)))
	for i, im := range immagini {
		b.WriteByte(im.larghezza)
		b.WriteByte(im.altezza)
		b.WriteByte(im.colori)
		b.WriteByte(im.riservato)
		binary.Write(&b, binary.LittleEndian, im.piani)
		binary.Write(&b, binary.LittleEndian, im.bitPerPixel)
		binary.Write(&b, binary.LittleEndian, uint32(len(im.dati)))
		binary.Write(&b, binary.LittleEndian, uint16(i+1)) // numero della risorsa
	}
	return b.Bytes()
}

// risorsa è una foglia dell'albero: un blocco di dati con il suo tipo e numero.
type risorsa struct {
	tipo, nome uint32
	dati       []byte
}

func componiSyso(immagini []immagine) ([]byte, error) {
	risorse := make([]risorsa, 0, len(immagini)+1)
	for i, im := range immagini {
		risorse = append(risorse, risorsa{tipo: rtIcon, nome: uint32(i + 1), dati: im.dati})
	}
	risorse = append(risorse, risorsa{tipo: rtGroupIcon, nome: 1, dati: gruppoIcone(immagini)})

	sezione, posizioniDati := componiSezione(risorse)

	// Ogni descrittore contiene l'indirizzo dei propri dati, che si conosce
	// solo a collegamento avvenuto. Nel file oggetto quel campo resta relativo
	// all'inizio della sezione, e una rilocazione dice al collegatore di
	// sommarci l'indirizzo definitivo.
	var reloc bytes.Buffer
	for _, p := range posizioniDati {
		binary.Write(&reloc, binary.LittleEndian, uint32(p))
		binary.Write(&reloc, binary.LittleEndian, uint32(0)) // simbolo .rsrc
		binary.Write(&reloc, binary.LittleEndian, uint16(3)) // ADDR32NB
	}

	const dimIntestazioneCoff = 20
	const dimIntestazioneSezione = 40
	inizioSezione := dimIntestazioneCoff + dimIntestazioneSezione*numeroSezioni
	inizioReloc := inizioSezione + len(sezione)
	inizioSimboli := inizioReloc + reloc.Len()

	var out bytes.Buffer

	// Intestazione COFF.
	binary.Write(&out, binary.LittleEndian, uint16(0x8664)) // macchina: x86-64
	binary.Write(&out, binary.LittleEndian, uint16(numeroSezioni))
	binary.Write(&out, binary.LittleEndian, uint32(0)) // data: irrilevante, e lasciarla a zero rende il file riproducibile
	binary.Write(&out, binary.LittleEndian, uint32(inizioSimboli))
	binary.Write(&out, binary.LittleEndian, uint32(2)) // simboli: quello di sezione più il suo ausiliario
	binary.Write(&out, binary.LittleEndian, uint16(0)) // nessuna intestazione facoltativa
	binary.Write(&out, binary.LittleEndian, uint16(0))

	// Intestazione della sezione .rsrc.
	out.Write([]byte(".rsrc\x00\x00\x00"))
	binary.Write(&out, binary.LittleEndian, uint32(0)) // dimensione virtuale
	binary.Write(&out, binary.LittleEndian, uint32(0)) // indirizzo virtuale
	binary.Write(&out, binary.LittleEndian, uint32(len(sezione)))
	binary.Write(&out, binary.LittleEndian, uint32(inizioSezione))
	binary.Write(&out, binary.LittleEndian, uint32(inizioReloc))
	binary.Write(&out, binary.LittleEndian, uint32(0)) // niente numeri di riga
	binary.Write(&out, binary.LittleEndian, uint16(len(posizioniDati)))
	binary.Write(&out, binary.LittleEndian, uint16(0))
	binary.Write(&out, binary.LittleEndian, uint32(0x40000040)) // dati inizializzati, sola lettura

	out.Write(sezione)
	out.Write(reloc.Bytes())

	// Tavola dei simboli: uno solo, quello che rappresenta la sezione stessa,
	// a cui le rilocazioni fanno riferimento.
	out.Write([]byte(".rsrc\x00\x00\x00"))
	binary.Write(&out, binary.LittleEndian, uint32(0)) // valore
	binary.Write(&out, binary.LittleEndian, uint16(1)) // sezione numero 1
	binary.Write(&out, binary.LittleEndian, uint16(0)) // tipo
	out.WriteByte(3)                                   // classe: statico
	out.WriteByte(1)                                   // un simbolo ausiliario segue

	// Simbolo ausiliario di sezione: descrive la sezione a cui il precedente
	// si riferisce.
	binary.Write(&out, binary.LittleEndian, uint32(len(sezione)))
	binary.Write(&out, binary.LittleEndian, uint16(len(posizioniDati)))
	binary.Write(&out, binary.LittleEndian, uint16(0))
	binary.Write(&out, binary.LittleEndian, uint32(0))
	binary.Write(&out, binary.LittleEndian, uint16(0))
	out.WriteByte(0)
	out.Write(make([]byte, 3))

	// Tavola delle stringhe: vuota, ma i quattro byte della sua lunghezza
	// devono esserci comunque.
	binary.Write(&out, binary.LittleEndian, uint32(4))

	return out.Bytes(), nil
}

// componiSezione costruisce l'albero delle risorse e restituisce, insieme ai
// byte, le posizioni dei campi che il collegatore dovrà correggere.
//
// L'albero ha tre livelli fissi — tipo, nome, lingua — e le voci di ogni
// livello vanno ordinate per numero crescente, altrimenti Windows, che le
// cerca per bisezione, non le trova.
func componiSezione(risorse []risorsa) ([]byte, []int) {
	tipi := []uint32{rtIcon, rtGroupIcon}
	perTipo := map[uint32][]risorsa{}
	for _, r := range risorse {
		perTipo[r.tipo] = append(perTipo[r.tipo], r)
	}

	// Prima passata: si calcola dove finisce ogni parte, perché ogni livello
	// deve contenere lo scostamento del successivo.
	offRadice := 0
	offNomi := dimDirectory + dimEntry*len(tipi)

	offLingue := offNomi
	for _, t := range tipi {
		offLingue += dimDirectory + dimEntry*len(perTipo[t])
	}

	offDescrittori := offLingue + (dimDirectory+dimEntry)*len(risorse)

	offDati := offDescrittori + dimDataEntry*len(risorse)
	if r := offDati % allineamento; r != 0 {
		offDati += allineamento - r
	}

	b := make([]byte, offDati)
	scriviDirectory := func(pos, voci int) {
		// I primi dodici byte sono caratteristiche, data e versione: per una
		// tabella di risorse restano tutti a zero.
		binary.LittleEndian.PutUint16(b[pos+12:], 0)            // voci con nome testuale
		binary.LittleEndian.PutUint16(b[pos+14:], uint16(voci)) // voci con numero
	}

	scriviDirectory(offRadice, len(tipi))

	cursoreNomi := offNomi
	cursoreLingue := offLingue
	cursoreDescr := offDescrittori
	cursoreDati := offDati

	var posizioniDaCorreggere []int
	var dati []byte

	pos := offRadice + dimDirectory
	for _, t := range tipi {
		binary.LittleEndian.PutUint32(b[pos:], t)
		// Il bit più alto acceso significa "qui sotto c'è un'altra tabella,
		// non dei dati".
		binary.LittleEndian.PutUint32(b[pos+4:], uint32(cursoreNomi)|0x80000000)
		pos += dimEntry

		elenco := perTipo[t]
		scriviDirectory(cursoreNomi, len(elenco))
		posNome := cursoreNomi + dimDirectory
		cursoreNomi += dimDirectory + dimEntry*len(elenco)

		for _, r := range elenco {
			binary.LittleEndian.PutUint32(b[posNome:], r.nome)
			binary.LittleEndian.PutUint32(b[posNome+4:], uint32(cursoreLingue)|0x80000000)
			posNome += dimEntry

			scriviDirectory(cursoreLingue, 1)
			binary.LittleEndian.PutUint32(b[cursoreLingue+dimDirectory:], langNeutral)
			binary.LittleEndian.PutUint32(b[cursoreLingue+dimDirectory+4:], uint32(cursoreDescr))
			cursoreLingue += dimDirectory + dimEntry

			// Il descrittore dei dati. Il primo campo è quello che verrà
			// corretto dal collegatore.
			posizioniDaCorreggere = append(posizioniDaCorreggere, cursoreDescr)
			binary.LittleEndian.PutUint32(b[cursoreDescr:], uint32(cursoreDati))
			binary.LittleEndian.PutUint32(b[cursoreDescr+4:], uint32(len(r.dati)))
			binary.LittleEndian.PutUint32(b[cursoreDescr+8:], 0) // tabella codici
			binary.LittleEndian.PutUint32(b[cursoreDescr+12:], 0)
			cursoreDescr += dimDataEntry

			dati = append(dati, r.dati...)
			cursoreDati += len(r.dati)
			if r := cursoreDati % allineamento; r != 0 {
				riempimento := allineamento - r
				dati = append(dati, make([]byte, riempimento)...)
				cursoreDati += riempimento
			}
		}
	}

	return append(b, dati...), posizioniDaCorreggere
}
