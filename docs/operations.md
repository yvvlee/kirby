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

Use JSON logs in production. Every HTTP response receives `X-Request-ID`; a
valid caller-supplied value is preserved. Search application and ingress logs
by that value. Logs omit request query strings and redact known credential
forms.

Never enable request-header logging at the ingress for `Authorization`, Cookie,
or `X-Kirby-API-Key`.

## Backup

Treat all environments in one deployment as one consistency unit. Do not back
up or restore a single environment by copying selected tables.

A recoverable backup contains:

- one transactionally consistent snapshot of the entire Kirby MySQL database;
- the complete S3 bucket version corresponding to that database snapshot;
- the deployed configuration version, with secrets stored in the secret
  manager rather than the backup archive;
- the application version or image digests and `deploy/schema.sql` revision.

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

Do not apply `deploy/schema.sql` over a restored database. It is an initial
schema, not an upgrade or repair script.

## Routine maintenance

- Keep MySQL, Redis, object storage, ingress, and pinned base images updated.
- Run `make ci`, `scripts/security-check.sh`, and `scripts/license-check.sh`
  before release.
- Monitor authentication failures, rate-limit responses, database saturation,
  Redis errors, S3 completion latency, and HTTP/gRPC error rates.
- Verify the `uploads/` lifecycle rule and the `environments/` public-read
  boundary after object-storage policy changes.

## Troubleshooting

`database schema is incomplete` means the fixed schema was not applied to the
configured database. Stop the server and apply the correct reviewed migration.

`mode=multi requires cache.driver=redis` or `object_storage.driver=s3` means a
process-local dependency was selected for a production layout.

Repeated `origin is not allowed` responses mean the browser's exact scheme,
host, and port are absent from `security.allowed_origins`.

Upload tickets that point at an internal hostname indicate an incorrect
`object_storage.s3.presign_endpoint`. Completed URLs that cannot be fetched
indicate an incorrect `public_base_url`, ingress route, or bucket policy.

Unexpected authorization failures after a role edit should be investigated as
a Redis or MySQL error. Kirby fails closed when permission state cannot be
resolved.

A runtime 401 indicates a missing, malformed, rotated, or revoked project key.
A runtime 403 indicates that the key belongs to a different project. A runtime
404 means no published config matches the requested project and key.
