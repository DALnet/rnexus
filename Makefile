# rnexus Makefile

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE) -X main.gitCommit=$(GIT_COMMIT)"

.PHONY: all build clean install test run

all: build

build:
	go build $(LDFLAGS) -o rnexus ./cmd/rnexus

install:
	go install $(LDFLAGS) ./cmd/rnexus

clean:
	rm -f rnexus pid.txt

test:
	go test ./...

run: build
	./rnexus -x

# Build for release (multiple platforms)
.PHONY: release
release:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o rnexus-linux-amd64 ./cmd/rnexus
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o rnexus-linux-arm64 ./cmd/rnexus
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o rnexus-darwin-amd64 ./cmd/rnexus
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o rnexus-darwin-arm64 ./cmd/rnexus

# Development: run with verbose logging
.PHONY: dev
dev: build
	./rnexus -x 2>&1 | tee rnexus.log

# Tidy dependencies
.PHONY: tidy
tidy:
	go mod tidy

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	golangci-lint run ./...
