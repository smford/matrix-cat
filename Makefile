BINARY_NAME := matrix-cat
BIN_DIR := bin
VERSION ?= 1.0.0
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: all build run test test-race lint clean install

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/matrix-cat

run:
	go run ./cmd/matrix-cat

test:
	go test -v ./...

test-race:
	go test -v -race ./...

lint:
	go vet ./...

clean:
	rm -rf $(BIN_DIR) coverage.txt

install: build
	cp $(BIN_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
