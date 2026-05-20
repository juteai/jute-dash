SHELL := /bin/bash

JUTE_MCP_URL ?= http://127.0.0.1:8790/mcp

.PHONY: setup check server server-mcp

setup:
	go mod download

check:
	go test ./...

server:
	go run .

server-mcp:
	JUTE_MCP_URL="$(JUTE_MCP_URL)" go run .
