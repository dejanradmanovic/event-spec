.PHONY: build test lint fmt vet tidy install-tools hooks help

BIN := event-spec

## build: compile the CLI binary
build:
	go build -o $(BIN) ./cmd/event-spec

## test: run tests with race detector
test:
	go test -race ./...

## test-cover: run tests and open coverage report
test-cover:
	go test -race -coverprofile=coverage.out ./...
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

## install-tools: install dev tools (golangci-lint, lefthook)
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/evilmartians/lefthook@latest

## hooks: install git hooks via lefthook
hooks:
	@lefthook install 2>/dev/null || $(GOPATH)/bin/lefthook install

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## //'