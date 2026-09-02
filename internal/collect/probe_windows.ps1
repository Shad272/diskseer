# Raccoglitore Windows per diskseer.
# Emette UN SOLO oggetto JSON su stdout. Nessuna sezione puo' far fallire le
# altre: ogni blocco e' isolato, e cio' che non si riesce a leggere diventa
# null. Un raccoglitore che va in errore su una macchina malandata e' inutile,
# perche' le macchine malandate sono esattamente quelle da diagnosticare.

$ErrorActionPreference = 'SilentlyContinue'
$ProgressPreference    = 'SilentlyContinue'

# Forza UTF-8 in uscita: senza questa riga i nomi con accenti arrivano a Go
# gia' corrotti dalla codepage della console.
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }

function Get-ChassisName([int[]]$codes) {
    if (-not $codes) { return 'Unknown' }
    switch ($codes[0]) {
        3  { 'Desktop' }  4  { 'Desktop' }  5 { 'Desktop' }  6 { 'Desktop' }
        7  { 'Desktop' }  15 { 'Desktop' }  16 { 'Desktop' }
        8  { 'Laptop' }   9  { 'Laptop' }   10 { 'Laptop' }  14 { 'Laptop' }
        30 { 'Tablet' }   31 { 'Laptop' }   32 { 'Tablet' }
        17 { 'Server' }   23 { 'Server' }
        default { 'Unknown' }
    }
}

$elevated = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator)

$cs  = Get-CimInstance Win32_ComputerSystem
$os  = Get-CimInstance Win32_OperatingSystem
$cpu = Get-CimInstance Win32_Processor | Select-Object -First 1
$enc = Get-CimInstance Win32_SystemEnclosure | Select-Object -First 1

$system = [ordered]@{
    manufacturer = [string]$cs.Manufacturer
    model        = [string]$cs.Model
    os           = [string]$os.Caption
    osVersion    = [string]$os.Version
    cpu          = [string]$cpu.Name
    cores        = [int]$cpu.NumberOfCores
    threads      = [int]$cpu.NumberOfLogicalProcessors
    ramBytes     = [uint64]$cs.TotalPhysicalMemory
    chassis      = Get-ChassisName $enc.ChassisTypes
    lastBoot     = if ($os.LastBootUpTime) { $os.LastBootUpTime.ToUniversalTime().ToString('o') } else { $null }
}

# Numero del disco che ospita il volume di sistema: serve per distinguere il
# disco di Windows da quelli dati, che e' cio' che rende utile la diagnosi.
$sysDiskNumber = -1
try {
    $sysDrive = ($env:SystemDrive).TrimEnd(':')
    $sysDiskNumber = (Get-Partition -DriveLetter $sysDrive | Get-Disk).Number
} catch { }

# I contatori di affidabilita' esistono solo con privilegi elevati.
$rel = @{}
if ($elevated) {
    foreach ($c in (Get-PhysicalDisk | Get-StorageReliabilityCounter)) {
        $rel[[string]$c.DeviceId] = $c
    }
}

$disks = @()
foreach ($p in (Get-PhysicalDisk)) {
    $id = [string]$p.DeviceId
    $r  = $rel[$id]
    $disks += [ordered]@{
        deviceId               = $id
        model                  = [string]$p.FriendlyName
        mediaType              = [string]$p.MediaType
        busType                = [string]$p.BusType
        healthStatus           = [string]$p.HealthStatus
        sizeBytes              = [uint64]$p.Size
        isSystemDisk           = ($id -eq [string]$sysDiskNumber)
        temperatureC           = if ($r -and $r.Temperature      -ne $null) { [int]$r.Temperature }           else { $null }
        temperatureMaxC        = if ($r -and $r.TemperatureMax   -ne $null) { [int]$r.TemperatureMax }        else { $null }
        wearPercent            = if ($r -and $r.Wear             -ne $null) { [int]$r.Wear }                  else { $null }
        powerOnHours           = if ($r -and $r.PowerOnHours     -ne $null) { [uint64]$r.PowerOnHours }       else { $null }
        startStopCycles        = if ($r -and $r.StartStopCycleCount -ne $null) { [uint64]$r.StartStopCycleCount } else { $null }
        readErrorsTotal        = if ($r -and $r.ReadErrorsTotal  -ne $null) { [uint64]$r.ReadErrorsTotal }    else { $null }
        readErrorsUncorrected  = if ($r -and $r.ReadErrorsUncorrected -ne $null) { [uint64]$r.ReadErrorsUncorrected } else { $null }
        writeErrorsTotal       = if ($r -and $r.WriteErrorsTotal -ne $null) { [uint64]$r.WriteErrorsTotal }   else { $null }
        writeErrorsUncorrected = if ($r -and $r.WriteErrorsUncorrected -ne $null) { [uint64]$r.WriteErrorsUncorrected } else { $null }
    }
}

$volumes = @()
foreach ($v in (Get-Volume | Where-Object { $_.DriveLetter })) {
    $volumes += [ordered]@{
        driveLetter  = [string]$v.DriveLetter
        fileSystem   = [string]$v.FileSystemType
        healthStatus = [string]$v.HealthStatus
        # OperationalStatus puo' arrivare come array: va appiattito, altrimenti
        # in JSON esce come lista e Go lo rifiuta.
        operationalStatus = (@($v.OperationalStatus) -join ', ')
        sizeBytes    = [uint64]$v.Size
        freeBytes    = [uint64]$v.SizeRemaining
    }
}

# Temperature ACPI: espresse in decimi di Kelvin. Molte macchine desktop non
# le espongono affatto, e va bene cosi': l'assenza non e' un errore.
$thermals = @()
foreach ($t in (Get-CimInstance -Namespace root\wmi -ClassName MSAcpi_ThermalZoneTemperature)) {
    if ($t.CurrentTemperature) {
        $thermals += [ordered]@{
            name    = [string]$t.InstanceName
            celsius = [math]::Round(($t.CurrentTemperature / 10.0) - 273.15, 1)
        }
    }
}

$battery = $null
$bw = Get-CimInstance Win32_Battery | Select-Object -First 1
if ($bw) {
    $design = $null; $full = $null; $cycles = $null
    $bs = Get-CimInstance -Namespace root\wmi -ClassName BatteryStaticData     | Select-Object -First 1
    $bf = Get-CimInstance -Namespace root\wmi -ClassName BatteryFullChargedCapacity | Select-Object -First 1
    if ($bs) { $design = [int]$bs.DesignedCapacity; if ($bs.CycleCount) { $cycles = [int]$bs.CycleCount } }
    if ($bf) { $full   = [int]$bf.FullChargedCapacity }
    $battery = [ordered]@{
        name           = [string]$bw.Name
        chargePercent  = if ($bw.EstimatedChargeRemaining -ne $null) { [int]$bw.EstimatedChargeRemaining } else { $null }
        designCapacity = $design
        fullCapacity   = $full
        cycleCount     = $cycles
    }
}

[ordered]@{
    elevated = [bool]$elevated
    system   = $system
    disks    = @($disks)
    volumes  = @($volumes)
    thermals = @($thermals)
    battery  = $battery
} | ConvertTo-Json -Depth 6 -Compress
