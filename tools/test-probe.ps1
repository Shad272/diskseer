# Tests use mocked providers, never the host's disk inventory.
$ErrorActionPreference = 'Stop'
$probe = Join-Path (Split-Path $PSScriptRoot -Parent) 'internal\collect\probe_windows.ps1'
$source = [IO.File]::ReadAllText($probe)
$functions = $source.Substring(0, $source.IndexOf('$elevated ='))
. ([scriptblock]::Create($functions))
$sample = @{text="Virgolette `" slash \ tab`t newline`n è 日本語"; empty=@(); single=@(1); null=$null; yes=$true; number=[uint64]9007199254740991}
$encoded = Write-Json $sample
$decoded = $encoded | ConvertFrom-Json
if ($decoded.text -ne $sample.text -or $decoded.empty.Count -ne 0 -or $decoded.single.Count -ne 1 -or $decoded.number -ne $sample.number -or $decoded.yes -ne $true) { throw 'Legacy JSON round-trip failed' }

$env:DISKSEER_LEGACY = '1'
$json = & {
    function Get-WmiObject {
        param($Class, $Namespace, $Query, $ErrorAction)
        if ($Query) { return [pscustomobject]@{DiskIndex=0} }
        switch ($Class) {
            'Win32_ComputerSystem' { [pscustomobject]@{Manufacturer='TEST'; Model='LEGACY'; NumberOfLogicalProcessors=2; TotalPhysicalMemory=2147483648} }
            'Win32_OperatingSystem' { [pscustomobject]@{Caption='Windows 7'; Version='6.1.7601'; LastBootUpTime='20260913080000.000000+000'} }
            'Win32_Processor' { [pscustomobject]@{Name='TEST CPU'; NumberOfCores=2} }
            'Win32_SystemEnclosure' { [pscustomobject]@{ChassisTypes=@(3)} }
            'Win32_DiskDrive' { [pscustomobject]@{Index=0; Model='TEST DRIVE'; Size=500000000000; InterfaceType='IDE'} }
            'Win32_LogicalDisk' { [pscustomobject]@{DeviceID='C:'; FileSystem='NTFS'; DriveType=3; Size=500000000000; FreeSpace=100000000000} }
        }
    }
    . ([scriptblock]::Create($source))
}
$env:DISKSEER_LEGACY = ''
$ErrorActionPreference = 'Stop'
$snapshot = $json | ConvertFrom-Json
if ($snapshot.disks.Count -ne 1 -or $snapshot.volumes.Count -ne 1 -or $snapshot.system.osVersion -ne '6.1.7601') { throw 'Legacy inventory failed' }
if ($snapshot.disks[0].healthStatus -ne '' -or $snapshot.disks[0].mediaType -ne 'Unspecified') { throw 'Unknown health/type was guessed' }
if ($snapshot.collectionNotes.Count -ne 2 -or !$snapshot.system.lastBoot) { throw 'Missing compatibility diagnostics or WMI boot date' }
Write-Output 'PASS: legacy JSON, WMI inventory, unknown data, compatibility notes and WMI timestamps'
