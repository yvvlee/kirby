# Operations

## Health and readiness

The backend exposes:

- `/healthz` for process liveness;
- `/readyz` for readiness;
- the standard gRPC health service on the runtime gRPC listener.

The binary checks both local listeners:

```sh
kirby healthcheck \
  --http-endpoint http://127.0.0.1:8080/readyz \
  --grpc-address 127.0.0.1:9090
```

Remove an instance from ingress traffic when readiness fails. Restarting a
backend does not lose durable state because production instances are stateless.

## Logs

Application and Kratos logs use the shared Zap logger through `kratos/log.Helper`.
Log records use `_TS_`, `_LEVEL_`, `_MSG_`, and `_CALLER_`; output encoding is fixed
and has no format setting. `log.level` follows the original project: whitespace
and case are ignored, `DEBUG`, `WARNING`, and `ERROR` select their respective
levels, and all other values select `INFO`.

Messages and fields pass through without application-side masking. The log
collection component handles auditing and redaction. Every HTTP response
receives `X-Request-ID`; a valid caller-supplied value is preserved. Search
application and ingress logs by that value. HTTP request logs omit query strings
and request headers.

## Backup

Treat all environments in one deployment as one consistency unit. Do not back
up or restore a single environment by copying selected tables.

A recoverable backup contains:

- one transactionally consistent snapshot of the entire Kirby database,
  including the `application_secrets` row containing JWT and API-key HMAC keys;
- the complete S3 bucket version corresponding to that database snapshot;
- the deployed configuration version, with secrets stored in the secret
  manager rather than the backup archive;
- the application version or image digests and the reviewed model version.

Quiesce management writes and uploads, or use coordinated database and object
storage snapshots with a recorded consistency point. Published object paths are
immutable, but an in-flight upload may otherwise exist in only one system.

Redis contains caches and rate-limit counters. It is not part of the durable
backup. Expect cold caches after recovery.

## Restore

1. Stop management and runtime traffic.
2. Restore the complete S3 bucket privately.
3. Restore the complete MySQL database from the matching consistency point.
4. Restore the matching configuration and application version.
5. Start one backend and verify `/readyz` and the gRPC health service.
6. Verify login, environment permissions, one published runtime read, and one
   asset URL.
7. Start the remaining stateless backend instances and reopen traffic.

Do not use `deploy/schema.sql` to repair a restored database. The service only
initializes an empty database; review any required schema change first.

## PostgreSQL seed sequence repair

PostgreSQL databases initialized before the lorm migration may have ID sequences
behind the seeded `roles` and `permissions` rows. Explicit seed IDs do not advance
`BIGSERIAL`; creating the first custom role can therefore fail with a primary-key
conflict. New databases initialized by the corrected schema already advance both
sequences. MySQL does not need this repair.

For an existing PostgreSQL deployment:

1. Take a database backup and pause management writes. Review
   `deploy/migrations/20261015_postgresql_seed_sequences.sql`.
2. Use a database owner account that can lock both tables and update their
   sequences. Confirm that `search_path` resolves `roles` and `permissions` to the
   Kirby application schema. For the default schema, run from the repository root:

   ```sh
   PGOPTIONS='-c search_path=public' psql "$KIRBY_DATABASE_DSN" \
     -X -v ON_ERROR_STOP=1 \
     -f deploy/migrations/20261015_postgresql_seed_sequences.sql
   ```

3. Create a custom role through the management UI, confirm that it appears in the
   role list, and resume management writes.

The repair locks both seeded tables in a transaction and only moves a sequence
forward when its next generated ID could collide with an existing row. It retains
larger sequence values and can be run again safely. PostgreSQL sequence changes
are not rolled back on transaction failure; a partial forward adjustment only
leaves harmless ID gaps. The server does not run this repair automatically.

## Administrator recovery

Once the first administrator has been created, `application_secrets.admin_initialized`
keeps `/setup` closed permanently. If every administrator is deleted, use
`kirby create-admin --config /etc/kirby/config.yaml --username <name>` to create
a replacement. Do not clear the initialization flag to recover access.

## Routine maintenance

- Keep MySQL, Redis, object storage, ingress, and pinned base images updated.
- Run `make ci`, `scripts/security-check.sh`, and `scripts/license-check.sh`
  before release.
- Monitor authentication failures, rate-limit responses, database saturation,
  Redis errors, S3 completion latency, and HTTP/gRPC error rates.
- Verify the `uploads/` lifecycle rule and the `environments/` public-read
  boundary after object-storage policy changes.

## Troubleshooting

`database schema is incomplete` means table initialization failed. Stop the
server and inspect the database connection and model synchronization error.

`mode=multi requires cache.driver=redis` or `object_storage.driver=s3` means a
process-local dependency was selected for a production layout.

Repeated `origin is not allowed` responses mean the browser origin does not
match the public request address. Serve the UI and API from the same scheme,
host, and port. Check that ingress proxies preserve Host, overwrite client
forwarding headers, and send `X-Forwarded-Proto` and `X-Forwarded-For`. Proxy
headers are trusted by default; check optional `security.trusted_proxies`
restrictions if they have been configured.

Upload tickets use the relative `/<bucket>/` path. Upload failures can indicate
an ingress that does not forward this path or an unreachable S3 endpoint.
Completed URLs default to the public request address; an unexpected scheme or
host indicates incorrect proxy forwarding headers. If `public_base_url` is
set, check that CDN/resource address. Also check the ingress route and bucket
public-read policy when completed resources cannot be fetched.

Unexpected authorization failures after a role edit should be investigated as
a Redis or MySQL error. Kirby fails closed when permission state cannot be
resolved.

A runtime 401 indicates a missing, malformed, rotated, or revoked project key.
A runtime 403 indicates that the key belongs to a different project. A runtime
404 means no published config matches the requested project and key.
