SHELL := /bin/bash

ROOT_DIR := ../../..
WEB_DIR := $(ROOT_DIR)/apps/web
AGENT_DIR := $(ROOT_DIR)/examples/agents/mock-a2a-agent
CONFIG := config/jute.dev-mock-a2a-mcp.yaml
DATA_DIR := .jute/dev-mock-a2a-mcp
AGENT_CARD_URL ?= http://127.0.0.1:9797/.well-known/agent-card.json
HUB_URL ?= http://127.0.0.1:8787
MCP_URL ?= http://127.0.0.1:8790/mcp
WEB_URL ?= http://127.0.0.1:5173
NPM ?= npm

.PHONY: dev reset check

dev:
	@echo "Mock A2A Agent Card: $(AGENT_CARD_URL)"
	@echo "Jute hub: $(HUB_URL)"
	@echo "Jute MCP: $(MCP_URL)"
	@echo "Jute web: $(WEB_URL)"
	@echo "Resetting harness store: $(DATA_DIR)"
	@set -e; \
	cleanup() { \
		if [[ -n "$$WEB_PID" ]]; then kill "$$WEB_PID" 2>/dev/null || true; fi; \
		if [[ -n "$$HUB_PID" ]]; then kill "$$HUB_PID" 2>/dev/null || true; fi; \
		if [[ -n "$$AGENT_PID" ]]; then kill "$$AGENT_PID" 2>/dev/null || true; fi; \
	}; \
	trap cleanup INT TERM EXIT; \
	(cd $(ROOT_DIR) && rm -rf "$(DATA_DIR)"); \
	make -C $(AGENT_DIR) server-mcp JUTE_MCP_URL="$(MCP_URL)" & AGENT_PID=$$!; \
	echo "Waiting for Mock A2A agent."; \
	ready=0; \
	for _ in $$(seq 1 900); do \
		if curl -fsS "$(AGENT_CARD_URL)" >/dev/null 2>&1; then ready=1; break; fi; \
		if ! kill -0 "$$AGENT_PID" 2>/dev/null; then \
			echo "Mock A2A agent exited before it became ready."; \
			wait "$$AGENT_PID"; \
			exit 1; \
		fi; \
		sleep 2; \
	done; \
	if [[ "$$ready" != "1" ]]; then \
		echo "Timed out waiting for Mock A2A agent at $(AGENT_CARD_URL)."; \
		exit 1; \
	fi; \
	(cd $(ROOT_DIR) && go run ./cmd/juted -config $(CONFIG) -data-dir $(DATA_DIR)) & HUB_PID=$$!; \
	(cd $(WEB_DIR) && $(NPM) run dev) & WEB_PID=$$!; \
	wait "$$AGENT_PID" "$$HUB_PID" "$$WEB_PID"

reset:
	cd $(ROOT_DIR) && rm -rf "$(DATA_DIR)"

check:
	$(MAKE) -C $(AGENT_DIR) check
	$(MAKE) -C $(ROOT_DIR) check
