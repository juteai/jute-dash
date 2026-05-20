SHELL := /bin/bash

WEB_DIR := apps/web
CONFIG ?= config/jute.example.yaml
A2A_CONFIG ?= config/jute.dev-a2a.yaml
A2A_MCP_CONFIG ?= config/jute.dev-a2a-mcp.yaml
A2A_DATA_DIR ?= .jute/dev-a2a
A2A_MCP_DATA_DIR ?= .jute/dev-a2a-mcp
A2A_AGENT_CARD_URL ?= http://127.0.0.1:9797/.well-known/agent-card.json
HUB_URL ?= http://127.0.0.1:8787
MCP_URL ?= http://127.0.0.1:8790/mcp
WEB_URL ?= http://127.0.0.1:5173
NPM ?= npm

.PHONY: setup dev run test web-dev web-check check a2a-v1-dev a2a-v1-dev-check dev-a2a dev-a2a-mcp dev-a2a-reset

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

a2a-v1-dev:
	cd examples/agents/a2a-v1-dev && go run .

a2a-v1-dev-check:
	cd examples/agents/a2a-v1-dev && go test ./...

dev-a2a:
	@echo "A2A 1.0 Dev Agent Card: $(A2A_AGENT_CARD_URL)"
	@echo "Jute hub: $(HUB_URL)"
	@echo "Jute web: $(WEB_URL)"
	@echo "Resetting dedicated A2A dev store: $(A2A_DATA_DIR)"
	@set -e; \
	cleanup() { \
		if [[ -n "$$WEB_PID" ]]; then kill "$$WEB_PID" 2>/dev/null || true; fi; \
		if [[ -n "$$HUB_PID" ]]; then kill "$$HUB_PID" 2>/dev/null || true; fi; \
		if [[ -n "$$A2A_PID" ]]; then kill "$$A2A_PID" 2>/dev/null || true; fi; \
	}; \
	trap cleanup INT TERM EXIT; \
	rm -rf "$(A2A_DATA_DIR)"; \
	(cd examples/agents/a2a-v1-dev && go run .) & A2A_PID=$$!; \
	echo "Waiting for A2A 1.0 dev agent."; \
	ready=0; \
	for _ in $$(seq 1 900); do \
		if curl -fsS "$(A2A_AGENT_CARD_URL)" >/dev/null 2>&1; then ready=1; break; fi; \
		if ! kill -0 "$$A2A_PID" 2>/dev/null; then \
			echo "A2A 1.0 dev agent exited before it became ready."; \
			wait "$$A2A_PID"; \
			exit 1; \
		fi; \
		sleep 2; \
	done; \
	if [[ "$$ready" != "1" ]]; then \
		echo "Timed out waiting for A2A 1.0 dev agent at $(A2A_AGENT_CARD_URL)."; \
		exit 1; \
	fi; \
	go run ./cmd/juted -config $(A2A_CONFIG) -data-dir $(A2A_DATA_DIR) & HUB_PID=$$!; \
	(cd $(WEB_DIR) && $(NPM) run dev) & WEB_PID=$$!; \
	wait "$$A2A_PID" "$$HUB_PID" "$$WEB_PID"

dev-a2a-mcp:
	@echo "A2A 1.0 Dev Agent Card: $(A2A_AGENT_CARD_URL)"
	@echo "Jute hub: $(HUB_URL)"
	@echo "Jute MCP: $(MCP_URL)"
	@echo "Jute web: $(WEB_URL)"
	@echo "Resetting dedicated A2A+MCP dev store: $(A2A_MCP_DATA_DIR)"
	@set -e; \
	cleanup() { \
		if [[ -n "$$WEB_PID" ]]; then kill "$$WEB_PID" 2>/dev/null || true; fi; \
		if [[ -n "$$HUB_PID" ]]; then kill "$$HUB_PID" 2>/dev/null || true; fi; \
		if [[ -n "$$A2A_PID" ]]; then kill "$$A2A_PID" 2>/dev/null || true; fi; \
	}; \
	trap cleanup INT TERM EXIT; \
	rm -rf "$(A2A_MCP_DATA_DIR)"; \
	(cd examples/agents/a2a-v1-dev && JUTE_MCP_URL="$(MCP_URL)" go run .) & A2A_PID=$$!; \
	echo "Waiting for A2A 1.0 dev agent."; \
	ready=0; \
	for _ in $$(seq 1 900); do \
		if curl -fsS "$(A2A_AGENT_CARD_URL)" >/dev/null 2>&1; then ready=1; break; fi; \
		if ! kill -0 "$$A2A_PID" 2>/dev/null; then \
			echo "A2A 1.0 dev agent exited before it became ready."; \
			wait "$$A2A_PID"; \
			exit 1; \
		fi; \
		sleep 2; \
	done; \
	if [[ "$$ready" != "1" ]]; then \
		echo "Timed out waiting for A2A 1.0 dev agent at $(A2A_AGENT_CARD_URL)."; \
		exit 1; \
	fi; \
	go run ./cmd/juted -config $(A2A_MCP_CONFIG) -data-dir $(A2A_MCP_DATA_DIR) & HUB_PID=$$!; \
	(cd $(WEB_DIR) && $(NPM) run dev) & WEB_PID=$$!; \
	wait "$$A2A_PID" "$$HUB_PID" "$$WEB_PID"

dev-a2a-reset:
	rm -rf $(A2A_DATA_DIR)
	rm -rf $(A2A_MCP_DATA_DIR)
