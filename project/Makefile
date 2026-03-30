COMPOSE_FILE := docker-compose.yml
PROTO_DIR := ../protobuf
GEN_DIR := ../gen

.PHONY: up down restart logs ps proto proto-clean

# Containers
up:
	@docker-compose -f $(COMPOSE_FILE) up --build -d

down:
	@docker-compose -f $(COMPOSE_FILE) down

restart:
	@docker-compose -f $(COMPOSE_FILE) down
	@docker-compose -f $(COMPOSE_FILE) up --build -d

logs:
	@docker-compose -f $(COMPOSE_FILE) logs -f --tail=100

ps:
	@docker-compose -f $(COMPOSE_FILE) ps

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
