<#
.SYNOPSIS
    Cattura uno snapshot della macchina e lo salva in testdata/ come campione.

.DESCRIPTION
    Serve a due cose:

      1. verificare quali contatori il sistema espone davvero su una data
         macchina, cosa che il referto non mostra (le regole scattano solo
         sopra soglia, quindi il loro silenzio e' ambiguo);

      2. raccogliere casi reali. Quando una macchina ha un guasto vero, il
         suo snapshot diventa un test permanente: il motore di regole e' una
         funzione pura, quindi si collauda su file JSON senza bisogno di
         avere il disco guasto sottomano.

.EXAMPLE
    .\tools\capture.ps1 -Name hdd-settori-riallocati
#>
param(
    [string]$Name = 'snapshot',
    # I campioni catturati in assistenza contengono marca, modello e orari di
    # accensione del computer di un cliente. Sono suoi, non tuoi, e non
    # servono a diagnosticare niente: per questo l'anonimizzazione e' attiva
    # di default e va disattivata di proposito, non ricordata di proposito.
    [switch]$ConDatiIdentificativi
)

$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$exe  = Join-Path $root 'diskseer.exe'
$dir  = Join-Path $root 'testdata'

if (-not (Test-Path $exe)) {
    Write-Error "diskseer.exe non trovato in $root. Esegui prima: go build -o diskseer.exe ."
    exit 1
}
if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir | Out-Null }

$elevated = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $elevated) {
    Write-Warning 'Non elevato: SMART, temperature e usura NON finiranno nello snapshot.'
}

$out = Join-Path $dir "$Name.json"

$opzioni = @('--json')
if (-not $ConDatiIdentificativi) { $opzioni += '--anonimo' }
$json = & $exe @opzioni | Out-String

# WriteAllText scrive UTF-8 senza BOM. Out-File e '>' aggiungono il BOM, e
# tre byte invisibili in testa fanno fallire json.Unmarshal in Go con un
# errore che non dice da dove arriva il problema.
[IO.File]::WriteAllText($out, $json)

Write-Host ""
$anon = if ($ConDatiIdentificativi) { 'NO' } else { 'si' }
Write-Host "Scritto $out ($((Get-Item $out).Length) byte, elevato=$elevated, anonimizzato=$anon)" -ForegroundColor Green
Write-Host ""
Write-Host "Premi INVIO per chiudere"
[void][Console]::ReadLine()
