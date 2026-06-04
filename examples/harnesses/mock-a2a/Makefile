SHELL := /bin/bash

HARNESS_DIR := $(CURDIR)
ROOT_DIR := $(abspath $(HARNESS_DIR)/../../..)
WEB_DIR := $(ROOT_DIR)/apps/web
FIXTURE_DIR := $(HARNESS_DIR)/fixture
CONFIG := $(HARNESS_DIR)/config.yaml
DATA_DIR := $(ROOT_DIR)/.jute/dev-mock-a2a
RUNNER := $(ROOT_DIR)/examples/harnesses/run-stack.sh
AGENT_CARD_URL ?= http://127.0.0.1:9797/.well-known/agent-card.json
AGENT_RPC_URL ?= http://127.0.0.1:9797/invoke
HUB_URL ?= http://127.0.0.1:8787
MCP_URL ?= http://127.0.0.1:8790/mcp
JUTE_MCP_AGENT_ID ?= mock-a2a-agent
WEB_URL ?= http://127.0.0.1:5173
NPM ?= npm

.DEFAULT_GOAL := run
.PHONY: run

run:
	@STACK_NAME="Mock A2A + MCP full-stack harness" \
	ROOT_DIR="$(ROOT_DIR)" WEB_DIR="$(WEB_DIR)" FIXTURE_DIR="$(FIXTURE_DIR)" \
	CONFIG="$(CONFIG)" DATA_DIR="$(DATA_DIR)" AGENT_TARGET="server-mcp" MCP_ENABLED="true" \
	JUTE_MCP_AGENT_ID="$(JUTE_MCP_AGENT_ID)" \
	AGENT_CARD_URL="$(AGENT_CARD_URL)" AGENT_RPC_URL="$(AGENT_RPC_URL)" \
	HUB_URL="$(HUB_URL)" MCP_URL="$(MCP_URL)" WEB_URL="$(WEB_URL)" NPM="$(NPM)" \
	$(RUNNER)
