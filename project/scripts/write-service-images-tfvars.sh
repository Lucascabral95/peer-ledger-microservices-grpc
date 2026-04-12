#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
Usage: write-service-images-tfvars.sh --output PATH [--region REGION] [--account-id ACCOUNT_ID] [--tag TAG] [--use-provided-tag]
EOF
}

die() {
  printf '%s\n' "$*" >&2
  exit 1
}

json_string() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  printf '"%s"' "$value"
}

build_images_json() {
  local region="$1"
  local account_id="$2"
  local tag="$3"
  local registry="${account_id}.dkr.ecr.${region}.amazonaws.com"

  printf '{'
  printf '"gateway":%s,' "$(json_string "${registry}/peer-ledger-gateway:${tag}")"
  printf '"user-service":%s,' "$(json_string "${registry}/peer-ledger-user-service:${tag}")"
  printf '"fraud-service":%s,' "$(json_string "${registry}/peer-ledger-fraud-service:${tag}")"
  printf '"wallet-service":%s,' "$(json_string "${registry}/peer-ledger-wallet-service:${tag}")"
  printf '"transaction-service":%s,' "$(json_string "${registry}/peer-ledger-transaction-service:${tag}")"
  printf '"db-migrator":%s' "$(json_string "${registry}/peer-ledger-db-migrator:${tag}")"
  printf '}'
}

output=""
region="us-east-1"
account_id=""
tag=""
use_provided_tag=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output)
      output="$2"
      shift 2
      ;;
    --region)
      region="$2"
      shift 2
      ;;
    --account-id)
      account_id="$2"
      shift 2
      ;;
    --tag)
      tag="$2"
      shift 2
      ;;
    --use-provided-tag)
      use_provided_tag=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      die "unknown argument: $1"
      ;;
  esac
done

[[ -n "$output" ]] || die "output path is required"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ "$use_provided_tag" == true ]]; then
  [[ -n "$tag" ]] || die "tag is required when --use-provided-tag is set"

  if [[ -z "$account_id" ]]; then
    account_id="$(aws sts get-caller-identity --query 'Account' --output text 2>/dev/null || true)"
    [[ -n "$account_id" && "$account_id" != "None" ]] || die "failed to resolve AWS account id"
  fi

  images_json="$(build_images_json "$region" "$account_id" "$tag")"
else
  resolve_args=(--region "$region")
  if [[ -n "$account_id" ]]; then
    resolve_args+=(--account-id "$account_id")
  fi
  if [[ -n "$tag" ]]; then
    resolve_args+=(--tag "$tag")
  fi

  images_json="$(bash "$script_dir/resolve-service-images.sh" "${resolve_args[@]}")"
fi

mkdir -p "$(dirname "$output")"
printf '{"service_images":%s}\n' "$images_json" > "$output"
