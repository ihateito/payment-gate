ifneq (,$(wildcard .env))
    include .env
    export
endif

.PHONY: help infra-up infra-down docker-up docker-down docker-full docker-logs test lint build run-gateway run-outbox run-ui

help:
	@echo "Payment Processing Platform commands:"
	@echo "  make docker-up    - Build and start all services in Docker (Postgres, Redis, Gateway, Worker)"
	@echo "  make docker-down  - Stop all Docker containers"
	@echo "  make docker-full  - Start with full telemetry & Kafka (Redpanda, Jaeger, Prometheus)"
	@echo "  make docker-logs  - View real-time container logs"
	@echo "  make infra-up     - Start only databases (Postgres, Redis) for local Go development"
	@echo "  make infra-down   - Stop local databases"
	@echo "  make run-gateway  - Run API Gateway locally on host (serves UI on :8080)"
	@echo "  make run-outbox   - Run Transactional Outbox Worker locally on host"
	@echo "  make run-ui       - Run standalone Web UI dev server on http://localhost:3000"
	@echo "  make build        - Compile all microservices"
	@echo "  make test         - Run all unit and integration tests"
	@echo "  make lint         - Run golangci-lint"

docker-up:
	docker compose -f deployments/docker-compose.yml up -d --build

docker-down:
	docker compose -f deployments/docker-compose.yml down

docker-full:
	docker compose -f deployments/docker-compose.yml --profile full up -d --build

docker-logs:
	docker compose -f deployments/docker-compose.yml logs -f

infra-up:
	docker compose -f deployments/docker-compose.yml up -d postgres redis

infra-down:
	docker compose -f deployments/docker-compose.yml down

test:
	go test -v ./...

lint:
	golangci-lint run ./...

build:
	go build -o bin/api-gateway ./cmd/api-gateway
	go build -o bin/payment-core ./cmd/payment-core
	go build -o bin/outbox-worker ./cmd/outbox-worker
	go build -o bin/ledger ./cmd/ledger

run-gateway:
	go run ./cmd/api-gateway

run-outbox:
	go run ./cmd/outbox-worker

run-ui:
	@echo "Starting PaymentGate UI on http://localhost:3000..."
	python3 -m http.server 3000 --directory web
