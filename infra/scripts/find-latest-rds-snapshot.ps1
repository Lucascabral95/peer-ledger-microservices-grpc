$ErrorActionPreference = "Stop"

$inputJson = [Console]::In.ReadToEnd()
$query = @{}

if (-not [string]::IsNullOrWhiteSpace($inputJson)) {
    $query = $inputJson | ConvertFrom-Json -AsHashtable
}

if (-not (Get-Command aws -ErrorAction SilentlyContinue)) {
    throw "AWS CLI is required to resolve the latest RDS snapshot."
}

$dbInstanceIdentifier = $query["db_instance_identifier"]
$snapshotType = if ($query.ContainsKey("snapshot_type")) { $query["snapshot_type"] } else { "manual" }
$snapshotPrefix = if ($query.ContainsKey("snapshot_prefix")) { $query["snapshot_prefix"] } else { "" }

if ([string]::IsNullOrWhiteSpace($dbInstanceIdentifier)) {
    throw "db_instance_identifier is required."
}

$response = aws rds describe-db-snapshots `
    --db-instance-identifier $dbInstanceIdentifier `
    --snapshot-type $snapshotType `
    --output json 2>$null

if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($response)) {
    @{ snapshot_identifier = "" } | ConvertTo-Json -Compress
    exit 0
}

$payload = $response | ConvertFrom-Json
$snapshots = @($payload.DBSnapshots)

if (-not [string]::IsNullOrWhiteSpace($snapshotPrefix)) {
    $snapshots = $snapshots | Where-Object { $_.DBSnapshotIdentifier -like "$snapshotPrefix*" }
}

$snapshots = $snapshots |
    Where-Object { $_.Status -eq "available" } |
    Sort-Object SnapshotCreateTime -Descending

$latestSnapshot = ""
if ($snapshots.Count -gt 0) {
    $latestSnapshot = $snapshots[0].DBSnapshotIdentifier
}

@{ snapshot_identifier = $latestSnapshot } | ConvertTo-Json -Compress
