SHELL := /bin/bash

JUTE_MCP_URL ?= http://127.0.0.1:8790/mcp
JUTE_MCP_AGENT_ID ?=

KRONK_LIB_VERSION ?= b9194
KRONK_A2A_GO_LDFLAGS ?= -linkmode=external
KRONK_PROBE_CACHE ?= .kronk-processor.choice

KRONK_RUN_ENV := KRONK_LIB_VERSION="$(KRONK_LIB_VERSION)"
KRONK_GO_RUN := go run -ldflags='$(KRONK_A2A_GO_LDFLAGS)' .

.PHONY: run check probe-processor reset-probe

probe-processor:
	@set -e; \
	if [[ -n "$$KRONK_PROCESSOR" ]]; then \
		printf '%s\n' "$$KRONK_PROCESSOR"; \
		exit 0; \
	fi; \
	if [[ -f "$(KRONK_PROBE_CACHE)" ]]; then \
		cat "$(KRONK_PROBE_CACHE)"; \
		exit 0; \
	fi; \
	log=$$(mktemp -t kronk-probe.XXXXXX); \
	printf 'kronk: probing Metal (one short inference; ~30-60s on cold cache)\n' >&2; \
	if KRONK_LIB_VERSION="$(KRONK_LIB_VERSION)" KRONK_PROCESSOR=metal KRONK_A2A_MODE=selftest $(KRONK_GO_RUN) >"$$log" 2>&1; then \
		printf 'kronk: Metal probe succeeded; caching choice in %s\n' "$(KRONK_PROBE_CACHE)" >&2; \
		echo metal | tee "$(KRONK_PROBE_CACHE)"; \
		rm -f "$$log"; \
	else \
		printf 'kronk: Metal probe failed, falling back to CPU. See %s for the full log.\n' "$$log" >&2; \
		echo cpu | tee "$(KRONK_PROBE_CACHE)"; \
	fi

reset-probe:
	@rm -f "$(KRONK_PROBE_CACHE)"
	@printf 'kronk: cleared processor probe cache; next server/run will re-probe.\n'

run:
	@chosen=$$($(MAKE) -s probe-processor); \
	while :; do \
		printf 'kronk: starting A2A server with processor=%s\n' "$$chosen" >&2; \
		$(KRONK_RUN_ENV) KRONK_PROCESSOR="$$chosen" JUTE_MCP_URL="$(JUTE_MCP_URL)" JUTE_MCP_AGENT_ID="$(JUTE_MCP_AGENT_ID)" KRONK_A2A_MODE=server $(KRONK_GO_RUN); \
		status=$$?; \
		if [[ "$$status" -eq 0 || "$$status" -eq 130 || "$$status" -eq 143 ]]; then \
			exit "$$status"; \
		fi; \
		if [[ "$$chosen" != "metal" ]]; then \
			exit "$$status"; \
		fi; \
		printf 'kronk: Metal server failed; falling back to CPU and caching choice in %s\n' "$(KRONK_PROBE_CACHE)" >&2; \
		echo cpu >"$(KRONK_PROBE_CACHE)"; \
		chosen=cpu; \
	done

check:
	go test ./...
