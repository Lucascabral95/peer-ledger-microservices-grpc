$ErrorActionPreference = "Stop"

$cacheRoot = Join-Path $env:LOCALAPPDATA "peer-ledger-go"
$tmpPath = Join-Path (Get-Location).Path ".go-tmp"

if (Test-Path $cacheRoot) {
    Remove-Item -Recurse -Force $cacheRoot
}

if (Test-Path $tmpPath) {
    Remove-Item -Recurse -Force $tmpPath
}
