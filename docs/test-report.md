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

The local command expects running containers named `mysql` and `redis` by
default. The names and published ports are configurable through the
`KIRBY_TEST_*` environment variables documented by the script.

## Verified environment

| Component | Version or immutable input |
|---|---|
| Local MySQL | 9.7.0 |
| Local Redis | 8.8.0 |
| Go build image | `sha256:81dc45d05a7444ead8c92a389621fafabc8e40f8fd1a19d7e5df14e61e98bc1a` |
| Node build image | `sha256:9e70124bd00f47dd023e349cd587132ae61892acc0e47ed641416c3e18f401c3` |
| Web runtime image | `sha256:ce2bd4775ed6859d35f47d65401ee9f35f1dd00b32ed05f0ce38b68aa1830195` |

## Result

The local development command passed with Node.js 24.19.0 and a real Chromium
browser. It created an isolated MySQL database and user, used a unique Redis key
prefix, and deleted the database, user, credentials, object files, processes,
and temporary configuration on exit.

The clean-room command rebuilt the Go server, web application, and both public
container images using only Git-tracked source and temporary caches.
