SHELL := /bin/bash

JUTE_MCP_URL ?= http://127.0.0.1:8790/mcp
JUTE_MCP_AGENT_ID ?=

# Local stability defaults for the Go 1.26 + macOS 26 (Tahoe) + Apple
# Silicon + kronk/yzma/llama.cpp stack. Each can be overridden from the
# environment when running a different host or once upstream fixes ship.
#
#   KRONK_LIB_VERSION pins llama.cpp to the build that the bundled
#     yzma version is wired against. Without this, kronk auto-upgrades
#     the on-disk library on every fresh checkout, which can drift away
#     from the FFI signatures in yzma and surface as a SIGABRT inside
#     the first tokenize call.
#
#   KRONK_PROCESSOR (when explicitly set) overrides the autodetected
#     hardware backend. Leave it unset to let the probe choose Metal
#     vs CPU automatically (see probe-processor below).
#
#   KRONK_A2A_GO_LDFLAGS forces the external linker, which makes the
#     resulting binary pick up the host macOS SDK version in
#     LC_BUILD_VERSION instead of the Go default of 12.0. rsc's
#     recommended Go 1.26 workaround in golang/go#77917; needed for
#     Metal to compile its newer shader kernels at all on this stack.
KRONK_LIB_VERSION ?= b9194
KRONK_A2A_GO_LDFLAGS ?= -linkmode=external

# Per-checkout cache for the Metal-vs-CPU probe result. Stored in this
# directory because the choice is host-specific (Metal works on Apple
# Silicon but not on Linux/CPU-only hosts) and not interesting to other
# contributors. Delete the file (or run `make reset-probe`) to re-probe.
KRONK_PROBE_CACHE ?= .kronk-processor.choice

KRONK_RUN_ENV := KRONK_LIB_VERSION="$(KRONK_LIB_VERSION)"
KRONK_GO_RUN := go run -ldflags='$(KRONK_A2A_GO_LDFLAGS)' .

.PHONY: setup check server server-mcp console probe-processor reset-probe

setup:
	go mod download

check:
	go test ./...

# probe-processor decides whether to use Metal or CPU and prints the
# chosen value (and nothing else) on stdout. Resolution order:
#   1. Honor an explicit KRONK_PROCESSOR override from the environment.
#   2. Use the cached choice in KRONK_PROBE_CACHE if present.
#   3. Run a one-shot end-to-end selftest with Metal. If it survives,
#      cache "metal"; otherwise cache "cpu". Any native abort() inside
#      libllama causes a nonzero exit, which the probe catches.
# Use `make reset-probe` to force a fresh probe.
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

server:
	@chosen=$$($(MAKE) -s probe-processor); \
	printf 'kronk: starting A2A server with processor=%s\n' "$$chosen" >&2; \
	$(KRONK_RUN_ENV) KRONK_PROCESSOR="$$chosen" KRONK_A2A_MODE=server $(KRONK_GO_RUN)

server-mcp:
	@chosen=$$($(MAKE) -s probe-processor); \
	printf 'kronk: starting A2A server (MCP-enabled) with processor=%s\n' "$$chosen" >&2; \
	$(KRONK_RUN_ENV) KRONK_PROCESSOR="$$chosen" JUTE_MCP_URL="$(JUTE_MCP_URL)" JUTE_MCP_AGENT_ID="$(JUTE_MCP_AGENT_ID)" KRONK_A2A_MODE=server $(KRONK_GO_RUN)

console:
	@chosen=$$($(MAKE) -s probe-processor); \
	printf 'kronk: starting console with processor=%s\n' "$$chosen" >&2; \
	$(KRONK_RUN_ENV) KRONK_PROCESSOR="$$chosen" KRONK_A2A_MODE=console $(KRONK_GO_RUN)
