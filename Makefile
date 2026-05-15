.PHONY: build test lint fmt clean help

BIN := bin/vpnkit
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags="-X main.Version=$(VERSION)"

help:
	@echo "vpnkit Makefile targets:"
	@echo "  build        - Build vpnkit binary"
	@echo "  test         - Run tests with race detection"
	@echo "  test-cov     - Run tests with coverage report"
	@echo "  lint         - Run linters (golangci-lint)"
	@echo "  fmt          - Format code (gofmt + goimports)"
	@echo "  clean        - Remove build artifacts"

build: clean
	@mkdir -p bin
	~/.local/go/bin/go build $(LDFLAGS) -o $(BIN) ./cmd/vpnkit

test:
	~/.local/go/bin/go test -race -v ./...

test-cov:
	~/.local/go/bin/go test -race -v -coverprofile=coverage.out ./...
	~/.local/go/bin/go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run ./...

fmt:
	~/.local/go/bin/go fmt ./...
	goimports -w .

clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	~/.local/go/bin/go clean

install: build
	cp $(BIN) $(GOPATH)/bin/vpnkit || cp $(BIN) ~/.local/bin/vpnkit
