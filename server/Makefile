GO ?= go
BINARY ?= bin/kirby
VERSION ?= dev
COMMIT ?= unknown
BUILD_DATE ?= unknown
LDFLAGS := -s -w \
	-X github.com/yvvlee/kirby/server/internal/version.Version=$(VERSION) \
	-X github.com/yvvlee/kirby/server/internal/version.Commit=$(COMMIT) \
	-X github.com/yvvlee/kirby/server/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: generate fmt lint test build clean

generate:
	$(GO) generate ./...

fmt:
	$(GO) fmt ./...

lint:
	@test -z "$$(gofmt -l .)"
	$(GO) vet ./...

test:
	$(GO) test ./...

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

clean:
	$(GO) clean -testcache
