SHELL := /bin/bash

APP_NAME := ipam
GO_FILES := $(shell find . -type f -name '*.go')
DB_DSN := postgres://ipam:ipam@localhost:5432/ipam?sslmode=disable
COMPOSE := podman compose -f dev/docker-compose.yaml
LOCAL_KEYCLOAK_URL := http://localhost:8080
LOCAL_KEYCLOAK_REALM := ipam
LOCAL_KEYCLOAK_CLIENT_ID := ipam-fe
LOCAL_KEYCLOAK_ISSUER := $(LOCAL_KEYCLOAK_URL)/realms/$(LOCAL_KEYCLOAK_REALM)
LOCAL_KEYCLOAK_AUDIENCE := ipam-api
LOCAL_KEYCLOAK_JWKS_URL := $(LOCAL_KEYCLOAK_ISSUER)/protocol/openid-connect/certs

.PHONY: docs format run run-api test-integration

## ------------------------------
## App commands
## ------------------------------

run:
	@echo "Starting backend (4040) and frontend (5173) with local Keycloak..."
	@cd frontend && test -d node_modules || npm install
	@bash -c 'trap "kill 0" EXIT; DB_CONN="$(DB_DSN)" PORT=4040 AUTH_ENABLED=true KEYCLOAK_ISSUER="$(LOCAL_KEYCLOAK_ISSUER)" KEYCLOAK_AUDIENCE="$(LOCAL_KEYCLOAK_AUDIENCE)" KEYCLOAK_JWKS_URL="$(LOCAL_KEYCLOAK_JWKS_URL)" go run ./cmd/api & VITE_KEYCLOAK_URL="$(LOCAL_KEYCLOAK_URL)" VITE_KEYCLOAK_REALM="$(LOCAL_KEYCLOAK_REALM)" VITE_KEYCLOAK_CLIENT_ID="$(LOCAL_KEYCLOAK_CLIENT_ID)" VITE_API_BASE="/api/v1" npm --prefix frontend run dev -- --host'

run-api:
	@echo "Running $(APP_NAME)..."
	DB_CONN="$(DB_DSN)" PORT=4040 AUTH_ENABLED=true KEYCLOAK_ISSUER="$(LOCAL_KEYCLOAK_ISSUER)" KEYCLOAK_AUDIENCE="$(LOCAL_KEYCLOAK_AUDIENCE)" KEYCLOAK_JWKS_URL="$(LOCAL_KEYCLOAK_JWKS_URL)" go run ./cmd/api

build:
	mkdir bin
	go build -o bin/$(APP_NAME) ./cmd/api

tidy:
	go mod tidy

sqlc:
	sqlc generate

docs:
	swag init -g main.go -d cmd/api,internal/http

format:
	gofmt -w $(GO_FILES)

test-integration:
	go test -tags=integration ./integration/api/...

docker-api:
	docker build -f deploy/docker/Dockerfile.api -t $(APP_NAME)-api:latest .

docker-fe:
	docker build -f deploy/docker/Dockerfile.fe -t $(APP_NAME)-fe:latest .

docker-migrate:
	docker build -f deploy/docker/Dockerfile.migrate -t $(APP_NAME)-migrate:latest .

docker-all: docker-api docker-fe docker-migrate

## ------------------------------
## Dev stack (docker-compose)
## ------------------------------

dev-up:
	@echo "Starting dev stack (db + keycloak)..."
	$(COMPOSE) up -d

dev-down:
	@echo "Stopping dev stack..."
	$(COMPOSE) down

dev-restart:
	$(MAKE) dev-down
	$(MAKE) dev-up

dev-logs:
	$(COMPOSE) logs -f

dev-ps:
	$(COMPOSE) ps

## ------------------------------
## Migrations (goose)
## ------------------------------

db-migrate: install-tools
	@echo "Running migrations..."
	goose -dir db/migrations postgres "$(DB_DSN)" up

db-rollback:
	@echo "Rolling back latest migration..."
	goose -dir db/migrations postgres "$(DB_DSN)" down

db-new:
ifndef name
	$(error Usage: make db-new name=<migration_name>)
endif
	goose -dir db/migrations create $(name) sql

db-status:
	goose -dir db/migrations postgres "$(DB_DSN)" status

## ------------------------------
## Tools (sqlc + goose + swag)
## ------------------------------

install-tools: install-sqlc install-goose install-swagger

install-sqlc:
	@if ! command -v sqlc >/dev/null 2>&1; then \
		echo "Installing sqlc..."; \
		go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest; \
	else \
		echo "sqlc already installed"; \
	fi

install-goose:
	@if ! command -v goose >/dev/null 2>&1; then \
		echo "Installing goose..."; \
		go install github.com/pressly/goose/v3/cmd/goose@latest; \
	else \
		echo "goose already installed"; \
	fi

install-swagger:
	@if ! command -v swag >/dev/null 2>&1; then \
		echo "Installing swag..."; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
	else \
		echo "swag already installed"; \
	fi
