param(
    [Parameter(Mandatory = $true)]
    [string]$OutputPath,
    [string]$Region = "us-east-1",
    [string]$AccountId = "",
    [string]$Tag = "",
    [switch]$UseProvidedTag
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($OutputPath)) {
    throw "OutputPath is required."
}

$resolveScript = Join-Path $PSScriptRoot "resolve-service-images.ps1"

if ($UseProvidedTag) {
    if ([string]::IsNullOrWhiteSpace($Tag)) {
        throw "Tag is required when -UseProvidedTag is set."
    }

    if ([string]::IsNullOrWhiteSpace($AccountId)) {
        $AccountId = (aws sts get-caller-identity --query Account --output text).Trim()
        if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($AccountId)) {
            throw "failed to resolve AWS account id"
        }
    }

    $registry = "$AccountId.dkr.ecr.$Region.amazonaws.com"
    $images = [ordered]@{
        gateway               = "$registry/peer-ledger-gateway:$Tag"
        "user-service"        = "$registry/peer-ledger-user-service:$Tag"
        "fraud-service"       = "$registry/peer-ledger-fraud-service:$Tag"
        "wallet-service"      = "$registry/peer-ledger-wallet-service:$Tag"
        "transaction-service" = "$registry/peer-ledger-transaction-service:$Tag"
        "db-migrator"         = "$registry/peer-ledger-db-migrator:$Tag"
    }
}
else {
    $resolvedJson = & $resolveScript -Region $Region -AccountId $AccountId -Tag $Tag
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($resolvedJson)) {
        throw "failed to resolve service images"
    }

    $images = $resolvedJson | ConvertFrom-Json
}

$payload = [ordered]@{
    service_images = $images
}

$directory = Split-Path -Parent $OutputPath
if (-not [string]::IsNullOrWhiteSpace($directory)) {
    New-Item -ItemType Directory -Force -Path $directory | Out-Null
}

$json = $payload | ConvertTo-Json -Compress
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($OutputPath, $json, $utf8NoBom)
