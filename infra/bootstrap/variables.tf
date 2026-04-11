variable "aws_region" {
  description = "AWS region used for Terraform bootstrap resources."
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Project identifier used in resource naming."
  type        = string
  default     = "peer-ledger"
}

variable "environment" {
  description = "Environment name."
  type        = string
  default     = "production"
}

variable "state_bucket_name" {
  description = "Globally unique S3 bucket name for Terraform state."
  type        = string
}

variable "lock_table_name" {
  description = "DynamoDB table name for Terraform state locking."
  type        = string
  default     = "peer-ledger-terraform-locks"
}
