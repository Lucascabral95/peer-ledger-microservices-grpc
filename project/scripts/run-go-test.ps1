param(
    [Parameter(Mandatory = $true)]
    [string]$PackagePath,

    [string]$CoverProfile,

    [string]$GoImage = "golang:1.25"
)

$ErrorActionPreference = "Stop"

$cacheRoot = Join-Path $env:LOCALAPPDATA "peer-ledger-go"
$cachePath = Join-Path $cacheRoot "cache"
$tmpPath = Join-Path (Get-Location).Path ".go-tmp"

New-Item -ItemType Directory -Force -Path $cachePath, $tmpPath | Out-Null

$env:GOCACHE = $cachePath
$env:GOTMPDIR = $tmpPath
$env:GOPROXY = "off"
$env:GOSUMDB = "off"

$goArgs = @(
    "test"
    "-mod=readonly"
    "-short"
    "-count=1"
)

if ($CoverProfile) {
    $goArgs += "-covermode=atomic"
    $goArgs += "-coverprofile=$CoverProfile"
}

$goArgs += $PackagePath

$captured = @()
& go @goArgs 2>&1 | Tee-Object -Variable captured
$exitCode = $LASTEXITCODE

if ($exitCode -eq 0) {
    exit 0
}

$appControlBlocked = $false
foreach ($line in $captured) {
    if ($line.ToString().Contains("Control de aplicaciones bloqueó este archivo")) {
        $appControlBlocked = $true
        break
    }
}

if (-not $appControlBlocked) {
    exit $exitCode
}

Write-Host "App Control bloqueo la ejecucion local de go test. Reintentando en Docker..."

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    Write-Error "Docker no esta disponible en PATH y la ejecucion local fue bloqueada por App Control."
    exit $exitCode
}

$repoRoot = Resolve-Path (Join-Path (Get-Location).Path "..")
$containerWorkdir = "/workspace/project"
$dockerArgs = @(
    "run"
    "--rm"
    "-v"
    ("{0}:/workspace" -f $repoRoot.Path)
    "-w"
    $containerWorkdir
    $GoImage
    "go"
)
$dockerArgs += $goArgs

& docker @dockerArgs
exit $LASTEXITCODE
