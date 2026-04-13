SHELL := cmd.exe
.SHELLFLAGS := /C

COMPOSE_FILE := docker-compose.yml
PROTO_DIR := ../protobuf
GEN_DIR := ../gen
PS := powershell -NoProfile -ExecutionPolicy Bypass -File
BASH ?= "C:/Program Files/Git/bin/bash.exe"
AWS_REGION ?= us-east-1
TF_LOCK_TABLE ?= peer-ledger-terraform-locks
AWS_CLUSTER ?= peer-ledger-production-cluster
AWS_GATEWAY_SERVICE ?= peer-ledger-production-gateway
AWS_USER_SERVICE ?= peer-ledger-production-user-service
AWS_FRAUD_SERVICE ?= peer-ledger-production-fraud-service
AWS_WALLET_SERVICE ?= peer-ledger-production-wallet-service
AWS_TRANSACTION_SERVICE ?= peer-ledger-production-transaction-service
AWS_ALB_DNS ?= peer-ledger-production-alb-1446361633.us-east-1.elb.amazonaws.com
AWS_ACCOUNT_ID ?= 560765037562
AWS_ECR_REGISTRY ?= $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com
TF_PROJECT_NAME ?= peer-ledger
TF_ENVIRONMENT ?= production
TF_STATE_BUCKET ?= peer-ledger-tfstate-560765037562
TF_FOUNDATION_STATE_KEY ?= foundation/terraform.tfstate
TF_STATE_REGION ?= $(AWS_REGION)
SERVICE_IMAGE_TAG ?=
DESTROY_IMAGE_TAG ?= destroy-placeholder
TF_PLATFORM_STATE_KEY ?= platform/terraform.tfstate
TF_SERVICES_STATE_KEY ?= services/terraform.tfstate
TF_COMMON_ARGS = --region "$(AWS_REGION)" --project-name "$(TF_PROJECT_NAME)" --environment "$(TF_ENVIRONMENT)" --state-bucket "$(TF_STATE_BUCKET)" --foundation-state-key "$(TF_FOUNDATION_STATE_KEY)" --foundation-state-region "$(TF_STATE_REGION)" --state-region "$(TF_STATE_REGION)" --lock-table "$(TF_LOCK_TABLE)"

.PHONY: up down restart logs ps restart-grafana tf-platform-init tf-platform-plan tf-platform-apply tf-platform-destroy tf-services-init tf-services-plan tf-services-apply tf-services-destroy tf-services-write-vars tf-services-write-destroy-vars tf-up tf-down proto proto-clean swagger swagger-clean test-wallet test-user test-fraud test-transaction test-gateway test-all test-clean-cache test-wallet-cover test-wallet-cover-html test-user-cover test-user-cover-html test-fraud-cover test-fraud-cover-html test-transaction-cover test-transaction-cover-html aws-restart-gateway aws-restart-user aws-restart-fraud aws-restart-wallet aws-restart-transaction aws-restart-internal aws-restart-all aws-wait-gateway aws-wait-internal aws-wait-all aws-rerun-migrator aws-health

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

# Terraform AWS
tf-platform-init:
	@$(BASH) ./scripts/tf-platform.sh init $(TF_COMMON_ARGS) --platform-state-key "$(TF_PLATFORM_STATE_KEY)"

tf-platform-plan: tf-platform-init
	@$(BASH) ./scripts/tf-platform.sh plan $(TF_COMMON_ARGS) --platform-state-key "$(TF_PLATFORM_STATE_KEY)"

tf-platform-apply: tf-platform-init
	@$(BASH) ./scripts/tf-platform.sh apply $(TF_COMMON_ARGS) --platform-state-key "$(TF_PLATFORM_STATE_KEY)"

tf-platform-destroy: tf-platform-init
	@$(BASH) ./scripts/tf-platform.sh destroy $(TF_COMMON_ARGS) --platform-state-key "$(TF_PLATFORM_STATE_KEY)"

tf-services-init:
	@$(BASH) ./scripts/tf-services.sh init $(TF_COMMON_ARGS) --account-id "$(AWS_ACCOUNT_ID)" --services-state-key "$(TF_SERVICES_STATE_KEY)" --platform-state-key "$(TF_PLATFORM_STATE_KEY)"

tf-services-write-vars:
	@$(BASH) ./scripts/tf-services.sh write-vars $(TF_COMMON_ARGS) --account-id "$(AWS_ACCOUNT_ID)" --services-state-key "$(TF_SERVICES_STATE_KEY)" --platform-state-key "$(TF_PLATFORM_STATE_KEY)" --tag "$(SERVICE_IMAGE_TAG)"

tf-services-write-destroy-vars:
	@$(BASH) ./scripts/tf-services.sh write-destroy-vars $(TF_COMMON_ARGS) --account-id "$(AWS_ACCOUNT_ID)" --services-state-key "$(TF_SERVICES_STATE_KEY)" --platform-state-key "$(TF_PLATFORM_STATE_KEY)" --destroy-tag "$(DESTROY_IMAGE_TAG)"

tf-services-plan: tf-services-init
	@$(BASH) ./scripts/tf-services.sh plan $(TF_COMMON_ARGS) --account-id "$(AWS_ACCOUNT_ID)" --services-state-key "$(TF_SERVICES_STATE_KEY)" --platform-state-key "$(TF_PLATFORM_STATE_KEY)" --tag "$(SERVICE_IMAGE_TAG)"

tf-services-apply: tf-services-init
	@$(BASH) ./scripts/tf-services.sh apply $(TF_COMMON_ARGS) --account-id "$(AWS_ACCOUNT_ID)" --services-state-key "$(TF_SERVICES_STATE_KEY)" --platform-state-key "$(TF_PLATFORM_STATE_KEY)" --tag "$(SERVICE_IMAGE_TAG)"

tf-services-destroy: tf-services-init
	@$(BASH) ./scripts/tf-services.sh destroy $(TF_COMMON_ARGS) --account-id "$(AWS_ACCOUNT_ID)" --services-state-key "$(TF_SERVICES_STATE_KEY)" --platform-state-key "$(TF_PLATFORM_STATE_KEY)" --destroy-tag "$(DESTROY_IMAGE_TAG)"

# Levantar toda la infraestructura (platform + services)
tf-up: tf-platform-apply tf-services-apply
# Destruir toda la infraestructura (services + platform)
tf-down: tf-services-destroy tf-platform-destroy

# AWS ECS
aws-restart-gateway:
	@aws ecs update-service --region $(AWS_REGION) --cluster $(AWS_CLUSTER) --service $(AWS_GATEWAY_SERVICE) --force-new-deployment

aws-restart-user:
	@aws ecs update-service --region $(AWS_REGION) --cluster $(AWS_CLUSTER) --service $(AWS_USER_SERVICE) --force-new-deployment

aws-restart-fraud:
	@aws ecs update-service --region $(AWS_REGION) --cluster $(AWS_CLUSTER) --service $(AWS_FRAUD_SERVICE) --force-new-deployment

aws-restart-wallet:
	@aws ecs update-service --region $(AWS_REGION) --cluster $(AWS_CLUSTER) --service $(AWS_WALLET_SERVICE) --force-new-deployment

aws-restart-transaction:
	@aws ecs update-service --region $(AWS_REGION) --cluster $(AWS_CLUSTER) --service $(AWS_TRANSACTION_SERVICE) --force-new-deployment

aws-restart-internal:
	@aws ecs update-service --region $(AWS_REGION) --cluster $(AWS_CLUSTER) --service $(AWS_USER_SERVICE) --force-new-deployment
	@aws ecs update-service --region $(AWS_REGION) --cluster $(AWS_CLUSTER) --service $(AWS_FRAUD_SERVICE) --force-new-deployment
	@aws ecs update-service --region $(AWS_REGION) --cluster $(AWS_CLUSTER) --service $(AWS_WALLET_SERVICE) --force-new-deployment
	@aws ecs update-service --region $(AWS_REGION) --cluster $(AWS_CLUSTER) --service $(AWS_TRANSACTION_SERVICE) --force-new-deployment

aws-restart-all: aws-restart-internal aws-restart-gateway

aws-wait-gateway:
	@aws ecs wait services-stable --region $(AWS_REGION) --cluster $(AWS_CLUSTER) --services $(AWS_GATEWAY_SERVICE)

aws-wait-internal:
	@aws ecs wait services-stable --region $(AWS_REGION) --cluster $(AWS_CLUSTER) --services $(AWS_USER_SERVICE) $(AWS_FRAUD_SERVICE) $(AWS_WALLET_SERVICE) $(AWS_TRANSACTION_SERVICE)

aws-wait-all:
	@aws ecs wait services-stable --region $(AWS_REGION) --cluster $(AWS_CLUSTER) --services $(AWS_USER_SERVICE) $(AWS_FRAUD_SERVICE) $(AWS_WALLET_SERVICE) $(AWS_TRANSACTION_SERVICE) $(AWS_GATEWAY_SERVICE)

aws-rerun-migrator:
	@$(BASH) ./scripts/tf-services.sh rerun-migrator $(TF_COMMON_ARGS) --account-id "$(AWS_ACCOUNT_ID)" --services-state-key "$(TF_SERVICES_STATE_KEY)" --platform-state-key "$(TF_PLATFORM_STATE_KEY)" --tag "$(SERVICE_IMAGE_TAG)"

aws-health:
	@curl -fsS http://$(AWS_ALB_DNS)/health

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

# Swagger (gateway)
swagger:
	@go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -d ../services/gateway/cmd/api -o ../services/gateway/docs
	@echo "OK: documentacion Swagger generada en services/gateway/docs"

swagger-clean:
	@Remove-Item -Recurse -Force "../services/gateway/docs" -ErrorAction SilentlyContinue
	@echo "OK: documentacion Swagger eliminada de services/gateway/docs"

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
