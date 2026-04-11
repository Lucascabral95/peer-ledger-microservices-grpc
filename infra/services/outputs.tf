output "gateway_service_name" {
  description = "Gateway ECS service name."
  value       = aws_ecs_service.gateway.name
}

output "ecs_cluster_name" {
  description = "ECS cluster name backing the deployed services."
  value       = data.terraform_remote_state.platform.outputs.ecs_cluster_name
}

output "private_subnet_ids" {
  description = "Private subnets used by ECS services and one-off tasks."
  value       = data.terraform_remote_state.platform.outputs.private_subnet_ids
}

output "operations_security_group_id" {
  description = "Security group used by the db-migrator task."
  value       = data.terraform_remote_state.platform.outputs.operations_security_group_id
}

output "alb_dns_name" {
  description = "Public ALB DNS name for smoke tests."
  value       = data.terraform_remote_state.platform.outputs.alb_dns_name
}

output "internal_service_names" {
  description = "Map of internal service logical names to ECS service names."
  value = {
    for name, service in aws_ecs_service.internal :
    name => service.name
  }
}

output "db_migrator_task_definition_arn" {
  description = "Task definition ARN used to run the database migrator one-off task."
  value       = aws_ecs_task_definition.db_migrator.arn
}
