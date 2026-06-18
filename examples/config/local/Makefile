SHELL := /bin/bash
JUTE_MCP_URL ?= http://127.0.0.1:8790/mcp
JUTE_WEB_URL ?= https://localhost:5173
JUTE_WEB_SCRIPT ?= dev:https

.PHONY: run run-http run-mock run-kronk run-ollama run-gemini clean

# 1. Run Jute Hub + Svelte Web Dashboard standalone (without launching an agent)
run:
	@echo "Starting Jute Hub + HTTPS Web Dashboard (standalone)..."
	@echo "Open $(JUTE_WEB_URL)"
	@trap 'kill 0' SIGINT; \
	(cd ../../../apps/hub && go run ./cmd/juted -config ../../examples/config/local/config.yaml) & \
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

# 5. Run Jute Dashboard along with the Ollama Agent
run-ollama:
	@echo "Starting Jute Stack + Ollama Agent..."
	@trap 'kill 0' SIGINT; \
	JUTE_MCP_URL="$(JUTE_MCP_URL)" JUTE_MCP_AGENT_ID="ollama-agent" $(MAKE) -C ../../agents/ollama-agent run & \
	$(MAKE) run & \
	wait

# 6. Run Jute Dashboard along with the Gemini Agent
run-gemini:
	@echo "Starting Jute Stack + Gemini Agent..."
	@trap 'kill 0' SIGINT; \
	JUTE_MCP_URL="$(JUTE_MCP_URL)" JUTE_MCP_AGENT_ID="gemini-agent" $(MAKE) -C ../../agents/gemini-agent run & \
	$(MAKE) run & \
	wait

# 7. Reset local development databases and caches
clean:
	rm -rf ../../../.jute/dev-mock-a2a ../../../.jute/dev-kronk-a2a
