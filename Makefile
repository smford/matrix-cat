BINARY_NAME := mcat
BIN_DIR := bin
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "1.0.0")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: all build run test test-race lint clean install update-docs

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/mcat

run:
	go run ./cmd/mcat

test:
	go test -v ./...

test-race:
	go test -v -race ./...

lint: shellcheck
	go vet ./...

shellcheck:
	@which shellcheck > /dev/null 2>&1 || (echo "Error: shellcheck not found. Install with 'brew install shellcheck'" && exit 1)
	shellcheck scripts/*.sh

clean:
	rm -rf $(BIN_DIR) coverage.txt

install: build
	cp $(BIN_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)

update-docs:
	@chmod +x scripts/update-docs-version.sh
	@./scripts/update-docs-version.sh $(VERSION)
