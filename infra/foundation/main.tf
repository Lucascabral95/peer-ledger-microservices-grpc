data "aws_caller_identity" "current" {}

locals {
  default_tags = {
    Project     = var.project_name
    Environment = var.environment
    ManagedBy   = "Terraform"
    Stack       = "foundation"
  }

  ecr_repositories = toset([
    "${var.project_name}-gateway",
    "${var.project_name}-user-service",
    "${var.project_name}-fraud-service",
    "${var.project_name}-wallet-service",
    "${var.project_name}-transaction-service",
    "${var.project_name}-db-migrator"
  ])

  secret_names = {
    auth_jwt_secret     = "${var.project_name}/${var.environment}/auth-jwt-secret"
    rds_master_username = "${var.project_name}/${var.environment}/rds-master-username"
    rds_master_password = "${var.project_name}/${var.environment}/rds-master-password"
  }
}

resource "aws_ecr_repository" "repositories" {
  for_each = local.ecr_repositories

  name                 = each.value
  image_tag_mutability = "IMMUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "AES256"
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_ecr_lifecycle_policy" "repositories" {
  for_each = aws_ecr_repository.repositories

  repository = each.value.name
  policy     = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep the most recent immutable images"
        selection    = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = var.ecr_image_retention_count
        }
        action       = {
          type = "expire"
        }
      }
    ]
  })
}

resource "aws_secretsmanager_secret" "auth_jwt_secret" {
  name = local.secret_names.auth_jwt_secret

  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_secretsmanager_secret_version" "auth_jwt_secret" {
  secret_id     = aws_secretsmanager_secret.auth_jwt_secret.id
  secret_string = var.auth_jwt_secret
}

resource "aws_secretsmanager_secret" "rds_master_username" {
  name = local.secret_names.rds_master_username

  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_secretsmanager_secret_version" "rds_master_username" {
  secret_id     = aws_secretsmanager_secret.rds_master_username.id
  secret_string = var.rds_master_username
}

resource "aws_secretsmanager_secret" "rds_master_password" {
  name = local.secret_names.rds_master_password

  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_secretsmanager_secret_version" "rds_master_password" {
  secret_id     = aws_secretsmanager_secret.rds_master_password.id
  secret_string = var.rds_master_password
}
