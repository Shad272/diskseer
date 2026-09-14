# Compatible with Windows PowerShell 2.0 (.NET 3.5) and newer.
# Queries only: no filesystem walk, repair, service or registry modification.
$ErrorActionPreference = 'SilentlyContinue'
$ProgressPreference = 'SilentlyContinue'
try { [Console]::OutputEncoding = (New-Object System.Text.UTF8Encoding) } catch {}
$legacy = ($env:DISKSEER_LEGACY -eq '1')
$hasCim = (!$legacy -and (Get-Command Get-CimInstance -ErrorAction SilentlyContinue))

function Read-Instances([string]$class, [string]$ns = 'root\cimv2') {
    if ($hasCim) {
        try { return Get-CimInstance -ClassName $class -Namespace $ns -ErrorAction Stop } catch {}
    }
    try { return Get-WmiObject -Class $class -Namespace $ns -ErrorAction Stop } catch {}
}
function Boot-Date($value) {
    if (!$value) { return $null }
    try {
        if ($value -is [DateTime]) { return $value.ToUniversalTime().ToString('o') }
        return [System.Management.ManagementDateTimeConverter]::ToDateTime([string]$value).ToUniversalTime().ToString('o')
    } catch { return $null }
}
function Chassis-Name($codes) {
    if (!$codes) { return 'Unknown' }
    switch ([int]$codes[0]) {
        3 {'Desktop'} 4 {'Desktop'} 5 {'Desktop'} 6 {'Desktop'} 7 {'Desktop'}
        15 {'Desktop'} 16 {'Desktop'} 8 {'Laptop'} 9 {'Laptop'} 10 {'Laptop'}
        14 {'Laptop'} 30 {'Tablet'} 31 {'Laptop'} 32 {'Tablet'}
        17 {'Server'} 23 {'Server'} default {'Unknown'}
    }
}
# ConvertTo-Json and ordered hashtables do not exist in PowerShell 2.
# A small serializer handles only the scalar/array/map types emitted below.
function Quote-Json([string]$value) {
    $b = New-Object System.Text.StringBuilder
    [void]$b.Append('"')
    foreach ($c in $value.ToCharArray()) {
        $n = [int]$c
        if ($n -eq 34) { [void]$b.Append('\"') }
        elseif ($n -eq 92) { [void]$b.Append('\\') }
        elseif ($n -lt 32) { [void]$b.Append(('\u{0:x4}' -f $n)) }
        else { [void]$b.Append($c) }
    }
    [void]$b.Append('"')
    return $b.ToString()
}
function Write-Json($value) {
    if ($null -eq $value) { return 'null' }
    if ($value -is [string]) { return Quote-Json $value }
    if ($value -is [bool]) { if ($value) { return 'true' } else { return 'false' } }
    if ($value -is [System.Collections.IDictionary]) {
        $parts = @()
        foreach ($key in ($value.Keys | Sort-Object)) { $parts += ((Quote-Json ([string]$key)) + ':' + (Write-Json $value[$key])) }
        return ('{' + ($parts -join ',') + '}')
    }
    if ($value -is [System.Collections.IEnumerable]) {
        $parts = @()
        foreach ($item in $value) { $parts += (Write-Json $item) }
        return ('[' + ($parts -join ',') + ']')
    }
    return [Convert]::ToString($value, [Globalization.CultureInfo]::InvariantCulture)
}

$elevated = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
$cs = Read-Instances 'Win32_ComputerSystem' | Select-Object -First 1
$os = Read-Instances 'Win32_OperatingSystem' | Select-Object -First 1
$cpu = Read-Instances 'Win32_Processor' | Select-Object -First 1
$enc = Read-Instances 'Win32_SystemEnclosure' | Select-Object -First 1
$system = @{
    manufacturer=[string]$cs.Manufacturer; model=[string]$cs.Model
    os=[string]$os.Caption; osVersion=[string]$os.Version
    cpu=[string]$cpu.Name; cores=[int]$cpu.NumberOfCores
    threads=[int]$cs.NumberOfLogicalProcessors; ramBytes=[uint64]$cs.TotalPhysicalMemory
    chassis=(Chassis-Name $enc.ChassisTypes); lastBoot=(Boot-Date $os.LastBootUpTime)
}
$notes = @()
$disks = @()
$modernDisks = @()
if (!$legacy -and (Get-Command Get-PhysicalDisk -ErrorAction SilentlyContinue)) {
    try { $modernDisks = @(Get-PhysicalDisk -ErrorAction Stop) } catch {}
}
if ($modernDisks.Count -gt 0) {
    $sysDisk = -1
    try { $sysDisk = (Get-Partition -DriveLetter ($env:SystemDrive).TrimEnd(':') -ErrorAction Stop | Get-Disk -ErrorAction Stop).Number } catch {}
    $rel = @{}
    if ($elevated -and (Get-Command Get-StorageReliabilityCounter -ErrorAction SilentlyContinue)) {
        foreach ($p in $modernDisks) {
            try { $rel[[string]$p.DeviceId] = $p | Get-StorageReliabilityCounter -ErrorAction Stop } catch {}
        }
    }
    foreach ($p in $modernDisks) {
        $r = $rel[[string]$p.DeviceId]
        $d = @{
            deviceId=[string]$p.DeviceId; model=[string]$p.FriendlyName
            mediaType=[string]$p.MediaType; busType=[string]$p.BusType
            healthStatus=[string]$p.HealthStatus; sizeBytes=[uint64]$p.Size
            isSystemDisk=([string]$p.DeviceId -eq [string]$sysDisk)
        }
        $fields = @{
            temperatureC='Temperature'; temperatureMaxC='TemperatureMax'; wearPercent='Wear'
            powerOnHours='PowerOnHours'; startStopCycles='StartStopCycleCount'
            readErrorsTotal='ReadErrorsTotal'; readErrorsUncorrected='ReadErrorsUncorrected'
            writeErrorsTotal='WriteErrorsTotal'; writeErrorsUncorrected='WriteErrorsUncorrected'
        }
        foreach ($key in $fields.Keys) { if ($r -and $null -ne $r.PSObject.Properties[$fields[$key]].Value) { $d[$key] = $r.PSObject.Properties[$fields[$key]].Value } }
        $disks += $d
    }
} else {
    $notes += 'legacy-storage'
    $systemIds = @{}
    # Association queries contain only a validated drive letter, never user text.
    if ($env:SystemDrive -match '^[A-Za-z]:$') {
        try {
            $q = "ASSOCIATORS OF {Win32_LogicalDisk.DeviceID='" + $env:SystemDrive + "'} WHERE AssocClass=Win32_LogicalDiskToPartition"
            foreach ($partition in (Get-WmiObject -Query $q -ErrorAction Stop)) { $systemIds[[string]$partition.DiskIndex] = $true }
        } catch {}
    }
    foreach ($p in (Read-Instances 'Win32_DiskDrive')) {
        # WMI cannot reliably distinguish HDD, SSD or NVMe. Do not guess from
        # model strings or translate WMI Status=OK into SMART Healthy.
        $bus = 'Unspecified'
        if ($p.InterfaceType -eq 'USB') { $bus='USB' }
        $disks += @{
            deviceId=[string]$p.Index; model=[string]$p.Model; mediaType='Unspecified'
            busType=$bus; healthStatus=''; sizeBytes=[uint64]$p.Size
            isSystemDisk=[bool]$systemIds[[string]$p.Index]
        }
    }
}
$volumes = @()
if (!$legacy -and (Get-Command Get-Volume -ErrorAction SilentlyContinue)) {
    try {
        foreach ($v in (Get-Volume -ErrorAction Stop | Where-Object { $_.DriveLetter })) {
            $volumes += @{
                driveLetter=[string]$v.DriveLetter; fileSystem=[string]$v.FileSystemType
                healthStatus=[string]$v.HealthStatus; operationalStatus=(@($v.OperationalStatus) -join ', ')
                sizeBytes=[uint64]$v.Size; freeBytes=[uint64]$v.SizeRemaining
            }
        }
    } catch {}
}
if ($volumes.Count -eq 0) {
    $notes += 'legacy-volumes'
    foreach ($v in (Read-Instances 'Win32_LogicalDisk')) {
        # Skip disconnected media and network providers: no unbounded remote IO.
        if (($v.DriveType -eq 2 -or $v.DriveType -eq 3) -and $v.Size -gt 0) {
            $volumes += @{
                driveLetter=([string]$v.DeviceID).TrimEnd(':'); fileSystem=[string]$v.FileSystem
                healthStatus=''; operationalStatus=''; sizeBytes=[uint64]$v.Size; freeBytes=[uint64]$v.FreeSpace
            }
        }
    }
}
$thermals = @()
foreach ($t in (Read-Instances 'MSAcpi_ThermalZoneTemperature' 'root\wmi')) {
    if ($t.CurrentTemperature) { $thermals += @{name=[string]$t.InstanceName; celsius=[math]::Round(($t.CurrentTemperature/10.0)-273.15,1)} }
}
$battery = $null
$bw = Read-Instances 'Win32_Battery' | Select-Object -First 1
if ($bw) {
    $battery = @{name=[string]$bw.Name}
    if ($null -ne $bw.EstimatedChargeRemaining) { $battery.chargePercent=[int]$bw.EstimatedChargeRemaining }
    $bs = Read-Instances 'BatteryStaticData' 'root\wmi' | Select-Object -First 1
    $bf = Read-Instances 'BatteryFullChargedCapacity' 'root\wmi' | Select-Object -First 1
    if ($bs -and $bs.DesignedCapacity -gt 0) { $battery.designCapacity=[int]$bs.DesignedCapacity }
    if ($bf -and $bf.FullChargedCapacity -gt 0) { $battery.fullCapacity=[int]$bf.FullChargedCapacity }
    if ($bs -and $bs.CycleCount -gt 0) { $battery.cycleCount=[int]$bs.CycleCount }
}
$result = @{elevated=[bool]$elevated; system=$system; disks=@($disks); volumes=@($volumes); thermals=@($thermals); battery=$battery; collectionNotes=@($notes)}
if (!$legacy -and (Get-Command ConvertTo-Json -ErrorAction SilentlyContinue)) {
    $result | ConvertTo-Json -Depth 6 -Compress
} else { Write-Json $result }