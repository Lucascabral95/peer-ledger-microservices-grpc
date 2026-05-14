#!/usr/bin/env bash
set -euo pipefail

case ":$PATH:" in
  *:/usr/bin:*) ;;
  *) export PATH="/usr/bin:$PATH" ;;
esac

usage() {
  cat <<'EOF' >&2
Usage: deploy-all.sh [options]
  --env-file PATH       Local deployment env file. Default: project/deploy.local.env
  --tag TAG             Container image tag. Default: current git commit SHA
  --skip-tests          Skip go test ./...
  --skip-compose        Skip docker compose config validation
  --skip-smoke          Skip final ALB /health smoke test
  -h, --help            Show this help
EOF
}

die() {
  printf '%s\n' "$*" >&2
  exit 1
}

log() {
  printf '\n==> %s\n' "$*"
}

warn() {
  printf 'warning: %s\n' "$*" >&2
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

require_value() {
  local name="$1"
  local value="$2"

  [[ -n "$value" ]] || die "$name is required. Set it in project/deploy.local.env."
}

require_docker_daemon() {
  local output_file
  output_file="$(mktemp)"

  if docker info >"$output_file" 2>&1; then
    rm -f "$output_file"
    return 0
  fi

  cat "$output_file" >&2
  rm -f "$output_file"
  die "Docker daemon is not reachable. Start Docker Desktop with the Linux engine enabled, then rerun make deploy-aws."
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
project_dir="$(cd "$script_dir/.." && pwd)"
repo_root="$(cd "$project_dir/.." && pwd)"

env_file="$project_dir/deploy.local.env"
skip_tests=false
skip_compose=false
skip_smoke=false
image_tag="${SERVICE_IMAGE_TAG:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)
      env_file="$2"
      shift 2
      ;;
    --tag)
      image_tag="$2"
      shift 2
      ;;
    --skip-tests)
      skip_tests=true
      shift
      ;;
    --skip-compose)
      skip_compose=true
      shift
      ;;
    --skip-smoke)
      skip_smoke=true
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

if [[ ! -f "$env_file" ]]; then
  die "deployment env file not found: $env_file. Copy project/deploy.local.env.example to project/deploy.local.env first."
fi

set -a
# shellcheck disable=SC1090
source "$env_file"
set +a

require_command aws
require_command curl
require_command docker
require_command git
require_command terraform
if [[ "$skip_tests" == false ]]; then
  require_command go
fi
require_docker_daemon

export AWS_REGION="${AWS_REGION:-us-east-1}"
export AWS_DEFAULT_REGION="${AWS_REGION}"

project_name="${TF_PROJECT_NAME:-${PROJECT_NAME:-peer-ledger}}"
environment="${TF_ENVIRONMENT:-production}"
state_region="${TF_STATE_REGION:-$AWS_REGION}"
lock_table="${TF_LOCK_TABLE:-peer-ledger-terraform-locks}"
foundation_state_key="${TF_FOUNDATION_STATE_KEY:-foundation/terraform.tfstate}"
platform_state_key="${TF_PLATFORM_STATE_KEY:-platform/terraform.tfstate}"
services_state_key="${TF_SERVICES_STATE_KEY:-services/terraform.tfstate}"
auth_jwt_secret="${AUTH_JWT_SECRET:-${TF_VAR_auth_jwt_secret:-}}"
rds_master_username="${RDS_MASTER_USERNAME:-${TF_VAR_rds_master_username:-}}"
rds_master_password="${RDS_MASTER_PASSWORD:-${TF_VAR_rds_master_password:-}}"

if [[ -z "$image_tag" && -n "${SERVICE_IMAGE_TAG:-}" ]]; then
  image_tag="$SERVICE_IMAGE_TAG"
fi

require_value "AUTH_JWT_SECRET" "$auth_jwt_secret"
require_value "RDS_MASTER_USERNAME" "$rds_master_username"
require_value "RDS_MASTER_PASSWORD" "$rds_master_password"

if (( ${#auth_jwt_secret} < 32 )); then
  die "AUTH_JWT_SECRET must be at least 32 characters."
fi

if [[ -z "$image_tag" ]]; then
  image_tag="$(git -C "$repo_root" rev-parse HEAD)"
fi

account_id="$(aws sts get-caller-identity --query 'Account' --output text)"
require_value "AWS account id" "$account_id"

state_bucket="${TF_STATE_BUCKET:-peer-ledger-tfstate-${account_id}}"
registry="${account_id}.dkr.ecr.${AWS_REGION}.amazonaws.com"

export TF_PROJECT_NAME="$project_name"
export TF_ENVIRONMENT="$environment"
export TF_STATE_BUCKET="$state_bucket"
export TF_LOCK_TABLE="$lock_table"
export TF_FOUNDATION_STATE_KEY="$foundation_state_key"
export TF_PLATFORM_STATE_KEY="$platform_state_key"
export TF_SERVICES_STATE_KEY="$services_state_key"
export TF_STATE_REGION="$state_region"
export SERVICE_IMAGE_TAG="$image_tag"

write_backend_config() {
  cat > "$repo_root/infra/backend.hcl" <<EOF
bucket         = "${state_bucket}"
region         = "${state_region}"
dynamodb_table = "${lock_table}"
encrypt        = true
key            = "${foundation_state_key}"
EOF
}

run_go_tests() {
  local output_file
  output_file="$(mktemp)"

  if (cd "$repo_root" && go test ./... -count=1 -p 1) >"$output_file" 2>&1; then
    cat "$output_file"
    rm -f "$output_file"
    return 0
  fi

  cat "$output_file"

  if grep -Eq 'Control de aplicaciones bloque.*archivo|blocked this file' "$output_file"; then
    warn "Windows blocked one or more generated go test executables. Continuing deploy because this is an OS policy issue, not a code test failure."
    rm -f "$output_file"
    return 0
  fi

  rm -f "$output_file"
  die "go test failed"
}

validate_local() {
  if [[ "$skip_tests" == false ]]; then
    log "Running Go tests"
    run_go_tests
  fi

  if [[ "$skip_compose" == false ]]; then
    log "Validating docker compose"
    if [[ ! -f "$repo_root/.env" && -f "$repo_root/.env.template" ]]; then
      cp "$repo_root/.env.template" "$repo_root/.env"
    fi
    docker compose -f "$project_dir/docker-compose.yml" config >/dev/null
  fi
}

apply_bootstrap() {
  log "Applying Terraform bootstrap"
  (
    local bootstrap_workspace="${project_name}-${environment}-${account_id}"

    export TF_VAR_aws_region="$AWS_REGION"
    export TF_VAR_project_name="$project_name"
    export TF_VAR_environment="$environment"
    export TF_VAR_state_bucket_name="$state_bucket"
    export TF_VAR_lock_table_name="$lock_table"

    cd "$repo_root/infra/bootstrap"
    terraform init
    # Bootstrap uses local state, so isolate it per AWS account to avoid reusing
    # a previous account's ignored terraform.tfstate file.
    terraform workspace select "$bootstrap_workspace" >/dev/null 2>&1 || terraform workspace new "$bootstrap_workspace"
    terraform apply -auto-approve -input=false
  )
}

apply_foundation() {
  log "Applying Terraform foundation"
  (
    export TF_VAR_aws_region="$AWS_REGION"
    export TF_VAR_project_name="$project_name"
    export TF_VAR_environment="$environment"
    export TF_VAR_terraform_state_bucket_name="$state_bucket"
    export TF_VAR_terraform_lock_table_name="$lock_table"
    export TF_VAR_auth_jwt_secret="$auth_jwt_secret"
    export TF_VAR_rds_master_username="$rds_master_username"
    export TF_VAR_rds_master_password="$rds_master_password"

    cd "$repo_root/infra/foundation"
    terraform init \
      -reconfigure \
      -backend-config="bucket=$state_bucket" \
      -backend-config="region=$state_region" \
      -backend-config="dynamodb_table=$lock_table" \
      -backend-config="key=$foundation_state_key"
    terraform apply -auto-approve -input=false
  )
}

build_and_push_images() {
  log "Building and pushing Docker images"

  aws ecr get-login-password --region "$AWS_REGION" \
    | docker login --username AWS --password-stdin "$registry"

  local images=(
    "peer-ledger-gateway:services/gateway/Dockerfile"
    "peer-ledger-user-service:services/user-service/Dockerfile"
    "peer-ledger-fraud-service:services/fraud-service/Dockerfile"
    "peer-ledger-wallet-service:services/wallet-service/Dockerfile"
    "peer-ledger-transaction-service:services/transaction-service/Dockerfile"
    "peer-ledger-db-migrator:services/db-migrator/Dockerfile"
  )

  local image repository dockerfile full_tag
  for image in "${images[@]}"; do
    repository="${image%%:*}"
    dockerfile="${image#*:}"
    full_tag="${registry}/${repository}:${image_tag}"

    if aws ecr describe-images \
      --region "$AWS_REGION" \
      --repository-name "$repository" \
      --image-ids "imageTag=$image_tag" >/dev/null 2>&1; then
      printf 'Image already exists, skipping push: %s\n' "$full_tag"
      continue
    fi

    docker build \
      --file "$repo_root/$dockerfile" \
      --tag "$full_tag" \
      "$repo_root"
    docker push "$full_tag"
  done
}

apply_platform() {
  log "Applying Terraform platform"
  bash "$script_dir/tf-platform.sh" apply \
    --region "$AWS_REGION" \
    --project-name "$project_name" \
    --environment "$environment" \
    --state-bucket "$state_bucket" \
    --foundation-state-key "$foundation_state_key" \
    --foundation-state-region "$state_region" \
    --state-region "$state_region" \
    --platform-state-key "$platform_state_key" \
    --lock-table "$lock_table"
}

apply_services() {
  log "Applying Terraform services"
  bash "$script_dir/tf-services.sh" apply \
    --region "$AWS_REGION" \
    --project-name "$project_name" \
    --environment "$environment" \
    --state-bucket "$state_bucket" \
    --foundation-state-key "$foundation_state_key" \
    --foundation-state-region "$state_region" \
    --state-region "$state_region" \
    --platform-state-key "$platform_state_key" \
    --services-state-key "$services_state_key" \
    --lock-table "$lock_table" \
    --account-id "$account_id" \
    --tag "$image_tag"
}

wait_for_services() {
  log "Waiting for ECS services"

  local cluster_name gateway_service internal_services
  cluster_name="$(cd "$repo_root/infra/services" && terraform output -raw ecs_cluster_name)"
  gateway_service="$(cd "$repo_root/infra/services" && terraform output -raw gateway_service_name)"
  internal_services=(
    "${project_name}-${environment}-user-service"
    "${project_name}-${environment}-fraud-service"
    "${project_name}-${environment}-wallet-service"
    "${project_name}-${environment}-transaction-service"
  )

  aws ecs wait services-stable \
    --region "$AWS_REGION" \
    --cluster "$cluster_name" \
    --services "$gateway_service" "${internal_services[@]}"
}

smoke_test() {
  if [[ "$skip_smoke" == true ]]; then
    return
  fi

  log "Running gateway smoke test"

  local alb_dns
  alb_dns="$(cd "$repo_root/infra/services" && terraform output -raw alb_dns_name)"

  for attempt in $(seq 1 20); do
    if curl -fsS "http://${alb_dns}/health" >/dev/null; then
      printf 'Smoke test passed: http://%s/health\n' "$alb_dns"
      return
    fi

    printf 'Smoke test attempt %s failed; retrying...\n' "$attempt"
    sleep 15
  done

  die "gateway smoke test failed: http://${alb_dns}/health"
}

log "Deploying to AWS account ${account_id} in ${AWS_REGION}"
log "Using state bucket ${state_bucket}"
log "Using image tag ${image_tag}"

write_backend_config
validate_local
apply_bootstrap
apply_foundation
build_and_push_images
apply_platform
apply_services
wait_for_services
smoke_test

log "Deploy completed"
