# Genera l'icona multi-risoluzione e la copia del logo che finisce nel referto.
#
#   .\tools\make-icon.ps1
#   go run tools\makesyso.go assets\diskseer.ico rsrc_windows_amd64.syso
#
# Va rieseguito solo se cambia il logo. I file che produce stanno sotto
# controllo di versione, quindi chi clona il progetto compila senza aver
# bisogno di questo script.
#
# Perche' piu' risoluzioni: Windows sceglie da sola quale usare secondo il
# contesto — 16 px nella barra del titolo, 32 px in Esplora file, 256 px nelle
# anteprime grandi. Un'icona con una sola dimensione viene ridimensionata al
# volo dal sistema, e su schermi ad alta densita' si vede.

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

$radice   = Split-Path -Parent $PSScriptRoot
$sorgente = Join-Path $radice 'assets\diskseer-pulse-1024.png'
$destIco  = Join-Path $radice 'assets\diskseer.ico'
$destLogo = Join-Path $radice 'internal\report\logo.png'

if (-not (Test-Path $sorgente)) { throw "logo sorgente non trovato: $sorgente" }

$img = [System.Drawing.Image]::FromFile($sorgente)
Write-Host "sorgente: $($img.Width)x$($img.Height) px"

function ScalaPng($immagine, [int]$lato) {
    $bmp = New-Object System.Drawing.Bitmap $lato, $lato
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.InterpolationMode = 'HighQualityBicubic'
    $g.SmoothingMode = 'HighQuality'
    $g.PixelOffsetMode = 'HighQuality'
    $g.CompositingQuality = 'HighQuality'
    $g.DrawImage($immagine, 0, 0, $lato, $lato)
    $g.Dispose()
    $ms = New-Object System.IO.MemoryStream
    $bmp.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
    $bmp.Dispose()
    # La virgola impedisce a PowerShell di "srotolare" l'array in uscita dalla
    # funzione: senza, i byte tornerebbero come oggetti generici e il
    # BinaryWriter piu' sotto ne scriverebbe uno solo per immagine invece
    # dell'intero PNG. E' un errore che non da' alcun messaggio: produce
    # semplicemente un'icona vuota.
    , $ms.ToArray()
}

$lati = @(16, 24, 32, 48, 64, 128, 256)
$dati = @{}
foreach ($l in $lati) {
    $dati[$l] = ScalaPng $img $l
    Write-Host ("  {0,3} px -> {1,6} byte" -f $l, $dati[$l].Length)
}
$img.Dispose()

# Formato ICO: sei byte di intestazione, poi una voce di indice da sedici byte
# per ogni risoluzione, poi le immagini. Ogni voce dice dove comincia la
# propria immagine, quindi gli scostamenti vanno calcolati sapendo in anticipo
# quanto occupa l'indice.
$ms = New-Object System.IO.MemoryStream
$w = New-Object System.IO.BinaryWriter $ms
$w.Write([uint16]0)             # riservato
$w.Write([uint16]1)             # tipo 1 = icona
$w.Write([uint16]$lati.Count)

$scostamento = 6 + 16 * $lati.Count
foreach ($l in $lati) {
    # Nel formato ICO il lato sta in un solo byte, quindi 256 si scrive come 0.
    $lato = if ($l -ge 256) { 0 } else { $l }
    $w.Write([byte]$lato)
    $w.Write([byte]$lato)
    $w.Write([byte]0)           # colori nella tavolozza: 0 = colore pieno
    $w.Write([byte]0)           # riservato
    $w.Write([uint16]1)         # piani
    $w.Write([uint16]32)        # bit per pixel
    $w.Write([uint32]$dati[$l].Length)
    $w.Write([uint32]$scostamento)
    $scostamento += $dati[$l].Length
}
foreach ($l in $lati) { $w.Write([byte[]]$dati[$l]) }
$w.Flush()
[IO.File]::WriteAllBytes($destIco, $ms.ToArray())
$w.Dispose()
Write-Host ("icona: {0} KB, {1} risoluzioni" -f [math]::Round((Get-Item $destIco).Length / 1KB, 1), $lati.Count)

# Copia a 64 px per il referto HTML. Sta dentro internal/report perche' go:embed
# non sa risalire fuori dalla cartella del proprio pacchetto.
[IO.File]::WriteAllBytes($destLogo, [byte[]]$dati[64])
Write-Host ("logo del referto: {0} byte" -f (Get-Item $destLogo).Length)
