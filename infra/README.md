# Infraestructura AWS

Esta carpeta contiene la infraestructura del proyecto dividida por stacks:

- `bootstrap`: crea el bucket S3 de remote state y la tabla DynamoDB para locking.
- `foundation`: crea ECR, Secrets Manager y el rol OIDC para GitHub Actions.
- `platform`: crea VPC, ALB, ECS cluster, Cloud Map, CloudWatch Logs y la instancia RDS.
- `services`: crea task definitions, ECS services, autoscaling y la task definition de `db-migrator`.

## Orden de aplicacion

1. `bootstrap`
2. `foundation`
3. `platform`
4. `services`

## Remote state

`bootstrap` se ejecuta con backend local. Los otros tres stacks usan backend `s3` con un archivo `backend.hcl` basado en `infra/backend.hcl.example`.

Ejemplo:

```bash
cd infra/foundation
terraform init -backend-config=../backend.hcl
terraform apply
```

## Restore de RDS despues de destroy

La persistencia despues de `terraform destroy` no se resuelve exportando la base a un bucket S3 arbitrario. El flujo profesional implementado aca usa snapshots administrados por RDS:

- al destruir `platform`, la instancia crea un `final snapshot`
- al volver a aplicar `platform`, Terraform consulta el snapshot manual mas reciente del mismo identificador
- si encuentra uno, restaura desde ese snapshot
- si no encuentra ninguno, crea una instancia nueva

Esto preserva datos reales y replica el comportamiento que querias despues de recrear la infraestructura.

Despues de recrear `platform`, volve a aplicar `services` o ejecuta el workflow de deploy para refrescar el endpoint de RDS en las task definitions.

## Migraciones y bases logicas

La instancia RDS tiene una sola instancia PostgreSQL y tres bases logicas:

- `users_db`
- `wallets_db`
- `transactions_db`

La task `db-migrator`:

- crea esas bases si faltan
- aplica migraciones idempotentes embebidas en la imagen
- registra versiones en `schema_migrations`

Si la base fue restaurada desde snapshot, la task vuelve a correr sin romper datos existentes.

## Variables sensibles

Los secretos persistentes viven en `foundation` para que sobrevivan a un `destroy` de `platform`:

- `auth_jwt_secret`
- `rds_master_username`
- `rds_master_password`

## Destruccion esperada

- `terraform destroy` en `services`: elimina ECS services y task definitions.
- `terraform destroy` en `platform`: elimina ALB, red privada del stack, ECS cluster y RDS, pero deja el snapshot final de RDS.
- `terraform destroy` en `foundation`: falla si intenta borrar ECR o secretos protegidos por `prevent_destroy`.
