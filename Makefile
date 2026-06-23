SHELL := /bin/bash

WEB_DIR := apps/web
NPM ?= npm

.PHONY: setup setup-local-examples install-local-voice voice-check run run-http run-mock run-kronk run-kronk-whisper run-kronk-local-voice run-ollama run-gemini pre-commit-install lint test test-coverage codegen generate-mocks integration-test-local web-lint web-format-check web-check web-test web-test-coverage web-build check reset

setup:
	@echo "Checking for Homebrew dependencies..."
	@if command -v brew >/dev/null 2>&1; then \
		echo "Running brew bundle install..."; \
		brew bundle install; \
	else \
		echo "Homebrew not found. Skipping system package installation."; \
	fi
	@echo "Setting up web app dependencies..."
	cd $(WEB_DIR) && $(NPM) install
	@echo "Setting up pre-commit hooks..."
	$(MAKE) pre-commit-install
	$(MAKE) setup-local-examples

setup-local-examples:
	@echo "Setting up local example harness..."
	$(MAKE) -C examples/config/local setup

install-local-voice voice-check:
	$(MAKE) -C examples/config/local $@

run run-http run-mock run-kronk run-kronk-whisper run-kronk-local-voice run-ollama run-gemini:
	$(MAKE) -C examples/config/local $@

pre-commit-install:
	@if command -v pre-commit >/dev/null 2>&1; then \
		echo "Installing pre-commit hooks..."; \
		pre-commit install --install-hooks -t pre-commit -t commit-msg; \
	else \
		echo "pre-commit command not found. Attempting brew install..."; \
		if command -v brew >/dev/null 2>&1; then \
			brew install pre-commit; \
			pre-commit install --install-hooks -t pre-commit -t commit-msg; \
		else \
			echo "Failed to install pre-commit automatically. Please install pre-commit manually (https://pre-commit.com)."; \
		fi \
	fi

lint:
	golangci-lint run --timeout=5m

test:
	go test $$(go list ./... | grep -v '/apps/hub/tests/integration')

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

codegen:
	go tool oapi-codegen --config=apps/hub/api/hub/v1/oapi-codegen.yaml apps/hub/api/hub/v1/openapi.yaml

generate-mocks:
	go tool mockery --config=.mockery.yaml
	go tool mockery --config=/dev/null --name=Syncer --srcpkg=jute-dash/apps/hub/internal/app/service --output=apps/hub/internal/app/service --filename=agent_syncer_mock_test.go --structname=AgentSyncer --with-expecter --inpackage --testonly

integration-test-local:
	JUTE_HUB_INTEGRATION=1 go tool ginkgo --label-filter=SMOKE ./apps/hub/tests/integration/specs

web-lint:
	cd $(WEB_DIR) && $(NPM) run lint

web-format-check:
	cd $(WEB_DIR) && $(NPM) run format:check

web-check:
	cd $(WEB_DIR) && $(NPM) run check

web-test:
	cd $(WEB_DIR) && $(NPM) run test

web-test-coverage:
	cd $(WEB_DIR) && $(NPM) run test:coverage

web-build:
	cd $(WEB_DIR) && $(NPM) run build

check: codegen generate-mocks lint test web-lint web-format-check web-check web-test web-build

reset:
	@echo "Resetting development store directories..."
	rm -rf .jute/dev-mock-a2a .jute/dev-kronk-a2a .jute/dev-kronk-a2a-mcp
