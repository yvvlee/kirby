# Verification report

This report records the standalone verification completed on 2026-08-17.

## Scope

- A clean checkout contains only Git-tracked Kirby files.
- Go, npm, and Docker build caches are isolated from the source checkout.
- Local backend verification reuses existing MySQL and Redis containers.
- The frontend runs through `npm run dev` with the Vite development proxy.
- A real Chromium session covers login, environments, roles, projects, configs,
  snapshots, import/export, assets, project API keys, and runtime reads.
- Backend multi-instance consistency and failover are outside this verification
  scope by deployment decision. Server processes remain stateless and use the
  configured MySQL and Redis services.

## Commands

```text
scripts/test-clean-room.sh
scripts/test-local-dev.sh
```

`KIRBY_NPM_REGISTRY` can select a temporary npm registry when the default
registry is unreachable. Package integrity remains enforced by
`package-lock.json`; the repository `.npmrc` is not modified.
`KIRBY_GO_PROXY` can select a temporary public Go module proxy for the
container build; `go.sum` verification remains enabled.

The local command expects running containers named `mysql` and `redis` by
default. The names and published ports are configurable through the
`KIRBY_TEST_*` environment variables documented by the script.

## Verified environment

| Component | Version or immutable input |
|---|---|
| Local MySQL | 9.7.0 |
| Local Redis | 8.8.0 |
| Go build image | `sha256:e401dae1bf814e29204a8cb7915682e1780951e609ca0dd8865ee1937f510c48` |
| Node build image | `sha256:9e70124bd00f47dd023e349cd587132ae61892acc0e47ed641416c3e18f401c3` |
| Web runtime image | `sha256:ddde39c6e51f02fde7410c2e9c234cf2d0a4c7bdbbe176aeb37d8ad7ab4eb58c` |

## Result

The local development command passed with Node.js 24.19.0 and a real Chromium
browser. It created an isolated MySQL database and user, used a unique Redis key
prefix, and deleted the database, user, credentials, object files, processes,
and temporary configuration on exit.

The clean-room command rebuilt the Go server, web application, and both public
container images using only Git-tracked source and temporary caches.
