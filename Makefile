.PHONY: dependencies generate generate-check lint test test-race build static-server-test schema-check license-check security-check workflow-check sbom ci

ACTIONLINT_VERSION := v1.7.12
SHELLCHECK_IMAGE := koalaman/shellcheck-alpine:v0.11.0@sha256:9955be09ea7f0dbf7ae942ac1f2094355bb30d96fffba0ec09f5432207544002

dependencies:
	cd server && go mod download
	npm --prefix web ci

generate:
	$(MAKE) -C server generate

generate-check:
	./scripts/ci/check-generated.sh

lint:
	$(MAKE) -C server lint
	npm --prefix web run lint

test:
	$(MAKE) -C server test
	npm --prefix web run test -- --run

test-race:
	cd server && go test -race ./...
	npm --prefix web run test -- --run

build:
	$(MAKE) -C server build
	npm --prefix web run build

static-server-test:
	cd web/cmd/static-server && go test ./... && go vet ./...

schema-check:
	./scripts/check-schema.sh deploy/schema.sql

license-check:
	./scripts/license-check.sh

security-check:
	./scripts/security-check.sh

workflow-check:
	./scripts/ci/check-workflow.sh
	go run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION) .github/workflows/*.yml
	docker run --rm --entrypoint /bin/sh -v "$(CURDIR):/mnt:ro" $(SHELLCHECK_IMAGE) -ec \
		'shellcheck --severity=warning /mnt/scripts/*.sh /mnt/scripts/ci/*.sh'

sbom:
	./scripts/generate-sbom.sh

ci: dependencies generate-check lint test-race build static-server-test schema-check license-check workflow-check security-check
