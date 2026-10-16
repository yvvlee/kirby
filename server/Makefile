GO ?= go
BINARY ?= bin/kirby
TOOLS_BIN := $(CURDIR)/bin/tools
GOLANGCI_LINT_VERSION := v2.14.0
GOLANGCI_LINT ?= $(TOOLS_BIN)/golangci-lint
PROTOC_VERSION := 35.1
PROTOC_GEN_GO_VERSION := v1.36.12
PROTOC_GEN_GO_GRPC_VERSION := v1.6.2
PROTOC_GEN_VALIDATE_VERSION := v1.3.3
KRATOS_VERSION := v2.9.2
# The HTTP and errors generators are nested modules. These versions use the commit
# tagged v2.9.2 in the main Kratos repository.
PROTOC_GEN_GO_HTTP_VERSION := v2.0.0-20251205160234-b9fab9a5a5ab
PROTOC_GEN_GO_ERRORS_VERSION := v2.0.0-20251205160234-b9fab9a5a5ab

PGV_DIR := $(shell $(GO) mod download -json github.com/envoyproxy/protoc-gen-validate@$(PROTOC_GEN_VALIDATE_VERSION) | sed -n 's/^[[:space:]]*"Dir": "\([^"]*\)",*$$/\1/p')
KRATOS_DIR := $(shell $(GO) mod download -json github.com/go-kratos/kratos/v2@$(KRATOS_VERSION) | sed -n 's/^[[:space:]]*"Dir": "\([^"]*\)",*$$/\1/p')
PROTO_INCLUDES := -Iapi -I$(PGV_DIR) -I$(KRATOS_DIR)/third_party
MANAGEMENT_PROTO_FILES := $(shell find api/admin api/common -name '*.proto' -type f | LC_ALL=C sort)
RUNTIME_PROTO_FILES := $(shell find api/runtime -name '*.proto' -type f | LC_ALL=C sort)
ERROR_PROTO_DIR := api/errors
ERROR_PROTO_FILE := errors.proto
VERSION ?= dev
COMMIT ?= unknown
BUILD_DATE ?= unknown
LDFLAGS := -s -w \
	-X github.com/yvvlee/kirby/server/internal/version.Version=$(VERSION) \
	-X github.com/yvvlee/kirby/server/internal/version.Commit=$(COMMIT) \
	-X github.com/yvvlee/kirby/server/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: tools generate generate-models generate-wire fmt lint lint-tools test build clean

tools:
	@test "$$(protoc --version)" = "libprotoc $(PROTOC_VERSION)" || \
		(echo "protoc $(PROTOC_VERSION) is required" >&2; exit 1)
	@test -n "$(PGV_DIR)" -a -d "$(PGV_DIR)" || \
		(echo "cannot resolve protoc-gen-validate module directory" >&2; exit 1)
	@test -n "$(KRATOS_DIR)" -a -d "$(KRATOS_DIR)/third_party" || \
		(echo "cannot resolve Kratos protobuf include directory" >&2; exit 1)
	@mkdir -p $(TOOLS_BIN)
	GOBIN=$(TOOLS_BIN) $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(TOOLS_BIN) $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	GOBIN=$(TOOLS_BIN) $(GO) install github.com/envoyproxy/protoc-gen-validate@$(PROTOC_GEN_VALIDATE_VERSION)
	GOBIN=$(TOOLS_BIN) $(GO) install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@$(PROTOC_GEN_GO_HTTP_VERSION)
	GOBIN=$(TOOLS_BIN) $(GO) install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v2@$(PROTOC_GEN_GO_ERRORS_VERSION)

generate: tools
	@find api -type f \( -name '*.pb.go' -o -name '*_http.pb.go' -o -name '*_grpc.pb.go' -o -name '*.pb.validate.go' -o -name '*.pb.json.go' \) -delete
	PATH="$(TOOLS_BIN):$$PATH" protoc --fatal_warnings $(PROTO_INCLUDES) \
		--go_out=paths=source_relative:api \
		--go-http_out=paths=source_relative:api \
		--validate_out=paths=source_relative,lang=go:api \
		$(MANAGEMENT_PROTO_FILES)
	PATH="$(TOOLS_BIN):$$PATH" protoc --fatal_warnings $(PROTO_INCLUDES) \
		--go_out=paths=source_relative:api \
		--go-http_out=paths=source_relative:api \
		--go-grpc_out=paths=source_relative:api \
		--validate_out=paths=source_relative,lang=go:api \
		$(RUNTIME_PROTO_FILES)
	PATH="$(TOOLS_BIN):$$PATH" protoc --fatal_warnings \
		-I$(ERROR_PROTO_DIR) -I$(PGV_DIR) -I$(KRATOS_DIR)/third_party \
		--go_out=paths=source_relative:$(ERROR_PROTO_DIR) \
		--validate_out=paths=source_relative,lang=go:$(ERROR_PROTO_DIR) \
		--go-errors_out=paths=source_relative:$(ERROR_PROTO_DIR) \
		$(ERROR_PROTO_FILE)
	$(GO) fmt ./api/...
	$(MAKE) generate-models generate-wire

generate-models:
	cd internal/model && $(GO) tool lormgen .

generate-wire:
	$(GO) tool wire ./cmd/kirby

fmt:
	$(GO) fmt ./...

lint-tools: $(TOOLS_BIN)/golangci-lint

$(TOOLS_BIN)/golangci-lint: Makefile
	@mkdir -p $(TOOLS_BIN)
	GOBIN=$(TOOLS_BIN) $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint: lint-tools
	@test -z "$$(gofmt -l .)"
	$(GOLANGCI_LINT) run --config .golangci.yml ./...
	$(GO) run ./tools/stylecheck
	$(GO) vet ./...

test:
	$(GO) test ./...

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

clean:
	$(GO) clean -testcache
