# Contributing

## Scope

Kirby is a standalone public repository. Changes must not depend on the former
private source trees, private domains, organization networks, private
registries, or unpublished credentials. Read `docs/source-provenance.md` before
moving code across the extraction boundary.

## Development flow

1. Make a focused change with tests proportional to its risk.
2. Update protobuf sources before generated code.
3. Run `make ci` from the repository root.
4. Run `scripts/test-local-dev.sh` for changes to user workflows, HTTP routing,
   storage, authentication, authorization, or database behavior.
5. Update documentation, dependency notices, and SBOMs when their inputs
   change.

Do not hide a failed check with an ignore rule or broad allowlist. Fix the
failure or document a narrow, reviewed exception where the check supports one.

## Generated code

Install protoc 35.1, then run:

```sh
make -C server generate
scripts/ci/check-generated.sh
```

Commit the API source and generated output together. Never edit files under
`server/gen` by hand.

## Tests

Go changes should include package tests and pass `go test -race ./...`.
Frontend changes should include Vitest coverage and pass lint, unit tests, and
the production build. Database changes must update `deploy/schema.sql` and its
static validation. Security-boundary changes need negative tests for foreign
environment and resource IDs.

## Dependencies and licenses

Use public dependencies with a clear compatible license. Commit lock and
checksum files. Run:

```sh
scripts/license-check.sh
scripts/security-check.sh
scripts/generate-sbom.sh
```

GPL, AGPL, SSPL, and unknown-license dependencies are rejected by the current
distribution policy.

## Commits

Keep generated output, product changes, and documentation reviewable. Use an
imperative subject with a conventional area when useful, for example
`fix(runtime): reject cross-project keys`. Do not import history from the source
repositories or commit credentials, local configuration, build artifacts, or
test databases.
