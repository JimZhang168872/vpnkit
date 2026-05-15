.PHONY: build test lint install clean

BIN_DIR := $(HOME)/.local/bin

build:
	go build -trimpath -ldflags "-s -w" -o ./bin/vpnkit ./cmd/vpnkit

install: build
	mkdir -p $(BIN_DIR)
	install -m 0755 ./bin/vpnkit $(BIN_DIR)/vpnkit

test:
	go test -race -cover ./...

lint:
	golangci-lint run

clean:
	rm -rf ./bin
