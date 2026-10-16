# Releasing

Publish a GitHub Release from a semantic-version tag to start
the release workflow. It uploads Linux binaries, checksums, CycloneDX SBOMs,
provenance attestations, and Sigstore bundles to that release, and pushes one
multi-platform image to Docker Hub at `mshadow/kirby`.

## Docker Hub configuration

Create the `mshadow/kirby` repository on Docker Hub. In the GitHub repository's
Settings → Secrets and variables → Actions, add these repository secrets:

| Secret | Value |
|---|---|
| `DOCKERHUB_USERNAME` | Docker Hub account with push access to `mshadow/kirby` |
| `DOCKERHUB_TOKEN` | Docker Hub access token with read/write permissions |

The workflow fails before building if either secret is missing. Registry
authentication uses this token; it does not use a Docker Hub account password.

## Prepare

Update dependencies, documentation, and the security report before tagging.
From a clean default branch, run:

```sh
make ci
scripts/test-clean-room.sh
scripts/release-check.sh --dry-run v0.1.0
```

The dry run accepts a tag that does not exist yet. If it does exist, it must be
a tag pointing to `HEAD`. Lightweight and annotated tags are both accepted.
The script builds each Linux binary twice
inside the pinned Go container and compares the bytes.

Set `KIRBY_GO_PROXY` when the default Go module proxy is not reachable. The
same value is used by clean-room image builds and the release binary build.

## Create the tag

Create a tag and push only after the dry run passes. For an annotated tag:

```sh
git tag -a v0.1.0 -m 'Kirby v0.1.0'
git push origin v0.1.0
```

Lightweight tags created by GitHub Releases also work. Tag signing is optional.
The workflow checks that the tag resolves to the checked-out source commit.
Release files and images are still signed through GitHub OIDC.

After pushing the tag, create and publish a GitHub Release for it, using the
GitHub Releases page or the CLI:

```sh
gh release create v0.1.0 --verify-tag --generate-notes --title 'Kirby v0.1.0'
```

The `release: published` event starts the workflow for both stable releases and
prereleases. A draft release or a tag push alone does not publish images. The
workflow checks out the release tag, so the binaries and image use that exact
commit.

To retry after a workflow fix, open **Actions → Release → Run workflow**,
select `main`, and enter the existing published release tag, for example
`v0.0.1`. The equivalent CLI command is:

```sh
gh workflow run release.yml --ref main -f tag=v0.0.1
```

This runs the updated workflow while building the original tagged source.
Version, build date, image labels, and SBOMs use that tag's commit. It reads the
release's prerelease status so a retry cannot accidentally update `latest` for
a prerelease. Existing release assets are replaced after the checks pass.
The release helper is kept outside the source checkout so older tags can use
the corrected checks without modifying their source tree.

Use **Re-run failed jobs** only when the workflow itself has not changed;
GitHub reruns use the original workflow revision.

## Published artifacts

The GitHub release contains:

- `kirby-vX.Y.Z-linux-amd64` and `kirby-vX.Y.Z-linux-arm64`;
- `SHA256SUMS` covering the binaries and attached SBOM files;
- source, frontend, and release-image CycloneDX SBOMs;
- one `.sigstore.json` bundle for every attached artifact.

The image packages are:

```text
docker.io/mshadow/kirby:vX.Y.Z
docker.io/mshadow/kirby:sha-FULL_GIT_SHA
docker.io/mshadow/kirby:latest
```

Only stable releases with no prerelease suffix update `latest`. For example,
`v0.1.0-rc.1` publishes its version and commit tags without changing `latest`,
even if it was accidentally published without GitHub's prerelease flag.

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
  docker.io/mshadow/kirby@sha256:IMAGE_MANIFEST_DIGEST
```

GitHub provenance for release files can also be verified with:

```sh
gh attestation verify kirby-v0.1.0-linux-amd64 --repo OWNER/kirby
```

For manual retries run from `main`, use
`https://github.com/OWNER/kirby/.github/workflows/release.yml@refs/heads/main`
as the Cosign certificate identity in both verification commands above.
GitHub provenance records the workflow revision and includes the tagged source
commit as a resolved dependency. The SBOMs also record the tagged source commit.

The standalone image SBOM files describe the linux/amd64 release image. The OCI
image's attached BuildKit SBOM covers each architecture.
