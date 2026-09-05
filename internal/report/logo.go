package report

import (
	_ "embed"
	"encoding/base64"
)

// Il logo viaggia dentro il binario e finisce dentro il referto come testo,
// non come file allegato.
//
// È la conseguenza di una scelta presa quando il referto è nato: deve essere
// un file solo, che si apre con un doppio clic su una macchina senza internet
// — spesso proprio quella che stai diagnosticando. Un logo caricato da un
// indirizzo web apparirebbe come un riquadro rotto, e uno salvato accanto al
// referto si perderebbe al primo inoltro per email.
//
// L'originale ad alta risoluzione sta in assets/. Qui c'è la copia a 64 px,
// che pesa 2,5 KB: quella grande gonfierebbe ogni referto di 35 KB per una
// differenza che a schermo non si vede.
//
//go:embed logo.png
var logoPNG []byte

// logoDataURI restituisce il logo nella forma che un browser sa leggere
// direttamente dal documento, senza andare a cercarlo da nessuna parte.
func logoDataURI() string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(logoPNG)
}
