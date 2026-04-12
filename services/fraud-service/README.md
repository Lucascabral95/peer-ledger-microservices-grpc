# Fraud Service

`fraud-service` es el motor de decision preventiva del sistema. Expone un servicio gRPC que evalua si una transferencia debe permitirse o bloquearse antes de tocar dinero en `wallet-service`.

## Responsibilidad

- validar transferencias antes de la mutacion monetaria
- aplicar reglas antifraude en memoria
- devolver una decision simple: `allowed`, `reason`, `rule_code`

No persiste datos en PostgreSQL. Todo su estado operativo vive en memoria.

## Cómo encaja en el sistema

Flujo normal de una transferencia:

1. el cliente llama al `gateway`
2. el `gateway` autentica al usuario y arma el request interno
3. el `gateway` llama a `fraud-service`
4. si `fraud-service` aprueba, el `gateway` llama a `wallet-service`
5. si `wallet-service` confirma, el `gateway` llama a `transaction-service`

Este servicio no llama a otros microservicios. Solo responde evaluaciones.

## gRPC API

Proto:

- [protobuf/fraud.proto](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/protobuf/fraud.proto)

RPC expuesto:

- `EvaluateTransfer(EvaluateRequest) returns (EvaluateResponse)`

Campos relevantes de entrada:

- `sender_id`
- `receiver_id`
- `amount`
- `idempotency_key`

Campos relevantes de salida:

- `allowed`
- `reason`
- `rule_code`

## Reglas implementadas

Segun la configuracion actual, evalua:

- limite maximo por transferencia
- limite diario por emisor
- limite de velocidad por ventana temporal
- cooldown por par `sender -> receiver`
- consistencia de `idempotency_key`

El repositorio interno mantiene:

- acumulados diarios
- ventanas de actividad
- cooldowns
- cache de decisiones por idempotencia

## Runtime

Entry point:

- [main.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/fraud-service/cmd/api/main.go)

Server implementation:

- [server.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/fraud-service/internal/server/server.go)

Config:

- [config.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/fraud-service/internal/config/config.go)

Puerto por default:

- `50052`

Health:

- registra `gRPC health`

Graceful shutdown:

- soportado via `SIGINT` / `SIGTERM`

## Configuración

Variables principales:

- `FRAUD_GRPC_PORT`: puerto gRPC donde escucha el servicio.
- `FRAUD_PER_TX_LIMIT`: monto maximo permitido por transferencia individual. Si una operacion supera este valor, la decision sale bloqueada.
- `FRAUD_DAILY_LIMIT`: acumulado maximo diario permitido por `sender_id`. Si el emisor supera ese total en la ventana diaria, la transferencia se rechaza.
- `FRAUD_VELOCITY_MAX_COUNT`: cantidad maxima de transferencias permitidas dentro de la ventana de velocidad configurada. Sirve para detectar rafagas anormales.
- `FRAUD_VELOCITY_WINDOW`: duracion de la ventana usada para la regla de velocidad. Se evalua junto con `FRAUD_VELOCITY_MAX_COUNT`.
- `FRAUD_PAIR_COOLDOWN`: tiempo minimo entre dos transferencias consecutivas del mismo `sender_id` hacia el mismo `receiver_id`. Evita repeticiones rapidas sobre el mismo par.
- `FRAUD_IDEMPOTENCY_TTL`: tiempo de retencion de decisiones por `idempotency_key`. Dentro de esa ventana, la misma key debe reutilizarse con el mismo payload o se marca mismatch.
- `FRAUD_TIMEZONE`: timezone usada para cortar el acumulado diario. Define cuando empieza y termina el "dia" para la regla de limite diario.
- `FRAUD_CLEANUP_INTERVAL`: frecuencia con la que el janitor interno limpia estado expirado de ventanas, cooldowns y cache de idempotencia.
- `GRACEFUL_SHUTDOWN_TIMEOUT`: timeout maximo para cerrar el servidor gRPC sin cortar requests en curso.

Valores por default relevantes:

- puerto gRPC: `50052`
- limite por transferencia: `20000`
- limite diario: `50000`
- ventana de velocidad: `10m`
- cooldown por par: `30s`
- TTL de idempotencia: `24h`

## Notas operativas

- reiniciar el servicio limpia el estado antifraude en memoria RAM
- eso simplifica el diseno, pero implica que no hay memoria historica durable
- para un entorno multi-instancia o altamente persistente, el siguiente paso seria externalizar este estado

## Docker

Dockerfile:

- [Dockerfile](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/fraud-service/Dockerfile)

Incluye `grpc-health-probe` para health checks del contenedor.
