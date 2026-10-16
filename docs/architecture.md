# Architecture

## System shape

Kirby has one administration plane and one runtime plane.

The administration plane consists of the React 19 application and management HTTP
API. Users log in once. Each request carries an access JWT, then the backend
resolves the user's current role for the requested environment.

The runtime plane exposes the same published configuration over HTTP and gRPC.
It uses project API keys instead of administrator JWTs. Management protobufs
generate HTTP handlers only. Runtime protobufs generate both HTTP and gRPC
handlers.

## State ownership

| Component | Responsibility |
|---|---|
| Configured relational database | All durable application and audit data |
| Redis | Permission and publication caches, generation markers, and rate-limit counters |
| S3-compatible storage | Temporary uploads and immutable published objects |
| Kirby process | HTTP/gRPC APIs, React assets, S3 same-origin forwarding; no durable process-local state |
| React application | User interface, in-memory access token, and environment-scoped query cache |

The configured relational database is authoritative. Redis may accelerate reads, but it is not an
authorization authority. Backend processes are stateless when configured with
Redis and S3.

## Backend code organization

The backend uses service, logic, repository, and infrastructure layers.
Business areas are files within a layer, rather than separate Go packages.

- `conf`: configuration loading and validation.
- `server`: HTTP/gRPC setup, routes, and application lifecycle.
- `middleware`: transport authentication, request logging, and rate limits.
- `service`: request/reply conversion and public error mapping.
- `logic`: business operations, authorization, and transaction boundaries.
- `repository`: model access, scoped queries, pagination, and persistence errors.
- `model`: database records and embedded schema initialization.
- `entity`: domain data and validation.
- `converter`: model/protobuf conversions, split by business area.
- `infra`: database, shared storage, logging, cache, and object storage.
- `util`: reusable file, integer, set, and time helpers.

Authentication primitives and permission resolution remain in `auth` and
`permission`. Services use names such as `ProjectService` and `NewProjectService`;
logic uses `ProjectLogic` and `NewProjectLogic`. Each layer declares its Wire
providers in `provider.go`. `cmd/kirby/wire.go` assembles the application and
returns a cleanup function for both startup failures and normal shutdown.

## Database access

`infra.Storage.CoreDB` is one `*lorm.Engine`. Each repository embeds the typed
lorm repository and uses generated column names. SQL NULL checks, environment
scope, soft deletion, and version predicates are explicit in every applicable
query. A logic operation owns the transaction; its context carries that
transaction through repository calls. Production has no optional transaction
runner or database-free execution path.

MySQL and PostgreSQL share the record models but retain their dialect-specific
embedded DDL and startup advisory locks. Schema initialization uses a dedicated
SQL connection before opening the application engine. This keeps the lock and
schema changes on the same connection without exposing lorm's connection pool.

## Environment isolation

One Kirby deployment contains many projects, and each project contains many
environments. The durable hierarchy is `project -> environment -> config`.
Environment IDs are part of repository queries and authorization checks.
Projects, configs, snapshots, imports, API keys, and object paths are checked
against the owning project and environment.

A user has one identity and one JWT session. The user's roles are separate for
each environment. A system administrator can manage users, roles, and
environments, but environment content still uses explicit environment scope.

## Publication flow

Editors build configurations and snapshots in MySQL. Publishers select a
snapshot as the published version. Runtime reads resolve the project and config
against that published version. Cache entries include project, config key, and
version so publication changes cannot reuse an older value.

## Upload flow

The browser asks the management API for an upload ticket. The server generates
the object key and scope. With S3, the browser posts to the relative
`/<bucket>/` path through Kirby's proxy, which writes the private `uploads/`
key to S3. Completion streams and validates the object before publishing an
immutable object under `environments/`. Completed URLs use the public request
origin by default; an optional resource base URL selects a separate CDN domain.

Local object storage exposes signed upload and read handlers from the backend.
It is intended only for one-process development and is rejected in `mode:
multi`.

## Deployment boundary

Public TLS terminates at an ingress or load balancer. One Kirby image contains
the Go server and React assets. Each container runs one Go process, which serves
the web application and HTTP APIs and may expose runtime gRPC separately.
MySQL, Redis, and object storage stay on private networks.

Every backend instance must receive the same database, Redis, JWT issuer and
lifetimes, and S3 settings. Proxy headers are trusted by default; an optional
proxy CIDR list can restrict this behavior. JWT signing keys and the API-key
pepper are persisted in the database and loaded into memory at startup. Browser
requests use the web application's public origin without a configured allowlist.
No sticky session is required.
