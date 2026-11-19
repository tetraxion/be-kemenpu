-include .env
export

SWAG := go run github.com/swaggo/swag/cmd/swag@v1.16.6

.PHONY: help db-up up down logs migrate-up migrate-down seed run swag fmt vet test test-race test-integration cover build docker-build

help: ## Show the available commands
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(firstword $(MAKEFILE_LIST)) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

# migrate-up, migrate-down and seed run inside the backend image, on the
# compose network. The database port is not published to the host, so they
# work without a local PostgreSQL or Go toolchain.
TOOL := docker compose run --rm --build --no-deps --entrypoint

db-up: ## Start PostgreSQL/PostGIS and wait until it is healthy
	docker compose up -d --wait db

up: ## Build the image and run the whole stack in containers (http://localhost:8080/api)
	docker compose up -d --build --wait

down: ## Stop and remove the containers
	docker compose down

logs: ## Follow the backend log
	docker compose logs -f backend

migrate-up: db-up ## Apply every pending migration
	$(TOOL) /app/migrate backend

migrate-down: db-up ## Roll back the last migration (this drops the imported data)
	$(TOOL) /app/migrate backend -down 1

seed: db-up ## Fill the database from the data folder; safe to repeat. Option: make seed ARGS="-follow-up"
	$(TOOL) /app/seed backend $(ARGS)

run: ## Run the API server on the host (needs PostgreSQL/PostGIS from DATABASE_URL)
	go run ./cmd/server

swag: ## Generate the OpenAPI documentation into docs/
	$(SWAG) fmt
	$(SWAG) init -g cmd/server/main.go -o docs --parseInternal

fmt: ## Format the code
	gofmt -w cmd internal migrations

vet: ## Static checks
	go vet ./...

test: ## Unit tests (no database needed)
	go test -count=1 ./...

test-race: ## Unit tests under the race detector (needs cgo; use a golang container on Windows)
	go test -race -count=1 ./...

test-integration: ## Integration tests against PostGIS; needs TEST_DATABASE_URL and rewrites that database
	go test -tags integration -p 1 -count=1 ./...

cover: ## Unit tests with a coverage report
	go test -count=1 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

build: ## Build the binaries into bin/
	CGO_ENABLED=0 go build -trimpath -o bin/ ./cmd/...

docker-build: ## Build the container image
	docker build -t irrigation-server .
