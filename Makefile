.PHONY: build up down restart logs test unit-test e2e-test lint clean health

PORT ?= 8080

build:
	docker compose build

up:
	PORT=$(PORT) docker compose up --build -d

down:
	docker compose down -v

restart:
	docker compose down -v
	PORT=$(PORT) docker compose up --build -d

logs:
	docker compose logs -f

test: unit-test

unit-test:
	go test -tags unit -race -v ./internal/...

e2e-test:
	PORT=$(PORT) go test -tags e2e -v ./e2e_tests/

lint:
	golangci-lint run ./...

health:
	@curl -sf http://localhost:$(PORT)/health && echo " OK" || echo " FAIL"

clean:
	docker compose down -v --rmi local
	rm -f coverage.out
