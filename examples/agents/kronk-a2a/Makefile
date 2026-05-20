SHELL := /bin/bash

.PHONY: setup check run server

setup:
	go mod download

check:
	go test ./...

run:
	KRONK_A2A_MODE=console go run .

server:
	KRONK_A2A_MODE=server go run .
