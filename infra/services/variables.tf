variable "aws_region" {
  description = "AWS region for the ECS services stack."
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
  description = "S3 bucket holding Terraform states."
  type        = string
}

variable "foundation_state_key" {
  description = "State key for the foundation stack."
  type        = string
  default     = "foundation/terraform.tfstate"
}

variable "platform_state_key" {
  description = "State key for the platform stack."
  type        = string
  default     = "platform/terraform.tfstate"
}

variable "state_region" {
  description = "AWS region hosting the remote state bucket."
  type        = string
  default     = "us-east-1"
}

variable "service_images" {
  description = "Container image URIs keyed by logical service name."
  type        = map(string)

  validation {
    condition = length(setsubtract(
      toset(["gateway", "user-service", "fraud-service", "wallet-service", "transaction-service", "db-migrator"]),
      toset(keys(var.service_images))
    )) == 0
    error_message = "service_images must contain gateway, user-service, fraud-service, wallet-service, transaction-service and db-migrator."
  }
}

variable "gateway_desired_count" {
  description = "Desired count for the public gateway service."
  type        = number
  default     = 2
}

variable "gateway_min_capacity" {
  description = "Minimum autoscaling capacity for the gateway."
  type        = number
  default     = 2
}

variable "gateway_max_capacity" {
  description = "Maximum autoscaling capacity for the gateway."
  type        = number
  default     = 4
}

variable "internal_services_desired_count" {
  description = "Desired count for internal gRPC services."
  type        = number
  default     = 1
}

variable "internal_services_min_capacity" {
  description = "Minimum autoscaling capacity for internal gRPC services."
  type        = number
  default     = 1
}

variable "internal_services_max_capacity" {
  description = "Maximum autoscaling capacity for internal gRPC services."
  type        = number
  default     = 2
}
