.PHONY: build run test lint fmt vet tidy install-tools hooks help

BIN := event-spec
PORT ?= 8080
DB_DSN ?= file:./registry.db

## build: compile the CLI binary
build:
	go build -o $(BIN) ./cmd/event-spec

## run: build and start the registry server (PORT and DB_DSN env vars override defaults)
run: build
	./$(BIN) serve --port $(PORT) --db $(DB_DSN)

## test: run all tests
test:
	go test ./...

## test-cover: run tests and open coverage report
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

## lint: run golangci-lint
lint:
	golangci-lint run --timeout=5m

## fmt: format all Go source files
fmt:
	gofmt -w .

## vet: run go vet
vet:
	go vet ./...

## tidy: tidy and verify go modules
tidy:
	go mod tidy
	go mod verify

## install-tools: install dev tools (golangci-lint, lefthook) and TypeScript dependencies
install-tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install github.com/evilmartians/lefthook@latest
	@command -v pnpm >/dev/null 2>&1 || npm install -g pnpm
	cd sdk/typescript && pnpm install

## hooks: install git hooks via lefthook (covers Go and TypeScript)
hooks:
	lefthook install

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## //'