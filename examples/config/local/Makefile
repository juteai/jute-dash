SHELL := /bin/bash
JUTE_MCP_URL ?= http://127.0.0.1:8790/mcp
JUTE_WEB_URL ?= https://localhost:5173
JUTE_WEB_SCRIPT ?= dev:https
JUTE_DATA_DIR ?= ../../../.jute/local-dev
JUTE_CONFIG ?= ../../examples/config/local/config.yaml
JUTE_VOICE_SMOKE_TEXT ?= hello from jute voice
JUTE_WHISPER_MODEL ?= tiny.en
JUTE_WAKE_MODEL ?= openwakeword-hey-jute

.PHONY: run run-http run-mock run-kronk run-kronk-voice-smoke run-kronk-whisper run-kronk-local-voice run-ollama run-gemini clean

# 1. Run Jute Hub + Svelte Web Dashboard standalone (without launching an agent)
run:
	@echo "Starting Jute Hub + HTTPS Web Dashboard (standalone)..."
	@echo "Open $(JUTE_WEB_URL)"
	@trap 'kill 0' SIGINT; \
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
	@JUTE_DEV_STT_TEXT="$(JUTE_VOICE_SMOKE_TEXT)" $(MAKE) run-kronk

# 6. Run Jute + Kronk with go-whisper command STT selected
run-kronk-whisper:
	@tmp=$$(mktemp -t jute-whisper-config.XXXXXX.yaml); \
	sed \
		-e 's/stt-provider-id: local-dev-stt/stt-provider-id: local-whisper-stt/' \
		-e 's/stt-model-id: dev-transcript/stt-model-id: $(JUTE_WHISPER_MODEL)/' \
		config.yaml > "$$tmp"; \
	trap 'rm -f "$$tmp"' EXIT; \
	JUTE_CONFIG="$$tmp" JUTE_DATA_DIR="../../../.jute/local-whisper-dev" $(MAKE) run-kronk

# 7. Run Jute + Kronk with openWakeWord, go-whisper, and Piper selected
run-kronk-local-voice:
	@tmp=$$(mktemp -t jute-local-voice-config.XXXXXX.yaml); \
	sed \
		-e 's/wake-word-model-id: hey-jute/wake-word-model-id: $(JUTE_WAKE_MODEL)/' \
		-e 's/stt-provider-id: local-dev-stt/stt-provider-id: local-whisper-stt/' \
		-e 's/stt-model-id: dev-transcript/stt-model-id: $(JUTE_WHISPER_MODEL)/' \
		-e 's/tts-provider-id: local-dev-tts/tts-provider-id: local-piper-tts/' \
		-e 's/tts-voice-id: amy/tts-voice-id: piper-default/' \
		config.yaml > "$$tmp"; \
	trap 'rm -f "$$tmp"' EXIT; \
	JUTE_CONFIG="$$tmp" JUTE_DATA_DIR="../../../.jute/local-voice-dev" $(MAKE) run-kronk

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
