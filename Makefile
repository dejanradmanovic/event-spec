.PHONY: build run test lint fmt vet tidy install-tools hooks help test-kotlin lint-kotlin fmt-kotlin test-swift lint-swift fmt-swift

BIN := event-spec
PORT ?= 8080
DB_DSN ?= file:./registry.db

## build: compile the CLI binary
build:
	go build -o $(BIN) ./cmd/event-spec

## run: build and start the registry server (PORT and DB_DSN env vars override defaults)
run: build
	./$(BIN) serve --port $(PORT) --db $(DB_DSN)

## test: run all tests (Go + Kotlin)
test:
	go test ./...
	$(MAKE) test-kotlin

## test-cover: run tests and open coverage report
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

## test-kotlin: run Kotlin SDK tests
test-kotlin:
	./sdk/kotlin/gradlew -p sdk/kotlin test

## test-swift: run Swift SDK tests (skipped when no Swift toolchain is present)
test-swift:
	@if ! command -v swift >/dev/null 2>&1; then \
		echo "swift not found - skipping Swift tests"; \
	elif [ ! -f sdk/swift/Package.swift ]; then \
		echo "sdk/swift/Package.swift not found - skipping Swift tests"; \
	else \
		swift test --package-path sdk/swift; \
	fi

## lint: run all linters (Go + Kotlin)
lint:
	golangci-lint run --timeout=5m
	$(MAKE) lint-kotlin

## lint-kotlin: check Kotlin source formatting
lint-kotlin:
	./sdk/kotlin/gradlew -p sdk/kotlin ktfmtCheck

## lint-swift: check Swift source formatting (skipped when no Swift toolchain is present)
lint-swift:
	@if ! command -v swift-format >/dev/null 2>&1; then \
		echo "swift-format not found - skipping Swift lint"; \
	elif [ ! -f sdk/swift/Package.swift ]; then \
		echo "sdk/swift/Package.swift not found - skipping Swift lint"; \
	else \
		swift-format lint --recursive --strict sdk/swift; \
	fi

## fmt: format all Go source files
fmt:
	gofmt -w .

## fmt-kotlin: format Kotlin source files
fmt-kotlin:
	./sdk/kotlin/gradlew -p sdk/kotlin ktfmtFormat

## fmt-swift: format Swift source files (skipped when no Swift toolchain is present)
fmt-swift:
	@if ! command -v swift-format >/dev/null 2>&1; then \
		echo "swift-format not found - skipping Swift format"; \
	elif [ ! -f sdk/swift/Package.swift ]; then \
		echo "sdk/swift/Package.swift not found - skipping Swift format"; \
	else \
		swift-format format --in-place --recursive sdk/swift; \
	fi

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