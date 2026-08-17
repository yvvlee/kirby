.PHONY: generate lint test build ci license-check sbom

generate:
	$(MAKE) -C server generate

lint:
	$(MAKE) -C server lint
	npm --prefix web run lint

test:
	$(MAKE) -C server test
	npm --prefix web run test -- --run

build:
	$(MAKE) -C server build
	npm --prefix web run build

license-check:
	./scripts/license-check.sh

sbom:
	./scripts/generate-sbom.sh

ci: generate lint test build license-check
