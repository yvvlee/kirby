# Environment API keys

API keys authenticate runtime HTTP and gRPC reads. Each key belongs to exactly
one project and one of its environments. The key selects both the project and environment;
clients cannot use it to read another environment or another project.

## Creation

Only a user with the environment API Key management permission can create a key.
Owners always have this permission; Maintainers have it by default. The
environment permission matrix can grant or remove it for the other project roles. The response
contains the full secret once. Store it immediately in the runtime client's
secret manager. Later list responses contain only metadata and
the last four secret characters.

A secret is 32 cryptographically random bytes encoded as 43 base64url characters, without a prefix or public identifier. The numeric management ID is not used to generate it.

Kirby stores an HMAC-SHA256 digest made with the API-key pepper from
`application_secrets`. It never stores the plaintext credential. The pepper is
generated during database initialization and loaded into each backend instance
at startup. Protect the database and its backups like signing keys.

## Use

For HTTP, send:

```text
GET /api/v1/config?key=<configuration-key>
X-Kirby-API-Key: <full-project-api-key>
```

The request only specifies the configuration key. No project or environment
parameter is needed; both come from the authenticated API key.

For gRPC, send metadata:

```text
x-kirby-api-key: <full-project-api-key>
```

Set only `ConfigRequest.key` in the gRPC request.

Do not put the key in a URL, source file, log field, browser storage, or error
message. Use a different key per workload so that rotation and audit metadata
identify the affected client.

## Rotation

Rotation replaces the secret on the existing key record. The old secret stops
working immediately. A safe client rollout is:

1. Create a second key for the same project and environment.
2. Deploy it to the client and verify successful reads.
3. Revoke the old key.

Use in-place rotation only when the client and key update can be coordinated
without overlap.

## Revocation and leakage

Revocation is immediate because runtime authentication verifies the key and
revocation state in MySQL for every request. A revoked key cannot be rotated.

If a key may have leaked:

1. Revoke it immediately.
2. Create a replacement with a new name.
3. Update the affected client through its secret manager.
4. Review audit records and runtime access logs by numeric key management ID and time.
5. If the stored API-key pepper leaked, replace it through a reviewed database
   maintenance procedure and restart every backend instance. This invalidates
   every project key, so issue replacement keys after the change.

Project environments own independent configuration data and API keys. Runtime requests select the environment through the API key. See [project and environment isolation](environment-isolation.md).
