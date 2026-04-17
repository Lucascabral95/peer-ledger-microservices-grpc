<p align="center">
  <img src="https://go.dev/blog/go-brand/Go-Logo/SVG/Go-Logo_Blue.svg"
       alt="Peer Ledger"
       width="220"/>
</p>

<h1 align="center">Peer Ledger: Wallet de Transferencias Internas</h1>

<p align="center">
  Plataforma de microservicios para transferencias P2P internas con autenticacion JWT, gateway HTTP, servicios gRPC desacoplados, antifraude en memoria, operaciones ACID en PostgreSQL y observabilidad con Prometheus, Alertmanager, Loki y Grafana.
</p>

---

## Table of contents

- [Descripcion general](#descripcion-general)
- [Caracteristicas principales](#caracteristicas-principales)
- [Estado actual del sistema](#estado-actual-del-sistema)
- [Estructura del proyecto](#estructura-del-proyecto)
- [Catalogo de microservicios](#catalogo-de-microservicios)
- [API publica del gateway](#api-publica-del-gateway)
- [Swagger / API Docs](#swagger--api-docs)
- [Observabilidad](#observabilidad)
- [Testing y calidad](#testing-y-calidad)
- [Guia de instalacion y ejecucion local](#guia-de-instalacion-y-ejecucion-local)
- [Infraestructura y despliegue AWS](#infraestructura-y-despliegue-aws)
- [Variables de entorno](#variables-de-entorno)
- [Datos de prueba](#datos-de-prueba)
- [Documentacion tecnica](#documentacion-tecnica)
- [Roadmap](#roadmap)

## Descripcion general

**Peer Ledger** es una plataforma backend para mover dinero entre usuarios registrados dentro de un sistema cerrado. No integra bancos ni tarjetas externas: el foco esta en la logica de wallet interna, autenticacion, antifraude, consistencia monetaria y auditoria.

El sistema esta construido como un monorepo Go con una arquitectura de microservicios donde el cliente solo conversa con `api-gateway`. El gateway valida identidad, aplica middlewares, orquesta el flujo de negocio y delega cada responsabilidad a un servicio gRPC especializado.

## Caracteristicas principales

- Gateway HTTP como unico punto de entrada.
- Comunicacion interna por gRPC entre servicios.
- Autenticacion JWT con `register`, `login` y middleware Bearer.
- Provision automatica de wallet al registrarse.
- `topup` autenticado para fondeo manual.
- `wallet-service` con transferencias ACID, locking pesimista e idempotencia persistente.
- `fraud-service` con reglas thread-safe en memoria.
- `transaction-service` para auditoria e historial.
- Rate limiting por IP en el gateway.
- Observabilidad base con Prometheus, Alertmanager, Loki y Grafana.
- Tests unitarios desacoplados de DB real mediante mocks e inyeccion de dependencias.
- Docker Compose para entorno local completo.
- Despliegue cloud con Terraform sobre AWS para `foundation`, `platform` y `services`.
- Flujo operativo bash-first para Terraform local y GitHub Actions.

## Estado actual del sistema

Servicios implementados:

- `api-gateway`
- `user-service`
- `fraud-service`
- `wallet-service`
- `transaction-service`
- `postgres`
- `prometheus`
- `grafana`

Stacks de infraestructura implementados:

- `infra/bootstrap`: bucket S3 y tabla DynamoDB para remote state
- `infra/foundation`: ECR, Secrets Manager y rol OIDC para GitHub Actions
- `infra/platform`: VPC, subnets, NAT Gateway, ALB, ECS cluster, Cloud Map, security groups y RDS PostgreSQL
- `infra/services`: task definitions, ECS services, autoscaling y `db-migrator`

Flujos implementados de punta a punta:

- registro y login por email/password
- emision y validacion de JWT
- creacion de wallet inicial al registrarse
- recarga de saldo autenticada
- transferencia autenticada entre usuarios
- registro de auditoria de movimientos
- consulta de historial por usuario autenticado
- exportacion de metricas HTTP del gateway

## Estructura del proyecto

```text
peer-ledger-microservices-grpc/
|-- .github/
|   `-- workflows/
|       `-- ci-cd.yml
|-- db/
|   `-- migrations/
|       |-- 01_users.sql
|       |-- 02_wallets.sql
|       `-- 03_transactions.sql
|-- gen/
|   |-- fraud/
|   |-- transaction/
|   |-- user/
|   `-- wallet/
|-- internal/
|   `-- security/
|-- project/
|   |-- docker-compose.yml
|   |-- Makefile
|   `-- scripts/
|       |-- resolve-service-images.sh
|       |-- tf-platform.sh
|       |-- tf-services.sh
|       `-- write-service-images-tfvars.sh
|   `-- monitoring/
|       |-- example/
|       |   |-- alertmanager/
|       |   |-- grafana/
|       |   |-- loki/
|       |   |-- prometheus/
|       |   `-- promtail/
|       |-- alertmanager/
|       |-- grafana/
|       |-- loki/
|       |-- prometheus/
|       `-- promtail/
|-- protobuf/
|   |-- fraud.proto
|   |-- transaction.proto
|   |-- user.proto
|   `-- wallet.proto
|-- infra/
|   |-- bootstrap/
|   |-- foundation/
|   |-- platform/
|   |-- services/
|   `-- scripts/
|       |-- find-latest-rds-snapshot.sh
|       `-- run-db-migrator.sh
|-- services/
|   |-- gateway/
|   |   |-- cmd/api/
|   |   `-- internal/
|   |       |-- config/
|   |       `-- middleware/
|   |-- user-service/
|   |-- fraud-service/
|   |-- wallet-service/
|   `-- transaction-service/
|-- architecture.md
`-- README.md
```

## Catalogo de microservicios

### API Gateway

- Ruta: `services/gateway`
- Puerto: `8080`
- Rol:
  - entrypoint HTTP
  - auth JWT
  - rate limiting
  - metricas Prometheus
  - orquestacion de flujos
  - traduccion de errores gRPC a HTTP

### User Service

- Ruta: `services/user-service`
- Puerto gRPC: `50051`
- Storage: PostgreSQL `users_db`
- Rol:
  - register
  - login
  - `GetUser`
  - `UserExists`

### Fraud Service

- Ruta: `services/fraud-service`
- Puerto gRPC: `50052`
- Storage: RAM
- Rol:
  - aplicar reglas antifraude antes de tocar dinero

### Wallet Service

- Ruta: `services/wallet-service`
- Puerto gRPC: `50053`
- Storage: PostgreSQL `wallets_db`
- Rol:
  - crear wallets
  - recargar saldo
  - transferir dinero de forma ACID

### Transaction Service

- Ruta: `services/transaction-service`
- Puerto gRPC: `50054`
- Storage: PostgreSQL `transactions_db`
- Rol:
  - registrar auditoria
  - exponer historial

## API publica del gateway

Base URL local:

```text
http://localhost:8080
```

Rutas actuales:

- `GET /health`
- `GET /ping`
- `GET /metrics`
- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`
- `GET /users/{userID}`
- `GET /users/{userID}/exists`
- `GET /history/{userID}` autenticada
- `POST /topups` autenticada
- `POST /transfers` autenticada

Ejemplos:

```bash
curl -X POST "http://localhost:8080/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Lucas",
    "email": "lucas+new@mail.com",
    "password": "Password123!"
  }'
```

```bash
curl -X POST "http://localhost:8080/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "lucas@mail.com",
    "password": "Password123!"
  }'
```

```bash
curl -X POST "http://localhost:8080/topups" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "amount": 5000
  }'
```

```bash
curl -X POST "http://localhost:8080/transfers" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "receiver_id": "user-002",
    "amount": 1000.01,
    "idempotency_key": "k1"
  }'
```

## Swagger / API Docs

Swagger/OpenAPI documenta el contrato HTTP publico del `gateway`. Los servicios internos gRPC no se fuerzan a Swagger: su contrato fuente de verdad sigue siendo `protobuf/*.proto`, los stubs generados y sus README por servicio.

Generacion desde `project/`:

```bash
make swagger
```

Artefactos versionados:

- `services/gateway/docs/docs.go`
- `services/gateway/docs/swagger.json`
- `services/gateway/docs/swagger.yaml`

Ruta publica de Swagger UI en el gateway:

- `GET /swagger/index.html`

En Swagger:

- las rutas publicas aparecen sin autenticacion
- `GET /history/{userID}`, `POST /topups` y `POST /transfers` usan `Authorization: Bearer <token>`

## Contratos gRPC y Protobuf

Los archivos bajo `protobuf/` son la fuente de verdad de los contratos internos gRPC. Los archivos bajo `gen/` son artefactos generados y no deben editarse manualmente.

Regla de mantenimiento:

- si cambia un RPC o un mensaje, editar primero `protobuf/*.proto`
- luego regenerar stubs desde `project/` con `make proto`
- no crear archivos manuales como `*_extra.pb.go`, `*_auth.pb.go` o `*_delete.pb.go`

Herramientas necesarias para regenerar:

```bash
protoc --version
protoc-gen-go --version
protoc-gen-go-grpc --version
```

En Windows, si falta `protoc`, instalarlo con:

```powershell
winget install protobuf
```

Plugins de Go:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

## Observabilidad

El proyecto ya incorpora observabilidad base del gateway con metricas, alertas y logs.

El `api-gateway` emite logs estructurados JSON (nivel aplicacion) con campos operativos consistentes, incluyendo `request_id`, `trace_id`, `route`, `status`, `latency_ms` y `user_id` cuando aplica.

La configuracion base versionada de observabilidad (Grafana, Prometheus, Alertmanager, Loki y Promtail) se mantiene en:

- `project/monitoring/example`

Ese directorio funciona como plantilla de referencia para replicar el stack sin exponer secretos.

Metricas expuestas hoy:

- `gateway_http_requests_total`
- `gateway_http_request_duration_seconds`

Stack local:

- Gateway metrics: `http://localhost:8080/metrics`
- Prometheus: `http://localhost:9090`
- Alertmanager: `http://localhost:9093`
- Loki: `http://localhost:3100`
- Grafana: `http://localhost:3000`

Dashboard provisionado:

- [gateway-overview.json](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/project/monitoring/grafana/dashboards/gateway-overview.json)
- [logs-operations.json](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/project/monitoring/grafana/dashboards/logs-operations.json)

Catalogo de consultas LogQL para Loki:

- [queries.md](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/project/monitoring/loki/queries.md)

Paneles incluidos en `gateway-overview.json`:

- request rate por ruta y metodo
- latencia `p95` y `p99`
- total request rate
- `4xx rate`
- `5xx rate`
- `5xx error ratio`
- average latency
- request rate por status code
- requests by route

Paneles incluidos en `logs-operations.json`:

- **Errores por servicio (5m)**: cuenta errores recientes por servicio para identificar rapidamente donde se concentra la falla.
- **Panics/Fatals (15m)**: muestra eventos criticos de proceso en ventana corta para deteccion de incidentes severos.
- **gRPC errors (5m)**: cuenta fallos de comunicacion interna entre servicios.
- **Eventos de rate limit (10m)**: mide cuantas respuestas de limitacion esta devolviendo el gateway.
- **Stream de errores del gateway**: vista en tiempo real de logs de error del gateway para debugging inmediato.
- **Stream de eventos de negocio**: stream filtrado del flujo operativo (`transfer`, `topup`, `wallet provisioning`, `insufficient funds`, `fraud blocked`).

Alertas activas en Prometheus + Alertmanager: `5`

- `GatewayDown`: se activa cuando `up{job="gateway"} == 0` durante `1m`.
- `High5xxRate`: se activa cuando el rate total de respuestas `5xx` supera `1 req/s` durante `5m`.
- `High5xxErrorRatio`: se activa cuando el ratio de `5xx` supera `2%` durante `5m`.
- `HighP99Latency`: se activa cuando la latencia `p99` del gateway supera `1.2s` durante `5m`.
- `High429Rate`: se activa cuando el rate de respuestas `429` supera `0.5 req/s` durante `10m`.

Destino de notificacion configurado:

- `lucasgamerpolar10@gmail.com`

## Testing y calidad

La suite esta orientada a pruebas unitarias con mocks e inyeccion de dependencias. El objetivo es validar reglas de negocio, servidores gRPC, configuracion y middleware sin depender de una DB real en la mayoria de los casos.

Cobertura actual fuerte:

- `user-service`: `config`, `db`, `repository`, `server`
- `fraud-service`: `config`, `repository`, `server`
- `wallet-service`: `config`, `db`, `repository`, `server`
- `transaction-service`: `config`, `db`, `repository`, `server`
- `gateway`: `handlers`, `routes`, `main`, `internal/config`, `internal/middleware`

Targets disponibles:

```bash
make test-user
make test-fraud
make test-wallet
make test-transaction
make test-gateway
make test-all
```

CI actual:

- `go test ./...`
- validacion de `docker-compose`
- build de imagenes Docker
- despliegue de `platform` y `services` con Terraform en GitHub Actions

## Guia de instalacion y ejecucion local

### Prerrequisitos

- Docker
- Docker Compose
- Go 1.25+

### 1. Configurar entorno

```bash
cp .env.template .env
```

Definir como minimo:

- `AUTH_JWT_SECRET`
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`

### 2. Levantar stack

```bash
docker compose -f project/docker-compose.yml up -d --build
```

### 3. Ver logs

```bash
docker compose -f project/docker-compose.yml logs -f gateway user-service fraud-service wallet-service transaction-service postgres prometheus grafana
```

### 4. Resetear Postgres si queres reaplicar migraciones

```bash
docker compose -f project/docker-compose.yml down -v
docker compose -f project/docker-compose.yml up -d --build
```

## Infraestructura y despliegue AWS

### Resumen de stacks

- `bootstrap`: crea el bucket S3 y la tabla DynamoDB del remote state
- `foundation`: crea ECR, secrets y el rol OIDC para GitHub Actions
- `platform`: crea la red, ALB, ECS cluster, Cloud Map y RDS
- `services`: crea task definitions, ECS services y ejecuta `db-migrator`

### Requisitos operativos

- `aws`
- `terraform`
- `bash`
- `make`

En Windows, los targets de infraestructura del [project/Makefile](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/project/Makefile) usan Git Bash por defecto.

### Flujo local de Terraform

Desde `project/`:

```bash
make tf-platform-init
make tf-platform-plan
make tf-platform-apply
```

```bash
make tf-services-init
make tf-services-plan SERVICE_IMAGE_TAG=<sha_de_ecr>
make tf-services-apply SERVICE_IMAGE_TAG=<sha_de_ecr>
```

Atajos principales:

```bash
make tf-up SERVICE_IMAGE_TAG=<sha_de_ecr>
make tf-down
```

Semantica actual:

- `tf-up` aplica primero `platform` y despues `services`
- `tf-down` destruye primero `services` y despues `platform`
- `tf-services-apply` genera `infra/services/service-images.auto.tfvars.json` y lo pasa a Terraform con `-var-file`
- `tf-services-destroy` usa un tag placeholder y no requiere una imagen real en ECR

### `db-migrator`

Durante `infra/services`, Terraform ejecuta una task one-off de ECS Fargate para aplicar migraciones sobre RDS antes de estabilizar los servicios internos.

Para forzar una nueva corrida:

```bash
make aws-rerun-migrator SERVICE_IMAGE_TAG=<sha_de_ecr>
```

### CI/CD en GitHub Actions

Workflows relevantes:

- [infra-terraform.yml](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/.github/workflows/infra-terraform.yml)
- [ci-cd.yml](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/.github/workflows/ci-cd.yml)

Comportamiento actual:

- `Terraform Infra` opera `bootstrap`, `foundation` o `platform`
- `CI/CD` testea, build-ea imagenes, publica en ECR y despliega `platform` + `services`
- la ruta operativa de infraestructura es bash-first tanto localmente como en `ubuntu-latest`

## Variables de entorno

Archivo de referencia:

- [.env.template](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/.env.template)

Variables destacadas del gateway:

- `PORT`
- `AUTH_JWT_SECRET`
- `AUTH_JWT_ISSUER`
- `AUTH_JWT_TTL`
- `AUTH_REFRESH_TOKEN_TTL`
- `GATEWAY_METRICS_ENABLED`
- `GATEWAY_METRICS_PATH`
- `GATEWAY_RATE_LIMIT_ENABLED`
- `GATEWAY_RATE_LIMIT_DEFAULT_REQUESTS`
- `GATEWAY_RATE_LIMIT_TRANSFERS_REQUESTS`
- `GATEWAY_RATE_LIMIT_EXEMPT_PATHS`

Variables de observabilidad:

- `GRAFANA_ADMIN_USER`
- `GRAFANA_ADMIN_PASSWORD`
- SMTP de alertas configurado en `project/monitoring/alertmanager/alertmanager.yml`

Variables operativas relevantes para infraestructura:

- `AWS_REGION`
- `TF_STATE_BUCKET`
- `TF_LOCK_TABLE`
- `FOUNDATION_STATE_KEY`
- `PLATFORM_STATE_KEY`
- `SERVICES_STATE_KEY`
- `TF_ENVIRONMENT`
- `SERVICE_IMAGE_TAG`

## Datos de prueba

Usuarios seed:

- `lucas@mail.com` / `Password123!`
- `ana@mail.com` / `Password123!`
- `marcos@mail.com` / `Password123!`

## Documentacion tecnica

La descripcion arquitectonica completa, con flujos, decisiones de diseno, tradeoffs y diagramas, esta en:

- [architecture.md](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/architecture.md)

## Roadmap

### Observabilidad

- [ ] Instrumentar métricas en `user-service`, `fraud-service`, `wallet-service` y `transaction-service`
- [ ] Agregar trazabilidad distribuida (OpenTelemetry)

### Escalabilidad

- [ ] Mover rate limiting a Redis para despliegues multi-instancia
      _(el actual es in-memory, no funciona con múltiples instancias)_

### UX de API

- [ ] Paginar historial en `GET /history/{userID}`

## Contribuciones

¡Las contribuciones son bienvenidas! Seguí estos pasos:

1. Hacé un fork del repositorio.
2. Creá una rama para tu feature o fix (`git checkout -b feature/nueva-funcionalidad`).
3. Realizá tus cambios y escribí pruebas si es necesario.
4. Hacé commit y push a tu rama (`git commit -m "feat: agrega nueva funcionalidad"`).
5. Abrí un Pull Request describiendo tus cambios.

### Convenciones de Commits

Este proyecto sigue [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` Nueva funcionalidad
- `fix:` Corrección de bugs
- `docs:` Cambios en documentación
- `style:` Cambios de formato (no afectan la lógica)
- `refactor:` Refactorización de código
- `test:` Añadir o modificar tests
- `chore:` Tareas de mantenimiento

---

## Licencia

Este proyecto está bajo la licencia **MIT**.

---

<a id="contact-anchor"></a>

## 📬 Contacto

- **Autor:** Lucas Cabral
- **Email:** lucassimple@hotmail.com
- **LinkedIn:** [https://www.linkedin.com/in/lucas-gastón-cabral/](https://www.linkedin.com/in/lucas-gastón-cabral/)
- **Portfolio:** [https://portfolio-web-dev-git-main-lucascabral95s-projects.vercel.app/](https://portfolio-web-dev-git-main-lucascabral95s-projects.vercel.app/)
- **Github:** [https://github.com/Lucascabral95](https://github.com/Lucascabral95/)

---

<p align="center">
  Desarrollado con ❤️ por Lucas Cabral
</p>
