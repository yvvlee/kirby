# Kirby API contracts

This directory is the source of truth for Kirby's public protobuf contracts.

- `admin` defines management HTTP APIs. These files deliberately do
  not produce gRPC stubs.
- `runtime` defines the API-key protected runtime HTTP and gRPC API.
- `common` contains shared public messages and annotations.
- `errors` contains the small, project-local error catalogue.

Run `make -C server generate` from the repository root. Generated Go files are
written next to their source files in `server/api` and must be committed.

The filesystem paths intentionally omit the product and version segments. The
protobuf package names remain versioned (`kirby.*.v1`) to preserve the existing
wire and generated type contracts.
