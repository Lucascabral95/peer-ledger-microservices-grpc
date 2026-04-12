SHELL := cmd.exe
.SHELLFLAGS := /C

COMPOSE_FILE := docker-compose.yml
PROTO_DIR := ../protobuf
GEN_DIR := ../gen
PS := powershell -NoProfile -ExecutionPolicy Bypass -File
AWS_REGION ?= us-east-1
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
TF_POWERSHELL_EXECUTABLE ?= powershell
SERVICE_IMAGE_TAG ?=
DESTROY_IMAGE_TAG ?= destroy-placeholder
TF_BACKEND_FILE ?= ../backend.hcl
TF_PLATFORM_STATE_KEY ?= platform/terraform.tfstate
TF_SERVICES_STATE_KEY ?= services/terraform.tfstate
TF_SERVICES_VAR_FILE ?= service-images.auto.tfvars.json
TF_SERVICES_VAR_FILE_HOST ?= ..\infra\services\service-images.auto.tfvars.json
TF_COMMON_VARS_CMD = set "TF_VAR_aws_region=$(AWS_REGION)" && set "TF_VAR_project_name=$(TF_PROJECT_NAME)" && set "TF_VAR_environment=$(TF_ENVIRONMENT)" && set "TF_VAR_terraform_state_bucket_name=$(TF_STATE_BUCKET)" && set "TF_VAR_foundation_state_key=$(TF_FOUNDATION_STATE_KEY)" && set "TF_VAR_foundation_state_region=$(TF_STATE_REGION)" && set "TF_VAR_platform_state_key=$(TF_PLATFORM_STATE_KEY)" && set "TF_VAR_state_region=$(TF_STATE_REGION)" && set "TF_VAR_powershell_executable=$(TF_POWERSHELL_EXECUTABLE)"

.PHONY: up down restart logs ps restart-grafana tf-platform-init tf-platform-plan tf-platform-apply tf-platform-destroy tf-services-init tf-services-plan tf-services-apply tf-services-destroy tf-services-write-vars tf-services-write-destroy-vars tf-up tf-down proto proto-clean test-wallet test-user test-fraud test-transaction test-gateway test-all test-clean-cache test-wallet-cover test-wallet-cover-html test-user-cover test-user-cover-html test-fraud-cover test-fraud-cover-html test-transaction-cover test-transaction-cover-html aws-restart-gateway aws-restart-user aws-restart-fraud aws-restart-wallet aws-restart-transaction aws-restart-internal aws-restart-all aws-wait-gateway aws-wait-internal aws-wait-all aws-rerun-migrator aws-health

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
	@echo "[platform] terraform init"
	@terraform -chdir=../infra/platform init -reconfigure -backend-config=$(TF_BACKEND_FILE) -backend-config="key=$(TF_PLATFORM_STATE_KEY)"

tf-platform-plan: tf-platform-init
	@echo "[platform] terraform plan"
	@$(TF_COMMON_VARS_CMD) && terraform -chdir=../infra/platform plan

tf-platform-apply: tf-platform-init
	@echo "[platform] terraform apply"
	@$(TF_COMMON_VARS_CMD) && terraform -chdir=../infra/platform apply -auto-approve

tf-platform-destroy: tf-platform-init
	@echo "[platform] terraform destroy"
	@$(TF_COMMON_VARS_CMD) && terraform -chdir=../infra/platform destroy -auto-approve

tf-services-init:
	@echo "[services] terraform init"
	@terraform -chdir=../infra/services init -reconfigure -backend-config=$(TF_BACKEND_FILE) -backend-config="key=$(TF_SERVICES_STATE_KEY)"

tf-services-write-vars:
	@powershell -NoProfile -ExecutionPolicy Bypass -File ".\scripts\write-service-images-tfvars.ps1" -OutputPath "$(TF_SERVICES_VAR_FILE_HOST)" -Region "$(AWS_REGION)" -AccountId "$(AWS_ACCOUNT_ID)" -Tag "$(SERVICE_IMAGE_TAG)"

tf-services-write-destroy-vars:
	@powershell -NoProfile -ExecutionPolicy Bypass -File ".\scripts\write-service-images-tfvars.ps1" -OutputPath "$(TF_SERVICES_VAR_FILE_HOST)" -Region "$(AWS_REGION)" -AccountId "$(AWS_ACCOUNT_ID)" -Tag "$(DESTROY_IMAGE_TAG)" -UseProvidedTag

tf-services-plan: tf-services-init tf-services-write-vars
	@echo "[services] terraform plan"
	@$(TF_COMMON_VARS_CMD) && terraform -chdir=../infra/services plan -var-file="$(TF_SERVICES_VAR_FILE)"

tf-services-apply: tf-services-init tf-services-write-vars
	@echo "[services] terraform apply"
	@$(TF_COMMON_VARS_CMD) && terraform -chdir=../infra/services apply -auto-approve -var-file="$(TF_SERVICES_VAR_FILE)"

tf-services-destroy: tf-services-init tf-services-write-destroy-vars
	@echo "[services] terraform destroy"
	@$(TF_COMMON_VARS_CMD) && terraform -chdir=../infra/services destroy -auto-approve -var-file="$(TF_SERVICES_VAR_FILE)"

# Levantar toda la infraestructura (services + platform)
tf-up: tf-services-apply tf-platform-apply
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
	@powershell -NoProfile -ExecutionPolicy Bypass -File ".\scripts\write-service-images-tfvars.ps1" -OutputPath "$(TF_SERVICES_VAR_FILE_HOST)" -Region "$(AWS_REGION)" -AccountId "$(AWS_ACCOUNT_ID)" -Tag "$(SERVICE_IMAGE_TAG)"
	@$(TF_COMMON_VARS_CMD) && terraform -chdir=../infra/services apply -replace=terraform_data.db_migrator_run -auto-approve -var-file="$(TF_SERVICES_VAR_FILE)"

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
