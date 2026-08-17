# Kirby API contracts

This directory is the source of truth for Kirby's public protobuf contracts.

- `kirby/admin/v1` defines management HTTP APIs. These files deliberately do
  not produce gRPC stubs.
- `kirby/runtime/v1` defines the API-key protected runtime HTTP and gRPC API.
- `kirby/common/v1` contains shared public messages and annotations.
- `kirby/errors/v1` contains the small, project-local error catalogue.

Run `make -C server generate` from the repository root. Generated Go files are
written to `server/gen` and must be committed.
