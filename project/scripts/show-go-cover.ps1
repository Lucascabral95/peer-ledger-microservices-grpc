param(
    [Parameter(Mandatory = $true)]
    [string]$CoverProfile,

    [switch]$Html,

    [string]$OutputFile
)

$ErrorActionPreference = "Stop"

if (!(Test-Path $CoverProfile)) {
    Write-Error "$CoverProfile no existe"
    exit 1
}

$cacheRoot = Join-Path $env:LOCALAPPDATA "peer-ledger-go"
$cachePath = Join-Path $cacheRoot "cache"
New-Item -ItemType Directory -Force -Path $cachePath | Out-Null
$env:GOCACHE = $cachePath

if ($Html) {
    if ([string]::IsNullOrWhiteSpace($OutputFile)) {
        Write-Error "OutputFile es requerido cuando se usa -Html"
        exit 1
    }
    & go tool cover "-html=$CoverProfile" "-o=$OutputFile"
    exit $LASTEXITCODE
}

& go tool cover "-func=$CoverProfile"
exit $LASTEXITCODE
