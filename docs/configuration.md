# Configuration

Kirby reads one strict YAML file. Pass it with `--config` or set
`KIRBY_CONFIG_FILE`. Environment variables are not expanded inside the YAML
file. Unknown fields, missing required values, and extra YAML documents fail
startup.

Start from `deploy/config.example.yaml` for production or
`deploy/config.compose.yaml` for the included Compose stack.

## Fields

| Field | Meaning |
|---|---|
| `mode` | `single` allows process-local dependencies; `multi` requires Redis and S3 |
| `http.address` | Management and runtime HTTP listener |
| `http.timeout` | Whole HTTP request timeout; must cover S3 completion copies |
| `grpc.address` | Runtime gRPC listener |
| `grpc.timeout` | Runtime gRPC request timeout |
| `mysql.dsn` | MySQL DSN with `parseTime=true` and an explicit time location |
| `mysql.max_open_conns` | Maximum database connections per backend process |
| `mysql.max_idle_conns` | Idle connections per backend process |
| `mysql.conn_max_lifetime` | Maximum connection lifetime |
| `cache.driver` | `memory` for single-process development or `redis` |
| `cache.redis.*` | Redis endpoint, credentials, database, and deployment-specific key prefix |
| `jwt.issuer` | Expected JWT issuer |
| `jwt.active_kid` | Key ID used for newly issued access tokens |
| `jwt.access_ttl` | Access-token lifetime |
| `jwt.refresh_ttl` | Must be `168h` in the current server |
| `jwt.keys` | Map of key IDs to signing secrets of at least 32 bytes |
| `security.api_key_pepper` | Shared HMAC pepper for project API keys, at least 32 bytes |
| `security.allowed_origins` | Exact browser origins; wildcards and paths are rejected |
| `security.trusted_proxies` | Canonical CIDRs allowed to supply forwarding headers |
| `object_storage.driver` | `local` or `s3` |
| `object_storage.local.directory` | Root used only by single-process development |
| `object_storage.s3.endpoint` | Private server-to-S3 host and optional port, without scheme |
| `object_storage.s3.presign_endpoint` | Browser-reachable S3 host and optional port |
| `object_storage.s3.presign_use_ssl` | Whether browser upload URLs use HTTPS |
| `object_storage.s3.public_base_url` | Public base URL for completed immutable objects |
| `log.level` | `debug`, `info`, `warn`, or `error` |
| `log.format` | `json` or `text` |

## Shared values

All production backend instances must use identical values for:

- the MySQL database;
- the Redis deployment and key prefix;
- `jwt.issuer`, `jwt.active_kid`, and the complete `jwt.keys` ring;
- `security.api_key_pepper`;
- allowed origins and trusted proxies;
- the S3 bucket and credentials.

Changing the API-key pepper invalidates every existing project key. Removing a
JWT key invalidates access tokens signed by that key. Rotate JWT keys by adding
the new key, switching `active_kid`, waiting longer than `access_ttl`, and only
then removing the old key.

## Connection sizing

MySQL limits apply per process. Multiply `max_open_conns` by the maximum number
of backend instances and leave capacity for administrative and backup clients.
Use a distinct Redis `key_prefix` for each deployment that shares a Redis
database.

## S3 addresses

`endpoint` is used by the backend. `presign_endpoint` is embedded in upload
tickets and must be reachable by browsers. `public_base_url` is stored in
completed asset URLs. These addresses can differ when an ingress exposes MinIO
or another S3 service under a public hostname.

The bucket must already exist. Apply `deploy/s3-public-read-policy.json` and
`deploy/s3-upload-lifecycle.json` as described in `deploy/README.md`.
