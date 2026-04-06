COMPOSE_FILE := docker-compose.yml
PROTO_DIR := ../protobuf
GEN_DIR := ../gen
PS := powershell -NoProfile -ExecutionPolicy Bypass -File

.PHONY: up down restart logs ps proto proto-clean test-wallet test-user test-fraud test-transaction test-gateway test-all test-clean-cache test-wallet-cover test-wallet-cover-html test-user-cover test-user-cover-html test-fraud-cover test-fraud-cover-html test-transaction-cover test-transaction-cover-html

# Containers
up:
	@docker-compose -f $(COMPOSE_FILE) up --build -d

down:
	@docker-compose -f $(COMPOSE_FILE) down

restart:
	@docker-compose -f $(COMPOSE_FILE) down
	@docker-compose -f $(COMPOSE_FILE) up --build -d

reset:
	@docker-compose -f $(COMPOSE_FILE) down -v
	@docker-compose -f $(COMPOSE_FILE) up --build -d
	@echo "OK: stack reiniciado con volúmenes limpios"

logs:
	@docker-compose -f $(COMPOSE_FILE) logs -f --tail=100

ps:
	@docker-compose -f $(COMPOSE_FILE) ps

# Restart Grafana
restart-grafana:
	@docker compose -f docker-compose.yml up -d --force-recreate grafana

# Protobuf
proto:
	@protoc \
		--proto_path=$(PROTO_DIR) "$(PROTO_DIR)/user.proto" \
		--go_out=$(GEN_DIR)/user --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR)/user --go-grpc_opt=paths=source_relative
	@protoc \
		--proto_path=$(PROTO_DIR) "$(PROTO_DIR)/fraud.proto" \
		--go_out=$(GEN_DIR)/fraud --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR)/fraud --go-grpc_opt=paths=source_relative
	@protoc \
		--proto_path=$(PROTO_DIR) "$(PROTO_DIR)/wallet.proto" \
		--go_out=$(GEN_DIR)/wallet --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR)/wallet --go-grpc_opt=paths=source_relative
	@protoc \
		--proto_path=$(PROTO_DIR) "$(PROTO_DIR)/transaction.proto" \
		--go_out=$(GEN_DIR)/transaction --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR)/transaction --go-grpc_opt=paths=source_relative
	@echo "OK: codigo protobuf generado en /gen"

proto-clean:
	@Remove-Item -Recurse -Force "$(GEN_DIR)/user/*","$(GEN_DIR)/fraud/*","$(GEN_DIR)/wallet/*","$(GEN_DIR)/transaction/*" -ErrorAction SilentlyContinue
	@echo "OK: archivos generados eliminados"

# Tests (Go)
# Modo rapido/offline: no descarga modulos. Si falta alguno en cache local, falla rapido.
test-wallet:
	@$(PS) ./scripts/run-go-test.ps1 -PackagePath ./../services/wallet-service/...

test-user:
	@$(PS) ./scripts/run-go-test.ps1 -PackagePath ./../services/user-service/...

test-fraud:
	@$(PS) ./scripts/run-go-test.ps1 -PackagePath ./../services/fraud-service/...

test-transaction:
	@$(PS) ./scripts/run-go-test.ps1 -PackagePath ./../services/transaction-service/...

test-gateway:
	@$(PS) ./scripts/run-go-test.ps1 -PackagePath ./../services/gateway/...

test-all:
	@$(PS) ./scripts/run-go-test.ps1 -PackagePath ./../...

test-clean-cache:
	@$(PS) ./scripts/clean-go-test-cache.ps1
	@echo "OK: caches de test eliminados"

test-wallet-cover:
	@$(PS) ./scripts/run-go-test.ps1 -PackagePath ./../services/wallet-service/... -CoverProfile wallet.cover.out
	@cmd /C "if not exist wallet.cover.out (echo wallet.cover.out no fue generado && exit /b 1)"
	@$(PS) ./scripts/show-go-cover.ps1 -CoverProfile wallet.cover.out

test-wallet-cover-html:
	@cmd /C "if not exist wallet.cover.out (echo wallet.cover.out no existe. Ejecuta make test-wallet-cover primero. && exit /b 1)"
	@$(PS) ./scripts/show-go-cover.ps1 -CoverProfile wallet.cover.out -Html -OutputFile wallet.cover.html
	@echo "OK: reporte HTML generado en project/wallet.cover.html"

test-user-cover:
	@$(PS) ./scripts/run-go-test.ps1 -PackagePath ./../services/user-service/... -CoverProfile user.cover.out
	@cmd /C "if not exist user.cover.out (echo user.cover.out no fue generado && exit /b 1)"
	@$(PS) ./scripts/show-go-cover.ps1 -CoverProfile user.cover.out

test-user-cover-html:
	@cmd /C "if not exist user.cover.out (echo user.cover.out no existe. Ejecuta make test-user-cover primero. && exit /b 1)"
	@$(PS) ./scripts/show-go-cover.ps1 -CoverProfile user.cover.out -Html -OutputFile user.cover.html
	@echo "OK: reporte HTML generado en project/user.cover.html"

test-fraud-cover:
	@$(PS) ./scripts/run-go-test.ps1 -PackagePath ./../services/fraud-service/... -CoverProfile fraud.cover.out
	@cmd /C "if not exist fraud.cover.out (echo fraud.cover.out no fue generado && exit /b 1)"
	@$(PS) ./scripts/show-go-cover.ps1 -CoverProfile fraud.cover.out

test-fraud-cover-html:
	@cmd /C "if not exist fraud.cover.out (echo fraud.cover.out no existe. Ejecuta make test-fraud-cover primero. && exit /b 1)"
	@$(PS) ./scripts/show-go-cover.ps1 -CoverProfile fraud.cover.out -Html -OutputFile fraud.cover.html
	@echo "OK: reporte HTML generado en project/fraud.cover.html"

test-transaction-cover:
	@$(PS) ./scripts/run-go-test.ps1 -PackagePath ./../services/transaction-service/... -CoverProfile transaction.cover.out
	@cmd /C "if not exist transaction.cover.out (echo transaction.cover.out no fue generado && exit /b 1)"
	@$(PS) ./scripts/show-go-cover.ps1 -CoverProfile transaction.cover.out

test-transaction-cover-html:
	@cmd /C "if not exist transaction.cover.out (echo transaction.cover.out no existe. Ejecuta make test-transaction-cover primero. && exit /b 1)"
	@$(PS) ./scripts/show-go-cover.ps1 -CoverProfile transaction.cover.out -Html -OutputFile transaction.cover.html
	@echo "OK: reporte HTML generado en project/transaction.cover.html"
