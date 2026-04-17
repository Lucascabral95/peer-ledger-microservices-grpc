# Wallet Service

`wallet-service` es la capa de dinero del sistema. Mantiene balances, procesa topups y ejecuta transferencias con consistencia fuerte.

## Responsibilidad

- crear wallets
- consultar balances
- acreditar saldo via topup
- transferir dinero entre usuarios
- aplicar idempotencia persistente

Es el servicio mas critico desde el punto de vista de integridad monetaria.

## Cómo encaja en el sistema

Relaciones principales:

- no recibe trafico directo del cliente
- es invocado por el `gateway`
- se usa al registrar un usuario para crear la wallet inicial
- se usa para topups autenticados
- se usa para transferencias despues del chequeo de `fraud-service`

Flujo de transferencia:

1. `gateway` autentica y valida request
2. `gateway` llama a `fraud-service`
3. si fraude aprueba, llama a `wallet-service.Transfer`
4. si wallet confirma, `gateway` registra auditoria en `transaction-service`

## gRPC API

Proto:

- [protobuf/wallet.proto](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/protobuf/wallet.proto)

RPCs expuestos:

- `GetBalance(GetBalanceRequest) returns (GetBalanceResponse)`
- `CreateWallet(CreateWalletRequest) returns (CreateWalletResponse)`
- `TopUp(TopUpRequest) returns (TopUpResponse)`
- `GetTopUpSummary(GetTopUpSummaryRequest) returns (GetTopUpSummaryResponse)`
- `ListTopUps(ListTopUpsRequest) returns (ListTopUpsResponse)`
- `Transfer(TransferRequest) returns (TransferResponse)`

### `CreateWallet`

- crea la wallet inicial para un `user_id`
- hoy la balancea en `0`
- se ejecuta como parte del flujo `POST /auth/register` del gateway
- debe mantener una wallet unica por usuario

### `TopUp`

- acredita saldo a una wallet existente
- valida `amount > 0`
- actualiza el balance y registra el evento en `wallet_topups` dentro de la misma transaccion SQL
- el historial de topups es exacto desde la migracion `002_wallet_topups.sql` en adelante

### `GetTopUpSummary`

- devuelve cantidad total de topups
- devuelve monto total recargado
- calcula cantidad y monto del dia usando la zona horaria solicitada por el gateway

### `ListTopUps`

- devuelve historial de topups ordenado por fecha descendente
- soporta filtros por rango de fechas
- permite construir la vista `Mi billetera` y el dashboard del frontend

### `Transfer`

Valida:

- `sender_id`
- `receiver_id`
- `amount`
- `idempotency_key`

Comportamiento:

- convierte monto a centavos
- aplica validacion de negocio
- delega a un repositorio SQL transaccional

Errores funcionales relevantes:

- `wallet not found`
- `insufficient funds`
- `idempotency key reused with different payload`

## Runtime

Entry point:

- [main.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/wallet-service/cmd/api/main.go)

Server implementation:

- [server.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/wallet-service/internal/server/server.go)

Config:

- [config.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/wallet-service/internal/config/config.go)

Puerto por default:

- `50053`

Almacenamiento:

- PostgreSQL `wallets_db`

## Modelo de datos y consistencia

El servicio trabaja con centavos (`int64`) para evitar errores de precision.

La transferencia real se apoya en:

- transacciones SQL
- locking pesimista
- control de saldo insuficiente
- persistencia de idempotencia

Objetivo:

- evitar doble gasto
- evitar saldos negativos bajo concurrencia
- permitir retries seguros

## Configuración

Variables principales:

- `WALLET_GRPC_PORT`
- `WALLET_DB_DSN`
- `WALLET_DB_HOST`
- `WALLET_DB_PORT`
- `WALLET_DB_USER`
- `WALLET_DB_PASSWORD`
- `WALLET_DB_NAME`
- `WALLET_DB_SSLMODE`
- `WALLET_DB_MAX_OPEN_CONNS`
- `WALLET_DB_MAX_IDLE_CONNS`
- `WALLET_DB_CONN_MAX_LIFETIME`
- `WALLET_DB_CONN_MAX_IDLE_TIME`
- `WALLET_DB_CONNECT_TIMEOUT`
- `WALLET_DB_CONNECT_MAX_RETRIES`
- `WALLET_DB_CONNECT_INITIAL_BACKOFF`
- `WALLET_DB_CONNECT_MAX_BACKOFF`
- `GRACEFUL_SHUTDOWN_TIMEOUT`

## Docker

Dockerfile:

- [Dockerfile](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/wallet-service/Dockerfile)

Incluye `grpc-health-probe` para health checks del contenedor.
