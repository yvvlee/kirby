# Project API keys

Project API keys authenticate runtime HTTP and gRPC reads. Each key belongs to
exactly one project. It cannot read another project, including a project in the
same environment.

## Creation

Only a user with `project:api_key:manage` can create a key. The response
contains the full secret once. Store it immediately in the runtime client's
secret manager. Later list responses contain only metadata, the public ID, and
the last four secret characters.

Kirby stores an HMAC digest made with `security.api_key_pepper`. It never stores
the plaintext credential. The pepper must be identical on every backend
instance and must be protected like a signing key.

## Use

For HTTP, send:

```text
X-Kirby-API-Key: <full-project-api-key>
```

For gRPC, send metadata:

```text
x-kirby-api-key: <full-project-api-key>
```

Do not put the key in a URL, source file, log field, browser storage, or error
message. Use a different key per workload so that rotation and audit metadata
identify the affected client.

## Rotation

Rotation replaces the secret on the existing key record. The old secret stops
working immediately. A safe client rollout is:

1. Create a second key for the same project.
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
4. Review audit records and runtime access logs by key public ID and time.
5. Rotate `security.api_key_pepper` only if the pepper itself leaked. That
   operation invalidates every project key.
