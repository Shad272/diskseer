# Compatibilità, avvio e distribuzione locale

Aggiornamento: 14 settembre 2026. Il workspace locale è la fonte autorevole.
Non sono stati eseguiti pull, fetch, clone, commit o push. Le modifiche della
revisione precedente sono state mantenute. L'unico download di sviluppo è il
compilatore ufficiale Go 1.20.14, con SHA-256 verificato; nessun codice DiskSeer
è stato recuperato online.

## Matrice effettiva

| Sistema | x64 | x86 | ARM64 | Stato della verifica |
| --- | --- | --- | --- | --- |
| Windows 7 SP1 | build legacy Go 1.20.14 | build legacy Go 1.20.14, softfloat | non applicabile | Compilazione e test del codice legacy eseguiti su Windows 11; macchina/VM Windows 7 non disponibile. Da validare sul sistema reale. |
| Windows 8.1 | build legacy | build legacy | non applicabile | Stesso percorso legacy; esecuzione su 8.1 non verificata. |
| Windows 10 | build modern | build modern, softfloat | build modern | Percorso moderno compilato; non disponibile una macchina Windows 10 distinta. |
| Windows 11 | build modern | build modern via WOW64 | build modern | Esecuzione e test x64/x86 su host x64 build 26100; ARM64 solo cross-build, senza hardware ARM64. |

Go 1.20 è l'ultima serie ufficiale compatibile con Windows 7/8; Go 1.21 e
successivi richiedono Windows 10. Cambiare soltanto `go.mod` non rende un
eseguibile moderno compatibile con Windows 7: è necessario il compilatore
legacy. Fonti: [Go 1.20](https://go.dev/doc/go1.20),
[requisiti Windows di Go](https://go.dev/wiki/Windows).

Windows 7 RTM, Windows RT, Windows XP e Windows ARM32 non sono target.
Le build legacy sono un percorso verificabile, non una certificazione di
funzionamento su ogni vecchio driver. Go 1.20 è fuori manutenzione: la build
moderna rimane quella da distribuire su Windows 10/11.

## Un punto di ingresso

Scegli l'eseguibile per sistema e architettura e, se preferisci, rinominalo
`diskseer.exe`. Ogni variante è autonoma e usa lo stesso codice di avvio.
Non servono Node.js, un'installazione di Go o servizi residenti sul PC esaminato.
Windows PowerShell è necessario per l'inventario ed è incluso nei sistemi
Windows supportati; se rimosso o bloccato, viene restituito un errore diagnostico
invece di un referto apparentemente sano.

- Doppio clic o collegamento: scelta automatica del terminale e menu interattivo.
- cmd, PowerShell, terminali integrati: riuso della console corrente per default.
- Windows Terminal già attivo: nessuna seconda finestra.
- Pipe, input/output rediretti e JSON: nessun rilancio automatico.
- `--direct` o `--terminal direct`: disabilitano esplicitamente il rilancio.
- Nessuna elevazione UAC automatica. Il menu permette di richiedere i privilegi
  quando servono le letture SATA/USB. `--no-elevate` resta accettato per compatibilità.

## Scelta e rilevamento del terminale

1. Preferenza esplicita, se utilizzabile.
2. Windows Terminal su sistemi con versione minima e API ConPTY disponibili.
3. `pwsh.exe` su Windows moderni.
4. Windows PowerShell.
5. `cmd.exe`.
6. Console corrente, con messaggio di fallback: l'assenza dei terminali esterni
   non impedisce di eseguire DiskSeer.

La verifica della versione usa `RtlGetVersion`, senza dipendere dal manifest
dell'eseguibile. Le API opzionali vengono risolte con `Find`. Le console sono
rilevate tramite handle reali e processi collegati, non con il solo nome del
processo padre. `WT_SESSION` e `WT_PROFILE_ID` impediscono rilanci dentro WT.

Windows Terminal viene cercato nel PATH e nei manifest dei pacchetti registrati
per l'utente corrente, tramite `Get-AppxPackage`/`Get-AppxPackageManifest`.
Si cerca l'alias ufficiale `wt.exe` e si legge l'eseguibile dell'applicazione
che lo dichiara. Il percorso non è costruito con nomi fissi di pacchetti,
versioni, publisher o cartelle WindowsApps: lo stesso percorso riconosce
Stable e Preview quando espongono l'alias. La scoperta AppX ha un limite di
quattro secondi e 64 KiB; se bloccata, restano PATH e shell di sistema.
Installazioni non registrate, prive di alias e fuori dal PATH non sono
individuabili automaticamente senza configurazioni specifiche.

Il percorso delle shell native deriva da `GetSystemDirectoryW`. Windows
Terminal è escluso su Windows 7/8.1. Su questi sistemi non viene avviata
nemmeno la scoperta AppX. Non vengono installati terminali né modificate
associazioni, registro o preferenze globali.

### Rilancio sicuro

Argomenti e directory sono conservati in un file JSON temporaneo; il percorso
dell'eseguibile viene passato attraverso l'ambiente. Le shell eseguono soltanto
un comando costante. I valori dell'utente non diventano codice PowerShell/cmd.
Le variabili d'ambiente sono ereditate; cmd disabilita AutoRun e delayed expansion.

Il figlio conferma di essere pronto, il padre autorizza l'avvio e poi attende
il vero processo figlio. Il codice di uscita arriva dal figlio, non dal
processo `wt.exe`, che può terminare prima. Un tentativo che non conferma
l'avvio entro dieci secondi viene abbandonato; un figlio tardivo trova la
richiesta scaduta e non avvia una seconda diagnosi. Il protocollo impedisce
loop di rilancio. I processi vengono attesi e gli handle chiusi.

La console iniziale di un doppio clic viene nascosta soltanto dopo il successo
del rilancio. Può essere brevemente visibile: eliminare completamente questo
effetto richiederebbe un launcher GUI separato. Un'interruzione forzata del
padre può lasciare file temporanei; nell'uscita normale vengono rimossi.

`--diagnostics` espone capacità, profilo e log di scoperta in JSON senza
inventario, modelli dei dischi o percorsi dei programmi individuati. I tentativi
di rilancio producono inoltre messaggi su stderr. Nessun log persistente viene
creato automaticamente.

## Profili adattivi

| Profilo | Concorrenza massima | Limite morbido memoria Go | Output inventario |
| --- | --- | --- | --- |
| conservative | 1 disco | 64 MiB | 4 MiB |
| balanced | 2 dischi | 128 MiB | 8 MiB |
| fast | 4 dischi | 256 MiB | 8 MiB |

Automatico sceglie conservativo con pochi thread, poca memoria disponibile,
forte pressione di memoria, processo x86 o rilevamento incompleto. Veloce
richiede almeno 8 CPU logiche e 4 GiB disponibili; gli altri casi usano
bilanciato. Restano limiti rigidi anche con override: massimo quattro richieste
simultanee, una sola su macchine molto povere di risorse, massimo due su x86
e su macchine con disco di sistema HDD. Una singola unità riceve una sola
richiesta alla volta. Il refresh ricontrolla la pressione della memoria.

`GOAMD64=v1` evita requisiti AVX/AVX2 imposti dalla build; `GO386=softfloat`
evita un requisito SSE2 aggiuntivo per le build x86. Il runtime moderno conserva
le proprie ottimizzazioni e il rilevamento delle istruzioni disponibili.

DiskSeer legge piccoli contatori di protocollo, non file e directory: la
dimensione in terabyte non richiede buffer proporzionali, cache di file o
batch di scansione. Non si attraversano junction, symlink o hard link, quindi
non esistono cicli di visita da eliminare. Non sono state aggiunte accelerazioni
SIMD personalizzate o I/O asincrono non misurato. I volumi di rete non vengono
interrogati nel refresh rapido.

Il limite Go è morbido, non un limite all'intero processo né a PowerShell.
La raccolta PowerShell ha timeout di 90 secondi e supporta Ctrl+C. La
cancellazione ferma nuovi lavori, ma non può garantire l'interruzione di una
DeviceIoControl sincrona bloccata dentro un driver. SAT richiede un timeout
di dieci secondi al driver; driver difettosi possono non rispettarlo.

## Raccolta legacy e affidabilità

### Correzione della vista dal vivo

La vista live legge le dimensioni visibili della console a ogni aggiornamento,
usa uno schermo separato ripristinato all'uscita e cancella ciascuna riga prima
di riscriverla. Le righe non raggiungono l'ultima colonna o l'ultima riga: il
ridisegno non causa scorrimenti e non lascia residui quando il testo si accorcia.
Se la finestra è piccola, vengono eliminati prima gli spazi verticali, poi
segnalate le righe non visibili. Il riepilogo e l'indicazione Ctrl+C rimangono
visibili nelle dimensioni usuali; il referto completo resta disponibile fuori
dalla vista live. I colori non influiscono sulla larghezza delle colonne.
L'uscita rediretta conserva tutti i dati e non contiene comandi di ridisegno.

I test simulano il contenuto dello schermo durante aggiornamenti successivi,
finestre da 40 a 140 colonne, molti dischi, Unicode, grafica semplice e colori.
Test e controlli statici moderni superati; test del report superati anche con
Go 1.20.14 a 32 bit. La correzione non riattiva i test di rilancio sospesi.

### Inventario e aggiornamenti

Il raccoglitore rileva la presenza dei comandi, tenta Storage/CIM e ripiega
su WMI quando assenti o non funzionanti. Lo script evita sintassi `[ordered]`
e dispone di un serializzatore JSON per PowerShell 2. I file temporanei dello
script hanno BOM UTF-8 per non corrompere Unicode nei vecchi PowerShell.

WMI fornisce inventario e spazio libero. Tipo del disco e salute non vengono
dedotti dal nome del modello o da WMI `Status=OK`. Le proprietà del dispositivo
sono interrogate direttamente dove disponibili; gli errori lasciano valori
sconosciuti. SMART/NVMe restano dipendenti da driver, bridge USB e permessi.
Le limitazioni dell'inventario sono visibili nei risultati.

Il layout SAT distingue x86 da x64/ARM64. Le risposte SAT/NVMe vengono
controllate prima di leggerne i campi, inclusi overflow degli offset su x86.
Gli identificativi dei dischi devono essere numerici. Gli aggiornamenti
falliti conservano i contatori precedenti ma li etichettano come non aggiornati,
senza declassare allarmi critici già presenti.

Report, esportazioni e impostazioni conservano la scrittura tramite file
temporaneo e sostituzione introdotta nella revisione precedente. Errori di
permessi, file bloccati e filesystem non compatibili restano errori espliciti.
I percorsi Unicode usano API wide; percorsi di avvio troppo lunghi per la shell
possono impedire quel rilancio e provocano fallback alla console corrente.
Non viene modificato il supporto globale Windows ai percorsi lunghi.

## Comandi

```powershell
.\diskseer.exe
.\diskseer.exe --direct
.\diskseer.exe --terminal wt
.\diskseer.exe --terminal powershell
.\diskseer.exe --profile conservative --workers 1
.\diskseer.exe --profile fast --workers 4
.\diskseer.exe --diagnostics
.\diskseer.exe --json --anonymous
```

Terminale e profilo sono configurabili anche nelle impostazioni del menu.
Nel JSON delle preferenze: `terminal`, `profile`, `workers` (0 = automatico).
Le opzioni CLI prevalgono. `DISKSEER_LEGACY=1` forza WMI e il serializzatore
legacy per verifiche; non cambia il runtime dell'eseguibile.

## Build e distribuzione

```powershell
powershell -NoProfile -ExecutionPolicy RemoteSigned -File tools/build.ps1 -Edition modern
powershell -NoProfile -ExecutionPolicy RemoteSigned -File tools/build.ps1 -Edition legacy -LegacyGo C:\toolchain\go1.20.14\bin\go.exe
powershell -NoProfile -ExecutionPolicy RemoteSigned -File tools/build.ps1 -Edition all -LegacyGo C:\toolchain\go1.20.14\bin\go.exe
```

Lo script non scarica nulla e non sincronizza Git. Produce cinque eseguibili
con nomi distinti e un file SHA-256 in `dist/`. La compilazione non include
percorsi del workspace (`-trimpath`) né metadati VCS (`-buildvcs=false`).
`RemoteSigned` vale solo per il processo di build: permette gli script locali
senza cambiare impostazioni persistenti o disattivare l'antivirus.
Distribuire il solo eseguibile adatto, insieme alla licenza e agli obblighi
di distribuzione del sorgente già previsti dal progetto. Le build non sono
firmate. L'icona incorporata preesistente è disponibile solo su x64.

## Verifiche e limiti residui

- Verifica finale: test Go e `go vet` moderni superati su amd64 e 386;
  test Go 1.20.14 superati su amd64 e 386. Il test reale di rilancio è escluso.
- Ricompilazione finale riuscita: modern amd64, 386 e arm64; legacy amd64 e 386.
  Verificato anche l'avvio `--version` dei quattro eseguibili x64/x86.
  `diskseer.exe` nella radice e in `dist/` corrisponde alla build modern amd64.
- Test x86 eseguiti tramite WOW64 sull'host Windows 11 x64.
- Race detector per raccolta, terminali e profili: passato.
- Matrice simulata Windows 7/8.1/10-11, WT già attivo, assente, Preview senza
  alias PATH, preferenze, pipe, redirezioni, modalità diretta e figlio rilanciato.
- La prima prova reale cmd/Windows PowerShell con finestre nascoste ha verificato
  argomenti vuoti, Unicode, virgolette, metacaratteri, directory e codice di uscita.
  La verifica finale estesa, che copia anche l'eseguibile in un percorso Unicode
  con metacaratteri e controlla ambiente e handle della console, è **incompleta**:
  Bitdefender ha messo in quarantena gli artefatti con rilevamento `Atc4.Detection`.
  Il test `TestRealShellHandoffPreservesArgumentsDirectoryAndExit` resta sospeso
  su questo host; si attiva soltanto con `DISKSEER_LAUNCH_TESTS=1`.
- Test PowerShell con provider simulati: inventario WMI, JSON legacy,
  timestamp WMI e manifest AppX con namespace/Preview.
- Test del limite di concorrenza, cancellazione, memoria di output, layout SAT
  e risposte NVMe corte o con overflow.
- I quattro test JavaScript della revisione precedente sono stati rieseguiti:
  tutti superati. Superati anche i due script PowerShell con dati simulati
  e il controllo `git diff --check`.
- Misura singola sui dischi dell'host: raccolta 3,211 s, refresh circa 3 ms.
  Non è un confronto prima/dopo né un risultato generalizzabile ad altri PC.

Il sandbox nega l'inventario WMI/Storage: il test hardware ha richiesto una
esecuzione locale autorizzata fuori da quelle restrizioni ed è passato.
La UI di Windows Terminal, Windows 7/8.1, Windows 10 su hardware distinto,
ARM64 reale, bridge USB differenti e comportamenti di driver guasti richiedono
verifiche dedicate. Il report HTML usa capacità di browser moderni; su vecchi
browser la diagnosi testuale e JSON resta il percorso affidabile. La verifica
visiva del report nel browser integrato rimane bloccata dalla sua policy sui
file locali. Non sono stati installati sistemi operativi o terminali per colmare
questi limiti e la pipeline remota non è stata eseguita.

Non sono state disattivate protezioni né aggiunte esclusioni antivirus. Il
rilevamento non è stato classificato come falso positivo: prima di distribuire
il rilancio automatico occorre una verifica dedicata con l'antivirus attivo.
`--direct` evita il rilancio del terminale, ma l'inventario usa comunque
PowerShell e non costituisce una garanzia di assenza di ulteriori rilevamenti.

## File interessati da questa estensione

- Avvio e configurazione: `main.go`, `menu.go`, `impostazioni.go`,
  `internal/settings/settings.go`.
- Capacità: `internal/platform/platform.go`, `platform_windows.go`,
  `platform_other.go`.
- Terminali: `internal/terminal/policy.go`, `handoff.go`, `process_windows.go`,
  `terminal_windows.go`, `terminal_other.go`, `policy_test.go`, `launch_windows_test.go`.
- Profili: `internal/tuning/profile.go`, `profile_test.go`.
- Raccolta: `internal/collect/collect.go`, `windows.go`, `unsupported.go`,
  `probe_windows.ps1`, `bounded.go`, `bounded_test.go`, `identify_windows.go`,
  `device_windows.go`, `nvme_windows.go`, `sat_windows.go`, `refresh_windows.go`,
  `refresh_other.go`, `compat_windows_test.go`.
- Risultati: `internal/model/model.go`, `internal/rules/engine.go`,
  `internal/rules/compatibility.go`, `internal/report/disk_status.go`,
  `disk_status_test.go`, `html.go`, `live.go`, `live_frame.go`,
  `live_frame_test.go`, `console_windows.go`, `console_other.go`.
- Quoting elevazione: `internal/elevate/elevate_windows.go`.
- Build/test/documentazione: `go.mod`, `.github/workflows/build.yml`,
  `tools/build.ps1`, `tools/test-probe.ps1`, `tools/test-terminal-discovery.ps1`,
  `README.md`, `REVISIONE.md`, questo documento.

Gli altri file già modificati nella precedente revisione, compresi salvataggi
sicuri, test UI, anonimizzazione e report, sono rimasti nel workspace.
