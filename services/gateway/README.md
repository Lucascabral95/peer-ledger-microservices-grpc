# API Gateway

`gateway` es el unico entrypoint HTTP del sistema. Expone la API publica, autentica requests con JWT, aplica middlewares operativos y orquesta llamadas gRPC a los servicios internos.

## Responsabilidad

- exponer la API HTTP publica
- emitir y validar JWT
- aplicar rate limiting mediante un algoritmo **token bucket**
- exponer metricas Prometheus
- traducir errores gRPC a respuestas HTTP
- coordinar flujos entre `user-service`, `fraud-service`, `wallet-service` y `transaction-service`

No tiene base de datos propia.

## Cómo encaja en el sistema

El cliente solo habla con este servicio.

Relaciones internas:

- llama a `user-service` para registro, login, consulta y existencia de usuarios
- llama a `fraud-service` antes de ejecutar una transferencia
- llama a `wallet-service` para provision de wallet, topups y transferencias
- llama a `transaction-service` para registrar auditoria y consultar historial

Regla arquitectonica importante:

- los servicios internos no se llaman entre si
- el `gateway` compone el caso de uso completo

## HTTP API pública

Routes:

- `GET /health`
- `GET /ping`
- `GET /metrics`
- `POST /auth/register`
- `POST /auth/login`
- `GET /users/{userID}`
- `GET /users/{userID}/exists`
- `GET /history/{userID}`
- `POST /topups`
- `POST /transfers`

Rutas autenticadas:

- `GET /history/{userID}`
- `POST /topups`
- `POST /transfers`

## Flujo interno

### Registro

1. valida payload HTTP
2. llama a `user-service.Register`
3. llama a `wallet-service.CreateWallet`
4. emite JWT
5. devuelve token + datos del usuario

### Login

1. valida credenciales
2. llama a `user-service.Login`
3. emite JWT
4. devuelve token + datos del usuario

### Transferencia

1. toma `sender_id` desde el JWT
2. valida que el receptor exista en `user-service`
3. llama a `fraud-service.EvaluateTransfer`
4. si aprueba, llama a `wallet-service.Transfer`
5. si wallet confirma, llama a `transaction-service.Record`
6. responde al cliente

Tradeoff actual:

- si `wallet-service` confirma pero `transaction-service` falla, el dinero ya se movio
- en ese caso el cliente recibe error retryable y debe reintentar con la misma `idempotency_key`

## Runtime

Entry point:

- [main.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/cmd/api/main.go)

Handlers:

- [handlers.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/cmd/api/handlers.go)

Routes:

- [routes.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/cmd/api/routes.go)

Config:

- [config.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/internal/config/config.go)

Puerto por default:

- `8080`

## Configuración

Variables principales:

- `PORT`
- `USER_SERVICE_GRPC_ADDR`
- `FRAUD_SERVICE_GRPC_ADDR`
- `WALLET_SERVICE_GRPC_ADDR`
- `TRANSACTION_SERVICE_GRPC_ADDR`
- `AUTH_JWT_SECRET`
- `AUTH_JWT_ISSUER`
- `AUTH_JWT_TTL`
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
- timeout de dial gRPC: `3s`
- max intentos gRPC: `10`
- metricas habilitadas en `/metrics`
- rate limit default: `120 req / 1m`
- rate limit transferencias: `20 req / 1m`

## Características operativas

- retry exponencial al conectar con servicios gRPC al arrancar
- middleware de autenticacion JWT
- logs de acceso
- metricas Prometheus
- rate limiting por IP con algoritmo **token bucket**
- CORS habilitado

## Docker

Dockerfile:

- [Dockerfile](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/Dockerfile)

# API Gateway

`gateway` es el unico entrypoint HTTP del sistema. Expone la API publica, autentica requests con JWT, aplica middlewares operativos y orquesta llamadas gRPC a los servicios internos.

## Responsabilidad

- exponer la API HTTP publica
- emitir y validar JWT
- aplicar rate limiting mediante un algoritmo **token bucket**
- exponer metricas Prometheus
- traducir errores gRPC a respuestas HTTP
- coordinar flujos entre `user-service`, `fraud-service`, `wallet-service` y `transaction-service`

No tiene base de datos propia.

## Cómo encaja en el sistema

El cliente solo habla con este servicio.

Relaciones internas:

- llama a `user-service` para registro, login, consulta y existencia de usuarios
- llama a `fraud-service` antes de ejecutar una transferencia
- llama a `wallet-service` para provision de wallet, topups y transferencias
- llama a `transaction-service` para registrar auditoria y consultar historial

Regla arquitectonica importante:

- los servicios internos no se llaman entre si
- el `gateway` compone el caso de uso completo

## HTTP API pública

Routes:

- `GET /health`
- `GET /ping`
- `GET /metrics`
- `POST /auth/register`
- `POST /auth/login`
- `GET /users/{userID}`
- `GET /users/{userID}/exists`
- `GET /history/{userID}`
- `POST /topups`
- `POST /transfers`

Rutas autenticadas:

- `GET /history/{userID}`
- `POST /topups`
- `POST /transfers`

## Flujo interno

### Registro

1. valida payload HTTP
2. llama a `user-service.Register`
3. llama a `wallet-service.CreateWallet`
4. emite JWT
5. devuelve token + datos del usuario

### Login

1. valida credenciales
2. llama a `user-service.Login`
3. emite JWT
4. devuelve token + datos del usuario

### Transferencia

1. toma `sender_id` desde el JWT
2. valida que el receptor exista en `user-service`
3. llama a `fraud-service.EvaluateTransfer`
4. si aprueba, llama a `wallet-service.Transfer`
5. si wallet confirma, llama a `transaction-service.Record`
6. responde al cliente

Tradeoff actual:

- si `wallet-service` confirma pero `transaction-service` falla, el dinero ya se movio
- en ese caso el cliente recibe error retryable y debe reintentar con la misma `idempotency_key`

## Runtime

Entry point:

- [main.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/cmd/api/main.go)

Handlers:

- [handlers.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/cmd/api/handlers.go)

Routes:

- [routes.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/cmd/api/routes.go)

Config:

- [config.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/internal/config/config.go)

Puerto por default:

- `8080`

## Configuración

Variables principales:

- `PORT`
- `USER_SERVICE_GRPC_ADDR`
- `FRAUD_SERVICE_GRPC_ADDR`
- `WALLET_SERVICE_GRPC_ADDR`
- `TRANSACTION_SERVICE_GRPC_ADDR`
- `AUTH_JWT_SECRET`
- `AUTH_JWT_ISSUER`
- `AUTH_JWT_TTL`
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
- timeout de dial gRPC: `3s`
- max intentos gRPC: `10`
- metricas habilitadas en `/metrics`
- rate limit default: `120 req / 1m`
- rate limit transferencias: `20 req / 1m`

## Características operativas

- retry exponencial al conectar con servicios gRPC al arrancar
- middleware de autenticacion JWT
- logs de acceso
- metricas Prometheus
- rate limiting por IP con algoritmo **token bucket**
- CORS habilitado

## Docker

Dockerfile:

- [Dockerfile](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/gateway/Dockerfile)
