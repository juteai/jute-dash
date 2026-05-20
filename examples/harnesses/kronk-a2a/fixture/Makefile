SHELL := /bin/bash

JUTE_MCP_URL ?= http://127.0.0.1:8790/mcp

.PHONY: setup check server server-mcp console

setup:
	go mod download

check:
	go test ./...

server:
	KRONK_A2A_MODE=server go run .

server-mcp:
	JUTE_MCP_URL="$(JUTE_MCP_URL)" KRONK_A2A_MODE=server go run .

console:
	KRONK_A2A_MODE=console go run .
