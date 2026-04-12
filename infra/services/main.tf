data "terraform_remote_state" "foundation" {
  backend = "s3"

  config = {
    bucket = var.terraform_state_bucket_name
    key    = var.foundation_state_key
    region = var.state_region
  }
}

data "terraform_remote_state" "platform" {
  backend = "s3"

  config = {
    bucket = var.terraform_state_bucket_name
    key    = var.platform_state_key
    region = var.state_region
  }
}

locals {
  default_tags = {
    Project     = var.project_name
    Environment = var.environment
    ManagedBy   = "Terraform"
    Stack       = "services"
  }

  name_prefix    = "${var.project_name}-${var.environment}"
  namespace_name = data.terraform_remote_state.platform.outputs.service_discovery_namespace_name
  rds_host       = data.terraform_remote_state.platform.outputs.rds_endpoint
  rds_port       = tostring(data.terraform_remote_state.platform.outputs.rds_port)

  gateway = {
    name   = "gateway"
    port   = 8080
    cpu    = 512
    memory = 1024
  }

  internal_services = {
    "user-service" = {
      port   = 50051
      cpu    = 256
      memory = 512
      env = [
        { name = "GRPC_PORT", value = "50051" },
        { name = "USER_DB_HOST", value = local.rds_host },
        { name = "USER_DB_PORT", value = local.rds_port },
        { name = "USER_DB_NAME", value = "users_db" },
        { name = "USER_DB_SSLMODE", value = "require" },
        { name = "USER_PASSWORD_HASH_ITERATIONS", value = "120000" },
        { name = "USER_PASSWORD_MIN_LENGTH", value = "8" },
        { name = "DB_MAX_OPEN_CONNS", value = "25" },
        { name = "DB_MAX_IDLE_CONNS", value = "10" },
        { name = "DB_CONN_MAX_LIFETIME", value = "30m" },
        { name = "DB_CONN_MAX_IDLE_TIME", value = "5m" },
        { name = "DB_CONNECT_TIMEOUT", value = "3s" },
        { name = "DB_CONNECT_MAX_RETRIES", value = "8" },
        { name = "DB_CONNECT_INITIAL_BACKOFF", value = "500ms" },
        { name = "DB_CONNECT_MAX_BACKOFF", value = "8s" },
        { name = "GRACEFUL_SHUTDOWN_TIMEOUT", value = "10s" }
      ]
      secrets = [
        { name = "USER_DB_USER", valueFrom = data.terraform_remote_state.foundation.outputs.rds_master_username_secret_arn },
        { name = "USER_DB_PASSWORD", valueFrom = data.terraform_remote_state.foundation.outputs.rds_master_password_secret_arn }
      ]
    }
    "fraud-service" = {
      port   = 50052
      cpu    = 256
      memory = 512
      env = [
        { name = "FRAUD_GRPC_PORT", value = "50052" },
        { name = "FRAUD_PER_TX_LIMIT", value = "20000" },
        { name = "FRAUD_DAILY_LIMIT", value = "50000" },
        { name = "FRAUD_VELOCITY_MAX_COUNT", value = "5" },
        { name = "FRAUD_VELOCITY_WINDOW", value = "10m" },
        { name = "FRAUD_PAIR_COOLDOWN", value = "30s" },
        { name = "FRAUD_IDEMPOTENCY_TTL", value = "24h" },
        { name = "FRAUD_TIMEZONE", value = "America/Argentina/Buenos_Aires" },
        { name = "FRAUD_CLEANUP_INTERVAL", value = "1m" },
        { name = "GRACEFUL_SHUTDOWN_TIMEOUT", value = "10s" }
      ]
      secrets = []
    }
    "wallet-service" = {
      port   = 50053
      cpu    = 256
      memory = 512
      env = [
        { name = "WALLET_GRPC_PORT", value = "50053" },
        { name = "WALLET_DB_HOST", value = local.rds_host },
        { name = "WALLET_DB_PORT", value = local.rds_port },
        { name = "WALLET_DB_NAME", value = "wallets_db" },
        { name = "WALLET_DB_SSLMODE", value = "require" },
        { name = "WALLET_DB_MAX_OPEN_CONNS", value = "25" },
        { name = "WALLET_DB_MAX_IDLE_CONNS", value = "10" },
        { name = "WALLET_DB_CONN_MAX_LIFETIME", value = "30m" },
        { name = "WALLET_DB_CONN_MAX_IDLE_TIME", value = "5m" },
        { name = "WALLET_DB_CONNECT_TIMEOUT", value = "3s" },
        { name = "WALLET_DB_CONNECT_MAX_RETRIES", value = "8" },
        { name = "WALLET_DB_CONNECT_INITIAL_BACKOFF", value = "500ms" },
        { name = "WALLET_DB_CONNECT_MAX_BACKOFF", value = "8s" },
        { name = "GRACEFUL_SHUTDOWN_TIMEOUT", value = "10s" }
      ]
      secrets = [
        { name = "WALLET_DB_USER", valueFrom = data.terraform_remote_state.foundation.outputs.rds_master_username_secret_arn },
        { name = "WALLET_DB_PASSWORD", valueFrom = data.terraform_remote_state.foundation.outputs.rds_master_password_secret_arn }
      ]
    }
    "transaction-service" = {
      port   = 50054
      cpu    = 256
      memory = 512
      env = [
        { name = "TRANSACTION_GRPC_PORT", value = "50054" },
        { name = "TRANSACTION_DB_HOST", value = local.rds_host },
        { name = "TRANSACTION_DB_PORT", value = local.rds_port },
        { name = "TRANSACTION_DB_NAME", value = "transactions_db" },
        { name = "TRANSACTION_DB_SSLMODE", value = "require" },
        { name = "TRANSACTION_DB_MAX_OPEN_CONNS", value = "25" },
        { name = "TRANSACTION_DB_MAX_IDLE_CONNS", value = "10" },
        { name = "TRANSACTION_DB_CONN_MAX_LIFETIME", value = "30m" },
        { name = "TRANSACTION_DB_CONN_MAX_IDLE_TIME", value = "5m" },
        { name = "TRANSACTION_DB_CONNECT_TIMEOUT", value = "3s" },
        { name = "TRANSACTION_DB_CONNECT_MAX_RETRIES", value = "8" },
        { name = "TRANSACTION_DB_CONNECT_INITIAL_BACKOFF", value = "500ms" },
        { name = "TRANSACTION_DB_CONNECT_MAX_BACKOFF", value = "8s" },
        { name = "GRACEFUL_SHUTDOWN_TIMEOUT", value = "10s" }
      ]
      secrets = [
        { name = "TRANSACTION_DB_USER", valueFrom = data.terraform_remote_state.foundation.outputs.rds_master_username_secret_arn },
        { name = "TRANSACTION_DB_PASSWORD", valueFrom = data.terraform_remote_state.foundation.outputs.rds_master_password_secret_arn }
      ]
    }
  }

  gateway_environment = [
    { name = "PORT", value = "8080" },
    { name = "USER_SERVICE_GRPC_ADDR", value = "user-service.${local.namespace_name}:50051" },
    { name = "FRAUD_SERVICE_GRPC_ADDR", value = "fraud-service.${local.namespace_name}:50052" },
    { name = "WALLET_SERVICE_GRPC_ADDR", value = "wallet-service.${local.namespace_name}:50053" },
    { name = "TRANSACTION_SERVICE_GRPC_ADDR", value = "transaction-service.${local.namespace_name}:50054" },
    { name = "AUTH_JWT_ISSUER", value = "peer-ledger-gateway" },
    { name = "AUTH_JWT_TTL", value = "24h" },
    { name = "GATEWAY_GRPC_DIAL_TIMEOUT", value = "3s" },
    { name = "GATEWAY_GRPC_MAX_ATTEMPTS", value = "10" },
    { name = "GATEWAY_METRICS_ENABLED", value = "true" },
    { name = "GATEWAY_METRICS_PATH", value = "/metrics" },
    { name = "GATEWAY_RATE_LIMIT_ENABLED", value = "true" },
    { name = "GATEWAY_RATE_LIMIT_DEFAULT_REQUESTS", value = "120" },
    { name = "GATEWAY_RATE_LIMIT_DEFAULT_WINDOW", value = "1m" },
    { name = "GATEWAY_RATE_LIMIT_TRANSFERS_REQUESTS", value = "20" },
    { name = "GATEWAY_RATE_LIMIT_TRANSFERS_WINDOW", value = "1m" },
    { name = "GATEWAY_RATE_LIMIT_CLEANUP_INTERVAL", value = "2m" },
    { name = "GATEWAY_RATE_LIMIT_TRUST_PROXY", value = "true" },
    { name = "GATEWAY_RATE_LIMIT_EXEMPT_PATHS", value = "/health,/ping,/metrics" },
    { name = "GATEWAY_GRACEFUL_SHUTDOWN_TIMEOUT", value = "10s" }
  ]

  db_migrator = {
    cpu    = 256
    memory = 512
    env = [
      { name = "DB_MIGRATOR_HOST", value = local.rds_host },
      { name = "DB_MIGRATOR_PORT", value = local.rds_port },
      { name = "DB_MIGRATOR_SSLMODE", value = "require" },
      { name = "DB_MIGRATOR_CONNECT_TIMEOUT", value = "10s" },
      { name = "DB_MIGRATOR_STATEMENT_TIMEOUT", value = "2m" }
    ]
    secrets = [
      { name = "DB_MIGRATOR_USERNAME", valueFrom = data.terraform_remote_state.foundation.outputs.rds_master_username_secret_arn },
      { name = "DB_MIGRATOR_PASSWORD", valueFrom = data.terraform_remote_state.foundation.outputs.rds_master_password_secret_arn }
    ]
  }
}

resource "aws_ecs_task_definition" "gateway" {
  family                   = "${local.name_prefix}-gateway"
  cpu                      = tostring(local.gateway.cpu)
  memory                   = tostring(local.gateway.memory)
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  execution_role_arn       = data.terraform_remote_state.platform.outputs.ecs_execution_role_arn
  task_role_arn            = data.terraform_remote_state.platform.outputs.ecs_task_role_arn

  container_definitions = jsonencode([
    {
      name      = local.gateway.name
      image     = var.service_images["gateway"]
      essential = true
      portMappings = [
        {
          containerPort = local.gateway.port
          hostPort      = local.gateway.port
          protocol      = "tcp"
        }
      ]
      environment = local.gateway_environment
      secrets = [
        {
          name      = "AUTH_JWT_SECRET"
          valueFrom = data.terraform_remote_state.foundation.outputs.auth_jwt_secret_arn
        }
      ]
      healthCheck = {
        command     = ["CMD-SHELL", "wget -qO- http://127.0.0.1:8080/health >/dev/null || exit 1"]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 20
      }
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = data.terraform_remote_state.platform.outputs.cloudwatch_log_group_names["gateway"]
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "ecs"
        }
      }
    }
  ])
}

resource "terraform_data" "db_migrator_run" {
  triggers_replace = {
    cluster_name       = data.terraform_remote_state.platform.outputs.ecs_cluster_name
    task_definition    = aws_ecs_task_definition.db_migrator.arn
    subnet_ids         = join(",", data.terraform_remote_state.platform.outputs.private_subnet_ids)
    security_group_id  = data.terraform_remote_state.platform.outputs.operations_security_group_id
    db_host            = local.rds_host
    db_port            = local.rds_port
    db_migrator_image  = var.service_images["db-migrator"]
  }

  # provisioner "local-exec" {
  #   interpreter = ["bash", "-lc"]
  #   command     = <<-EOT
  #     set -euo pipefail

  #     cluster='${data.terraform_remote_state.platform.outputs.ecs_cluster_name}'
  #     task_definition='${aws_ecs_task_definition.db_migrator.arn}'
  #     network_configuration='${jsonencode({
  #       awsvpcConfiguration = {
  #         subnets         = data.terraform_remote_state.platform.outputs.private_subnet_ids
  #         securityGroups  = [data.terraform_remote_state.platform.outputs.operations_security_group_id]
  #         assignPublicIp  = "DISABLED"
  #       }
  #     })}'

  #     task_arn="$(aws ecs run-task \
  #       --cluster "$cluster" \
  #       --launch-type FARGATE \
  #       --task-definition "$task_definition" \
  #       --network-configuration "$network_configuration" \
  #       --query 'tasks[0].taskArn' \
  #       --output text)"

  #     if [ -z "$task_arn" ] || [ "$task_arn" = "None" ]; then
  #       echo "failed to start db-migrator task"
  #       exit 1
  #     fi

  #     aws ecs wait tasks-stopped --cluster "$cluster" --tasks "$task_arn"

  #     exit_code="$(aws ecs describe-tasks \
  #       --cluster "$cluster" \
  #       --tasks "$task_arn" \
  #       --query 'tasks[0].containers[?name==`db-migrator`].exitCode | [0]' \
  #       --output text)"

  #     if [ "$exit_code" != "0" ]; then
  #       aws ecs describe-tasks --cluster "$cluster" --tasks "$task_arn"
  #       echo "db-migrator task exited with code $exit_code"
  #       exit 1
  #     fi
  #   EOT
  # }
  provisioner "local-exec" {
  interpreter = ["powershell.exe", "-NoProfile", "-NonInteractive", "-Command"]
  command     = <<-EOT
    $ErrorActionPreference = "Stop"

    $cluster = "${data.terraform_remote_state.platform.outputs.ecs_cluster_name}"
    $taskDefinition = "${aws_ecs_task_definition.db_migrator.arn}"

    $networkConfiguration = 'awsvpcConfiguration={subnets=[${join(",", data.terraform_remote_state.platform.outputs.private_subnet_ids)}],securityGroups=[${data.terraform_remote_state.platform.outputs.operations_security_group_id}],assignPublicIp=DISABLED}'

    $taskArnQuery = 'tasks[0].taskArn'
    $exitCodeQuery = "tasks[0].containers[?name=='db-migrator'].exitCode | [0]"

    $taskArn = aws ecs run-task `
      --cluster $cluster `
      --launch-type FARGATE `
      --task-definition $taskDefinition `
      --network-configuration $networkConfiguration `
      --query $taskArnQuery `
      --output text

    if ($LASTEXITCODE -ne 0) {
      throw "aws ecs run-task failed"
    }

    if ([string]::IsNullOrWhiteSpace($taskArn) -or $taskArn -eq "None") {
      throw "failed to start db-migrator task"
    }

    aws ecs wait tasks-stopped --cluster $cluster --tasks $taskArn

    if ($LASTEXITCODE -ne 0) {
      throw "aws ecs wait tasks-stopped failed"
    }

    $exitCode = aws ecs describe-tasks `
      --cluster $cluster `
      --tasks $taskArn `
      --query $exitCodeQuery `
      --output text

    if ($LASTEXITCODE -ne 0) {
      throw "aws ecs describe-tasks failed"
    }

    if ([string]::IsNullOrWhiteSpace($exitCode) -or $exitCode -eq "None") {
      aws ecs describe-tasks --cluster $cluster --tasks $taskArn
      throw "db-migrator task exit code could not be determined"
    }

    if ($exitCode -ne "0") {
      aws ecs describe-tasks --cluster $cluster --tasks $taskArn
      throw "db-migrator task exited with code $exitCode"
    }
  EOT
}
}

resource "aws_ecs_service" "gateway" {
  name                               = "${local.name_prefix}-gateway"
  cluster                            = data.terraform_remote_state.platform.outputs.ecs_cluster_arn
  task_definition                    = aws_ecs_task_definition.gateway.arn
  desired_count                      = var.gateway_desired_count
  launch_type                        = "FARGATE"
  enable_execute_command             = true
  health_check_grace_period_seconds  = 60
  deployment_minimum_healthy_percent = 50
  deployment_maximum_percent         = 200

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  network_configuration {
    subnets          = data.terraform_remote_state.platform.outputs.private_subnet_ids
    security_groups  = [data.terraform_remote_state.platform.outputs.gateway_security_group_id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = data.terraform_remote_state.platform.outputs.gateway_target_group_arn
    container_name   = local.gateway.name
    container_port   = local.gateway.port
  }

  depends_on = [aws_ecs_task_definition.gateway, aws_ecs_service.internal]
}

resource "aws_appautoscaling_target" "gateway" {
  max_capacity       = var.gateway_max_capacity
  min_capacity       = var.gateway_min_capacity
  resource_id        = "service/${data.terraform_remote_state.platform.outputs.ecs_cluster_name}/${aws_ecs_service.gateway.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "gateway_cpu" {
  name               = "${local.name_prefix}-gateway-cpu"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.gateway.resource_id
  scalable_dimension = aws_appautoscaling_target.gateway.scalable_dimension
  service_namespace  = aws_appautoscaling_target.gateway.service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    target_value = 60
  }
}

resource "aws_appautoscaling_policy" "gateway_memory" {
  name               = "${local.name_prefix}-gateway-memory"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.gateway.resource_id
  scalable_dimension = aws_appautoscaling_target.gateway.scalable_dimension
  service_namespace  = aws_appautoscaling_target.gateway.service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageMemoryUtilization"
    }
    target_value = 70
  }
}

resource "aws_ecs_task_definition" "internal" {
  for_each = local.internal_services

  family                   = "${local.name_prefix}-${each.key}"
  cpu                      = tostring(each.value.cpu)
  memory                   = tostring(each.value.memory)
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  execution_role_arn       = data.terraform_remote_state.platform.outputs.ecs_execution_role_arn
  task_role_arn            = data.terraform_remote_state.platform.outputs.ecs_task_role_arn

  container_definitions = jsonencode([
    {
      name      = each.key
      image     = var.service_images[each.key]
      essential = true
      portMappings = [
        {
          containerPort = each.value.port
          hostPort      = each.value.port
          protocol      = "tcp"
        }
      ]
      environment = each.value.env
      secrets     = each.value.secrets
      healthCheck = {
        command     = ["CMD-SHELL", "/bin/grpc_health_probe -addr=127.0.0.1:${each.value.port}"]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 20
      }
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = data.terraform_remote_state.platform.outputs.cloudwatch_log_group_names[each.key]
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "ecs"
        }
      }
    }
  ])
}

resource "aws_ecs_service" "internal" {
  for_each = local.internal_services

  name                               = "${local.name_prefix}-${each.key}"
  cluster                            = data.terraform_remote_state.platform.outputs.ecs_cluster_arn
  task_definition                    = aws_ecs_task_definition.internal[each.key].arn
  desired_count                      = var.internal_services_desired_count
  launch_type                        = "FARGATE"
  enable_execute_command             = true
  deployment_minimum_healthy_percent = 50
  deployment_maximum_percent         = 200

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  network_configuration {
    subnets          = data.terraform_remote_state.platform.outputs.private_subnet_ids
    security_groups  = [data.terraform_remote_state.platform.outputs.internal_services_security_group_id]
    assign_public_ip = false
  }

  service_registries {
    registry_arn = data.terraform_remote_state.platform.outputs.service_discovery_service_arns[each.key]
  }

  depends_on = [terraform_data.db_migrator_run]
}

resource "aws_appautoscaling_target" "internal" {
  for_each = local.internal_services

  max_capacity       = var.internal_services_max_capacity
  min_capacity       = var.internal_services_min_capacity
  resource_id        = "service/${data.terraform_remote_state.platform.outputs.ecs_cluster_name}/${aws_ecs_service.internal[each.key].name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "internal_cpu" {
  for_each = local.internal_services

  name               = "${local.name_prefix}-${each.key}-cpu"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.internal[each.key].resource_id
  scalable_dimension = aws_appautoscaling_target.internal[each.key].scalable_dimension
  service_namespace  = aws_appautoscaling_target.internal[each.key].service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    target_value = 70
  }
}

resource "aws_appautoscaling_policy" "internal_memory" {
  for_each = local.internal_services

  name               = "${local.name_prefix}-${each.key}-memory"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.internal[each.key].resource_id
  scalable_dimension = aws_appautoscaling_target.internal[each.key].scalable_dimension
  service_namespace  = aws_appautoscaling_target.internal[each.key].service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageMemoryUtilization"
    }
    target_value = 70
  }
}

resource "aws_ecs_task_definition" "db_migrator" {
  family                   = "${local.name_prefix}-db-migrator"
  cpu                      = tostring(local.db_migrator.cpu)
  memory                   = tostring(local.db_migrator.memory)
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  execution_role_arn       = data.terraform_remote_state.platform.outputs.ecs_execution_role_arn
  task_role_arn            = data.terraform_remote_state.platform.outputs.ecs_task_role_arn

  container_definitions = jsonencode([
    {
      name      = "db-migrator"
      image     = var.service_images["db-migrator"]
      essential = true
      environment = local.db_migrator.env
      secrets     = local.db_migrator.secrets
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = data.terraform_remote_state.platform.outputs.cloudwatch_log_group_names["db-migrator"]
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "ecs"
        }
      }
    }
  ])
}
