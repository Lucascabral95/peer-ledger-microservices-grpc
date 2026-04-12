# Transaction Service

`transaction-service` es la capa de auditoria e historial. Registra transferencias confirmadas y expone consultas por usuario.

## Responsibilidad

- persistir auditoria de transferencias
- garantizar idempotencia en el registro
- exponer historial por usuario

No mueve dinero. Ese trabajo pertenece a `wallet-service`.

## Cómo encaja en el sistema

Relacion con el resto:

- no recibe trafico directo del cliente
- es invocado por el `gateway`
- normalmente se llama despues de un `wallet-service.Transfer` exitoso

Flujo tipico:

1. `gateway` llama a `wallet-service.Transfer`
2. si wallet confirma, `gateway` llama a `transaction-service.Record`
3. luego `gateway` puede servir historial con `GetHistory`

## gRPC API

Proto:

- [protobuf/transaction.proto](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/protobuf/transaction.proto)

RPCs expuestos:

- `Record(RecordRequest) returns (RecordResponse)`
- `GetHistory(GetHistoryRequest) returns (GetHistoryResponse)`

### `Record`

Input principal:

- `transaction_id`
- `sender_id`
- `receiver_id`
- `amount`
- `idempotency_key`

Comportamiento:

- valida input
- convierte monto a centavos
- persiste la transaccion con estado `completed`
- aplica idempotencia estricta

Casos relevantes:

- misma key + mismo payload -> no duplica
- misma key + distinto payload -> error
- `transaction_id` repetido con colision -> conflicto

### `GetHistory`

Input:

- `user_id`

Output:

- lista de `TransactionRecord`

Incluye:

- `transaction_id`
- `sender_id`
- `receiver_id`
- `amount`
- `status`
- `created_at`

## Runtime

Entry point:

- [main.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/transaction-service/cmd/api/main.go)

Server implementation:

- [server.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/transaction-service/internal/server/server.go)

Config:

- [config.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/transaction-service/internal/config/config.go)

Puerto por default:

- `50054`

Storage:

- PostgreSQL `transactions_db`

## Configuración

Variables principales:

- `TRANSACTION_GRPC_PORT`
- `TRANSACTION_DB_DSN`
- `TRANSACTION_DB_HOST`
- `TRANSACTION_DB_PORT`
- `TRANSACTION_DB_USER`
- `TRANSACTION_DB_PASSWORD`
- `TRANSACTION_DB_NAME`
- `TRANSACTION_DB_SSLMODE`
- `TRANSACTION_DB_MAX_OPEN_CONNS`
- `TRANSACTION_DB_MAX_IDLE_CONNS`
- `TRANSACTION_DB_CONN_MAX_LIFETIME`
- `TRANSACTION_DB_CONN_MAX_IDLE_TIME`
- `TRANSACTION_DB_CONNECT_TIMEOUT`
- `TRANSACTION_DB_CONNECT_MAX_RETRIES`
- `TRANSACTION_DB_CONNECT_INITIAL_BACKOFF`
- `TRANSACTION_DB_CONNECT_MAX_BACKOFF`
- `GRACEFUL_SHUTDOWN_TIMEOUT`

## Notas de diseño

- los montos se convierten a centavos (`int64`) antes de persistir
- la consulta de historial es responsabilidad de este servicio, no del wallet
- la auditoria separada reduce acoplamiento entre dinero y reporting

Tradeoff actual:

- si `wallet-service` confirma y este servicio falla, la transferencia ya fue ejecutada
- el sistema se apoya en idempotencia para retries seguros desde el `gateway`

## Docker

Dockerfile:

- [Dockerfile](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/transaction-service/Dockerfile)

Incluye `grpc-health-probe` para health checks del contenedor.
