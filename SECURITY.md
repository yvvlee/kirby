# Security policy

## Reporting a vulnerability

Do not open a public issue for an unpatched vulnerability. Use GitHub's private
vulnerability reporting for this repository. Include the affected version or
commit, impact, reproduction steps, and any suggested remediation.

Do not include real passwords, access tokens, refresh cookies, project API
keys, signing keys, database credentials, or customer configuration in the
report. Use synthetic data and redact logs.

Maintainers should acknowledge a complete report within five business days.
Fix timing depends on severity and reproducibility. Public disclosure should be
coordinated after a fix and release are available.

## Supported versions

Until the first stable release, only the current default branch is supported.
After stable releases begin, the supported release lines will be listed here.

## Operational security

Deployers are responsible for TLS termination, private MySQL/Redis/S3 networks,
secret management, backups, ingress limits, and timely dependency updates. Read
`docs/security.md` and `docs/threat-model.md` before exposing Kirby.

Before a release, run:

```sh
make ci
scripts/security-check.sh
scripts/license-check.sh
```

High and critical reachable vulnerabilities or container findings block a
release. Known residual risks, including the Vue 2 maintenance state, are
recorded in `docs/security-report.md`.
