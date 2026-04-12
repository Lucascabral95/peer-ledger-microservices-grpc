#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
Usage: run-db-migrator.sh --cluster NAME --task-definition ARN --subnets subnet-1,subnet-2 --security-group-id SG_ID [--region REGION]
EOF
}

die() {
  printf '%s\n' "$*" >&2
  exit 1
}

cluster=""
task_definition=""
subnets=""
security_group_id=""
region="${AWS_REGION:-${TF_VAR_aws_region:-}}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --cluster)
      cluster="$2"
      shift 2
      ;;
    --task-definition)
      task_definition="$2"
      shift 2
      ;;
    --subnets)
      subnets="$2"
      shift 2
      ;;
    --security-group-id)
      security_group_id="$2"
      shift 2
      ;;
    --region)
      region="$2"
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

command -v aws >/dev/null 2>&1 || die "required command not found: aws"

[[ -n "$cluster" ]] || die "cluster is required"
[[ -n "$task_definition" ]] || die "task definition is required"
[[ -n "$subnets" ]] || die "subnets are required"
[[ -n "$security_group_id" ]] || die "security group id is required"

aws_args=()
if [[ -n "$region" ]]; then
  aws_args+=(--region "$region")
fi

network_configuration="awsvpcConfiguration={subnets=[${subnets}],securityGroups=[${security_group_id}],assignPublicIp=DISABLED}"
exit_code_query='tasks[0].containers[?name==`db-migrator`].exitCode | [0]'

task_arn="$(
  aws "${aws_args[@]}" ecs run-task \
    --cluster "$cluster" \
    --launch-type FARGATE \
    --task-definition "$task_definition" \
    --network-configuration "$network_configuration" \
    --query 'tasks[0].taskArn' \
    --output text
)"

if [[ -z "$task_arn" || "$task_arn" == "None" ]]; then
  die "failed to start db-migrator task"
fi

aws "${aws_args[@]}" ecs wait tasks-stopped --cluster "$cluster" --tasks "$task_arn"

exit_code="$(
  aws "${aws_args[@]}" ecs describe-tasks \
    --cluster "$cluster" \
    --tasks "$task_arn" \
    --query "$exit_code_query" \
    --output text
)"

if [[ -z "$exit_code" || "$exit_code" == "None" ]]; then
  aws "${aws_args[@]}" ecs describe-tasks --cluster "$cluster" --tasks "$task_arn" >&2
  die "db-migrator task exit code could not be determined"
fi

if [[ "$exit_code" != "0" ]]; then
  aws "${aws_args[@]}" ecs describe-tasks --cluster "$cluster" --tasks "$task_arn" >&2
  die "db-migrator task exited with code $exit_code"
fi
