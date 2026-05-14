.PHONY: build run-server run-worker dev dev-server dev-worker migrate-up migrate-down lint test tidy sqlc docs

build:
	go build -o bin/server ./cmd/server
	go build -o bin/worker ./cmd/worker
	go build -o bin/migrate ./cmd/migrate

run-server:
	go run ./cmd/server

run-worker:
	go run ./cmd/worker

dev-server:
	air -c .air.toml

dev-worker:
	air -c .air.worker.toml

dev:
	@echo "Starting server + worker with live reload (requires tmux or two terminals)"
	@echo "  Terminal 1: make dev-server"
	@echo "  Terminal 2: make dev-worker"

migrate-up:
	go run ./cmd/migrate -direction=up

migrate-down:
	go run ./cmd/migrate -direction=down

lint:
	golangci-lint run ./...

test:
	go test -race ./...

tidy:
	go mod tidy

sqlc:
	cd database && sqlc generate

docs:
	swag init -g cmd/server/main.go --output docs --parseDependency --parseInternal
