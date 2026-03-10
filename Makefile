.PHONY: build run test lint migrate docker-up docker-down

BINARY=bin/hookrelay
CONFIG=config/config.yaml

build:
	go build -o $(BINARY) ./cmd/hookrelay

run: build
	$(BINARY) --config $(CONFIG)

test:
	go test ./... -v -count=1

lint:
	golangci-lint run ./...

migrate:
	psql "$(DATABASE_URL)" -f migrations/001_init.sql

docker-up:
	docker-compose -f deploy/docker-compose.yml up -d

docker-down:
	docker-compose -f deploy/docker-compose.yml down
