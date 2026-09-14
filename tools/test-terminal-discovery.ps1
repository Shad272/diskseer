$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$source = [IO.File]::ReadAllText((Join-Path $root 'internal\terminal\terminal_windows.go'))
$probe = [regex]::Match($source, '(?s)const appxProbe = `(.*?)`').Groups[1].Value
if (!$probe) { throw 'Discovery script not found' }
$output = & {
    function Get-AppxPackage { [pscustomobject]@{PackageFullName='Variant_2026'; InstallLocation='C:\Packages\Variant Preview 2026'} }
    function Get-AppxPackageManifest {
        param($Package)
        if ($Package -ne 'Variant_2026') { throw 'Wrong package identity' }
        [xml]'<Package xmlns:x="urn:test"><Applications><Application Executable="Terminal.exe"><Extensions><Extension><x:ExecutionAlias Alias="wt.exe" /></Extension></Extensions></Application><Application Executable="Other.exe"><Extensions><Extension><x:ExecutionAlias Alias="other.exe" /></Extension></Extensions></Application></Applications></Package>'
    }
    function Test-Path { param($LiteralPath) return $true }
    # The production script writes Console output. Capture it locally so the
    # assertion tests the real XML traversal including namespaced extensions.
    $writer=New-Object IO.StringWriter
    $previous=[Console]::Out
    [Console]::SetOut($writer)
    try { . ([scriptblock]::Create($probe)); $writer.ToString() }
    finally { [Console]::SetOut($previous); $writer.Dispose() }
}
if ($output.Trim() -ne 'C:\Packages\Variant Preview 2026\Terminal.exe') { throw ('Manifest discovery failed: '+$output) }
Write-Output 'PASS: registered alias discovery without PATH or fixed package names, including namespaced Preview manifests'
