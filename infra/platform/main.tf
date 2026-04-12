data "aws_availability_zones" "available" {
  state = "available"
}

data "terraform_remote_state" "foundation" {
  backend = "s3"

  config = {
    bucket = var.terraform_state_bucket_name
    key    = var.foundation_state_key
    region = var.foundation_state_region
  }
}

data "aws_secretsmanager_secret_version" "rds_master_username" {
  secret_id = data.terraform_remote_state.foundation.outputs.rds_master_username_secret_arn
}

data "aws_secretsmanager_secret_version" "rds_master_password" {
  secret_id = data.terraform_remote_state.foundation.outputs.rds_master_password_secret_arn
}

data "external" "latest_rds_snapshot" {
  count   = var.rds_restore_from_latest_snapshot ? 1 : 0
  program = [var.powershell_executable, "-File", "${path.module}/../scripts/find-latest-rds-snapshot.ps1"]

  query = {
    db_instance_identifier = local.rds_identifier
    snapshot_type          = "manual"
    snapshot_prefix        = "${local.rds_identifier}-final"
  }
}

resource "random_id" "final_snapshot_suffix" {
  byte_length = 4
}

locals {
  default_tags = {
    Project     = var.project_name
    Environment = var.environment
    ManagedBy   = "Terraform"
    Stack       = "platform"
  }

  name_prefix           = "${var.project_name}-${var.environment}"
  rds_identifier        = "${local.name_prefix}-postgres"
  namespace_name        = "${var.project_name}.internal"
  restore_snapshot_id   = var.rds_restore_from_latest_snapshot ? trimspace(try(data.external.latest_rds_snapshot[0].result.snapshot_identifier, "")) : ""
  service_log_targets   = toset(["gateway", "user-service", "fraud-service", "wallet-service", "transaction-service", "db-migrator"])
  private_service_names = toset(["user-service", "fraud-service", "wallet-service", "transaction-service"])
}

resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id
}

resource "aws_subnet" "public" {
  for_each = tomap({
    for index, cidr in var.public_subnet_cidrs :
    index => cidr
  })

  vpc_id                  = aws_vpc.main.id
  cidr_block              = each.value
  availability_zone       = data.aws_availability_zones.available.names[tonumber(each.key)]
  map_public_ip_on_launch = true
}

resource "aws_subnet" "private" {
  for_each = tomap({
    for index, cidr in var.private_subnet_cidrs :
    index => cidr
  })

  vpc_id            = aws_vpc.main.id
  cidr_block        = each.value
  availability_zone = data.aws_availability_zones.available.names[tonumber(each.key)]
}

resource "aws_eip" "nat" {
  domain = "vpc"
}

resource "aws_nat_gateway" "main" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public["0"].id

  depends_on = [aws_internet_gateway.main]
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }
}

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.main.id
  }
}

resource "aws_route_table_association" "public" {
  for_each = aws_subnet.public

  subnet_id      = each.value.id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table_association" "private" {
  for_each = aws_subnet.private

  subnet_id      = each.value.id
  route_table_id = aws_route_table.private.id
}

resource "aws_security_group" "alb" {
  name        = "${local.name_prefix}-alb"
  description = "Public ingress to the peer-ledger ALB"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = var.alb_ingress_cidrs
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "gateway" {
  name        = "${local.name_prefix}-gateway"
  description = "Gateway tasks behind the ALB"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "internal_services" {
  name        = "${local.name_prefix}-internal-services"
  description = "Private gRPC services"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port       = 50051
    to_port         = 50054
    protocol        = "tcp"
    security_groups = [aws_security_group.gateway.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "operations" {
  name        = "${local.name_prefix}-operations"
  description = "One-off operational tasks such as DB migrations"
  vpc_id      = aws_vpc.main.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "rds" {
  name        = "${local.name_prefix}-rds"
  description = "Database access for application tasks"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.internal_services.id, aws_security_group.operations.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_lb" "gateway" {
  name               = substr("${local.name_prefix}-alb", 0, 32)
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = [for subnet in aws_subnet.public : subnet.id]
}

resource "aws_lb_target_group" "gateway" {
  name        = substr("${local.name_prefix}-gateway", 0, 32)
  port        = 8080
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = aws_vpc.main.id

  health_check {
    enabled             = true
    healthy_threshold   = 2
    interval            = 30
    matcher             = "200"
    path                = "/health"
    protocol            = "HTTP"
    timeout             = 5
    unhealthy_threshold = 3
  }
}

resource "aws_lb_listener" "gateway_http" {
  load_balancer_arn = aws_lb.gateway.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.gateway.arn
  }
}

resource "aws_cloudwatch_log_group" "ecs" {
  for_each = local.service_log_targets

  name              = "/aws/ecs/${local.name_prefix}/${each.value}"
  retention_in_days = var.container_log_retention_days
}

resource "aws_ecs_cluster" "main" {
  name = "${local.name_prefix}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

resource "aws_service_discovery_private_dns_namespace" "main" {
  name = local.namespace_name
  vpc  = aws_vpc.main.id
}

resource "aws_service_discovery_service" "internal" {
  for_each = local.private_service_names

  name = each.value

  dns_config {
    namespace_id = aws_service_discovery_private_dns_namespace.main.id

    dns_records {
      ttl  = 10
      type = "A"
    }

    routing_policy = "MULTIVALUE"
  }
}

data "aws_iam_policy_document" "ecs_task_assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }

    actions = ["sts:AssumeRole"]
  }
}

resource "aws_iam_role" "ecs_execution" {
  name               = "${local.name_prefix}-ecs-execution"
  assume_role_policy = data.aws_iam_policy_document.ecs_task_assume_role.json
}

resource "aws_iam_role_policy_attachment" "ecs_execution_managed" {
  role       = aws_iam_role.ecs_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

data "aws_iam_policy_document" "ecs_execution_secrets" {
  statement {
    effect = "Allow"
    actions = [
      "secretsmanager:GetSecretValue",
      "kms:Decrypt"
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "ecs_execution_secrets" {
  name   = "${local.name_prefix}-ecs-execution-secrets"
  role   = aws_iam_role.ecs_execution.id
  policy = data.aws_iam_policy_document.ecs_execution_secrets.json
}

resource "aws_iam_role" "ecs_task" {
  name               = "${local.name_prefix}-ecs-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_task_assume_role.json
}

resource "aws_db_subnet_group" "main" {
  name       = "${local.name_prefix}-rds"
  subnet_ids = [for subnet in aws_subnet.private : subnet.id]
}

resource "aws_db_instance" "main" {
  identifier               = local.rds_identifier
  instance_class           = var.rds_instance_class
  db_subnet_group_name     = aws_db_subnet_group.main.name
  vpc_security_group_ids   = [aws_security_group.rds.id]
  backup_retention_period  = var.rds_backup_retention_days
  copy_tags_to_snapshot    = true
  delete_automated_backups = false
  deletion_protection      = false
  final_snapshot_identifier = "${local.rds_identifier}-final-${random_id.final_snapshot_suffix.hex}"
  publicly_accessible      = false
  skip_final_snapshot      = false
  storage_encrypted        = true
  storage_type             = "gp3"
  apply_immediately        = true

  allocated_storage     = local.restore_snapshot_id == "" ? var.rds_allocated_storage : null
  max_allocated_storage = local.restore_snapshot_id == "" ? var.rds_max_allocated_storage : null
  engine                = local.restore_snapshot_id == "" ? "postgres" : null
  engine_version        = local.restore_snapshot_id == "" ? var.rds_engine_version : null
  password              = local.restore_snapshot_id == "" ? data.aws_secretsmanager_secret_version.rds_master_password.secret_string : null
  snapshot_identifier   = local.restore_snapshot_id != "" ? local.restore_snapshot_id : null
  username              = local.restore_snapshot_id == "" ? data.aws_secretsmanager_secret_version.rds_master_username.secret_string : null
}
