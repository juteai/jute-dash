SHELL := /bin/bash
JUTE_MCP_URL ?= http://127.0.0.1:8790/mcp
JUTE_WEB_URL ?= https://localhost:5173
JUTE_WEB_SCRIPT ?= dev:https
JUTE_DATA_DIR ?= ../../../.jute/local-dev
JUTE_CONFIG ?= ../../examples/config/local/config.yaml
JUTE_VOICE_ENV ?= ../../../.jute/local-voice-tools/local-voice.env
JUTE_VOICE_SMOKE_TEXT ?= hello from jute voice

.PHONY: setup install-local-voice run run-http run-mock run-kronk run-kronk-voice-smoke run-kronk-whisper run-kronk-local-voice run-ollama run-gemini clean

setup: install-local-voice

install-local-voice:
	@./install-local-voice.sh

# 1. Run Jute Hub + Svelte Web Dashboard standalone (without launching an agent)
run:
	@echo "Starting Jute Hub + HTTPS Web Dashboard (standalone)..."
	@echo "Open $(JUTE_WEB_URL)"
	@trap 'kill 0' SIGINT; \
	if [ -f "$(JUTE_VOICE_ENV)" ]; then source "$(JUTE_VOICE_ENV)"; fi; \
	(cd ../../../apps/hub && go run ./cmd/juted -config "$(JUTE_CONFIG)" -data-dir "$(JUTE_DATA_DIR)") & \
	(cd ../../../apps/web && npm run $(JUTE_WEB_SCRIPT)) & \
	wait

# 2. Run Jute Hub + Svelte Web Dashboard over plain HTTP for non-OAuth testing
run-http:
	@$(MAKE) run JUTE_WEB_URL=http://localhost:5173 JUTE_WEB_SCRIPT=dev

# 3. Run Jute Dashboard along with the Mock Agent
run-mock:
	@echo "Starting Jute Stack + Mock Agent..."
	@trap 'kill 0' SIGINT; \
	JUTE_MCP_URL="$(JUTE_MCP_URL)" JUTE_MCP_AGENT_ID="mock-agent" $(MAKE) -C ../../agents/mock-agent run & \
	$(MAKE) run & \
	wait

# 4. Run Jute Dashboard along with the Kronk Agent
run-kronk:
	@echo "Starting Jute Stack + Kronk Agent..."
	@trap 'kill 0' SIGINT; \
	JUTE_MCP_URL="$(JUTE_MCP_URL)" JUTE_MCP_AGENT_ID="kronk-agent" $(MAKE) -C ../../agents/kronk-agent run & \
	$(MAKE) run & \
	wait

# 5. Run Jute + Kronk with deterministic dev STT for voice routing smoke tests
run-kronk-voice-smoke:
	@tmp=$$(mktemp -t jute-voice-smoke-config.XXXXXX.yaml); \
	sed \
		-e 's/wake-word-model-id: hey_jarvis/wake-word-model-id: hey-jute/' \
		-e 's/wake-word-phrase: hey jarvis/wake-word-phrase: Hey Jute/' \
		-e 's/stt-provider-id: local-whisper-stt/stt-provider-id: local-dev-stt/' \
		-e 's/stt-model-id: tiny.en/stt-model-id: dev-transcript/' \
		-e 's/tts-provider-id: local-piper-tts/tts-provider-id: local-dev-tts/' \
		-e 's/tts-model-id: piper-default/tts-model-id: \"\"/' \
		-e 's/tts-voice-id: piper-default/tts-voice-id: amy/' \
		config.yaml > "$$tmp"; \
	trap 'rm -f "$$tmp"' EXIT; \
	JUTE_CONFIG="$$tmp" JUTE_DATA_DIR="../../../.jute/local-smoke-dev" JUTE_DEV_STT_TEXT="$(JUTE_VOICE_SMOKE_TEXT)" $(MAKE) run-kronk

# 6. Backwards-compatible alias for the default real local voice stack
run-kronk-whisper:
	@$(MAKE) run-kronk

# 7. Backwards-compatible alias for the default real local voice stack
run-kronk-local-voice:
	@$(MAKE) run-kronk

# 8. Run Jute Dashboard along with the Ollama Agent
run-ollama:
	@echo "Starting Jute Stack + Ollama Agent..."
	@trap 'kill 0' SIGINT; \
	JUTE_MCP_URL="$(JUTE_MCP_URL)" JUTE_MCP_AGENT_ID="ollama-agent" $(MAKE) -C ../../agents/ollama-agent run & \
	$(MAKE) run & \
	wait

# 9. Run Jute Dashboard along with the Gemini Agent
run-gemini:
	@echo "Starting Jute Stack + Gemini Agent..."
	@trap 'kill 0' SIGINT; \
	JUTE_MCP_URL="$(JUTE_MCP_URL)" JUTE_MCP_AGENT_ID="gemini-agent" $(MAKE) -C ../../agents/gemini-agent run & \
	$(MAKE) run & \
	wait

# 10. Reset local development databases and caches
clean:
	rm -rf ../../../.jute/local-dev ../../../.jute/local-whisper-dev ../../../.jute/local-voice-dev ../../../.jute/dev-mock-a2a ../../../.jute/dev-kronk-a2a
