# User Service

`user-service` concentra identidad y credenciales. Es responsable de registrar usuarios, autenticar email/password y resolver informacion basica de usuario.

## Responsabilidad

- registrar usuarios
- autenticar credenciales
- obtener usuario por ID
- verificar existencia de usuario

No emite JWT. El token lo emite el `gateway` despues de un registro o login exitoso.

## Cómo encaja en el sistema

Relaciones principales:

- el cliente nunca llama a este servicio directamente
- el `gateway` lo usa para `register`, `login`, `get user` y `user exists`
- el `gateway` usa su respuesta para emitir JWT
- `wallet-service` depende indirectamente de usuarios validos, pero no llama a `user-service`

## gRPC API

Proto:

- [protobuf/user.proto](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/protobuf/user.proto)

RPCs expuestos:

- `GetUser(GetUserRequest) returns (GetUserResponse)`
- `UserExists(UserExistsRequest) returns (UserExistsResponse)`
- `Register(RegisterRequest) returns (RegisterResponse)`
- `Login(LoginRequest) returns (LoginResponse)`

### `Register`

Valida:

- `name`
- `email`
- `password`

Comportamiento:

- normaliza email
- valida formato
- aplica minimo de password
- hashea password con PBKDF2-SHA256
- genera `user_id`
- persiste el usuario

### `Login`

Comportamiento:

- busca usuario por email
- compara password contra hash persistido
- devuelve datos del usuario si las credenciales son validas

## Runtime

Entry point:

- [main.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/user-service/cmd/api/main.go)

Server implementation:

- [server.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/user-service/internal/server/server.go)

Config:

- [config.go](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/user-service/internal/config/config.go)

Puerto por default:

- `50051`

Storage:

- PostgreSQL `users_db`

## Configuración

Variables principales:

- `GRPC_PORT`
- `USER_DB_DSN`
- `USER_DB_HOST`
- `USER_DB_PORT`
- `USER_DB_USER`
- `USER_DB_PASSWORD`
- `USER_DB_NAME`
- `USER_DB_SSLMODE`
- `USER_PASSWORD_HASH_ITERATIONS`
- `USER_PASSWORD_MIN_LENGTH`
- `DB_MAX_OPEN_CONNS`
- `DB_MAX_IDLE_CONNS`
- `DB_CONN_MAX_LIFETIME`
- `DB_CONN_MAX_IDLE_TIME`
- `DB_CONNECT_TIMEOUT`
- `DB_CONNECT_MAX_RETRIES`
- `DB_CONNECT_INITIAL_BACKOFF`
- `DB_CONNECT_MAX_BACKOFF`
- `GRACEFUL_SHUTDOWN_TIMEOUT`

Defaults importantes:

- puerto gRPC: `50051`
- hash iterations: `120000`
- password minima: `8`

## Notas de Seguridad

- passwords nunca se almacenan en texto plano
- el servicio solo verifica credenciales y devuelve identidad
- el manejo de JWT queda fuera de este servicio para mantener separacion clara entre identidad y borde HTTP

## Docker

Dockerfile:

- [Dockerfile](/C:/Users/lucas/OneDrive/Desktop/practices-with-go/peer-ledger-microservices-grpc/services/user-service/Dockerfile)
