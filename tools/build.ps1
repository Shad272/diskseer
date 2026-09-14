param(
    [ValidateSet('modern','legacy','all')][string]$Edition = 'modern',
    [string]$Go = 'go',
    [string]$LegacyGo = ''
)
# No downloads, remote Git operations or global environment changes.
$ErrorActionPreference = 'Stop'
$root = Split-Path (Split-Path $MyInvocation.MyCommand.Path -Parent) -Parent
$oldLocation = Get-Location
$saved = @{}
foreach ($key in @('GOOS','GOARCH','GOAMD64','GO386','CGO_ENABLED','GOTOOLCHAIN')) { $saved[$key]=[Environment]::GetEnvironmentVariable($key,'Process') }
try {
    Set-Location -LiteralPath $root
    if (!(Test-Path -LiteralPath 'dist')) { [void](New-Item -ItemType Directory -Path 'dist') }
    $env:GOOS='windows'; $env:GOAMD64='v1'; $env:GO386='softfloat'; $env:CGO_ENABLED='0'; $env:GOTOOLCHAIN='local'
    $lanes = @()
    if ($Edition -eq 'modern' -or $Edition -eq 'all') { $lanes += @{Name='modern'; Compiler=$Go; Architectures=@('amd64','386','arm64')} }
    if ($Edition -eq 'legacy' -or $Edition -eq 'all') {
        if (!$LegacyGo) { throw 'Legacy builds require -LegacyGo pointing to the official Go 1.20.14 compiler. No toolchains are downloaded automatically.' }
        $legacyVersion = & $LegacyGo version
        if ($LASTEXITCODE -ne 0 -or $legacyVersion -notmatch 'go1\.20\.14 ') { throw 'Windows 7/8.1 builds must use Go 1.20.14, not a newer runtime.' }
        $lanes += @{Name='legacy'; Compiler=$LegacyGo; Architectures=@('amd64','386')}
    }
    $checksums = @()
    foreach ($lane in $lanes) {
        foreach ($arch in $lane.Architectures) {
            $env:GOARCH=$arch
            $name='diskseer-'+$lane.Name+'-'+$arch+'.exe'
            $output=Join-Path 'dist' $name
            & $lane.Compiler build -trimpath -buildvcs=false -ldflags '-s -w' -o $output .
            if ($LASTEXITCODE -ne 0) { throw ('Build failed: '+$name) }
            $stream=[IO.File]::OpenRead((Join-Path $root $output))
            $sha=[Security.Cryptography.SHA256]::Create()
            try { $hash=([BitConverter]::ToString($sha.ComputeHash($stream))).Replace('-','').ToLowerInvariant() }
            finally { $stream.Dispose(); $sha.Dispose() }
            $checksums += ($hash+'  '+$name)
            Write-Output ('Built '+$name)
        }
    }
    [IO.File]::WriteAllLines((Join-Path $root ('dist\SHA256SUMS-'+$Edition+'.txt')), [string[]]$checksums)
} finally {
    foreach ($key in $saved.Keys) { [Environment]::SetEnvironmentVariable($key,$saved[$key],'Process') }
    Set-Location -LiteralPath $oldLocation.Path
}
