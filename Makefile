.PHONY: help infra-up infra-down test lint build run-gateway

help:
	@echo "Payment Processing Platform commands:"
	@echo "  make infra-up     - Start local Docker infrastructure (Postgres, Redis, Redpanda, Jaeger)"
	@echo "  make infra-down   - Stop local Docker infrastructure"
	@echo "  make test         - Run all unit and integration tests"
	@echo "  make lint         - Run golangci-lint"
	@echo "  make build        - Compile all microservices"
	@echo "  make run-gateway  - Run API Gateway locally"

infra-up:
	docker compose -f deployments/docker-compose.yml up -d

infra-down:
	docker compose -f deployments/docker-compose.yml down

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

build:
	go build -o bin/api-gateway ./cmd/api-gateway
	go build -o bin/payment-core ./cmd/payment-core
	go build -o bin/outbox-worker ./cmd/outbox-worker
	go build -o bin/ledger ./cmd/ledger

run-gateway:
	go run ./cmd/api-gateway
