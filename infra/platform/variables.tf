variable "aws_region" {
  description = "AWS region for platform resources."
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
  description = "S3 bucket that stores Terraform state files."
  type        = string
}

variable "foundation_state_key" {
  description = "State object key for the foundation stack."
  type        = string
  default     = "foundation/terraform.tfstate"
}

variable "foundation_state_region" {
  description = "AWS region hosting the foundation stack state."
  type        = string
  default     = "us-east-1"
}

variable "vpc_cidr" {
  description = "CIDR block for the platform VPC."
  type        = string
  default     = "10.30.0.0/16"
}

variable "public_subnet_cidrs" {
  description = "CIDR blocks for public subnets."
  type        = list(string)
  default     = ["10.30.0.0/24", "10.30.1.0/24"]
}

variable "private_subnet_cidrs" {
  description = "CIDR blocks for private subnets."
  type        = list(string)
  default     = ["10.30.10.0/24", "10.30.11.0/24"]
}

variable "alb_ingress_cidrs" {
  description = "CIDR ranges allowed to reach the public ALB."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "container_log_retention_days" {
  description = "Retention in days for ECS CloudWatch log groups."
  type        = number
  default     = 30
}

variable "rds_instance_class" {
  description = "Instance class for the PostgreSQL RDS instance."
  type        = string
  default     = "db.t3.micro"
}

variable "rds_allocated_storage" {
  description = "Initial allocated storage for the RDS instance."
  type        = number
  default     = 20
}

variable "rds_max_allocated_storage" {
  description = "Maximum autoscaled storage for the RDS instance."
  type        = number
  default     = 100
}

variable "rds_engine_version" {
  description = "PostgreSQL engine version used for fresh instances."
  type        = string
  default     = "16.4"
}

variable "rds_backup_retention_days" {
  description = "Backup retention for the RDS instance."
  type        = number
  default     = 1
}

variable "rds_restore_from_latest_snapshot" {
  description = "If true, restores the next RDS instance from the latest manual snapshot when available."
  type        = bool
  default     = true
}
