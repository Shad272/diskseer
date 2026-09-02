// diskseer — diagnostica hardware che dà un verdetto, non una tabella di numeri.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/shad272/diskseer/internal/collect"
	"github.com/shad272/diskseer/internal/report"
	"github.com/shad272/diskseer/internal/rules"
)

const version = "0.1.0"

func main() {
	var (
		asJSON      = flag.Bool("json", false, "stampa i dati grezzi in JSON invece del referto")
		noColor     = flag.Bool("no-color", false, "disattiva i colori ANSI")
		showVersion = flag.Bool("version", false, "stampa la versione ed esce")
		htmlPath    = flag.String("html", "", "salva il referto come pagina HTML nel percorso indicato")
		tecnico     = flag.String("tecnico", "", "nome di chi esegue la diagnosi, stampato sul referto")
		contatto    = flag.String("contatto", "", "recapito di chi esegue la diagnosi")
		cliente     = flag.String("cliente", "", "nome del cliente, stampato sul referto")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("diskseer", version)
		return
	}

	ansiOK := report.PrepareConsole()

	snap, err := collect.Collect()
	if err != nil {
		fmt.Fprintln(os.Stderr, "diskseer: raccolta dati fallita:", err)
		os.Exit(3)
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(snap); err != nil {
			fmt.Fprintln(os.Stderr, "diskseer:", err)
			os.Exit(3)
		}
		return
	}

	findings := rules.Run(snap)

	if *htmlPath != "" {
		opts := report.HTMLOptions{Tecnico: *tecnico, Contatto: *contatto, Cliente: *cliente}
		if err := report.WriteHTML(*htmlPath, snap, findings, opts); err != nil {
			fmt.Fprintln(os.Stderr, "diskseer: referto HTML non salvato:", err)
			os.Exit(3)
		}
		fmt.Fprintln(os.Stderr, "Referto salvato in", *htmlPath)
	}

	report.Printer{
		W:     os.Stdout,
		Color: ansiOK && !*noColor && os.Getenv("NO_COLOR") == "",
	}.Print(snap, findings)

	// Codice di uscita utilizzabile negli script: permette di far girare
	// diskseer su più macchine e raccogliere solo quelle che hanno problemi.
	switch rules.Overall(findings) {
	case rules.SevCritical:
		os.Exit(2)
	case rules.SevWarn:
		os.Exit(1)
	}
}
