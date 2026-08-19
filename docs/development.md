# Development

## Toolchain

CI uses Go 1.25.13, Node.js 24.19.0, npm's committed lock file, and protoc
35.1. Generator versions are fixed in `server/Makefile`.

Install dependencies from the repository root:

```sh
make dependencies
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

## Protobuf generation

Files under `server/api` are the source of truth. Generated Go files are
committed next to their `.proto` files.

```sh
make -C server generate
scripts/ci/check-generated.sh
```

Do not edit generated files. Do not replace the two redistributed generators
under `server/tools/third_party` without updating their source record, license,
version checks, and dependency audit.

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
together. Run license and security checks. Regenerate the four SBOM files with
`scripts/generate-sbom.sh` when the dependency graph or release image changes.
