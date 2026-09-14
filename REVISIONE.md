# Revisione tecnica del prodotto

Aggiornamento del 14 settembre: le priorità sulla freschezza dei dati e sul
timeout PowerShell sono state implementate nell'estensione descritta in
[COMPATIBILITA.md](COMPATIBILITA.md), che contiene la matrice delle piattaforme,
le build disponibili e i limiti di verifica aggiornati.

Revisione del 13 settembre 2026. Ambito: flussi CLI e menu, privacy,
presentazione dei risultati, salvataggio dei file e test automatici.

## Miglioramenti applicati

| Problema verificato nel codice | Correzione |
| --- | --- |
| Rifare la diagnosi dal menu perdeva `--anonymous`. | La sessione conserva la scelta e anonimizza ogni nuova raccolta prima di calcolare e mostrare i risultati. |
| Una raccolta fallita poteva essere esportata o terminare con codice 0. | Report, esportazione, dettagli e live richiedono una raccolta riuscita; uscita dal menu con codice 3 in caso di errore. |
| Le schede HTML mostravano solo lo stato Windows; il live ignorava alcuni allarmi SMART/NVMe. | Stato diagnostico ricavato dalle regole esistenti, con precedenza agli allarmi e indicazione dei contatori non verificati. Lo stato Windows rimane separato nel report. |
| Il salvataggio troncava direttamente report, impostazioni ed esportazioni. | Scrittura su file temporaneo nella stessa cartella, chiusura e sostituzione della destinazione. Gli errori di sostituzione vengono restituiti e il temporaneo viene rimosso. |
| Gli errori di aggiornamento HTML erano ignorati. | Messaggio visibile; Ctrl+C salva una versione statica del report. Un errore di salvataggio CLI restituisce codice 3. |
| Due esportazioni nello stesso minuto si sovrascrivevano. | Nomi automatici con secondi e frazioni di secondo. |
| `--gui` ignorava la cartella dei report salvata nelle preferenze. | Stessa scelta della cartella usata dal menu. |
| Valori CLI vuoti non cancellavano i dettagli del cliente o del tecnico salvati. | Le opzioni esplicitamente specificate prevalgono anche quando vuote. |
| Un campo JSON con tipo errato applicava comunque parte delle impostazioni. | In caso di errore di decodifica vengono usate tutte le impostazioni predefinite. |
| Il blocco di localStorage interrompeva anche i filtri del report. | Accesso protetto alla memoria del browser e filtri funzionanti anche senza persistenza. |
| La ricarica live perdeva il filtro; la stampa poteva omettere segnalazioni filtrate. | Filtro conservato per singolo report, stampa di tutte le segnalazioni e ricarica sospesa durante la stampa. |
| Alcune etichette del report italiano rimanevano in inglese. | Tradotti conteggi, filtri ed evidenze; precisato l'avviso sui dati disponibili senza privilegi. |

La scrittura tramite sostituzione evita il troncamento del file precedente;
non rappresenta una garanzia di durabilità in caso di perdita di alimentazione
né una garanzia di atomicità su qualunque filesystem Windows o di rete.

## Verifiche

- Suite Go e `go vet ./...` superati su Windows.
- Quattro test JavaScript senza dipendenze npm: storage negato, persistenza dei
  filtri, valori salvati non validi, coordinamento fra stampa e ricarica.
- Regressioni Go per privacy, raccolta fallita, stato SMART/NVMe,
  configurazione parzialmente invalida e sostituzione dei file.
- Compilazione Windows completata in `dist/diskseer.exe`.
- Test JavaScript aggiunti alla pipeline CI esistente; la pipeline remota
  non è stata eseguita in questa revisione.
- Report dimostrativi generati da fixture anonime in `dist/preview/`.
  Verifica visiva non completata: la policy del browser integrato impedisce
  l'apertura dei file locali. I test JavaScript verificano la logica, non il layout.

## Priorità successive

1. **Freschezza dei dati live.** `internal/collect/refresh_windows.go` conserva
   le letture precedenti quando un disco o volume non risponde. Aggiungere
   timestamp ed errori per dispositivo e mostrarli come dati non aggiornati,
   conservando lo storico senza presentarlo come una lettura attuale.
2. **Timeout della raccolta.** Il processo PowerShell in
   `internal/collect/windows.go` non ha un limite di durata. Introdurre
   cancellazione e timeout con test del processo bloccato; valutare separatamente
   le chiamate dirette ai dispositivi, che non sono interrotte dal timeout di PowerShell.
3. **Copertura hardware e grafica.** Verificare scollegamento USB, driver che
   restituiscono dati incompleti e file HTML aperti durante la sostituzione;
   controllare il report nei browser Windows, su schermo piccolo e in stampa.

Non sono state modificate le soglie diagnostiche o le strutture binarie dei
protocolli dei dischi: richiedono una validazione hardware dedicata.
