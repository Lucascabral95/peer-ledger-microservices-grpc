# API Gateway

`gateway` es el unico entrypoint HTTP publico del sistema. Expone la API externa, emite y valida JWT, aplica middleware operativo y orquesta llamadas gRPC a los servicios internos.

## Responsabilidad

- exponer la API HTTP publica
- emitir y validar JWT
- aplicar rate limiting por IP con algoritmo token bucket
- exponer metricas Prometheus
- traducir fallos gRPC a respuestas HTTP
- orquestar flujos entre `user-service`, `fraud-service`, `wallet-service` y `transaction-service`

Este servicio no tiene base de datos propia.

## Como Encaja

Los clientes solo hablan con el `gateway`.

Relaciones internas:

- llama a `user-service` para registro, login, consulta de usuarios y verificacion de existencia
- llama a `fraud-service` antes de ejecutar transferencias
- llama a `wallet-service` para provision de wallets, topups y transferencias de saldo
- llama a `transaction-service` para auditoria e historial

Regla arquitectonica:

- los servicios internos no se llaman entre si directamente
- el `gateway` compone el caso de uso de punta a punta

## API HTTP Publica

Rutas:

- `GET /health`
- `GET /ping`
- `GET /metrics`
- `GET /swagger/index.html`
- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`
- `GET /users/{userID}`
- `GET /users/{userID}/exists`
- `GET /history/{userID}`
- `POST /topups`
- `POST /transfers`

Rutas protegidas:

- `GET /history/{userID}`
- `POST /topups`
- `POST /transfers`

## Swagger / OpenAPI

Swagger documenta el contrato HTTP publico del `gateway`. Los servicios internos gRPC no se exponen por Swagger; su fuente de verdad sigue siendo los archivos `.proto` bajo `protobuf/`.

Flujo de generacion desde `project/`:

```bash
make swagger
```

Artefactos generados:

- `services/gateway/docs/docs.go`
- `services/gateway/docs/swagger.json`
- `services/gateway/docs/swagger.yaml`

Swagger UI se sirve desde el propio gateway en:

- `GET /swagger/index.html`

Comportamiento de autenticacion en Swagger:

- las rutas publicas aparecen sin autenticacion
- `POST /auth/refresh` acepta `refresh_token` sin bearer access token
- `GET /history/{userID}`, `POST /topups` y `POST /transfers` requieren `Authorization: Bearer <token>`

## Flujo Interno

### Registro

1. valida el payload HTTP
2. llama a `user-service.Register`
3. llama a `wallet-service.CreateWallet`
4. si la wallet se crea correctamente, emite access token y refresh token
5. devuelve tokens y datos del usuario

Regla de consistencia:

- cada usuario registrado debe tener una wallet inicial unica con balance `0`
- si `wallet-service.CreateWallet` falla despues de crear el usuario, el gateway intenta revertir la creacion llamando a `user-service.DeleteUser`
- ante una falla de provision de wallet, el gateway debe responder un error controlado y no cortar la conexion HTTP

### Login

1. valida credenciales
2. llama a `user-service.Login`
3. emite un JWT
4. devuelve token y datos del usuario

### Refresh Token

1. recibe un `refresh_token`
2. valida firma, expiracion y tipo de token
3. consulta `user-service` para verificar que el usuario siga existiendo
4. rota el refresh token y emite un nuevo access token
5. devuelve ambos tokens al cliente

Duracion de tokens:

- `AUTH_JWT_TTL` define cuanto tiempo vive el access token
- `AUTH_REFRESH_TOKEN_TTL` define cuanto tiempo vive el refresh token
- cuando el access token expira, el frontend puede usar un refresh token aun valido para pedir un nuevo par de tokens
- si el refresh token tambien expiro, el usuario debe autenticarse de nuevo

### Transferencia

1. toma `sender_id` desde el JWT
2. valida que el receptor exista via `user-service`
3. llama a `fraud-service.EvaluateTransfer`
4. si aprueba, llama a `wallet-service.Transfer`
5. si wallet confirma, llama a `transaction-service.Record`
6. devuelve el resultado al cliente

Tradeoff actual:

- si `wallet-service` confirma pero `transaction-service` falla, el dinero ya se movio
- en ese caso el cliente recibe un error reintentable y debe reintentar con la misma `idempotency_key`

## Runtime

Entry point:

- [main.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/cmd/api/main.go)

Handlers:

- [handlers.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/cmd/api/handlers.go)

Routes:

- [routes.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/cmd/api/routes.go)

Config:

- [config.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/internal/config/config.go)

Puerto por defecto:

- `8080`

## Configuracion

Variables principales:

- `PORT`
- `USER_SERVICE_GRPC_ADDR`
- `FRAUD_SERVICE_GRPC_ADDR`
- `WALLET_SERVICE_GRPC_ADDR`
- `TRANSACTION_SERVICE_GRPC_ADDR`
- `AUTH_JWT_SECRET`
- `AUTH_JWT_ISSUER`
- `AUTH_JWT_TTL`
- `AUTH_REFRESH_TOKEN_TTL`
- `GATEWAY_GRPC_DIAL_TIMEOUT`
- `GATEWAY_GRPC_MAX_ATTEMPTS`
- `GATEWAY_METRICS_ENABLED`
- `GATEWAY_METRICS_PATH`
- `GATEWAY_RATE_LIMIT_ENABLED`
- `GATEWAY_RATE_LIMIT_DEFAULT_REQUESTS`
- `GATEWAY_RATE_LIMIT_DEFAULT_WINDOW`
- `GATEWAY_RATE_LIMIT_TRANSFERS_REQUESTS`
- `GATEWAY_RATE_LIMIT_TRANSFERS_WINDOW`
- `GATEWAY_RATE_LIMIT_CLEANUP_INTERVAL`
- `GATEWAY_RATE_LIMIT_TRUST_PROXY`
- `GATEWAY_RATE_LIMIT_EXEMPT_PATHS`
- `GATEWAY_GRACEFUL_SHUTDOWN_TIMEOUT`

Defaults importantes:

- puerto HTTP: `8080`
- access token TTL: `24h`
- refresh token TTL: `168h` (`7d`)
- timeout de dial gRPC: `3s`
- maximo de intentos gRPC: `10`
- metricas habilitadas en `/metrics`
- rate limit default: `120 req / 1m`
- rate limit de transferencias: `20 req / 1m`

Significado de JWT vs refresh:

- `AUTH_JWT_TTL`: tiempo de vida del token bearer usado para acceder a rutas protegidas
- `AUTH_REFRESH_TOKEN_TTL`: tiempo de vida del token usado para llamar a `POST /auth/refresh` y renovar la sesion sin pedir login otra vez

## Caracteristicas Operativas

- retry exponencial al conectar con servicios gRPC al arrancar
- middleware de autenticacion JWT
- logging estructurado de acceso
- metricas Prometheus
- rate limiting por IP con algoritmo token bucket
- CORS habilitado

## Docker

Dockerfile:

- [Dockerfile](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/Dockerfile)
