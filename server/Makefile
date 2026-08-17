GO ?= go
BINARY ?= bin/kirby
TOOLS_BIN := $(CURDIR)/bin/tools
PROTOC_VERSION := 35.1
PROTOC_GEN_GO_VERSION := v1.36.12
PROTOC_GEN_GO_GRPC_VERSION := v1.6.2
PROTOC_GEN_VALIDATE_VERSION := v1.3.3
# The HTTP generator is a nested module. This pseudo-version is the commit
# tagged v2.9.2 in the main Kratos repository.
PROTOC_GEN_GO_HTTP_VERSION := v2.0.0-20251205160234-b9fab9a5a5ab

PGV_DIR := $(shell $(GO) list -m -f '{{.Dir}}' github.com/envoyproxy/protoc-gen-validate)
KRATOS_DIR := $(shell $(GO) list -m -f '{{.Dir}}' github.com/go-kratos/kratos/v2)
PROTO_INCLUDES := -I../api -I$(PGV_DIR) -I$(KRATOS_DIR)/third_party -Itools/third_party/protoc-gen-go-errors
MANAGEMENT_PROTO_FILES := $(shell find ../api/kirby/admin ../api/kirby/common ../api/kirby/errors -name '*.proto' -type f | LC_ALL=C sort)
RUNTIME_PROTO_FILES := $(shell find ../api/kirby/runtime -name '*.proto' -type f | LC_ALL=C sort)
ERROR_PROTO_FILE := ../api/kirby/errors/v1/errors.proto
VERSION ?= dev
COMMIT ?= unknown
BUILD_DATE ?= unknown
LDFLAGS := -s -w \
	-X github.com/yvvlee/kirby/server/internal/version.Version=$(VERSION) \
	-X github.com/yvvlee/kirby/server/internal/version.Commit=$(COMMIT) \
	-X github.com/yvvlee/kirby/server/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: tools generate fmt lint test build clean

tools:
	@test "$$(protoc --version)" = "libprotoc $(PROTOC_VERSION)" || \
		(echo "protoc $(PROTOC_VERSION) is required" >&2; exit 1)
	@mkdir -p $(TOOLS_BIN)
	GOBIN=$(TOOLS_BIN) $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(TOOLS_BIN) $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	GOBIN=$(TOOLS_BIN) $(GO) install github.com/envoyproxy/protoc-gen-validate@$(PROTOC_GEN_VALIDATE_VERSION)
	GOBIN=$(TOOLS_BIN) $(GO) install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@$(PROTOC_GEN_GO_HTTP_VERSION)
	cd tools/third_party/protoc-gen-go-errors && $(GO) build -trimpath -o $(TOOLS_BIN)/protoc-gen-go-errors .
	cd tools/third_party/protoc-gen-go-json && $(GO) build -trimpath -o $(TOOLS_BIN)/protoc-gen-go-json .

generate: tools
	@rm -rf gen/kirby
	@mkdir -p gen
	PATH="$(TOOLS_BIN):$$PATH" protoc $(PROTO_INCLUDES) \
		--go_out=paths=source_relative:gen \
		--go-http_out=paths=source_relative:gen \
		--validate_out=paths=source_relative,lang=go:gen \
		--go-json_out=paths=source_relative:gen \
		$(MANAGEMENT_PROTO_FILES)
	PATH="$(TOOLS_BIN):$$PATH" protoc $(PROTO_INCLUDES) \
		--go_out=paths=source_relative:gen \
		--go-http_out=paths=source_relative:gen \
		--go-grpc_out=paths=source_relative:gen \
		--validate_out=paths=source_relative,lang=go:gen \
		--go-json_out=paths=source_relative:gen \
		$(RUNTIME_PROTO_FILES)
	PATH="$(TOOLS_BIN):$$PATH" protoc $(PROTO_INCLUDES) \
		--go-errors_out=paths=source_relative:gen \
		$(ERROR_PROTO_FILE)
	$(GO) fmt ./gen/...

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
