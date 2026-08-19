# Threat model

## Assets

The protected assets are administrator credentials, refresh sessions, JWT
signing keys, project API keys, unpublished and published configuration,
environment role assignments, audit records, uploaded objects, and dependency
or deployment credentials.

## Trust boundaries

1. A browser crosses the public HTTP boundary at the Kirby process.
2. The Kirby process crosses authenticated boundaries to MySQL, Redis, and object
   storage.
3. A runtime client crosses a separate HTTP or gRPC boundary using a project API
   key.
4. Build workers cross public dependency and container-registry boundaries.

The browser, request headers, route IDs, filenames, object keys, imported JSON,
and runtime project/config keys are untrusted. MySQL is authoritative for
identity, ownership, versions, publication state, and API-key revocation. Redis
is shared acceleration and rate-limit state, not an authorization authority.

## Main threats and controls

| Threat | Control |
|---|---|
| Password theft or offline guessing | Argon2id, bounded hash parameters, TTY or mode-0600 password file input |
| JWT forgery or stale keys | Configured issuer, key ID, fixed algorithms, short access lifetime, explicit key ring |
| Refresh-token theft or replay | `HttpOnly` cookie, exact origin validation, hash-only storage, rotation, replay revocation |
| Project API-key disclosure | One-time secret response, peppered digest, constant-time verification, redacted logs |
| Cross-environment access | Environment permission checks plus environment-scoped repository queries |
| Resource-ID substitution | Project, config, snapshot, import, and API-key ownership checked in the database |
| Object-key traversal or substitution | Strict parser, generated UUID paths, environment/project comparison, rooted local file access |
| Oversized or disguised upload | Signed size/type declaration, upload limits, metadata revalidation, incomplete-object deletion |
| Secret leakage through errors or logs | Public error mapping, structured redaction, request query/header exclusion tests |
| Audit tampering by partial writes | Business change and audit record share one database transaction |
| Rate-limit bypass | Shared Redis counters; cache failure denies the request |
| Stale permission or publication cache | Database generation/version is read before cache use |
| Supply-chain compromise | Checksums and lockfile, pinned container digests, source/secret scans, vulnerability scans, SBOM |

## Assumptions and residual risk

- The ingress, MySQL, Redis, and object-storage control planes are operated by
  trusted administrators and are not exposed directly to untrusted networks.
- Server memory can contain active credentials. Host or process compromise is
  outside the application's containment boundary.
- Formily's dependency declarations still target older React type shapes.
  Runtime compatibility is covered by focused Formily and browser tests.
- Fixed-window limits constrain abuse but are not a complete denial-of-service
  defense. The ingress still needs connection, body-size, and global traffic
  controls.
- Local object storage is intentionally process-local and is not suitable for
  horizontally scaled deployment.
