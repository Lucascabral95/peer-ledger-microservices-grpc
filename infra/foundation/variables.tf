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
}

variable "rds_master_username" {
  description = "Master username for the PostgreSQL RDS instance."
  type        = string
  sensitive   = true
}

variable "rds_master_password" {
  description = "Master password for the PostgreSQL RDS instance."
  type        = string
  sensitive   = true
}

variable "ecr_image_retention_count" {
  description = "How many images to keep per ECR repository."
  type        = number
  default     = 30
}
