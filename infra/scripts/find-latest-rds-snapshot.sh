#!/usr/bin/env bash
set -euo pipefail

die() {
  printf '%s\n' "$*" >&2
  exit 1
}

extract_json_value() {
  local key="$1"
  local input="$2"

  printf '%s' "$input" | sed -n "s/.*\"${key}\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/p"
}

command -v aws >/dev/null 2>&1 || die "AWS CLI is required to resolve the latest RDS snapshot."

input_json="$(cat)"
db_instance_identifier="$(extract_json_value "db_instance_identifier" "$input_json")"
snapshot_type="$(extract_json_value "snapshot_type" "$input_json")"
snapshot_prefix="$(extract_json_value "snapshot_prefix" "$input_json")"

[[ -n "$db_instance_identifier" ]] || die "db_instance_identifier is required."
[[ -n "$snapshot_type" ]] || snapshot_type="manual"

query="reverse(sort_by(DBSnapshots[?Status=='available'], &SnapshotCreateTime))[0].DBSnapshotIdentifier"
if [[ -n "$snapshot_prefix" ]]; then
  query="reverse(sort_by(DBSnapshots[?Status=='available' && starts_with(DBSnapshotIdentifier, '${snapshot_prefix}')], &SnapshotCreateTime))[0].DBSnapshotIdentifier"
fi

snapshot_identifier="$(
  aws rds describe-db-snapshots \
    --db-instance-identifier "$db_instance_identifier" \
    --snapshot-type "$snapshot_type" \
    --query "$query" \
    --output text 2>/dev/null || true
)"

if [[ -z "$snapshot_identifier" || "$snapshot_identifier" == "None" ]]; then
  snapshot_identifier=""
fi

printf '{"snapshot_identifier":"%s"}\n' "$snapshot_identifier"
