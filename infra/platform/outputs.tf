output "vpc_id" {
  description = "VPC ID for the platform network."
  value       = aws_vpc.main.id
}

output "public_subnet_ids" {
  description = "Public subnet IDs."
  value       = [for subnet in aws_subnet.public : subnet.id]
}

output "private_subnet_ids" {
  description = "Private subnet IDs used by ECS and RDS."
  value       = [for subnet in aws_subnet.private : subnet.id]
}

output "alb_dns_name" {
  description = "Public DNS name of the API load balancer."
  value       = aws_lb.gateway.dns_name
}

output "gateway_target_group_arn" {
  description = "Target group ARN used by the gateway ECS service."
  value       = aws_lb_target_group.gateway.arn
}

output "gateway_security_group_id" {
  description = "Security group ID attached to the gateway ECS service."
  value       = aws_security_group.gateway.id
}

output "internal_services_security_group_id" {
  description = "Security group ID attached to the private gRPC services."
  value       = aws_security_group.internal_services.id
}

output "operations_security_group_id" {
  description = "Security group ID for one-off ECS tasks such as db-migrator."
  value       = aws_security_group.operations.id
}

output "ecs_cluster_arn" {
  description = "ECS cluster ARN."
  value       = aws_ecs_cluster.main.arn
}

output "ecs_cluster_name" {
  description = "ECS cluster name."
  value       = aws_ecs_cluster.main.name
}

output "ecs_execution_role_arn" {
  description = "Execution role ARN for ECS task definitions."
  value       = aws_iam_role.ecs_execution.arn
}

output "ecs_task_role_arn" {
  description = "Task role ARN for ECS task definitions."
  value       = aws_iam_role.ecs_task.arn
}

output "service_discovery_namespace_name" {
  description = "Private DNS namespace used for service discovery."
  value       = aws_service_discovery_private_dns_namespace.main.name
}

output "service_discovery_service_arns" {
  description = "Map of internal service names to Cloud Map service ARNs."
  value = {
    for name, service in aws_service_discovery_service.internal :
    name => service.arn
  }
}

output "cloudwatch_log_group_names" {
  description = "Map of ECS log group names by service."
  value = {
    for name, group in aws_cloudwatch_log_group.ecs :
    name => group.name
  }
}

output "rds_endpoint" {
  description = "Hostname of the PostgreSQL RDS instance."
  value       = aws_db_instance.main.address
}

output "rds_port" {
  description = "Port of the PostgreSQL RDS instance."
  value       = aws_db_instance.main.port
}

output "rds_identifier" {
  description = "Identifier of the PostgreSQL RDS instance."
  value       = aws_db_instance.main.identifier
}

output "rds_restored_from_snapshot" {
  description = "Whether the current RDS instance was restored from a retained snapshot."
  value       = local.restore_snapshot_id != ""
}
