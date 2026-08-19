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
| MySQL | All durable application and audit data |
| Redis | Permission and publication caches, generation markers, and rate-limit counters |
| S3-compatible storage | Temporary uploads and immutable published objects |
| Kirby process | HTTP/gRPC APIs, React assets, S3 same-origin forwarding; no durable process-local state |
| React application | User interface, in-memory access token, and environment-scoped query cache |

MySQL is authoritative. Redis may accelerate reads, but it is not an
authorization authority. Backend processes are stateless when configured with
Redis and S3.

## Environment isolation

One Kirby deployment contains many environments. Environment IDs are part of
repository queries as well as authorization checks. Projects, configs,
snapshots, imports, API keys, and object paths are checked against their owning
environment.

A user has one identity and one JWT session. The user's roles are separate for
each environment. A system administrator can manage users, roles, and
environments, but environment content still uses explicit environment scope.

## Publication flow

Editors build configurations and snapshots in MySQL. Publishers select a
snapshot as the published version. Runtime reads resolve the project and config
against that published version. Cache entries include environment, project,
config key, and version so publication changes cannot reuse an older value.

## Upload flow

The browser asks the management API for an upload ticket. The server generates
the object key and scope. With S3, the browser posts directly to a private
`uploads/` key. Completion streams and validates the object before publishing
an immutable object under `environments/`.

Local object storage exposes signed upload and read handlers from the backend.
It is intended only for one-process development and is rejected in `mode:
multi`.

## Deployment boundary

Public TLS terminates at an ingress or load balancer. One Kirby image contains
the Go server and React assets. Each container runs one Go process, which serves
the web application and HTTP APIs and may expose runtime gRPC separately.
MySQL, Redis, and object storage stay on private networks.

Every backend instance must receive the same database, Redis, JWT key ring, API
key pepper, allowed origins, trusted proxy list, and S3 settings. No sticky
session is required.
