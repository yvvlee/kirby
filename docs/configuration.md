# Configuration

Kirby reads one strict YAML file. Pass it with `--config` or set
`KIRBY_CONFIG_FILE`. Environment variables are not expanded inside the YAML
file. Unknown fields, missing required values, and extra YAML documents fail
startup.

Start from `deploy/config.example.yaml` for production or
`deploy/config.compose.yaml` for the included Compose stack.
Both templates listen on container ports `8080` (HTTP) and `9090` (gRPC).
Compose maps these to host ports `8000` and `9000` by default.

## Fields

| Field | Meaning |
|---|---|
| `mode` | `single` allows process-local dependencies; `multi` requires Redis and S3 |
| `http.address` | Management and runtime HTTP listener |
| `http.timeout` | Whole HTTP request timeout; must cover S3 completion copies |
| `grpc.address` | Runtime gRPC listener |
| `grpc.timeout` | Runtime gRPC request timeout |
| `database.driver` | `mysql` or `pgsql` |
| `database.dsn` | DSN for the selected database driver |
| `database.max_open_conns` | Maximum database connections per backend process |
| `database.max_idle_conns` | Idle connections per backend process |
| `database.conn_max_lifetime` | Maximum connection lifetime |
| `cache.driver` | `memory` for single-process development or `redis` |
| `cache.redis.*` | Redis endpoint, credentials, database, and deployment-specific key prefix |
| `jwt.issuer` | Expected JWT issuer |
| `jwt.access_ttl` | Access-token lifetime |
| `jwt.refresh_ttl` | Must be `168h` in the current server |
| `security.trusted_proxies` | Optional canonical proxy CIDRs; omitted/null trusts all, `[]` ignores forwarding headers |
| `object_storage.driver` | `local` or `s3` |
| `object_storage.local.directory` | Root used only by single-process development |
| `object_storage.s3.endpoint` | Private server-to-S3 host and optional port, without scheme |
| `object_storage.s3.public_base_url` | Optional CDN/resource base URL; omitted or empty uses the current public request origin and bucket path |
| `log.level` | Case-insensitive `DEBUG`, `WARNING`, or `ERROR`; other values use `INFO` |

## Shared values

All production backend instances must use identical values for:

- the selected relational database;
- the Redis deployment and key prefix;
- `jwt.issuer` and token lifetimes;
- the S3 bucket and credentials.

JWT signing keys and the API-key pepper are generated with secure randomness
when the database is initialized. They are persisted in the singleton
`application_secrets` record and loaded into memory at startup. Restarting a
server or adding instances reuses the same values. Do not delete or regenerate
this record: replacing the JWT key invalidates access tokens, and replacing the
pepper invalidates every project API key.

The web application and management API use the same origin. Browser requests
are checked against the request's public scheme, host, and port, so an origin
allowlist is not needed. Nginx and Ingress are supported by default without
listing proxy addresses. The proxy must overwrite incoming forwarding headers,
preserve the original Host, and send `X-Forwarded-Proto` and `X-Forwarded-For`.
It may also send `X-Forwarded-Host`.

`security.trusted_proxies` is optional. Omitted or null values trust both IPv4
and IPv6 proxies. Set explicit canonical CIDRs to restrict that behavior, or
set `[]` for direct connections that must ignore forwarding headers.

## Connection sizing

Database limits apply per process. Multiply `max_open_conns` by the maximum number
of backend instances and leave capacity for administrative and backup clients.
Use a distinct Redis `key_prefix` for each deployment that shares a Redis
database.

## S3 addresses

`object_storage.s3.endpoint` is the S3 address reachable by the backend;
`use_ssl` selects TLS for that connection. With the default configuration,
browser upload tickets point to the relative `/<bucket>/` path. Kirby proxies
those uploads to S3, so no browser-facing S3 endpoint or CORS configuration is
needed.

Completed asset URLs default to absolute URLs built from the public request's
scheme, host, and port plus `/<bucket>/<object-key>`. Trusted proxy forwarding
headers supply the external address when Nginx or Ingress terminates TLS.
Changing the public domain or mapped host port does not require editing S3
address settings.

`object_storage.s3.public_base_url` is optional. Leave it absent or empty for
the default behavior. Set it to a full resource base URL, for example
`https://cdn.example.com/kirby`, only when completed assets should use a CDN or
another resource domain. Uploads still use Kirby's same-origin proxy.

The bucket must already exist. Apply `deploy/s3-public-read-policy.json` and
`deploy/s3-upload-lifecycle.json` as described in `deploy/README.md`.
Kirby does not create buckets or configure policies or lifecycle rules.
