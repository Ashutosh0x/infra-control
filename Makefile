VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

VERSION_PKG := github.com/ashutosh0x/infra-control/pkg/version
LDFLAGS := -w -s -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).Commit=$(COMMIT) -X $(VERSION_PKG).BuildDate=$(BUILD_DATE)

BIN_DIR := bin

.PHONY: all build test test-unit test-integration test-e2e bench lint generate proto migrate-up migrate-down docker-build clean help

all: build

build: generate
	@echo "==> Building binaries..."
	@mkdir -p $(BIN_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/infractl ./cmd/infractl
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/controller ./cmd/controller
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/worker ./cmd/worker
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/mcp-server ./cmd/mcp-server

test: test-unit test-integration

test-unit:
	@echo "==> Running unit tests..."
	go test -v -short ./...

test-integration:
	@echo "==> Running integration tests..."
	go test -v -run Integration ./...

test-e2e:
	@echo "==> Running e2e tests..."
	go test -v -tags=e2e ./...

bench:
	@echo "==> Running benchmarks..."
	go test -bench=. -benchmem ./...

lint:
	@echo "==> Running golangci-lint..."
	golangci-lint run ./...

generate:
	@echo "==> Running go generate..."
	go generate ./...

proto:
	@echo "==> Generating protobuf..."
	buf generate

migrate-up:
	@echo "==> Running migrations up..."

migrate-down:
	@echo "==> Running migrations down..."

docker-build:
	@echo "==> Building docker images..."
	docker build -t infra-control-controller:$(VERSION) -f deployments/docker/controller.Dockerfile .
	docker build -t infra-control-worker:$(VERSION) -f deployments/docker/worker.Dockerfile .

clean:
	@echo "==> Cleaning..."
	rm -rf $(BIN_DIR)

help:
	@echo "Available targets:"
	@echo "  build            - Build all binaries in cmd/"
	@echo "  test             - Run all tests"
	@echo "  test-unit        - Run unit tests only"
	@echo "  test-integration - Run integration tests"
	@echo "  test-e2e         - Run e2e tests"
	@echo "  bench            - Run benchmarks"
	@echo "  lint             - Run golangci-lint"
	@echo "  generate         - Run go generate"
	@echo "  proto            - Generate protobuf code"
	@echo "  migrate-up       - Database migrations up"
	@echo "  migrate-down     - Database migrations down"
	@echo "  docker-build     - Build docker images"
	@echo "  clean            - Clean build artifacts"
	@echo "  help             - Show this help message"
