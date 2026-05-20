SHELL := /bin/bash

ROOT_DIR := ../../..
WEB_DIR := $(ROOT_DIR)/apps/web
AGENT_DIR := $(ROOT_DIR)/examples/agents/kronk-a2a
CONFIG := config/jute.dev-kronk-a2a.yaml
MCP_CONFIG := config/jute.dev-kronk-a2a-mcp.yaml
DATA_DIR := .jute/dev-kronk-a2a
MCP_DATA_DIR := .jute/dev-kronk-a2a-mcp
AGENT_CARD_URL ?= http://127.0.0.1:9797/.well-known/agent-card.json
HUB_URL ?= http://127.0.0.1:8787
MCP_URL ?= http://127.0.0.1:8790/mcp
WEB_URL ?= http://127.0.0.1:5173
NPM ?= npm

.PHONY: dev dev-mcp run-stack reset check

dev:
	@$(MAKE) run-stack STACK_CONFIG="$(CONFIG)" STACK_DATA_DIR="$(DATA_DIR)" STACK_AGENT_TARGET="server"

dev-mcp:
	@$(MAKE) run-stack STACK_CONFIG="$(MCP_CONFIG)" STACK_DATA_DIR="$(MCP_DATA_DIR)" STACK_AGENT_TARGET="server-mcp"

run-stack:
	@echo "Kronk A2A Agent Card: $(AGENT_CARD_URL)"
	@echo "Jute hub: $(HUB_URL)"
	@if [[ "$(STACK_AGENT_TARGET)" == "server-mcp" ]]; then echo "Jute MCP: $(MCP_URL)"; fi
	@echo "Jute web: $(WEB_URL)"
	@echo "Resetting harness store: $(STACK_DATA_DIR)"
	@echo "First Kronk run may download runtime and model assets."
	@set -e; \
	cleanup() { \
		if [[ -n "$$WEB_PID" ]]; then kill "$$WEB_PID" 2>/dev/null || true; fi; \
		if [[ -n "$$HUB_PID" ]]; then kill "$$HUB_PID" 2>/dev/null || true; fi; \
		if [[ -n "$$AGENT_PID" ]]; then kill "$$AGENT_PID" 2>/dev/null || true; fi; \
	}; \
	trap cleanup INT TERM EXIT; \
	(cd $(ROOT_DIR) && rm -rf "$(STACK_DATA_DIR)"); \
	make -C $(AGENT_DIR) $(STACK_AGENT_TARGET) JUTE_MCP_URL="$(MCP_URL)" & AGENT_PID=$$!; \
	echo "Waiting for Kronk A2A agent."; \
	ready=0; \
	for _ in $$(seq 1 900); do \
		if curl -fsS "$(AGENT_CARD_URL)" >/dev/null 2>&1; then ready=1; break; fi; \
		if ! kill -0 "$$AGENT_PID" 2>/dev/null; then \
			echo "Kronk A2A agent exited before it became ready."; \
			wait "$$AGENT_PID"; \
			exit 1; \
		fi; \
		sleep 2; \
	done; \
	if [[ "$$ready" != "1" ]]; then \
		echo "Timed out waiting for Kronk A2A agent at $(AGENT_CARD_URL)."; \
		exit 1; \
	fi; \
	(cd $(ROOT_DIR) && go run ./cmd/juted -config $(STACK_CONFIG) -data-dir $(STACK_DATA_DIR)) & HUB_PID=$$!; \
	(cd $(WEB_DIR) && $(NPM) run dev) & WEB_PID=$$!; \
	wait "$$AGENT_PID" "$$HUB_PID" "$$WEB_PID"

reset:
	cd $(ROOT_DIR) && rm -rf "$(DATA_DIR)" "$(MCP_DATA_DIR)"

check:
	$(MAKE) -C $(AGENT_DIR) check
	$(MAKE) -C $(ROOT_DIR) check
