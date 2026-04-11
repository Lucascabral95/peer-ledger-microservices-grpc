variable "aws_region" {
  description = "AWS region for foundation resources."
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Project identifier used in resource names."
  type        = string
  default     = "peer-ledger"
}

variable "environment" {
  description = "Environment name."
  type        = string
  default     = "production"
}

variable "terraform_state_bucket_name" {
  description = "S3 bucket name used by Terraform remote state."
  type        = string
}

variable "terraform_lock_table_name" {
  description = "DynamoDB table name used by Terraform remote state locking."
  type        = string
}

variable "github_repository" {
  description = "GitHub repository in org/repo format."
  type        = string
}

variable "github_main_ref" {
  description = "Git ref allowed to assume the deployment role."
  type        = string
  default     = "refs/heads/main"
}

variable "github_environment_name" {
  description = "GitHub Environment name used for protected deployments."
  type        = string
  default     = "production"
}

variable "auth_jwt_secret" {
  description = "JWT signing secret used by the gateway."
  type        = string
  sensitive   = true

  validation {
    condition     = length(var.auth_jwt_secret) >= 32
    error_message = "auth_jwt_secret must be at least 32 characters."
  }
}

variable "rds_master_username" {
  description = "Master username for the PostgreSQL RDS instance."
  type        = string
  sensitive   = true

  validation {
    condition     = can(regex("^[A-Za-z][A-Za-z0-9_]{0,15}$", var.rds_master_username))
    error_message = "rds_master_username must start with a letter and contain only letters, numbers, or underscore (max 16 chars)."
  }
}

variable "rds_master_password" {
  description = "Master password for the PostgreSQL RDS instance."
  type        = string
  sensitive   = true

  validation {
    condition = (
      length(var.rds_master_password) >= 8 &&
      length(var.rds_master_password) <= 128 &&
      can(regex("^[ -~]+$", var.rds_master_password)) &&
      length(regexall("[/@\" ]", var.rds_master_password)) == 0
    )
    error_message = "rds_master_password must be 8-128 printable ASCII chars and cannot contain '/', '@', '\"', or spaces."
  }
}

variable "ecr_image_retention_count" {
  description = "How many images to keep per ECR repository."
  type        = number
  default     = 30
}
