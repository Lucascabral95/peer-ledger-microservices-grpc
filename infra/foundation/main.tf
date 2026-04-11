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
    auth_jwt_secret    = "${var.project_name}/${var.environment}/auth-jwt-secret"
    rds_master_username = "${var.project_name}/${var.environment}/rds-master-username"
    rds_master_password = "${var.project_name}/${var.environment}/rds-master-password"
  }

  github_subjects = [
    "repo:${var.github_repository}:ref:${var.github_main_ref}",
    "repo:${var.github_repository}:environment:${var.github_environment_name}"
  ]
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
  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep the most recent immutable images"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = var.ecr_image_retention_count
        }
        action = {
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

resource "aws_iam_openid_connect_provider" "github" {
  url = "https://token.actions.githubusercontent.com"

  client_id_list = [
    "sts.amazonaws.com"
  ]

  thumbprint_list = [
    "6938fd4d98bab03faadb97b34396831e3780aea1"
  ]
}

data "aws_iam_policy_document" "github_actions_assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.github.arn]
    }

    actions = ["sts:AssumeRoleWithWebIdentity"]

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      values   = local.github_subjects
    }
  }
}

resource "aws_iam_role" "github_actions" {
  name               = "${var.project_name}-${var.environment}-github-actions"
  assume_role_policy = data.aws_iam_policy_document.github_actions_assume_role.json
}

data "aws_iam_policy_document" "github_actions_permissions" {
  statement {
    sid    = "TerraformStateAccess"
    effect = "Allow"
    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:DeleteObject",
      "s3:ListBucket",
      "dynamodb:DescribeTable",
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:DeleteItem",
      "dynamodb:UpdateItem"
    ]
    resources = [
      "arn:aws:s3:::${var.terraform_state_bucket_name}",
      "arn:aws:s3:::${var.terraform_state_bucket_name}/*",
      "arn:aws:dynamodb:${var.aws_region}:${data.aws_caller_identity.current.account_id}:table/${var.terraform_lock_table_name}"
    ]
  }

  statement {
    sid    = "ECRPushPull"
    effect = "Allow"
    actions = [
      "ecr:GetAuthorizationToken",
      "ecr:BatchCheckLayerAvailability",
      "ecr:CompleteLayerUpload",
      "ecr:InitiateLayerUpload",
      "ecr:PutImage",
      "ecr:UploadLayerPart",
      "ecr:BatchGetImage",
      "ecr:DescribeRepositories",
      "ecr:DescribeImages",
      "ecr:ListImages"
    ]
    resources = ["*"]
  }

  statement {
    sid    = "PlatformManagement"
    effect = "Allow"
    actions = [
      "ec2:*",
      "ecs:*",
      "elasticloadbalancing:*",
      "logs:*",
      "rds:*",
      "servicediscovery:*",
      "secretsmanager:*"
    ]
    resources = ["*"]
  }

  statement {
    sid    = "IAMRoleManagementForPlatform"
    effect = "Allow"
    actions = [
      "iam:AttachRolePolicy",
      "iam:CreateRole",
      "iam:CreateServiceLinkedRole",
      "iam:DeleteRole",
      "iam:DeleteRolePolicy",
      "iam:DetachRolePolicy",
      "iam:GetRole",
      "iam:ListAttachedRolePolicies",
      "iam:PassRole",
      "iam:PutRolePolicy",
      "iam:TagRole",
      "iam:UntagRole",
      "iam:UpdateAssumeRolePolicy"
    ]
    resources = ["*"]
  }

  statement {
    sid    = "ReadOnlySupportingServices"
    effect = "Allow"
    actions = [
      "acm:DescribeCertificate",
      "application-autoscaling:*",
      "cloudwatch:*",
      "iam:ListRoles",
      "kms:DescribeKey",
      "kms:ListAliases",
      "sts:GetCallerIdentity"
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "github_actions" {
  name   = "${var.project_name}-${var.environment}-github-actions-inline"
  role   = aws_iam_role.github_actions.id
  policy = data.aws_iam_policy_document.github_actions_permissions.json
}
