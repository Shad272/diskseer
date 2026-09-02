//go:build !windows

package elevate

// Fuori da Windows non esiste il meccanismo di elevazione al volo su cui si
// basa questo pacchetto. Un programma che ha bisogno di più privilegi si
// avvia con sudo, e chiederli da solo a metà esecuzione non è il modo in cui
// funzionano quei sistemi.

func Elevato() bool { return false }

func Richiedi(eseguibile string, argomenti []string) bool { return false }
