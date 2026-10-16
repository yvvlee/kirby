# Development

## Toolchain

CI uses Go 1.27.2, Node.js 24.19.0, npm's committed lock file, and protoc
35.1. Generator versions are fixed in `server/Makefile`.

Install dependencies from the repository root:

```sh
make dependencies
make -C server lint-tools
```

Run the backend and frontend locally as described in the root README. Local
browser testing uses one backend process, the existing MySQL and Redis
containers, and Vite through `npm run dev`.

## Common checks

```sh
make lint
make test-race
make build
make schema-check
make license-check
make security-check
make ci
```

`make ci` is the local core gate. The single-server browser integration is:

```sh
scripts/test-local-dev.sh
```

The script expects running containers named `mysql` and `redis`. Override them
with `KIRBY_TEST_MYSQL_CONTAINER` and `KIRBY_TEST_REDIS_CONTAINER`.

The CI browser regression, `scripts/ci/integration.sh`, requires a disposable,
empty MySQL database plus Redis and prepared S3 storage. It starts the backend
without an administrator, runs the `/setup` browser tests to create
`setup-owner`, then creates the `admin` automation account with the CLI and runs
the existing user journey. Do not point this script at an existing deployment;
its setup tests require an uninitialized database.

## Backend dependency injection

After changing constructors or a layer's `ProviderSet`, regenerate the command's
Wire initializer and run the backend checks:

```sh
cd server
make generate-wire
go test ./...
make lint
```

Business services and logic live directly in `internal/service` and
`internal/logic`. Name files by business area and keep transport setup in
`internal/server`, persistence resources in `internal/infra`, and reusable
helpers in `internal/util`.

## Database models and backend checks

The database layer uses `github.com/yvvlee/lorm v1.1.5`. Models declare `lorm`
column tags; repositories embed `*lorm.Repository[*model.X]` and receive the
shared `infra.Storage.CoreDB` engine. Regenerate column metadata after changing
models:

```sh
make -C server generate-models
```

Queries must explicitly apply project/environment scope and `IS NULL` for soft
deletes. Mutable updates include the expected version and advance it once.
Business transactions use the same lorm engine and pass the transaction context
to every repository operation. Test transaction substitutes live only in tests.
Schema initialization still uses the embedded MySQL/PostgreSQL DDL under a
connection-scoped database lock; SQL argument logging is disabled because
credentials and installation secrets are stored as parameters.

`make -C server lint-tools` installs public golangci-lint v2.14.0.
`make -C server lint` runs that version with `.golangci.yml`, then the local
`tools/stylecheck` and `go vet`. The local checker covers argument count,
structured log pairs, explicit constants, two-level nesting, field layout,
interface parameter names, and exported function contracts. Constructors may
list their injected dependencies. Generated and test code are excluded from
production style rules; both remain included in compilation and tests.

## Protobuf generation

Files under `server/api` are the source of truth. Generated Go files are
committed next to their `.proto` files.

```sh
make -C server generate
scripts/ci/check-generated.sh
```

Do not edit generated files. HTTP and error helpers use the official Kratos
generators pinned in `server/Makefile`. HTTP JSON options are configured through
Kratos in `server/internal/server/codec.go`; no JSON methods are generated.

## Frontend

The administration application uses React 19, TypeScript 6, Ant Design 5,
TanStack Query, and Formily React. Run:

```sh
cd web
npm run lint
npm run typecheck
npm run test -- --run
npm run build
npm run test:e2e
```

Vite proxies `/api` to `KIRBY_DEV_API_TARGET`. It also proxies local signed
asset paths, so the browser workflow works with the development object store.

## Source boundary

The standalone repository must build without the former `dashboard` and `old`
source trees. Do not add private domains, private registries, private Go
proxies, old source paths, or organization-only credentials. See
`docs/source-provenance.md` for the copied-code boundary.

## Dependency changes

Commit `go.mod`, `go.sum`, `package.json`, and `package-lock.json` changes
together. Run license and security checks. Regenerate the three SBOM files with
`scripts/generate-sbom.sh` when the dependency graph or release image changes.
