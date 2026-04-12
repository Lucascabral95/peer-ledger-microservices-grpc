param(
    [string]$Region = "us-east-1",
    [string]$AccountId = "",
    [string]$Tag = ""
)

$ErrorActionPreference = "Stop"

$repositories = @(
    @{ Key = "gateway"; Name = "peer-ledger-gateway" },
    @{ Key = "user-service"; Name = "peer-ledger-user-service" },
    @{ Key = "fraud-service"; Name = "peer-ledger-fraud-service" },
    @{ Key = "wallet-service"; Name = "peer-ledger-wallet-service" },
    @{ Key = "transaction-service"; Name = "peer-ledger-transaction-service" },
    @{ Key = "db-migrator"; Name = "peer-ledger-db-migrator" }
)

function Get-RepositoryTags {
    param(
        [string]$RepositoryName
    )

    $response = aws ecr describe-images --region $Region --repository-name $RepositoryName --output json 2>$null
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($response)) {
        throw "failed to list images for repository '$RepositoryName'"
    }

    $payload = $response | ConvertFrom-Json
    $tagInfo = @{}

    foreach ($detail in @($payload.imageDetails)) {
        $pushedAt = Get-Date $detail.imagePushedAt
        foreach ($imageTag in @($detail.imageTags)) {
            if ([string]::IsNullOrWhiteSpace($imageTag)) {
                continue
            }

            if (-not $tagInfo.ContainsKey($imageTag) -or $tagInfo[$imageTag] -lt $pushedAt) {
                $tagInfo[$imageTag] = $pushedAt
            }
        }
    }

    return $tagInfo
}

if ([string]::IsNullOrWhiteSpace($AccountId)) {
    $AccountId = (aws sts get-caller-identity --query Account --output text).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($AccountId)) {
        throw "failed to resolve AWS account id"
    }
}

$repositoryTags = @{}
foreach ($repository in $repositories) {
    $repositoryTags[$repository.Key] = Get-RepositoryTags -RepositoryName $repository.Name
}

$resolvedTag = $Tag.Trim()
if ([string]::IsNullOrWhiteSpace($resolvedTag)) {
    $gatewayCandidates = $repositoryTags["gateway"].GetEnumerator() |
        Sort-Object Value -Descending |
        ForEach-Object { $_.Key }

    foreach ($candidate in $gatewayCandidates) {
        $existsInAll = $true

        foreach ($repository in $repositories) {
            if (-not $repositoryTags[$repository.Key].ContainsKey($candidate)) {
                $existsInAll = $false
                break
            }
        }

        if ($existsInAll) {
            $resolvedTag = $candidate
            break
        }
    }
}

if ([string]::IsNullOrWhiteSpace($resolvedTag)) {
    throw "failed to resolve a common image tag across all ECR repositories"
}

foreach ($repository in $repositories) {
    if (-not $repositoryTags[$repository.Key].ContainsKey($resolvedTag)) {
        throw "image tag '$resolvedTag' does not exist in repository '$($repository.Name)'"
    }
}

$registry = "$AccountId.dkr.ecr.$Region.amazonaws.com"
$images = [ordered]@{}

foreach ($repository in $repositories) {
    $images[$repository.Key] = "$registry/$($repository.Name):$resolvedTag"
}

$images | ConvertTo-Json -Compress
