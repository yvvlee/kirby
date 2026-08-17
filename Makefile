.PHONY: dependencies generate generate-check lint test test-race build static-server-test schema-check license-check security-check workflow-check sbom ci

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

sbom:
	./scripts/generate-sbom.sh

ci: dependencies generate-check lint test-race build static-server-test schema-check license-check workflow-check security-check
