output "ecr_repository_urls" {
  description = "Map of ECR repository names to repository URLs."
  value = {
    for name, repo in aws_ecr_repository.repositories :
    name => repo.repository_url
  }
}

output "github_actions_role_arn" {
  description = "IAM role ARN assumed by GitHub Actions through OIDC."
  value       = aws_iam_role.github_actions.arn
}

output "auth_jwt_secret_arn" {
  description = "Secrets Manager ARN containing the gateway JWT secret."
  value       = aws_secretsmanager_secret.auth_jwt_secret.arn
}

output "rds_master_username_secret_arn" {
  description = "Secrets Manager ARN containing the RDS master username."
  value       = aws_secretsmanager_secret.rds_master_username.arn
}

output "rds_master_password_secret_arn" {
  description = "Secrets Manager ARN containing the RDS master password."
  value       = aws_secretsmanager_secret.rds_master_password.arn
}
