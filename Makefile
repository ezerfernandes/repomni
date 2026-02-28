BINARY_NAME := repoinjector
BUILD_DIR := bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X github.com/ezerfernandes/repoinjector/internal/cmd.version=$(VERSION)"

PREFIX ?= /usr/local
COVERAGE_THRESHOLD ?= 70

.PHONY: help build run test test_coverage test_coverage_html test_coverage_report clean install uninstall
.PHONY: lint format format_check vet check_all

help:
	@echo "=== Build Commands ==="
	@echo "  make build              - Build the binary"
	@echo "  make run                - Build and run"
	@echo "  make clean              - Remove build artifacts"
	@echo "  make install            - Install to $(PREFIX)/bin"
	@echo "  make uninstall          - Remove from $(PREFIX)/bin"
	@echo ""
	@echo "=== Development Commands ==="
	@echo "  make lint               - Run golangci-lint"
	@echo "  make vet                - Run go vet"
	@echo "  make format             - Format code with gofmt"
	@echo "  make format_check       - Check formatting (no changes)"
	@echo ""
	@echo "=== Testing ==="
	@echo "  make test               - Run tests"
	@echo "  make test_coverage      - Run tests with coverage ($(COVERAGE_THRESHOLD)% threshold)"
	@echo "  make test_coverage_html - Run tests and open HTML coverage report"
	@echo "  make test_coverage_report - Show per-function coverage breakdown"
	@echo ""
	@echo "=== Unified Checks ==="
	@echo "  make check_all          - Run all checks (format, vet, lint, test)"

# Build
build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/repoinjector

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html

# Install
install: build
	install -d $(PREFIX)/bin
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(PREFIX)/bin/$(BINARY_NAME)

uninstall:
	rm -f $(PREFIX)/bin/$(BINARY_NAME)

# Linting and formatting
lint:
	golangci-lint run ./...

vet:
	go vet ./...

format:
	gofmt -w .

format_check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)

# Testing
test:
	go test ./...

test_coverage:
	go test -coverprofile=coverage.out -covermode=atomic -count=1 ./...
	@go tool cover -func=coverage.out | tail -1
	@total=$$(go tool cover -func=coverage.out | tail -1 | awk '{print $$NF}' | tr -d '%'); \
	if [ $$(echo "$$total < $(COVERAGE_THRESHOLD)" | bc -l) -eq 1 ]; then \
		echo "Coverage $$total% is below threshold $(COVERAGE_THRESHOLD)%"; exit 1; \
	else \
		echo "Coverage $$total% meets threshold $(COVERAGE_THRESHOLD)%"; \
	fi

test_coverage_html: test_coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
	@command -v open >/dev/null 2>&1 && open coverage.html || true

test_coverage_report: test_coverage
	@echo ""
	@echo "=== Per-function coverage ==="
	@go tool cover -func=coverage.out

# Unified checks
check_all:
	@echo "=== Running all checks ==="
	@echo ">> Checking format..."
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)
	@echo ">> Running go vet..."
	@go vet ./...
	@echo ">> Running linter..."
	@golangci-lint run ./...
	@echo ">> Running tests..."
	@go test ./... -count=1
	@echo "=== All checks passed ==="
