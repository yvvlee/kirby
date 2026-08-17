# Deployment

Example configuration, the manual database schema, and local multi-instance deployment files live here.

## S3 upload isolation and cleanup

Kirby uploads browser data only under the private `uploads/` prefix. The server
validates it and conditionally publishes an immutable object under
`environments/`. Never grant public read access or a public ACL to `uploads/`.

The example policies assume the bucket is named `kirby`. Change both the CLI
bucket argument and policy ARN when using another bucket.

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
