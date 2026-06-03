SHELL := /bin/bash

WEB_DIR := apps/web
CONFIG ?= examples/config/jute.example.yaml
HUB_URL ?= http://127.0.0.1:8787
WEB_URL ?= http://127.0.0.1:5173
NPM ?= npm

.PHONY: setup dev run lint test web-dev web-lint web-format-check web-check web-test web-build check

setup:
	cd $(WEB_DIR) && $(NPM) install

dev:
	@echo "Jute hub: $(HUB_URL)"
	@echo "Jute web: $(WEB_URL)"
	@set -e; \
	cleanup() { \
		if [[ -n "$$HUB_PID" ]]; then kill "$$HUB_PID" 2>/dev/null || true; fi; \
		if [[ -n "$$WEB_PID" ]]; then kill "$$WEB_PID" 2>/dev/null || true; fi; \
	}; \
	trap cleanup INT TERM EXIT; \
	go run ./cmd/juted -config $(CONFIG) & HUB_PID=$$!; \
	(cd $(WEB_DIR) && $(NPM) run dev) & WEB_PID=$$!; \
	wait "$$HUB_PID" "$$WEB_PID"

run:
	go run ./cmd/juted -config $(CONFIG)

lint:
	golangci-lint run --timeout=5m

test:
	go list ./... | grep -v '/apps/web/node_modules/' | xargs go test

web-dev:
	cd $(WEB_DIR) && $(NPM) run dev

web-lint:
	cd $(WEB_DIR) && $(NPM) run lint

web-format-check:
	cd $(WEB_DIR) && $(NPM) run format:check

web-check:
	cd $(WEB_DIR) && $(NPM) run check

web-test:
	cd $(WEB_DIR) && $(NPM) run test

web-build:
	cd $(WEB_DIR) && $(NPM) run build

check: lint test web-lint web-format-check web-check web-test web-build
