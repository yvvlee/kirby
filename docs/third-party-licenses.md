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

The committed SBOM set was produced from Git commit
`03e30a621af9cbb5abfe42a363e6faf915ff5163`. Each CycloneDX document contains
that commit and the SHA-256 values of `server/go.sum` and
`web/package-lock.json`. Image documents also contain the immutable local image
digest produced from that source commit.

The React migration is currently an uncommitted working tree. Its replacement
SBOM must be generated after the migration commit so the recorded source commit
and archived dependency manifests describe the same tree.

The complete transitive inventories are:

- `dist/sbom/source.cdx.json`: repository manifests, including development dependencies
- `dist/sbom/web.cdx.json`: frontend runtime and development dependencies
- `dist/sbom/server-image.cdx.json`: compiled Kirby release image and web assets

## Direct runtime dependencies

The server directly uses Go modules under Apache-2.0, BSD-2-Clause,
BSD-3-Clause, MIT, and MPL-2.0. They include Kratos, gRPC, protobuf,
protoc-gen-validate, MySQL and Redis clients, MinIO, Xorm, Cobra, JWT, UUID,
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
| Go toolchain image | `golang:1.25-bookworm` tag, resolved at build time | Go BSD-3-Clause; build stage only |
| Node.js toolchain image | `node:24-bookworm-slim` tag, resolved at build time | Node.js MIT; build stage only |
| Kirby runtime | `scratch` | one Go process, React assets, no operating-system packages |
| protoc | 35.1 | BSD-3-Clause |
| protoc-gen-go | v1.36.12 | BSD-3-Clause |
| protoc-gen-go-grpc | v1.6.2 | Apache-2.0 |
| protoc-gen-validate | v1.3.3 | Apache-2.0 |
| protoc-gen-go-http | v2.9.2 source commit `b9fab9a5a5ab` | MIT |
| Wire | v0.7.0 | Apache-2.0 |
| go-licenses | v1.6.0 | Apache-2.0 |
| license-checker | v25.0.1 | BSD-3-Clause |
| Trivy | 0.67.2, `sha256:e2b22eac59c02003d8749f5b8d9bd073b62e30fefaef5b7c8371204e0a4b0c08` | Apache-2.0 |

Build-stage operating-system packages are not copied into the scratch release
stages. Their licenses remain governed by the pinned Debian-based builder
images.

## Redistributed generator source

| Generator | Tag | Source commit | License |
|---|---|---|---|
| protoc-gen-go-errors | v1.0.0 | `04802e1d2794f6fc2e8a2d0d9d770eb32f8600cd` | MIT |
| protoc-gen-go-json | v1.0.1 | `217c673080d2518111c3b3330a7647803058b865` | MIT |

The `SOURCE` and `LICENSE` files beside each copied generator are authoritative.
The errors generator has only its module path and matching self-import rewritten
to the public repository path. The JSON generator source is unchanged.

## Verification

Run from the repository root after `npm ci` in `web`:

```text
scripts/license-check.sh
scripts/generate-sbom.sh
```

Both commands fail on missing tools, missing package metadata, unknown
licenses, integrity mismatches, or incomplete output.
