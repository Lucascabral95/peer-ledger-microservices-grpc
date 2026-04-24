#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF' >&2
Usage: run-db-migrator.sh --cluster NAME --task-definition ARN --subnets subnet-1,subnet-2 --security-group-id SG_ID [--region REGION] [--log-group LOG_GROUP]
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
log_group=""
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
    --log-group)
      log_group="$2"
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

print_task_logs() {
  if [[ -z "$log_group" ]]; then
    printf '%s\n' "db-migrator log group was not provided; skipping CloudWatch logs" >&2
    return 0
  fi

  local task_id="${task_arn##*/}"
  local log_stream="ecs/db-migrator/${task_id}"

  printf '\n%s\n' "--- db-migrator CloudWatch logs: ${log_group}/${log_stream} ---" >&2
  if ! aws "${aws_args[@]}" logs get-log-events \
    --log-group-name "$log_group" \
    --log-stream-name "$log_stream" \
    --start-from-head \
    --query 'events[].message' \
    --output text >&2; then
    printf '%s\n' "failed to fetch db-migrator CloudWatch logs" >&2
  fi
  printf '%s\n\n' "--- end db-migrator CloudWatch logs ---" >&2
}

exit_code="$(
  aws "${aws_args[@]}" ecs describe-tasks \
    --cluster "$cluster" \
    --tasks "$task_arn" \
    --query "$exit_code_query" \
    --output text
)"

if [[ -z "$exit_code" || "$exit_code" == "None" ]]; then
  print_task_logs
  aws "${aws_args[@]}" ecs describe-tasks --cluster "$cluster" --tasks "$task_arn" >&2
  die "db-migrator task exit code could not be determined"
fi

if [[ "$exit_code" != "0" ]]; then
  print_task_logs
  aws "${aws_args[@]}" ecs describe-tasks --cluster "$cluster" --tasks "$task_arn" >&2
  die "db-migrator task exited with code $exit_code"
fi
