#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
Usage: tf-services.sh <init|write-vars|write-destroy-vars|plan|apply|destroy|rerun-migrator> [options]
  --region REGION
  --project-name NAME
  --environment ENV
  --state-bucket BUCKET
  --foundation-state-key KEY
  --foundation-state-region REGION
  --state-region REGION
  --platform-state-key KEY
  --services-state-key KEY
  --lock-table TABLE
  --account-id ACCOUNT_ID
  --tag TAG
  --destroy-tag TAG
EOF
}

die() {
  printf '%s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

command_name="${1:-}"
if [[ -z "$command_name" ]]; then
  usage
  exit 1
fi
shift

region="${AWS_REGION:-${TF_VAR_aws_region:-us-east-1}}"
project_name="${TF_PROJECT_NAME:-${PROJECT_NAME:-peer-ledger}}"
environment="${TF_ENVIRONMENT:-${TF_VAR_environment:-production}}"
state_bucket="${TF_STATE_BUCKET:-${TF_VAR_terraform_state_bucket_name:-}}"
foundation_state_key="${TF_FOUNDATION_STATE_KEY:-${TF_VAR_foundation_state_key:-foundation/terraform.tfstate}}"
foundation_state_region="${TF_FOUNDATION_STATE_REGION:-${TF_VAR_foundation_state_region:-${AWS_REGION:-us-east-1}}}"
state_region="${TF_STATE_REGION:-${TF_VAR_state_region:-${foundation_state_region}}}"
platform_state_key="${TF_PLATFORM_STATE_KEY:-${TF_VAR_platform_state_key:-platform/terraform.tfstate}}"
services_state_key="${TF_SERVICES_STATE_KEY:-${SERVICES_STATE_KEY:-services/terraform.tfstate}}"
lock_table="${TF_LOCK_TABLE:-${TF_VAR_terraform_lock_table_name:-}}"
account_id="${AWS_ACCOUNT_ID:-}"
tag="${SERVICE_IMAGE_TAG:-}"
destroy_tag="${DESTROY_IMAGE_TAG:-destroy-placeholder}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --region)
      region="$2"
      shift 2
      ;;
    --project-name)
      project_name="$2"
      shift 2
      ;;
    --environment)
      environment="$2"
      shift 2
      ;;
    --state-bucket)
      state_bucket="$2"
      shift 2
      ;;
    --foundation-state-key)
      foundation_state_key="$2"
      shift 2
      ;;
    --foundation-state-region)
      foundation_state_region="$2"
      shift 2
      ;;
    --state-region)
      state_region="$2"
      shift 2
      ;;
    --platform-state-key)
      platform_state_key="$2"
      shift 2
      ;;
    --services-state-key)
      services_state_key="$2"
      shift 2
      ;;
    --lock-table)
      lock_table="$2"
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
    --destroy-tag)
      destroy_tag="$2"
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

require_command bash
require_command aws
require_command terraform

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
project_dir="$(cd "$script_dir/.." && pwd)"
repo_root="$(cd "$project_dir/.." && pwd)"
services_dir="$repo_root/infra/services"
backend_file="$repo_root/infra/backend.hcl"
var_file="$services_dir/service-images.auto.tfvars.json"
var_file_name="$(basename "$var_file")"

[[ -n "$state_bucket" ]] || die "TF state bucket is required"

export TF_VAR_aws_region="$region"
export TF_VAR_project_name="$project_name"
export TF_VAR_environment="$environment"
export TF_VAR_terraform_state_bucket_name="$state_bucket"
export TF_VAR_foundation_state_key="$foundation_state_key"
export TF_VAR_foundation_state_region="$foundation_state_region"
export TF_VAR_platform_state_key="$platform_state_key"
export TF_VAR_state_region="$state_region"

init_backend() {
  echo "[services] terraform init"

  if [[ -f "$backend_file" ]]; then
    (
      cd "$services_dir"
      terraform init -reconfigure -backend-config=../backend.hcl -backend-config="key=$services_state_key"
    )
    return
  fi

  [[ -n "$lock_table" ]] || die "TF lock table is required when infra/backend.hcl is not present"

  (
    cd "$services_dir"
    terraform init \
      -reconfigure \
      -backend-config="bucket=$state_bucket" \
      -backend-config="region=$state_region" \
      -backend-config="dynamodb_table=$lock_table" \
      -backend-config="key=$services_state_key"
  )
}

write_vars() {
  local writer_args=(--output "$var_file" --region "$region")

  if [[ -n "$account_id" ]]; then
    writer_args+=(--account-id "$account_id")
  fi

  if [[ "$1" == "destroy" ]]; then
    writer_args+=(--tag "$destroy_tag" --use-provided-tag)
  else
    if [[ -n "$tag" ]]; then
      writer_args+=(--tag "$tag")
    fi
  fi

  bash "$script_dir/write-service-images-tfvars.sh" "${writer_args[@]}"
}

run_terraform() {
  case "$1" in
    plan)
      echo "[services] terraform plan"
      (cd "$services_dir" && terraform plan -input=false -var-file="$var_file_name")
      ;;
    apply)
      echo "[services] terraform apply"
      (cd "$services_dir" && terraform apply -auto-approve -input=false -var-file="$var_file_name")
      ;;
    destroy)
      echo "[services] terraform destroy"
      (cd "$services_dir" && terraform destroy -auto-approve -input=false -var-file="$var_file_name")
      ;;
    rerun-migrator)
      echo "[services] terraform apply -replace=terraform_data.db_migrator_run"
      (cd "$services_dir" && terraform apply -replace=terraform_data.db_migrator_run -auto-approve -input=false -var-file="$var_file_name")
      ;;
    *)
      die "unsupported terraform command: $1"
      ;;
  esac
}

case "$command_name" in
  init)
    init_backend
    ;;
  write-vars)
    write_vars real
    ;;
  write-destroy-vars)
    write_vars destroy
    ;;
  plan)
    init_backend
    write_vars real
    run_terraform plan
    ;;
  apply)
    init_backend
    write_vars real
    run_terraform apply
    ;;
  destroy)
    init_backend
    write_vars destroy
    run_terraform destroy
    ;;
  rerun-migrator)
    init_backend
    write_vars real
    run_terraform rerun-migrator
    ;;
  *)
    usage
    die "unknown command: $command_name"
    ;;
esac
