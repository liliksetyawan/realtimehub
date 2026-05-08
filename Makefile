.PHONY: help up down logs psql redis-cli tidy build test run dev clean tune-kernel loadtest

SHELL := /bin/bash

## help: list common targets
help:
	@grep -E '^##' Makefile | sed -e 's/## //'

## up: start postgres, redis, jaeger
up:
	docker compose up -d
	@docker compose ps

## down: stop infra
down:
	docker compose down

## logs: tail infra logs
logs:
	docker compose logs -f

## psql: psql shell into realtimehub db
psql:
	docker compose exec postgres psql -U realtimehub -d realtimehub

## redis-cli: open redis-cli
redis-cli:
	docker compose exec redis redis-cli

## tidy: go mod tidy
tidy:
	go mod tidy

## build: build the server binary into bin/server
build:
	@mkdir -p bin
	go build -o bin/server ./cmd/server

## test: run all tests (with race detector)
test:
	go test -race ./...

## run: run the server locally
run:
	go run ./cmd/server

## dev: run server with reload (requires `air`)
dev:
	@command -v air >/dev/null || (echo "install air: go install github.com/air-verse/air@latest" && exit 1)
	air

## tune-kernel: apply sysctl + ulimit tweaks for high-conn benchmarks (Linux only, sudo)
tune-kernel:
	sudo bash scripts/tune-kernel.sh

## loadtest: run k6 ramp to 50k connections (requires k6)
loadtest:
	k6 run loadtest/ramp-50k.js

## clean: remove build artifacts + coverage
clean:
	rm -rf bin/ coverage.out coverage.html
