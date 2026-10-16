# Third-party licenses

## Policy and result

The license gate rejects forbidden, reciprocal, or unknown Go licenses. It
also rejects npm packages whose declared or detected license is outside the
reviewed set. GPL, AGPL, LGPL, SSPL, and missing license results fail the gate.
The current dependency graph passes.

The policy applies to code and assets distributed in the Kirby release image.
MySQL, Redis, MinIO, and the external ingress are independently operated
infrastructure and are not copied into the image.

## Reproducible inputs

The checked-in SBOM set describes a committed source snapshot. Generate it with
`scripts/generate-sbom.sh`, which archives `KIRBY_SOURCE_COMMIT` (`HEAD` when
unset) without reading uncommitted changes. Each CycloneDX document records
`kirby:source:mode=commit` and the exact `kirby:source:git-commit`. The source
commit is also embedded in the local image build.

For an uncommitted source snapshot, use
`scripts/generate-sbom.sh --working-tree`. Documents then record
`kirby:source:mode=working-tree`, the HEAD base commit, the dirty state, and a
SHA-256 fingerprint over snapshot paths, executable modes, symlink targets, and
file contents. The hash input is the compact JSON encoding of sorted
`[path, Git mode, content SHA-256]` entries. The source reference combines the
base commit and that fingerprint.

The working-tree snapshot includes tracked files at their current contents and
untracked files that Git does not ignore. Deleted files stay absent. Git metadata,
local configuration, dependency caches, build output, and the previous SBOMs are
excluded. This mode additionally requires Python 3 and never stages or commits
files. The fingerprint identifies the captured files; it does not claim those
files can be retrieved from the base commit alone.

Both modes record the SHA-256 values of
`server/go.sum` and `web/package-lock.json`; the image document also records the
immutable digest of the locally built image. Images are not published.

The complete transitive inventories are:

- `dist/sbom/source.cdx.json`: repository manifests, including development dependencies
- `dist/sbom/web.cdx.json`: frontend runtime and development dependencies
- `dist/sbom/server-image.cdx.json`: compiled Kirby release image and web assets

## Direct runtime dependencies

The server directly uses Go modules under Apache-2.0, BSD-2-Clause,
BSD-3-Clause, MIT, and MPL-2.0. They include Kratos, gRPC, protobuf,
protoc-gen-validate, MySQL and Redis clients, MinIO, lorm v1.1.5, Zap, Cobra, JWT, UUID,
Argon2 support, and the Go `x/*` libraries. `go-licenses v1.6.0` checks the
complete compiled and test package graph, rather than trusting this summary.

The web runtime directly uses React, React Router, TanStack Query, Ant Design,
Formily, Axios, and Monaco Editor. These packages are MIT licensed. The complete npm tree
also contains reviewed 0BSD, Apache-2.0, BSD, BlueOak, CC0, ISC, MPL, and
Python-2.0 license expressions. Browser compatibility data from caniuse-lite is
CC-BY-4.0 and is attributed in `NOTICE`. `license-checker v25.0.1` reads the installed
package metadata and license files after `npm ci`.

## Build tools and images

| Input | Fixed version or digest | License |
|---|---|---|
| Go toolchain image | `golang:1.27.2-bookworm` tag, resolved at build time | Go BSD-3-Clause; build stage only |
| Node.js toolchain image | `node:24-bookworm-slim` tag, resolved at build time | Node.js MIT; build stage only |
| Kirby runtime | `scratch` | one Go process, React assets, no operating-system packages |
| protoc | 35.1 | BSD-3-Clause |
| protoc-gen-go | v1.36.12 | BSD-3-Clause |
| protoc-gen-go-grpc | v1.6.2 | Apache-2.0 |
| protoc-gen-validate | v1.3.3 | Apache-2.0 |
| protoc-gen-go-http | v2.9.2 source commit `b9fab9a5a5ab` | MIT |
| protoc-gen-go-errors | v2.9.2 source commit `b9fab9a5a5ab` | MIT |
| Wire | v0.7.0 | Apache-2.0 |
| lorm / lormgen | v1.1.5 | MIT |
| golangci-lint | v2.14.0 | GPL-3.0; development tool, not distributed in the release image |
| govulncheck | v1.9.0 | BSD-3-Clause |
| gosec | v2.29.0 | Apache-2.0 |
| go-licenses | v1.6.0 | Apache-2.0 |
| license-checker | v25.0.1 | BSD-3-Clause |
| Trivy | 0.67.2, `sha256:e2b22eac59c02003d8749f5b8d9bd073b62e30fefaef5b7c8371204e0a4b0c08` | Apache-2.0 |

Build-stage operating-system packages are not copied into the scratch release
stages. Their licenses remain governed by the pinned Debian-based builder
images.

## Verification

Run from the repository root after `npm ci` in `web`:

```text
scripts/license-check.sh
scripts/generate-sbom.sh
```

Both commands fail on missing tools, missing package metadata, unknown
licenses, integrity mismatches, or incomplete output.
