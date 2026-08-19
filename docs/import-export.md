# Snapshot import and export

Snapshot transfer copies configuration data between environments in the same
Kirby deployment. It does not bypass authorization and it is not a legacy
database migration tool.

## Permissions

Export requires all of these permissions in the source environment:

- `snapshot:export` and `snapshot:read`;
- `config:read`;
- `structure:read`;
- `enum:read`.

Import requires all of these permissions in the target environment:

- `snapshot:import` and `snapshot:write`;
- `config:write`;
- `structure:write`;
- `enum:write`.

The same user must satisfy both source and target checks. The source snapshot
and target project are rechecked against their respective environments.

## Conflict strategies

`FAIL` creates a new target config and fails when the target project already
contains the source config key. It must not include `target_config_id`.

`REPLACE` requires `target_config_id`. The target config is locked and replaced
with the imported value, structures, enums, description, and tags. It does not
select a target by name or guess which record to replace.

The imported snapshot is not automatically published. A publisher must review
and publish it in the target environment.

## Idempotency

Every import requires an ASCII `idempotency_key` containing 16 to 128
characters. Idempotency is scoped by user and target environment.

Repeating the exact request with the same key returns the original successful
snapshot and sets `replayed` to true. Reusing the key for different source,
target, description, tags, or conflict settings fails with a conflict. A
concurrent request using the same key also fails rather than running twice.

The import record, target changes, snapshot, current-snapshot pointer, and audit
record share one MySQL transaction. Failed transactions do not leave a partial
target configuration.

## HTTP endpoints

```text
GET  /admin/environments/{source_environment_id}/snapshots/{snapshot_id}/export
POST /admin/environments/{target_environment_id}/snapshot-imports
```

Both endpoints use the administrator bearer token. The protobuf source in
`server/api/admin/snapshot_transfer.proto` is the request and response source
of truth.
