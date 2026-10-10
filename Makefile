# Defaults for running binaries on the host against docker-compose services.
export AWS_ENDPOINT_URL      ?= http://localhost:4566
export AWS_REGION            ?= eu-central-1
export AWS_ACCESS_KEY_ID     ?= test
export AWS_SECRET_ACCESS_KEY ?= test

.PHONY: up down app gen lint test migrate api consumer

up: ## Start MySQL and LocalStack
	docker compose up -d --wait

down:
	docker compose --profile app down -v

app: ## Start everything, including api and consumer, in containers
	docker compose --profile app up --build

gen: ## Regenerate sqlc code
	sqlc generate

lint:
	golangci-lint run

test: ## Unit + integration tests (needs Docker)
	go test -race ./...

migrate:
	go run ./cmd/migrate

api: migrate
	go run ./cmd/api

consumer:
	go run ./cmd/consumer
