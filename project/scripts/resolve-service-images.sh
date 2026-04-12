#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
Usage: resolve-service-images.sh [--region REGION] [--account-id ACCOUNT_ID] [--tag TAG]
EOF
}

die() {
  printf '%s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

json_string() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  printf '"%s"' "$value"
}

repository_has_tag() {
  local region="$1"
  local repository_name="$2"
  local tag="$3"

  aws ecr describe-images \
    --region "$region" \
    --repository-name "$repository_name" \
    --image-ids "imageTag=$tag" >/dev/null 2>&1
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

region="us-east-1"
account_id=""
tag=""

while [[ $# -gt 0 ]]; do
  case "$1" in
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

require_command aws

repositories=(
  "gateway:peer-ledger-gateway"
  "user-service:peer-ledger-user-service"
  "fraud-service:peer-ledger-fraud-service"
  "wallet-service:peer-ledger-wallet-service"
  "transaction-service:peer-ledger-transaction-service"
  "db-migrator:peer-ledger-db-migrator"
)

if [[ -z "$account_id" ]]; then
  account_id="$(aws sts get-caller-identity --query 'Account' --output text 2>/dev/null || true)"
  [[ -n "$account_id" && "$account_id" != "None" ]] || die "failed to resolve AWS account id"
fi

if [[ -n "$tag" ]]; then
  for repository in "${repositories[@]}"; do
    repository_name="${repository#*:}"
    repository_has_tag "$region" "$repository_name" "$tag" || die "image tag '$tag' does not exist in repository '$repository_name'"
  done

  build_images_json "$region" "$account_id" "$tag"
  exit 0
fi

gateway_candidates="$(
  aws ecr describe-images \
    --region "$region" \
    --repository-name "peer-ledger-gateway" \
    --query 'reverse(sort_by(imageDetails[?imageTags != null], &imagePushedAt))[].imageTags[]' \
    --output text 2>/dev/null || true
)"

[[ -n "$gateway_candidates" ]] || die "failed to resolve a common image tag across all ECR repositories"

resolved_tag=""
seen_tags=""

while IFS= read -r candidate; do
  [[ -n "$candidate" && "$candidate" != "None" ]] || continue

  if printf '%s\n' "$seen_tags" | grep -Fqx "$candidate"; then
    continue
  fi
  seen_tags="${seen_tags}${candidate}"$'\n'

  exists_in_all=true
  for repository in "${repositories[@]}"; do
    repository_name="${repository#*:}"
    if ! repository_has_tag "$region" "$repository_name" "$candidate"; then
      exists_in_all=false
      break
    fi
  done

  if [[ "$exists_in_all" == true ]]; then
    resolved_tag="$candidate"
    break
  fi
done < <(printf '%s\n' "$gateway_candidates" | tr '\t' '\n')

[[ -n "$resolved_tag" ]] || die "failed to resolve a common image tag across all ECR repositories"

build_images_json "$region" "$account_id" "$resolved_tag"
