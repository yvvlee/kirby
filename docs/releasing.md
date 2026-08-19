# Releasing

Kirby releases are created only from signed annotated semantic-version tags.
The release workflow publishes Linux binaries, one multi-platform image,
checksums, CycloneDX SBOMs, provenance attestations, Sigstore bundles, and
generated release notes.

## Prepare

Update dependencies, documentation, and the security report before tagging.
From a clean default branch, run:

```sh
make ci
scripts/test-clean-room.sh
scripts/release-check.sh --dry-run v0.1.0
```

The dry run accepts a tag that does not exist yet. If it does exist, it must be
an annotated tag pointing to `HEAD`. The script builds each Linux binary twice
inside the pinned Go container and compares the bytes.

Set `KIRBY_GO_PROXY` when the default Go module proxy is not reachable. The
same value is used by clean-room image builds and the release binary build.

## Create the tag

Create a signed annotated tag and push only after the dry run passes:

```sh
git tag -s v0.1.0 -m 'Kirby v0.1.0'
git verify-tag v0.1.0
git push origin v0.1.0
```

GitHub must recognize the tag signature as verified. A lightweight or unsigned
tag fails before any package is published.

## Published artifacts

The GitHub release contains:

- `kirby-vX.Y.Z-linux-amd64` and `kirby-vX.Y.Z-linux-arm64`;
- `SHA256SUMS` covering the binaries and attached SBOM files;
- source, frontend, and release-image CycloneDX SBOMs;
- one `.sigstore.json` bundle for every attached artifact.

The image packages are:

```text
ghcr.io/OWNER/kirby:vX.Y.Z
ghcr.io/OWNER/kirby:sha-FULL_GIT_SHA
```

Each image tag is an amd64/arm64 manifest. BuildKit attaches per-platform SBOM
and maximum-mode provenance records. Cosign signs the manifest digest with the
workflow's GitHub OIDC identity.

## Verify

Verify downloaded checksums:

```sh
sha256sum --check SHA256SUMS
```

Verify a release file's keyless signature:

```sh
cosign verify-blob \
  --bundle kirby-v0.1.0-linux-amd64.sigstore.json \
  --certificate-identity \
  'https://github.com/OWNER/kirby/.github/workflows/release.yml@refs/tags/v0.1.0' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  kirby-v0.1.0-linux-amd64
```

Verify an image by immutable digest:

```sh
cosign verify \
  --certificate-identity \
  'https://github.com/OWNER/kirby/.github/workflows/release.yml@refs/tags/v0.1.0' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ghcr.io/OWNER/kirby@sha256:IMAGE_MANIFEST_DIGEST
```

GitHub provenance for release files can also be verified with:

```sh
gh attestation verify kirby-v0.1.0-linux-amd64 --repo OWNER/kirby
```

The standalone image SBOM files describe the linux/amd64 release image. The OCI
image's attached BuildKit SBOM covers each architecture.
