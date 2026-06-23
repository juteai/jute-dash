SHELL := /bin/bash
ROOT_DIR := $(abspath $(CURDIR)/../../..)
JUTE_MCP_URL ?= http://127.0.0.1:8790/mcp
JUTE_WEB_URL ?= https://localhost:5173
JUTE_WEB_SCRIPT ?= dev:https
JUTE_DATA_DIR ?= $(ROOT_DIR)/.jute/local-dev
JUTE_CONFIG ?= $(ROOT_DIR)/examples/config/local/config.yaml
JUTE_VOICE_ENV ?= $(ROOT_DIR)/.jute/local-voice-tools/local-voice.env

.PHONY: setup ensure-local-voice install-local-voice voice-check run run-http run-mock run-kronk run-kronk-whisper run-kronk-local-voice run-ollama run-gemini clean

setup: install-local-voice voice-check

ensure-local-voice:
	@if [ -f "$(JUTE_VOICE_ENV)" ]; then source "$(JUTE_VOICE_ENV)"; fi; \
	if ./install-local-voice.sh --check >/dev/null 2>&1; then \
		./install-local-voice.sh --check; \
	else \
		./install-local-voice.sh; \
	fi

install-local-voice:
	@./install-local-voice.sh

voice-check:
	@if [ -f "$(JUTE_VOICE_ENV)" ]; then source "$(JUTE_VOICE_ENV)"; fi; \
	./install-local-voice.sh --check

# 1. Run Jute Hub + Svelte Web Dashboard standalone (without launching an agent)
run: ensure-local-voice
	@echo "Starting Jute Hub + HTTPS Web Dashboard (standalone)..."
	@echo "Open $(JUTE_WEB_URL)"
	@trap 'kill 0' SIGINT; \
	if [ -f "$(JUTE_VOICE_ENV)" ]; then source "$(JUTE_VOICE_ENV)"; fi; \
	(cd "$(ROOT_DIR)/apps/hub" && go run ./cmd/juted -config "$(JUTE_CONFIG)" -data-dir "$(JUTE_DATA_DIR)") & \
	(cd "$(ROOT_DIR)/apps/web" && npm run $(JUTE_WEB_SCRIPT)) & \
	wait

# 2. Run Jute Hub + Svelte Web Dashboard over plain HTTP for non-OAuth testing
run-http:
	@$(MAKE) run JUTE_WEB_URL=http://localhost:5173 JUTE_WEB_SCRIPT=dev

# 3. Run Jute Dashboard along with the Mock Agent
run-mock:
	@echo "Starting Jute Stack + Mock Agent..."
	@trap 'kill 0' SIGINT; \
	JUTE_MCP_URL="$(JUTE_MCP_URL)" JUTE_MCP_AGENT_ID="mock-agent" $(MAKE) -C "$(ROOT_DIR)/examples/agents/mock-agent" run & \
	$(MAKE) run & \
	wait

# 4. Run Jute Dashboard along with the Kronk Agent
run-kronk:
	@echo "Starting Jute Stack + Kronk Agent..."
	@trap 'kill 0' SIGINT; \
	JUTE_MCP_URL="$(JUTE_MCP_URL)" JUTE_MCP_AGENT_ID="kronk-agent" $(MAKE) -C "$(ROOT_DIR)/examples/agents/kronk-agent" run & \
	$(MAKE) run & \
	wait

# 5. Backwards-compatible alias for the default real local voice stack
run-kronk-whisper:
	@$(MAKE) run-kronk

# 6. Backwards-compatible alias for the default real local voice stack
run-kronk-local-voice:
	@$(MAKE) run-kronk

# 7. Run Jute Dashboard along with the Ollama Agent
run-ollama:
	@echo "Starting Jute Stack + Ollama Agent..."
	@trap 'kill 0' SIGINT; \
	JUTE_MCP_URL="$(JUTE_MCP_URL)" JUTE_MCP_AGENT_ID="ollama-agent" $(MAKE) -C "$(ROOT_DIR)/examples/agents/ollama-agent" run & \
	$(MAKE) run & \
	wait

# 8. Run Jute Dashboard along with the Gemini Agent
run-gemini:
	@echo "Starting Jute Stack + Gemini Agent..."
	@trap 'kill 0' SIGINT; \
	JUTE_MCP_URL="$(JUTE_MCP_URL)" JUTE_MCP_AGENT_ID="gemini-agent" $(MAKE) -C "$(ROOT_DIR)/examples/agents/gemini-agent" run & \
	$(MAKE) run & \
	wait

# 9. Reset local development databases and caches
clean:
	rm -rf "$(ROOT_DIR)/.jute/local-dev" "$(ROOT_DIR)/.jute/local-whisper-dev" "$(ROOT_DIR)/.jute/local-voice-dev" "$(ROOT_DIR)/.jute/dev-mock-a2a" "$(ROOT_DIR)/.jute/dev-kronk-a2a"
