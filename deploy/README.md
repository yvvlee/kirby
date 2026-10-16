# Deployment

Example configuration, the SQL schema reference, and local multi-instance deployment files live here.

## Before starting the image

The application listens on container ports `8080` (HTTP) and `9090` (gRPC).
Mount the edited configuration at `/etc/kirby/config.yaml`. Compose publishes
those listeners on host ports `8000` and `9000` by default.

S3 must be prepared separately. Create the configured bucket before starting
Kirby and grant its account permission to check the bucket and read, write,
inspect, and delete objects. Kirby checks the bucket at startup but does not
create it or configure bucket policies, lifecycle rules, or CORS.

Apply the policies below before using uploads and published resource links.
Bucket credentials alone are not sufficient. The temporary-upload lifecycle
rule removes abandoned upload objects after one day.

Set the storage `endpoint` to the internal S3 address and `use_ssl` for its TLS
setting. Upload tickets use the relative `/<bucket>/` path through Kirby's
same-origin proxy. Forward that path through Nginx or Ingress without rewriting
it. No browser-facing S3 endpoint or bucket CORS rule is required.

Completed asset URLs default to the current public request origin and bucket
path. Proxy forwarding headers supply the external protocol and host, so no
public domain needs to be entered in the configuration. Optionally set
`object_storage.s3.public_base_url` to an independent CDN/resource base URL;
leave it absent or empty for the default. Uploads still go through Kirby.

## First administrator

An empty database is initialized automatically with tables, signing keys,
permissions, and roles. No default account or password is created. After the
first start, open `/setup` through a controlled network or ingress and create
the system administrator with a username, display name, and password of at
least 12 bytes. Continue to `/login` after setup completes.

The database records setup completion permanently; removing all administrators
does not reopen it. `create-admin` remains available for automation and access
recovery. If it creates the first administrator, browser setup also closes.

## S3 upload isolation and cleanup

Kirby uploads browser data only under the private `uploads/` prefix. The server
validates it and conditionally publishes an immutable object under
`environments/`. Never grant public read access or a public ACL to `uploads/`.

The example policies assume the bucket is named `kirby`. Change both the CLI
bucket argument and policy ARN when using another bucket.

The AWS CLI commands below use the account and region selected by your CLI
configuration. For MinIO or another S3-compatible provider, also pass its
`--endpoint-url` so the commands target that service.

Apply the public-read policy, which permits reads only below `environments/`:

```sh
aws s3api put-bucket-policy \
  --bucket kirby \
  --policy file://deploy/s3-public-read-policy.json
```

Apply the one-day cleanup rule for abandoned temporary uploads:

```sh
aws s3api put-bucket-lifecycle-configuration \
  --bucket kirby \
  --lifecycle-configuration file://deploy/s3-upload-lifecycle.json
```

`put-bucket-lifecycle-configuration` replaces the existing lifecycle document.
Merge the `expire-kirby-temporary-uploads` rule into the current document first
when the bucket already has lifecycle rules. Verify both controls after changes:

```sh
aws s3api get-bucket-policy --bucket kirby
aws s3api get-bucket-lifecycle-configuration --bucket kirby
```

Completing an S3 upload streams at most 64 MiB from the private upload key to
the immutable public key. Measure this path in the deployment network. Set
`http.timeout` above the observed worst-case copy time. The example uses two
minutes; do not reduce it to a value that truncates valid uploads.
