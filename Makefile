SHELL := /bin/bash

WEB_DIR := apps/web
CONFIG ?= config/jute.example.yaml
A2A_CONFIG ?= config/jute.dev-a2a.yaml
A2A_DATA_DIR ?= .jute/dev-a2a
A2A_AGENT_CARD_URL ?= http://127.0.0.1:9797/.well-known/agent-card.json
HUB_URL ?= http://127.0.0.1:8787
WEB_URL ?= http://127.0.0.1:5173
NPM ?= npm

.PHONY: setup dev run test web-dev web-check check kronk-a2a-setup kronk-a2a kronk-a2a-server kronk-a2a-check dev-a2a dev-a2a-reset

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

test:
	go test ./...

web-dev:
	cd $(WEB_DIR) && $(NPM) run dev

web-check:
	cd $(WEB_DIR) && $(NPM) run check

check: test web-check

kronk-a2a-setup:
	cd examples/agents/kronk-a2a && go mod download

kronk-a2a:
	cd examples/agents/kronk-a2a && go run .

kronk-a2a-server:
	cd examples/agents/kronk-a2a && KRONK_A2A_MODE=server go run .

kronk-a2a-check:
	cd examples/agents/kronk-a2a && go test ./...

dev-a2a:
	@echo "Kronk A2A Agent Card: $(A2A_AGENT_CARD_URL)"
	@echo "Jute hub: $(HUB_URL)"
	@echo "Jute web: $(WEB_URL)"
	@set -e; \
	cleanup() { \
		if [[ -n "$$WEB_PID" ]]; then kill "$$WEB_PID" 2>/dev/null || true; fi; \
		if [[ -n "$$HUB_PID" ]]; then kill "$$HUB_PID" 2>/dev/null || true; fi; \
		if [[ -n "$$A2A_PID" ]]; then kill "$$A2A_PID" 2>/dev/null || true; fi; \
	}; \
	trap cleanup INT TERM EXIT; \
	(cd examples/agents/kronk-a2a && KRONK_A2A_MODE=server go run .) & A2A_PID=$$!; \
	echo "Waiting for Kronk A2A server. First run may download model assets."; \
	ready=0; \
	for _ in $$(seq 1 900); do \
		if curl -fsS "$(A2A_AGENT_CARD_URL)" >/dev/null 2>&1; then ready=1; break; fi; \
		if ! kill -0 "$$A2A_PID" 2>/dev/null; then \
			echo "Kronk A2A server exited before it became ready."; \
			wait "$$A2A_PID"; \
			exit 1; \
		fi; \
		sleep 2; \
	done; \
	if [[ "$$ready" != "1" ]]; then \
		echo "Timed out waiting for Kronk A2A server at $(A2A_AGENT_CARD_URL)."; \
		exit 1; \
	fi; \
	go run ./cmd/juted -config $(A2A_CONFIG) -data-dir $(A2A_DATA_DIR) & HUB_PID=$$!; \
	(cd $(WEB_DIR) && $(NPM) run dev) & WEB_PID=$$!; \
	wait "$$A2A_PID" "$$HUB_PID" "$$WEB_PID"

dev-a2a-reset:
	rm -rf $(A2A_DATA_DIR)
